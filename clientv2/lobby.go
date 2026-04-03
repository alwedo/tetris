package client

import (
	"context"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/alwedo/tetris/pb"
	tetris "github.com/alwedo/tetris/tetrisv2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type LobbyState int

const (
	LobbyStateMenu LobbyState = iota
	LobbyStateConnecting
	LobbyStateWaiting
)

type LobbyModel struct {
	// UI state
	selectedMode int
	gameModes    []string
	notification string
	keys         lobbyKeyMap
	help         help.Model

	// Persisted game states (injected by root)
	localGameState  tetris.GameMessage
	remoteGameState *pb.GameMessage

	// Lobby internal state
	lobbyState LobbyState

	// Connection state (only used when lobbyState != Menu)
	spinner spinner.Model
	ctx     context.Context
	cancel  context.CancelFunc
	conn    *grpc.ClientConn
	stream  grpc.BidiStreamingClient[pb.GameMessage, pb.GameMessage]
}

func NewLobbyModel() *LobbyModel {
	return &LobbyModel{
		selectedMode: 0,
		gameModes:    []string{"Single Player", "Multiplayer"},
		keys:         lobbyKeys,
		help:         help.New(),
		lobbyState:   LobbyStateMenu,
		spinner: spinner.New(
			spinner.WithSpinner(spinner.Points),
			spinner.WithStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("205"))),
		),
	}
}

func (m *LobbyModel) Init() tea.Cmd {
	return nil
}

func (m *LobbyModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.lobbyState {
	case LobbyStateMenu:
		return m.updateMenu(msg)
	case LobbyStateConnecting:
		return m.updateConnecting(msg)
	case LobbyStateWaiting:
		return m.updateWaiting(msg)
	}

	return m, nil
}

func (m *LobbyModel) updateMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if m.notification != "" {
			if key.Matches(msg, m.keys.Select) {
				m.notification = ""
			}
			return m, nil
		}

		switch {
		case key.Matches(msg, m.keys.Up):
			if m.selectedMode > 0 {
				m.selectedMode--
			}
		case key.Matches(msg, m.keys.Down):
			if m.selectedMode < len(m.gameModes)-1 {
				m.selectedMode++
			}
		case key.Matches(msg, m.keys.Help):
			m.help.ShowAll = !m.help.ShowAll
		case key.Matches(msg, m.keys.Select):
			if m.selectedMode == 0 {
				return m, func() tea.Msg {
					return TransitionToSingleGameMsg{}
				}
			} else {
				m.lobbyState = LobbyStateConnecting
				ctx, cancel := context.WithCancel(context.Background())
				m.ctx = ctx
				m.cancel = cancel

				return m, tea.Batch(
					m.spinner.Tick,
					m.connectToServer(),
				)
			}
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m *LobbyModel) updateConnecting(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case connectionSuccessMsg:
		m.conn = msg.conn
		m.stream = msg.stream
		m.lobbyState = LobbyStateWaiting
		m.stream.Send(pb.GameMessage_builder{Name: new("Player")}.Build())
		return m, tea.Batch(
			m.spinner.Tick,
			m.waitForOpponent(),
		)

	case connectionErrorMsg:
		m.cleanup()
		m.lobbyState = LobbyStateMenu
		m.notification = msg.err.Error()
		return m, nil

	case tea.KeyPressMsg:
		if key.Matches(msg, m.keys.Quit) {
			m.cleanup()
			m.lobbyState = LobbyStateMenu
			return m, nil
		}
	}

	return m, nil
}

func (m *LobbyModel) updateWaiting(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case *pb.GameMessage:
		return m, func() tea.Msg {
			return TransitionToMPGameMsg{
				Conn:          m.conn,
				Stream:        m.stream,
				OpponentState: msg,
			}
		}

	case streamErrorMsg:
		m.cleanup()
		m.lobbyState = LobbyStateMenu
		m.notification = "Connection lost: " + msg.err.Error()
		return m, nil

	case tea.KeyPressMsg:
		if key.Matches(msg, m.keys.Quit) {
			m.cleanup()
			m.lobbyState = LobbyStateMenu
			return m, nil
		}
	}

	return m, nil
}

func (m *LobbyModel) View() tea.View {
	var rStack string
	if m.remoteGameState != nil {
		rStack = renderRemoteStack(m.remoteGameState)
	}

	base := lipgloss.JoinHorizontal(lipgloss.Bottom,
		renderStack(m.localGameState.Tetris),
		m.renderCenterPanel(),
		rStack,
	)

	if m.lobbyState == LobbyStateMenu && m.notification != "" {
		base = m.applyNotificationOverlay(base)
	}

	return tea.NewView(base)
}

func (m *LobbyModel) renderCenterPanel() string {
	var top string

	switch m.lobbyState {
	case LobbyStateMenu:
		top = m.renderMenuPanel()
	case LobbyStateConnecting:
		top = m.renderConnectingPanel()
	case LobbyStateWaiting:
		top = m.renderWaitingPanel()
	}

	helpText := m.help.View(m.keys)
	if m.lobbyState == LobbyStateConnecting || m.lobbyState == LobbyStateWaiting {
		helpText = "esc: cancel"
	}

	bottom := lipgloss.NewStyle().
		Width(22).
		Align(lipgloss.Center).
		Border(lipgloss.RoundedBorder()).
		Foreground(lipgloss.Color("#FF75B7")).
		Render(helpText)

	return lipgloss.JoinVertical(lipgloss.Center, top, bottom)
}

func (m *LobbyModel) renderMenuPanel() string {
	var menu strings.Builder
	for i, mode := range m.gameModes {
		if i == m.selectedMode {
			fmt.Fprintf(&menu, "> [%s] <\n", mode)
		} else {
			fmt.Fprintf(&menu, "  %s\n", mode)
		}
	}

	return lipgloss.NewStyle().
		Width(22).
		Align(lipgloss.Center).
		Border(lipgloss.RoundedBorder()).
		Render(lipgloss.JoinVertical(lipgloss.Center,
			lipgloss.NewStyle().Bold(true).Render(appName),
			"\n",
			menu.String(),
		))
}

func (m *LobbyModel) renderConnectingPanel() string {
	return lipgloss.NewStyle().
		Width(22).
		Align(lipgloss.Center).
		Border(lipgloss.RoundedBorder()).
		Render(lipgloss.JoinVertical(lipgloss.Center,
			lipgloss.NewStyle().Bold(true).Render(appName),
			"\n",
			m.spinner.View()+" Connecting to server...",
		))
}

func (m *LobbyModel) renderWaitingPanel() string {
	return lipgloss.NewStyle().
		Width(22).
		Align(lipgloss.Center).
		Border(lipgloss.RoundedBorder()).
		Render(lipgloss.JoinVertical(lipgloss.Center,
			lipgloss.NewStyle().Bold(true).Render(appName),
			"\n",
			m.spinner.View()+" Waiting for opponent...",
		))
}

func (m *LobbyModel) cleanup() {
	if m.cancel != nil {
		m.cancel()
	}
	if m.stream != nil {
		m.stream.CloseSend()
	}
	if m.conn != nil {
		m.conn.Close()
	}
	m.conn = nil
	m.stream = nil
	m.ctx = nil
	m.cancel = nil
}

func (m *LobbyModel) connectToServer() tea.Cmd {
	return func() tea.Msg {
		// TODO: pass server addr and port as env vars
		conn, err := grpc.NewClient("127.0.0.1:9000",
			// TODO: change insecure creds
			grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return connectionErrorMsg{err: fmt.Errorf("unable to connect: %w", err)}
		}

		stream, err := pb.NewTetrisServiceClient(conn).PlayTetris(context.Background())
		if err != nil {
			return connectionErrorMsg{err: fmt.Errorf("unable to start game: %w", err)}
		}

		return connectionSuccessMsg{conn: conn, stream: stream}
	}
}

func (m *LobbyModel) waitForOpponent() tea.Cmd {
	return func() tea.Msg {
		msg, err := m.stream.Recv()
		if err != nil {
			return streamErrorMsg{err: fmt.Errorf("connection lost: %w", err)}
		}
		return msg
	}
}

func (m *LobbyModel) applyNotificationOverlay(base string) string {
	notificationBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Render(lipgloss.JoinVertical(
			lipgloss.Center,
			lipgloss.NewStyle().Bold(true).Render(m.notification+"\n"),
			lipgloss.NewStyle().Faint(true).Render("[OK]"),
		))

	bw := lipgloss.Width(base)
	bh := lipgloss.Height(base)
	nw := lipgloss.Width(notificationBox)
	nh := lipgloss.Height(notificationBox)

	return lipgloss.NewCompositor(
		lipgloss.NewLayer(base),
		lipgloss.NewLayer(notificationBox).X((bw-nw)/2).Y((bh-nh)/2).Z(1),
	).Render()
}
