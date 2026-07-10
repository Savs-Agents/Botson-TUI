package tui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Send     key.Binding
	Scroll   key.Binding
	Approve  key.Binding
	Deny     key.Binding
	AutoMode key.Binding
	Quit     key.Binding
}

var keys = keyMap{
	Send:     key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "send")),
	Scroll:   key.NewBinding(key.WithKeys("pgup", "pgdown"), key.WithHelp("pgup/pgdn", "scroll")),
	Approve:  key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "approve")),
	Deny:     key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "deny")),
	AutoMode: key.NewBinding(key.WithKeys("ctrl+a"), key.WithHelp("ctrl+a", "toggle auto mode")),
	Quit:     key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("ctrl+c", "quit")),
}
