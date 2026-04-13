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
	"github.com/alwedo/tetris"
	"github.com/alwedo/tetris/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	messageYouQuit = "You quit! 🐔"
	messageYouLost = "You lost!"
	messageYouWon  = "You won!"
)

type gameOverMessage struct {
	msg string
}

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
	parentCtx context.Context,
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
		ctx:         parentCtx,
	}
}

func (m *MPPlayingModel) Init() tea.Cmd {
	ctx, cancel := context.WithCancel(m.ctx) //nolint: gosec
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
		if err := m.stream.Send(tetris2Proto(&msg, m.playerName)); err != nil {
			var message string
			if errors.Is(err, io.EOF) {
				message = messageYouWon
			} else {
				message = "error in stream send():\n" + err.Error()
			}
			return m, m.toLobby(message)
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

	case gameOverMessage:
		return m, m.toLobby(msg.msg)

	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, m.toLobby(messageYouQuit)
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
		}
	}

	return m, nil
}

func (m *MPPlayingModel) View() tea.View {
	center := lipgloss.JoinHorizontal(
		lipgloss.Top,
		renderStack(m.localState.Tetris),
		renderCenterPanel(m.localState.Tetris, m.playerName, m.remoteState),
		renderRemoteStack(m.remoteState),
	)

	cw, ch := lipgloss.Size(center)
	help := helpStyle.Width(cw).Render(m.help.View(m.keys))

	return tea.NewView(lipgloss.NewCompositor(
		lipgloss.NewLayer(center),
		lipgloss.NewLayer(help).Y(ch),
	).Render())
}

func (m *MPPlayingModel) toLobby(msg string) tea.Cmd {
	return func() tea.Msg {
		if m.cancel != nil {
			m.cancel()
		}
		if m.stream != nil {
			m.stream.CloseSend() // nolint: errcheck
		}
		if m.conn != nil {
			m.conn.Close()
		}
		return TransitionToLobbyMsg{
			Message:         msg,
			LocalGameState:  m.localState,
			RemoteGameState: m.remoteState,
		}
	}
}

func (m *MPPlayingModel) listenToGameUpdates() tea.Cmd {
	return func() tea.Msg {
		select {
		case msg, ok := <-m.localGame.GameMessageCh:
			if !ok {
				return gameOverMessage{msg: messageYouLost}
			}
			return msg
		case <-m.ctx.Done():
			return gameOverMessage{msg: "cancelled"}
		}
	}
}

func (m *MPPlayingModel) listenToStreamUpdates() tea.Cmd {
	return func() tea.Msg {
		msg, err := m.stream.Recv()
		if err != nil {
			message := fmt.Sprintf("listening stream: %v", err)
			if err == io.EOF {
				return gameOverMessage{msg: messageYouWon}
			}
			st, ok := status.FromError(err)
			if ok && st.Code() == codes.Canceled { //nolint: gocritic
				message = messageYouWon
			}
			return gameOverMessage{msg: message}
		}
		return msg
	}
}

// tetris2Proto converts local game state to protobuf for sending to opponent
func tetris2Proto(t *tetris.GameMessage, name string) *pb.GameMessage {
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
		Name:       new(name),
		LinesClear: new(int32(t.Tetris.Lines)), // nolint: gosec
		Stack:      rendered,
	}.Build()
}
