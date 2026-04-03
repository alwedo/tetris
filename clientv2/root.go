package client

import (
	tea "charm.land/bubbletea/v2"
	"github.com/alwedo/tetris/pb"
	tetris "github.com/alwedo/tetris/tetrisv2"
)

const appName = "Terminal Tetris"

// RootModel is the top-level model that orchestrates transitions between major states
type RootModel struct {
	currentModel tea.Model

	// Panel persistence - what to show in lobby
	lastLocalGame  *tetris.GameMessage
	lastRemoteGame *pb.GameMessage

	// Model instances (lobby is reused, games are created fresh)
	lobbyModel *LobbyModel
}

func NewRootModel() *RootModel {
	lobby := NewLobbyModel()
	return &RootModel{
		currentModel: lobby,
		lobbyModel:   lobby,
	}
}

func (m *RootModel) Init() tea.Cmd {
	return m.currentModel.Init()
}

func (m *RootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		if keyMsg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	}

	switch msg := msg.(type) {
	case TransitionToLobbyMsg:
		return m.transitionToLobby(msg)

	case TransitionToSingleGameMsg:
		return m.transitionToSingleGame()

	case TransitionToMPGameMsg:
		return m.transitionToMPGame(msg)
	}

	var cmd tea.Cmd
	m.currentModel, cmd = m.currentModel.Update(msg)
	return m, cmd
}

func (m *RootModel) transitionToLobby(msg TransitionToLobbyMsg) (tea.Model, tea.Cmd) {
	m.lobbyModel.localGameState = msg.LocalGameState
	m.lobbyModel.remoteGameState = msg.RemoteGameState
	m.lobbyModel.notification = msg.Message
	m.lobbyModel.lobbyState = LobbyStateMenu

	m.currentModel = m.lobbyModel
	return m, m.currentModel.Init()
}

func (m *RootModel) transitionToSingleGame() (tea.Model, tea.Cmd) {
	// Clear remote game for single player
	m.lastRemoteGame = nil

	m.currentModel = NewSingleGameModel()
	return m, m.currentModel.Init()
}

func (m *RootModel) transitionToMPGame(msg TransitionToMPGameMsg) (tea.Model, tea.Cmd) {
	m.currentModel = NewMPPlayingModel(msg.Conn, msg.Stream, msg.OpponentState)
	return m, m.currentModel.Init()
}

func (m *RootModel) View() tea.View {
	view := m.currentModel.View()
	view.WindowTitle = appName
	view.AltScreen = true
	return view
}
