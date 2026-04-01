package client

import "charm.land/bubbles/v2/key"

type gameKeyMap struct {
	MoveLeft    key.Binding
	RotateLeft  key.Binding
	RotateRight key.Binding
	MoveRight   key.Binding
	MoveDown    key.Binding
	DropDown    key.Binding
	Help        key.Binding
	Quit        key.Binding
}

func (k gameKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Quit}
}

func (k gameKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.MoveLeft, k.MoveRight, k.RotateLeft, k.RotateRight, k.MoveDown, k.DropDown},
		{k.Help, k.Quit},
	}
}

var gameKeys = gameKeyMap{
	MoveLeft: key.NewBinding(
		key.WithKeys("left", "a"),
		key.WithHelp("←/a", "move left"),
	),
	RotateLeft: key.NewBinding(
		key.WithKeys("q"),
		key.WithHelp("q", "rotate left"),
	),
	MoveRight: key.NewBinding(
		key.WithKeys("right", "d"),
		key.WithHelp("→/d", "move right"),
	),
	RotateRight: key.NewBinding(
		key.WithKeys("up", "e"),
		key.WithHelp("↑/e", "rotate right"),
	),
	MoveDown: key.NewBinding(
		key.WithKeys("down", "s"),
		key.WithHelp("↓/s", "move down"),
	),
	DropDown: key.NewBinding(
		key.WithKeys("space"),
		key.WithHelp("space", "drop down"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "toggle help"),
	),
	Quit: key.NewBinding(
		key.WithKeys("esc", "ctrl+c"),
		key.WithHelp("esc", "quit"),
	),
}
