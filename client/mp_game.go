package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/alwedo/tetris/pb"
	"github.com/alwedo/tetris/tetris"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const youQuit = "You quit! 🐔"

// TODO: refactor this
var ErrYouLose error = errors.New("You Lose!")                           // nolint: revive, staticcheck
var ErrYouWon error = errors.New("You Won!")                             // nolint: revive, staticcheck
var ErrSadAndAlone error = errors.New("There is no one to play with :(") // nolint: revive, staticcheck

type MPPlayingModel struct {
	playerName  string
	localGame   *tetris.Game
	localState  tetris.GameMessage
	remoteState *pb.GameMessage
	conn        *grpc.ClientConn
	stream      grpc.BidiStreamingClient[pb.GameMessage, pb.GameMessage]
	ctx         context.Context
	cancel      context.CancelFunc
	keys        gameKeyMap
	help        help.Model
}

func NewMPPlayingModel(
	playerName string,
	conn *grpc.ClientConn,
	stream grpc.BidiStreamingClient[pb.GameMessage, pb.GameMessage],
	initialOpponentState *pb.GameMessage,
) *MPPlayingModel {
	return &MPPlayingModel{
		playerName:  playerName,
		conn:        conn,
		stream:      stream,
		remoteState: initialOpponentState,
		keys:        gameKeys,
		help:        help.New(),
	}
}

func (m *MPPlayingModel) Init() tea.Cmd {
	ctx, cancel := context.WithCancel(context.Background())
	m.ctx = ctx
	m.cancel = cancel

	m.localGame = tetris.Start(ctx)

	return tea.Batch(
		m.listenToGameUpdates(),
		m.listenToStreamUpdates(),
	)
}

func (m *MPPlayingModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tetris.GameMessage:
		m.localState = msg

		// Send to opponent
		sendMsg := tetris2Proto(&msg)
		sendMsg.SetName(m.playerName)
		if err := m.stream.Send(sendMsg); err != nil {
			transition := TransitionToLobbyMsg{
				LocalGameState:  m.localState,
				RemoteGameState: m.remoteState,
			}
			if errors.Is(err, io.EOF) {
				transition.Message = ErrYouWon.Error()
			} else {
				transition.Message = "error in stream send():\n" + err.Error()
			}
			m.cleanup()
			return m, func() tea.Msg {
				return transition
			}
		}

		if len(msg.ClearedLines) > 0 {
			// TODO: fix animation to be overlay mask instead of object manipulation
			complete := make(map[int][]tetris.Shape)
			for _, v := range msg.ClearedLines {
				complete[v] = msg.Tetris.Stack[v]
			}
			return m, newAnimationMsg(complete)
		}

		return m, m.listenToGameUpdates()

	case AnimationMessage:
		if msg.frames == 0 {
			return m, m.listenToGameUpdates()
		}
		if m.localState.Tetris.Tetromino != nil {
			m.localState.Tetris.Tetromino = nil
		}
		for k, v := range msg.completedRows {
			if msg.frames%2 == 0 {
				m.localState.Tetris.Stack[k] = make([]tetris.Shape, 10)
			} else {
				m.localState.Tetris.Stack[k] = v
			}
		}
		msg.frames--
		return m, tea.Tick(40*time.Millisecond, func(time.Time) tea.Msg {
			return msg
		})

	case *pb.GameMessage:
		// TODO: add animation
		m.localGame.Do(tetris.AddRemoteLines(int(msg.GetLinesClear())))
		m.remoteState = msg
		return m, m.listenToStreamUpdates()

	case streamErrorMsg:
		m.cleanup()
		return m, func() tea.Msg {
			return TransitionToLobbyMsg{
				LocalGameState:  m.localState,
				RemoteGameState: m.remoteState,
				Message:         msg.err.Error(),
			}
		}

	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			m.cleanup()
			return m, func() tea.Msg {
				return TransitionToLobbyMsg{
					LocalGameState:  m.localState,
					RemoteGameState: m.remoteState,
					Message:         youQuit,
				}
			}

		case key.Matches(msg, m.keys.MoveLeft):
			m.localGame.Do(tetris.MoveLeft())

		case key.Matches(msg, m.keys.MoveRight):
			m.localGame.Do(tetris.MoveRight())

		case key.Matches(msg, m.keys.MoveDown):
			m.localGame.Do(tetris.MoveDown())

		case key.Matches(msg, m.keys.DropDown):
			m.localGame.Do(tetris.DropDown())

		case key.Matches(msg, m.keys.RotateLeft):
			m.localGame.Do(tetris.RotateLeft())

		case key.Matches(msg, m.keys.RotateRight):
			m.localGame.Do(tetris.RotateRight())

		case key.Matches(msg, m.keys.Help):
			m.help.ShowAll = !m.help.ShowAll
		}
	}

	return m, nil
}

func (m *MPPlayingModel) View() tea.View {
	leftPanel := renderStack(m.localState.Tetris)
	rightPanel := renderRemoteStack(m.remoteState)
	centerPanel := m.renderCenterPanel()

	composed := lipgloss.JoinHorizontal(lipgloss.Bottom,
		leftPanel,
		centerPanel,
		rightPanel,
	)

	return tea.NewView(composed)
}

func (m *MPPlayingModel) renderCenterPanel() string {
	gameName := lipgloss.NewStyle().Bold(true).Render(appName)
	nextPiece := renderNextPiece(m.localState.Tetris)

	opponentName := "Opponent"
	if m.remoteState != nil && m.remoteState.GetName() != "" {
		opponentName = m.remoteState.GetName()
	}

	opponentLines := int32(0)
	if m.remoteState != nil {
		opponentLines = m.remoteState.GetLinesClear()
	}

	stats := lipgloss.NewStyle().Width(22).Align(lipgloss.Center).
		Border(lipgloss.RoundedBorder()).
		Render(lipgloss.JoinVertical(lipgloss.Center,
			gameName,
			fmt.Sprintf("You: %d lines", m.localState.Tetris.Lines),
			fmt.Sprintf("%s: %d lines", opponentName, opponentLines),
			nextPiece,
		))

	help := lipgloss.NewStyle().Width(22).Align(lipgloss.Center).
		Border(lipgloss.RoundedBorder()).
		Foreground(lipgloss.Color("#FF75B7")).
		Render(m.help.View(m.keys))

	return lipgloss.JoinVertical(lipgloss.Center, stats, help)
}

func (m *MPPlayingModel) cleanup() {
	if m.cancel != nil {
		m.cancel()
	}
	if m.stream != nil {
		m.stream.CloseSend() // nolint: errcheck
	}
	if m.conn != nil {
		m.conn.Close()
	}
}

func (m *MPPlayingModel) listenToGameUpdates() tea.Cmd {
	return func() tea.Msg {
		select {
		case msg, ok := <-m.localGame.GameMessageCh:
			if !ok {
				// Channel closed = game over (you lost)
				return streamErrorMsg{err: ErrYouLose}
			}
			return msg
		case <-m.ctx.Done():
			return nil
		}
	}
}

func (m *MPPlayingModel) listenToStreamUpdates() tea.Cmd {
	return func() tea.Msg {
		msg, err := m.stream.Recv()
		if err != nil {
			if err == io.EOF {
				// you won
				return ErrYouWon
				// return streamErrorMsg{err: fmt.Errorf("listening stream: %w", err)}
			}
			st, ok := status.FromError(err)
			if ok && st.Code() == codes.Canceled { //nolint: gocritic
				// you won
				return nil
			} else if ok && st.Code() == codes.DeadlineExceeded {
				// opponent didnt show up
				return streamErrorMsg{err: ErrSadAndAlone}
			}

			return streamErrorMsg{err: fmt.Errorf("listening stream: %w", err)}
		}
		return msg
	}
}

// tetris2Proto converts local game state to protobuf for sending to opponent
func tetris2Proto(t *tetris.GameMessage) *pb.GameMessage {
	rendered := pb.Stack_builder{Rows: make([]*pb.Row, 20)}.Build()

	for i := range rendered.GetRows() {
		rendered.GetRows()[i] = pb.Row_builder{
			Cells: make([]string, 10),
		}.Build()
	}

	for iy, y := range t.Tetris.Stack {
		for ix, x := range y {
			if x != tetris.Shape("") {
				rendered.GetRows()[iy].GetCells()[ix] = string(x)
			}
		}
	}

	// Render current tetromino if it exists
	if t.Tetris.Tetromino != nil {
		for iy, y := range t.Tetris.Tetromino.Grid {
			for ix, x := range y {
				if x {
					rendered.GetRows()[t.Tetris.Tetromino.Y-iy].GetCells()[t.Tetris.Tetromino.X+ix] = string(t.Tetris.Tetromino.Shape)
				}
			}
		}
	}

	return pb.GameMessage_builder{
		Name:       new("Player"),
		LinesClear: new(int32(t.Tetris.Lines)), // nolint: gosec
		Stack:      rendered,
	}.Build()
}
