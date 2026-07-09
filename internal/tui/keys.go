package tui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Send    key.Binding
	Approve key.Binding
	Deny    key.Binding
	Quit    key.Binding
}

var keys = keyMap{
	Send:    key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "send")),
	Approve: key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "approve")),
	Deny:    key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "deny")),
	Quit:    key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("ctrl+c", "quit")),
}
