package client

import (
	"context"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/alwedo/tetris"
)

type SingleGameModel struct {
	game      *tetris.Game
	gameState tetris.GameMessage
	ctx       context.Context
	cancel    context.CancelFunc
	keys      gameKeyMap
	help      help.Model
}

func NewSingleGameModel() *SingleGameModel {
	return &SingleGameModel{
		keys: gameKeys,
		help: help.New(),
	}
}

func (m *SingleGameModel) Init() tea.Cmd {
	ctx, cancel := context.WithCancel(context.Background()) // nolint: gosec
	m.ctx = ctx
	m.cancel = cancel
	m.game = tetris.Start(ctx)

	return m.listenToGameUpdates()
}

func (m *SingleGameModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tetris.GameMessage:
		m.gameState = msg

		if len(msg.ClearedLines) > 0 {
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
		// Skip rendering the tetromino during animation
		if m.gameState.Tetris.Tetromino != nil {
			m.gameState.Tetris.Tetromino = nil
		}
		for k, v := range msg.completedRows {
			if msg.frames%2 == 0 {
				m.gameState.Tetris.Stack[k] = make([]tetris.Shape, 10)
			} else {
				m.gameState.Tetris.Stack[k] = v
			}
		}
		msg.frames--
		return m, tea.Tick(40*time.Millisecond, func(time.Time) tea.Msg {
			return msg
		})

	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			m.cancel()
			return m, func() tea.Msg {
				return TransitionToLobbyMsg{
					LocalGameState: m.gameState,
				}
			}

		case key.Matches(msg, m.keys.MoveLeft):
			m.game.Do(tetris.MoveLeft())

		case key.Matches(msg, m.keys.MoveRight):
			m.game.Do(tetris.MoveRight())

		case key.Matches(msg, m.keys.MoveDown):
			m.game.Do(tetris.MoveDown())

		case key.Matches(msg, m.keys.DropDown):
			m.game.Do(tetris.DropDown())

		case key.Matches(msg, m.keys.RotateLeft):
			m.game.Do(tetris.RotateLeft())

		case key.Matches(msg, m.keys.RotateRight):
			m.game.Do(tetris.RotateRight())

		case key.Matches(msg, m.keys.Help):
			m.help.ShowAll = !m.help.ShowAll
		}
	}

	return m, nil
}

func (m *SingleGameModel) View() tea.View {
	center := lipgloss.JoinHorizontal(
		lipgloss.Top,
		renderStack(m.gameState.Tetris),
		renderCenterPanel(m.gameState.Tetris, "", nil),
	)

	cw, ch := lipgloss.Size(center)
	help := helpStyle.Width(cw).Render(m.help.View(m.keys))

	return tea.NewView(lipgloss.NewCompositor(
		lipgloss.NewLayer(center),
		lipgloss.NewLayer(help).Y(ch),
	).Render())
}

func (m *SingleGameModel) listenToGameUpdates() tea.Cmd {
	return func() tea.Msg {
		select {
		case msg, ok := <-m.game.GameMessageCh:
			if !ok {
				// Channel closed = game over
				m.cancel()
				return TransitionToLobbyMsg{
					LocalGameState: m.gameState,
					Message:        "Game Over!",
				}
			}
			return msg
		case <-m.ctx.Done():
			return TransitionToLobbyMsg{LocalGameState: m.gameState}
		}
	}
}

// AnimationMessage for line clear animation
type AnimationMessage struct {
	frames        int
	completedRows map[int][]tetris.Shape
}

func newAnimationMsg(c map[int][]tetris.Shape) tea.Cmd {
	return func() tea.Msg {
		return AnimationMessage{
			frames:        8,
			completedRows: c,
		}
	}
}
