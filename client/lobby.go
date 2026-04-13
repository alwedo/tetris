package client

import (
	"context"
	"errors"
	"fmt"
	"strings"

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

	localGameState  tetris.GameMessage
	remoteGameState *pb.GameMessage

	lobbyState LobbyState

	// connection state (only used when lobbyState != Menu)
	spinner   spinner.Model
	parentCtx context.Context
	ctx       context.Context
	cancel    context.CancelFunc
	conn      *grpc.ClientConn
	stream    grpc.BidiStreamingClient[pb.GameMessage, pb.GameMessage]
}

func NewLobbyModel(ctx context.Context) *LobbyModel {
	return &LobbyModel{
		selectedMode: 0,
		gameModes:    []string{"Single Player", "Multiplayer"},
		keys:         lobbyKeys,
		help:         help.New(),
		lobbyState:   LobbyStateMenu,
		parentCtx:    ctx,
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
	switch msg := msg.(type) { // nolint: gocritic
	case tea.KeyPressMsg:
		if m.notification != "" {
			// if key.Matches(msg, m.keys.Select) {
			m.notification = ""
			// }
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
		case key.Matches(msg, m.keys.Select):
			if m.selectedMode == 0 {
				return m, func() tea.Msg {
					return TransitionToSingleGameMsg{}
				}
			}
			m.lobbyState = LobbyStateConnecting
			ctx, cancel := context.WithCancel(m.parentCtx)
			m.ctx = ctx
			m.cancel = cancel

			return m, tea.Batch(
				m.spinner.Tick,
				m.connectToServer(),
			)
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
		if err := m.stream.Send(pb.GameMessage_builder{Name: new("Player")}.Build()); err != nil {
			return m, func() tea.Msg {
				return connectionErrorMsg{
					err: fmt.Errorf("sending first message: %w", err),
				}
			}
		}
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

func (m *LobbyModel) View() tea.View {
	var rStack string
	if m.remoteGameState != nil {
		rStack = renderRemoteStack(m.remoteGameState)
	}

	base := lipgloss.JoinHorizontal(lipgloss.Top,
		renderStack(m.localGameState.Tetris),
		m.renderMenuPanel(),
		rStack,
	)
	bw, bh := lipgloss.Size(base)

	var overlay string
	switch {
	// TODO: refactor
	case m.lobbyState == LobbyStateMenu && m.notification != "":
		overlay = lipgloss.JoinVertical(
			lipgloss.Center,
			lipgloss.Wrap(lipgloss.NewStyle().Bold(true).Render(m.notification), bw-8, " "),
			"",
			lipgloss.NewStyle().Faint(true).Render("Press Enter to continue"))
	case m.lobbyState == LobbyStateConnecting:
		overlay = lipgloss.JoinVertical(
			lipgloss.Center,
			m.spinner.View()+" Connecting to server...",
			"",
			lipgloss.NewStyle().Faint(true).Render("Press esc to cancel"))
	case m.lobbyState == LobbyStateWaiting:
		overlay = lipgloss.JoinVertical(
			lipgloss.Center,
			m.spinner.View()+" Waiting for opponent...",
			"",
			lipgloss.NewStyle().Faint(true).Render("Press esc to cancel"))
	}

	if overlay != "" {
		overlay = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("69")).
			Padding(1, 2).
			Render(overlay)
	}

	nw, nh := lipgloss.Size(overlay)
	help := helpStyle.Width(bw).Render(m.help.View(m.keys))

	mainscreen := lipgloss.NewCompositor(
		lipgloss.NewLayer(base),
		lipgloss.NewLayer(help).Y(bh),
		lipgloss.NewLayer(overlay).X((bw-nw)/2).Y((bh-nh)/2).Z(1),
	).Render()

	return tea.NewView(mainscreen)
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
			lipgloss.NewStyle().Bold(true).Render(gameName),
			"\n",
			menu.String(),
		))
}

func (m *LobbyModel) cleanup() {
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

func (m *LobbyModel) connectToServer() tea.Cmd {
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

func (m *LobbyModel) waitForOpponent() tea.Cmd {
	return func() tea.Msg {
		msg, err := m.stream.Recv()
		if err != nil {
			st, ok := status.FromError(err)
			if ok && st.Code() == codes.DeadlineExceeded { //nolint: gocritic
				return streamErrorMsg{err: errors.New("There is no one to play with :(")} //nolint: staticcheck
			}
			return streamErrorMsg{err: fmt.Errorf("connection lost: %w", err)}
		}
		return msg
	}
}
