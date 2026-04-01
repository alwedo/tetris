package client

import (
	tea "charm.land/bubbletea/v2"
)

const appName = "Terminal Tetris"

type PlayGameMsg struct{}

type BackToLobbyMsg struct {
	Reason string
}

type ClientModel struct {
	currentModel tea.Model
	lobbyModel   *LobbyModel
	gameModel    *GameModel
}

func NewClientModel() *ClientModel {
	lobbyModel := NewLobbyModel()
	return &ClientModel{
		currentModel: lobbyModel,
		lobbyModel:   &lobbyModel,
		gameModel:    nil,
	}
}

func (m *ClientModel) Init() tea.Cmd {
	return m.currentModel.Init()
}

func (m *ClientModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

	case PlayGameMsg:
		gameModel := NewGameModel()
		m.gameModel = &gameModel
		m.currentModel = m.gameModel
		return m, m.currentModel.Init()

	case BackToLobbyMsg:
		m.currentModel = m.lobbyModel
		return m, m.currentModel.Init()
	}

	var cmd tea.Cmd
	m.currentModel, cmd = m.currentModel.Update(msg)
	return m, cmd
}

func (m *ClientModel) View() tea.View {
	v := m.currentModel.View()
	v.AltScreen = true
	v.WindowTitle = appName
	return v
}
