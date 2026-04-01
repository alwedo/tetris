package client

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// LobbyModel represents the game mode selection lobby screen
type LobbyModel struct {
	selectedMode int
	gameModes    []string
}

func NewLobbyModel() LobbyModel {
	return LobbyModel{
		selectedMode: 0,
		gameModes:    []string{"Single Player", "Multiplayer", "Quit"},
	}
}

func (m LobbyModel) Init() tea.Cmd {
	return nil
}

func (m LobbyModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "up", "k":
			if m.selectedMode > 0 {
				m.selectedMode--
			}
			return m, nil

		case "down", "j":
			if m.selectedMode < len(m.gameModes)-1 {
				m.selectedMode++
			}
			return m, nil

		case "enter", " ":
			gameMode := m.gameModes[m.selectedMode]
			if gameMode == "Quit" || gameMode == "Multiplayer" {
				return m, tea.Quit
			}
			return m, func() tea.Msg {
				return PlayGameMsg{}
			}

		case "q": // TODO: change to esc
			return m, tea.Quit
		}
	}

	return m, nil
}

// View renders the lobby screen
func (m LobbyModel) View() tea.View {
	content := strings.Builder{}

	for i, mode := range m.gameModes {
		if i == m.selectedMode {
			content.WriteString(fmt.Sprintf("> [%s] <\n", mode))
		} else {
			content.WriteString(fmt.Sprintf("  %s\n", mode))
		}
	}

	content.WriteString("\n\nControls:\n  ↑/k - Move Up\n  ↓/j - Move Down\n  ENTER/SPACE - Play\n  q - Quit\n")

	return tea.NewView(lipgloss.NewStyle().
		Width(66).
		Height(22).
		Border(lipgloss.RoundedBorder()).Render(content.String()))
}
