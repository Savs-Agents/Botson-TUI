package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

// cwdPromptModel is a one-field step shown before creating a brand-new
// session: an optional absolute path overriding the core's default
// workspace for that session (sent as "botson:cwd" state on the first
// turn -- see docs/nats-api.md in Botson-ADKv2). Never shown when
// resuming an existing session, which already has whatever cwd it was
// created with.
type cwdPromptModel struct {
	form *huh.Form
	cwd  string
}

func newCwdPromptModel() cwdPromptModel {
	m := cwdPromptModel{}
	m.form = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Working directory for this session").
				Description("Optional. An absolute path the agent's file/command tools should use instead of the core's default workspace. Leave blank to use the default.").
				Value(&m.cwd),
		),
	)
	return m
}

func (m cwdPromptModel) Init() tea.Cmd {
	return m.form.Init()
}

func (m cwdPromptModel) Update(msg tea.Msg) (cwdPromptModel, tea.Cmd) {
	form, cmd := m.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.form = f
	}
	return m, cmd
}

func (m cwdPromptModel) View() string {
	return m.form.View()
}

// Completed reports whether the user has submitted the form.
func (m cwdPromptModel) Completed() bool {
	return m.form.State == huh.StateCompleted
}
