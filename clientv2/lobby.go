package client

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	tetris "github.com/alwedo/tetris/tetrisv2"
)

// LobbyModel represents the game mode selection lobby screen
type LobbyModel struct {
	selectedMode   int
	gameModes      []string
	keys           lobbyKeyMap
	help           help.Model
	localGameState tetris.GameMessage
}

func NewLobbyModel() LobbyModel {
	return LobbyModel{
		selectedMode: 0,
		gameModes:    []string{"Single Player", "Multiplayer"},
		keys:         lobbyKeys,
		help:         help.New(),
	}
}

func (m LobbyModel) Init() tea.Cmd {
	return nil
}

func (m LobbyModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keys.Up):
			if m.selectedMode > 0 {
				m.selectedMode--
			}
		case key.Matches(msg, m.keys.Down):
			if m.selectedMode < len(m.gameModes)-1 {
				m.selectedMode++
			}
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Help):
			m.help.ShowAll = !m.help.ShowAll
		case key.Matches(msg, m.keys.Select):
			gameMode := m.gameModes[m.selectedMode]
			if gameMode == "Multiplayer" {
				return m, tea.Quit
			}
			return m, func() tea.Msg {
				return PlayGameMsg{}
			}
		}
	}

	return m, nil
}

// View renders the lobby screen
func (m LobbyModel) View() tea.View {
	localGame := renderStack(m.localGameState.Tetris)

	gameName := lipgloss.NewStyle().Bold(true).Render(appName)
	controls := strings.Builder{}

	for i, mode := range m.gameModes {
		if i == m.selectedMode {
			controls.WriteString(fmt.Sprintf("> [%s] <\n", mode))
		} else {
			controls.WriteString(fmt.Sprintf("  %s\n", mode))
		}
	}

	menu := lipgloss.JoinVertical(lipgloss.Center, gameName, "\n", controls.String(), m.help.View(m.keys))

	return tea.NewView(lipgloss.JoinHorizontal(lipgloss.Bottom, localGame, menu))
}
