package client

import "charm.land/bubbles/v2/key"

type gameKeyMap struct {
	MoveLeft    key.Binding
	RotateLeft  key.Binding
	RotateRight key.Binding
	MoveRight   key.Binding
	MoveDown    key.Binding
	DropDown    key.Binding
	Quit        key.Binding
}

func (k gameKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.MoveLeft, k.RotateLeft, k.RotateRight, k.DropDown, k.Quit}
}

func (k gameKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{}
}

var gameKeys = gameKeyMap{
	MoveLeft: key.NewBinding(
		key.WithKeys("left"),
		key.WithHelp("←/→/↓", "move"),
	),
	RotateRight: key.NewBinding(
		key.WithKeys("up"),
		key.WithHelp("↑", "↷"),
	),
	RotateLeft: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "↶"),
	),
	MoveRight: key.NewBinding(
		key.WithKeys("right"),
	),
	MoveDown: key.NewBinding(
		key.WithKeys("down"),
	),
	DropDown: key.NewBinding(
		key.WithKeys("space"),
		key.WithHelp("space", "drop"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q"),
		key.WithHelp("q", "quit"),
	),
}

type lobbyKeyMap struct {
	Up     key.Binding
	Down   key.Binding
	Select key.Binding
	Quit   key.Binding
}

func (k lobbyKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Select, k.Quit}
}

func (k lobbyKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{}
}

var lobbyKeys = lobbyKeyMap{
	Up: key.NewBinding(
		key.WithKeys("up"),
		key.WithHelp("↑", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down"),
		key.WithHelp("↓", "down"),
	),
	Select: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("↵", "select"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q"),
		key.WithHelp("q", "quit"),
	),
}
