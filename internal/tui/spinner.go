package tui

import (
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

// spinnerModel is a tiny bubbletea-model wrapper around bubbles/spinner,
// used for the connect/agents/session loading screens.
type spinnerModel struct {
	spinner.Model
}

func newSpinnerModel() spinnerModel {
	return spinnerModel{Model: spinner.New(spinner.WithSpinner(spinner.Dot))}
}

func (m spinnerModel) Init() tea.Cmd {
	return m.Tick
}

func (m spinnerModel) Update(msg tea.Msg) (spinnerModel, tea.Cmd) {
	var cmd tea.Cmd
	m.Model, cmd = m.Model.Update(msg)
	return m, cmd
}
