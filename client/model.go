package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/alwedo/tetris"
	"github.com/alwedo/tetris/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

const (
	messageYouQuit  = "You quit! 🐔"
	messageYouLost  = "You lost!"
	messageYouWon   = "You won!"
	messageGameOver = "Game Over"
)

type GameState int

const (
	StateMenu GameState = iota
	StateConnecting
	StateWaitingForOpponent
	StateSinglePlayer
	StateMultiplayer
)

type Model struct {
	// Current state
	state GameState

	// Player info
	playerName string

	// Menu state
	menuCursor   int
	gameModes    []string
	notification string

	// Connection state
	spinner spinner.Model
	conn    *grpc.ClientConn
	stream  grpc.BidiStreamingClient[pb.GameMessage, pb.GameMessage]

	// Game state (shared by both SP and MP)
	game             *tetris.Game
	localState       tetris.GameMessage
	remoteState      *pb.GameMessage
	localAnimating   bool
	localAnimFrame   int
	localAnimLayout  []int
	remoteAnimating  bool
	remoteAnimFrame  int
	remoteAnimLayout []int32

	// Context management
	parentCtx context.Context
	ctx       context.Context
	cancel    context.CancelFunc

	// UI
	gameKeys  gameKeyMap
	lobbyKeys lobbyKeyMap
	help      help.Model
}

func NewModel(ctx context.Context, playerName string) *Model {
	return &Model{
		state:      StateMenu,
		playerName: playerName,
		menuCursor: 0,
		gameModes:  []string{"Single Player", "Multiplayer"},
		parentCtx:  ctx,
		gameKeys:   gameKeys,
		lobbyKeys:  lobbyKeys,
		help:       help.New(),
		spinner: spinner.New(
			spinner.WithSpinner(spinner.Points),
			spinner.WithStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("205"))),
		),
	}
}

func (m *Model) Init() tea.Cmd {
	return nil
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Global ctrl+c handler
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		if keyMsg.String() == "ctrl+c" {
			m.cleanup()
			return m, tea.Quit
		}
	}

	switch m.state {
	case StateMenu:
		return m.updateMenu(msg)
	case StateConnecting:
		return m.updateConnecting(msg)
	case StateWaitingForOpponent:
		return m.updateWaiting(msg)
	case StateSinglePlayer:
		return m.updateSinglePlayer(msg)
	case StateMultiplayer:
		return m.updateMultiplayer(msg)
	}

	return m, nil
}

// ========== Menu State ==========

func (m *Model) updateMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		// Clear notification on any key press
		if m.notification != "" {
			m.notification = ""
			return m, nil
		}

		switch {
		case key.Matches(msg, m.lobbyKeys.Up):
			if m.menuCursor > 0 {
				m.menuCursor--
			}
		case key.Matches(msg, m.lobbyKeys.Down):
			if m.menuCursor < len(m.gameModes)-1 {
				m.menuCursor++
			}
		case key.Matches(msg, m.lobbyKeys.Select):
			if m.menuCursor == 0 {
				// Start single player
				return m.startSinglePlayer()
			}
			// Start multiplayer connection
			return m.startConnecting()
		case key.Matches(msg, m.lobbyKeys.Quit):
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m *Model) startSinglePlayer() (tea.Model, tea.Cmd) {
	m.state = StateSinglePlayer
	ctx, cancel := context.WithCancel(m.parentCtx)
	m.ctx = ctx
	m.cancel = cancel
	m.game = tetris.Start(ctx)
	m.localState = tetris.GameMessage{}
	m.remoteState = nil

	return m, m.listenToGameUpdates()
}

func (m *Model) startConnecting() (tea.Model, tea.Cmd) {
	m.state = StateConnecting
	ctx, cancel := context.WithCancel(m.parentCtx)
	m.ctx = ctx
	m.cancel = cancel

	return m, tea.Batch(
		m.spinner.Tick,
		m.connectToServer(),
	)
}

// ========== Connecting State ==========

type connectionSuccessMsg struct {
	conn   *grpc.ClientConn
	stream grpc.BidiStreamingClient[pb.GameMessage, pb.GameMessage]
}

type connectionErrorMsg struct {
	err error
}

func (m *Model) updateConnecting(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case connectionSuccessMsg:
		m.conn = msg.conn
		m.stream = msg.stream
		m.state = StateWaitingForOpponent

		// Send player name as first message
		if err := m.stream.Send(pb.GameMessage_builder{Name: new(m.playerName)}.Build()); err != nil {
			return m.toMenu(fmt.Sprintf("Error sending name: %v", err))
		}

		return m, tea.Batch(
			m.spinner.Tick,
			m.waitForOpponent(),
		)

	case connectionErrorMsg:
		return m.toMenu(msg.err.Error())

	case tea.KeyPressMsg:
		if key.Matches(msg, m.lobbyKeys.Quit) {
			return m.toMenu("")
		}
	}

	return m, nil
}

func (m *Model) connectToServer() tea.Cmd {
	return func() tea.Msg {
		// TODO: pass server addr and port as env vars
		conn, err := grpc.NewClient("127.0.0.1:9000",
			// TODO: change insecure creds
			grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return connectionErrorMsg{err: fmt.Errorf("unable to connect: %w", err)}
		}

		stream, err := pb.NewTetrisServiceClient(conn).PlayTetris(m.ctx)
		if err != nil {
			return connectionErrorMsg{err: fmt.Errorf("unable to start game: %w", err)}
		}

		return connectionSuccessMsg{conn: conn, stream: stream}
	}
}

// ========== Waiting for Opponent State ==========

type streamErrorMsg struct {
	err error
}

func (m *Model) updateWaiting(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case *pb.GameMessage:
		// Received opponent state, start multiplayer game
		m.state = StateMultiplayer
		m.remoteState = msg

		ctx, cancel := context.WithCancel(m.ctx)
		m.ctx = ctx
		m.cancel = cancel
		m.game = tetris.Start(ctx)
		m.localState = tetris.GameMessage{}

		return m, tea.Batch(
			m.listenToGameUpdates(),
			m.listenToStreamUpdates(),
		)

	case streamErrorMsg:
		return m.toMenu(msg.err.Error())

	case tea.KeyPressMsg:
		if key.Matches(msg, m.lobbyKeys.Quit) {
			return m.toMenu("")
		}
	}

	return m, nil
}

func (m *Model) waitForOpponent() tea.Cmd {
	return func() tea.Msg {
		msg, err := m.stream.Recv()
		if err != nil {
			st, ok := status.FromError(err)
			if ok && st.Code() == codes.DeadlineExceeded {
				return streamErrorMsg{err: errors.New("There is no one to play with :(")}
			}
			return streamErrorMsg{err: fmt.Errorf("connection lost: %w", err)}
		}
		return msg
	}
}

// ========== Single Player State ==========

type localAnimationMessage struct{}

func (m *Model) updateSinglePlayer(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tetris.GameMessage:
		m.localState = msg

		if len(msg.ClearedLines) > 0 {
			m.localAnimating = true
			m.localAnimFrame = 8
			m.localAnimLayout = slices.Clone(msg.ClearedLines)
			return m, func() tea.Msg { return localAnimationMessage{} }
		}

		return m, m.listenToGameUpdates()

	case localAnimationMessage:
		m.localAnimFrame--
		if m.localAnimFrame == 0 {
			m.localAnimating = false
			return m, m.listenToGameUpdates()
		}
		return m, tea.Tick(40*time.Millisecond, func(time.Time) tea.Msg {
			return msg
		})

	case tea.KeyPressMsg:
		return m.handleGameKeys(msg)
	}

	return m, nil
}

// ========== Multiplayer State ==========

type remoteAnimationMessage struct{}

type gameOverMessage struct {
	msg string
}

func (m *Model) updateMultiplayer(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tetris.GameMessage:
		m.localState = msg

		// Try to send to opponent first - if error, abort immediately
		sendCmd := m.sendToOpponent(tetris2Proto(&msg, m.playerName))
		if sendCmd != nil {
			return m, sendCmd
		}

		// Successfully sent, continue with animation or listening
		if len(msg.ClearedLines) > 0 {
			m.localAnimating = true
			m.localAnimFrame = 8
			m.localAnimLayout = slices.Clone(msg.ClearedLines)
			return m, func() tea.Msg { return localAnimationMessage{} }
		}

		return m, m.listenToGameUpdates()

	case *pb.GameMessage:
		if msg.GetLinesClear() > 0 {
			m.game.Do(tetris.AddRemoteLines(int(msg.GetLinesClear())))
		}
		m.remoteState = msg

		if len(msg.GetClearedRowsIndexes().GetCells()) > 0 {
			m.remoteAnimating = true
			m.remoteAnimFrame = 8
			m.remoteAnimLayout = slices.Clone(msg.GetClearedRowsIndexes().GetCells())
			return m, func() tea.Msg { return remoteAnimationMessage{} }
		}
		return m, m.listenToStreamUpdates()

	case localAnimationMessage:
		m.localAnimFrame--
		if m.localAnimFrame == 0 {
			m.localAnimating = false
			return m, m.listenToGameUpdates()
		}
		return m, tea.Tick(40*time.Millisecond, func(time.Time) tea.Msg {
			return msg
		})

	case remoteAnimationMessage:
		m.remoteAnimFrame--
		if m.remoteAnimFrame == 0 {
			m.remoteAnimating = false
			return m, m.listenToStreamUpdates()
		}
		return m, tea.Tick(40*time.Millisecond, func(time.Time) tea.Msg {
			return msg
		})

	case gameOverMessage:
		return m.toMenu(msg.msg)

	case tea.KeyPressMsg:
		return m.handleGameKeys(msg)
	}

	return m, nil
}

// ========== Shared Game Handlers ==========

func (m *Model) handleGameKeys(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.gameKeys.Quit):
		quitMsg := ""
		if m.state == StateMultiplayer {
			quitMsg = messageYouQuit
		}
		return m.toMenu(quitMsg)
	case key.Matches(msg, m.gameKeys.MoveLeft):
		m.game.Do(tetris.MoveLeft())
	case key.Matches(msg, m.gameKeys.MoveRight):
		m.game.Do(tetris.MoveRight())
	case key.Matches(msg, m.gameKeys.MoveDown):
		m.game.Do(tetris.MoveDown())
	case key.Matches(msg, m.gameKeys.DropDown):
		m.game.Do(tetris.DropDown())
	case key.Matches(msg, m.gameKeys.RotateLeft):
		m.game.Do(tetris.RotateLeft())
	case key.Matches(msg, m.gameKeys.RotateRight):
		m.game.Do(tetris.RotateRight())
	}

	return m, nil
}

func (m *Model) listenToGameUpdates() tea.Cmd {
	return func() tea.Msg {
		select {
		case msg, ok := <-m.game.GameMessageCh:
			if !ok {
				if m.state == StateMultiplayer {
					return gameOverMessage{msg: messageYouLost}
				}
				return gameOverMessage{msg: messageGameOver}
			}
			return msg
		case <-m.ctx.Done():
			return gameOverMessage{msg: "cancelled"}
		}
	}
}

func (m *Model) listenToStreamUpdates() tea.Cmd {
	return func() tea.Msg {
		msg, err := m.stream.Recv()
		if err != nil {
			message := fmt.Sprintf("listening stream: %v", err)
			if err == io.EOF {
				return gameOverMessage{msg: messageYouWon}
			}
			st, ok := status.FromError(err)
			if ok && st.Code() == codes.Canceled {
				message = messageYouWon
			}
			return gameOverMessage{msg: message}
		}
		return msg
	}
}

func (m *Model) sendToOpponent(msg *pb.GameMessage) tea.Cmd {
	if err := m.stream.Send(msg); err != nil {
		var message string
		if errors.Is(err, io.EOF) {
			message = messageYouWon
		} else {
			message = "error in stream send():\n" + err.Error()
		}
		return func() tea.Msg {
			return gameOverMessage{msg: message}
		}
	}
	return nil
}

// ========== State Transitions ==========

func (m *Model) toMenu(notification string) (tea.Model, tea.Cmd) {
	m.cleanup()
	m.state = StateMenu
	m.notification = notification
	return m, nil
}

func (m *Model) cleanup() {
	if m.cancel != nil {
		m.cancel()
	}
	if m.stream != nil {
		m.stream.CloseSend() // nolint: errcheck
	}
	if m.conn != nil {
		m.conn.Close()
	}
	m.conn = nil
	m.stream = nil
	m.ctx = nil
	m.cancel = nil
}

// ========== View ==========

func (m *Model) View() tea.View {
	var content string

	switch m.state {
	case StateMenu:
		content = m.renderMenu()
	case StateConnecting:
		content = m.renderConnecting()
	case StateWaitingForOpponent:
		content = m.renderWaiting()
	case StateSinglePlayer:
		content = m.renderSinglePlayer()
	case StateMultiplayer:
		content = m.renderMultiplayer()
	}

	view := tea.NewView(content)
	view.WindowTitle = gameName
	view.AltScreen = true
	return view
}

func (m *Model) renderMenu() string {
	// Build menu overlay
	var menu strings.Builder
	for i, mode := range m.gameModes {
		if i == m.menuCursor {
			fmt.Fprintf(&menu, "> [%s] <\n", mode)
		} else {
			fmt.Fprintf(&menu, "  %s\n", mode)
		}
	}

	var overlay string
	if m.notification != "" {
		overlay = lipgloss.JoinVertical(
			lipgloss.Center,
			lipgloss.Wrap(boldStyle.Render(m.notification), 40, " "),
			"",
			faintStyle.Render("Press any key to continue"))
	} else {
		overlay = menu.String()
	}

	// Render base game boards
	center := renderCenterPanel(m.localState.Tetris, m.playerName, m.remoteState)
	cw, _ := lipgloss.Size(center)

	var rStack string
	if m.remoteState != nil {
		rStack = renderRemoteStack(m.remoteState)
	}

	base := lipgloss.JoinHorizontal(lipgloss.Top,
		renderStack(m.localState.Tetris),
		center,
		rStack,
	)
	bw, bh := lipgloss.Size(base)

	// Wrap overlay with border
	overlayStyled := lipgloss.NewStyle().
		Width(cw).
		Background(lipgloss.Color("")).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("69")).
		Padding(1, 2).
		Render(overlay)

	_, nh := lipgloss.Size(overlayStyled)
	help := helpStyle.Width(bw).Render(m.help.View(m.lobbyKeys))

	return lipgloss.NewCompositor(
		lipgloss.NewLayer(base),
		lipgloss.NewLayer(help).Y(bh),
		lipgloss.NewLayer(overlayStyled).X(23).Y(bh-nh).Z(1),
	).Render()
}

func (m *Model) renderConnecting() string {
	overlay := lipgloss.JoinVertical(
		lipgloss.Center,
		m.spinner.View()+" Connecting to server...",
		"",
		faintStyle.Render("Press q to cancel"))

	return m.renderOverlay(overlay)
}

func (m *Model) renderWaiting() string {
	overlay := lipgloss.JoinVertical(
		lipgloss.Center,
		m.spinner.View()+" Waiting for opponent...",
		"",
		faintStyle.Render("Press q to cancel"))

	return m.renderOverlay(overlay)
}

func (m *Model) renderOverlay(overlay string) string {
	center := renderCenterPanel(m.localState.Tetris, m.playerName, m.remoteState)
	cw, _ := lipgloss.Size(center)

	var rStack string
	if m.remoteState != nil {
		rStack = renderRemoteStack(m.remoteState)
	}

	base := lipgloss.JoinHorizontal(lipgloss.Top,
		renderStack(m.localState.Tetris),
		center,
		rStack,
	)
	bw, bh := lipgloss.Size(base)

	overlayStyled := lipgloss.NewStyle().
		Width(cw).
		Background(lipgloss.Color("")).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("69")).
		Padding(1, 2).
		Render(overlay)

	_, nh := lipgloss.Size(overlayStyled)
	help := helpStyle.Width(bw).Render(m.help.View(m.lobbyKeys))

	return lipgloss.NewCompositor(
		lipgloss.NewLayer(base),
		lipgloss.NewLayer(help).Y(bh),
		lipgloss.NewLayer(overlayStyled).X(23).Y(bh-nh).Z(1),
	).Render()
}

func (m *Model) renderSinglePlayer() string {
	center := lipgloss.JoinHorizontal(
		lipgloss.Top,
		renderStack(m.localState.Tetris),
		renderCenterPanel(m.localState.Tetris, "", nil),
	)
	c := lipgloss.NewCompositor(lipgloss.NewLayer(center))

	// Add local animation overlay
	if m.localAnimating && m.localAnimFrame%2 == 0 {
		for _, i := range m.localAnimLayout {
			c.AddLayers(lipgloss.NewLayer(strings.Repeat(" ", 20)).X(1).Y(20 - i))
		}
	}

	cw, ch := lipgloss.Size(center)
	c.AddLayers(lipgloss.NewLayer(helpStyle.Width(cw).Render(m.help.View(m.gameKeys))).Y(ch))

	return c.Render()
}

func (m *Model) renderMultiplayer() string {
	center := lipgloss.JoinHorizontal(
		lipgloss.Top,
		renderStack(m.localState.Tetris),
		renderCenterPanel(m.localState.Tetris, m.playerName, m.remoteState),
		renderRemoteStack(m.remoteState),
	)

	c := lipgloss.NewCompositor(lipgloss.NewLayer(center))
	cw, ch := lipgloss.Size(center)

	// Add local animation overlay
	if m.localAnimating && m.localAnimFrame%2 == 0 {
		for _, i := range m.localAnimLayout {
			c.AddLayers(lipgloss.NewLayer(strings.Repeat(" ", 20)).X(1).Y(20 - i))
		}
	}

	// Add remote animation overlay
	if m.remoteAnimating && m.remoteAnimFrame%2 == 0 {
		for _, i := range m.remoteAnimLayout {
			c.AddLayers(lipgloss.NewLayer(strings.Repeat(" ", 20)).X(cw - 21).Y(20 - int(i)))
		}
	}

	c.AddLayers(lipgloss.NewLayer(helpStyle.Width(cw).Render(m.help.View(m.gameKeys))).Y(ch))

	return c.Render()
}

// ========== Helper Functions ==========

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

	cl := []int32{}
	for _, v := range t.ClearedLines {
		cl = append(cl, int32(v)) //nolint: gosec
	}

	return pb.GameMessage_builder{
		ClearedRowsIndexes: pb.ClearedRowsIndexes_builder{
			Cells: cl,
		}.Build(),
		Name:       new(name),
		LinesClear: new(int32(t.Tetris.Lines)), // nolint: gosec
		Stack:      rendered,
	}.Build()
}
