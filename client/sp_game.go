package client

import (
	"context"
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/alwedo/tetris"
)

type animationMessage struct{}

type SingleGameModel struct {
	game      *tetris.Game
	gameState tetris.GameMessage
	ctx       context.Context
	cancel    context.CancelFunc
	keys      gameKeyMap
	help      help.Model

	animationFrames int
	animationLayout []int
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
			m.animationFrames = 8
			m.animationLayout = slices.Clone(msg.ClearedLines)
			return m, func() tea.Msg { return animationMessage{} }
		}

		return m, m.listenToGameUpdates()

	case animationMessage:
		m.animationFrames--
		if m.animationFrames == 0 {
			return m, m.listenToGameUpdates()
		}
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
	c := lipgloss.NewCompositor(lipgloss.NewLayer(center))

	if m.animationFrames > 0 && m.animationFrames%2 == 0 {
		for _, i := range m.animationLayout {
			c.AddLayers(lipgloss.NewLayer(strings.Repeat(" ", 20)).X(1).Y(20 - i))
		}
	}

	cw, ch := lipgloss.Size(center)
	c.AddLayers(lipgloss.NewLayer(helpStyle.Width(cw).Render(m.help.View(m.keys))).Y(ch))

	return tea.NewView(c.Render())
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
