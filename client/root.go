package client

import (
	"context"

	tea "charm.land/bubbletea/v2"
)

type RootModel struct {
	currentModel tea.Model
	lobbyModel   *LobbyModel
	playerName   string
	ctx          context.Context
}

func NewRootModel(ctx context.Context, pn string) *RootModel {
	lobby := NewLobbyModel(ctx)
	return &RootModel{
		currentModel: lobby,
		lobbyModel:   lobby,
		playerName:   pn,
		ctx:          ctx,
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
		m.lobbyModel.localGameState = msg.LocalGameState
		m.lobbyModel.remoteGameState = msg.RemoteGameState
		m.lobbyModel.notification = msg.Message
		m.lobbyModel.lobbyState = LobbyStateMenu
		m.lobbyModel.playerName = m.playerName

		m.currentModel = m.lobbyModel
		return m, m.currentModel.Init()

	case TransitionToSingleGameMsg:
		m.currentModel = NewSingleGameModel()
		return m, m.currentModel.Init()

	case TransitionToMPGameMsg:
		m.currentModel = NewMPPlayingModel(m.ctx, m.playerName, msg.Conn, msg.Stream, msg.OpponentState)
		return m, m.currentModel.Init()
	}

	var cmd tea.Cmd
	m.currentModel, cmd = m.currentModel.Update(msg)
	return m, cmd
}

func (m *RootModel) View() tea.View {
	view := m.currentModel.View()
	view.WindowTitle = gameName
	view.AltScreen = true
	return view
}
