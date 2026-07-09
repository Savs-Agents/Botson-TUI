package tui

import (
	"strconv"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

// connectModel is the first screen: where's the core, and who am I to it.
type connectModel struct {
	form   *huh.Form
	host   string
	port   string
	userID string
	errMsg string
}

func newConnectModel(host string, port int, userID string) connectModel {
	m := connectModel{
		host:   host,
		port:   strconv.Itoa(port),
		userID: userID,
	}
	m.form = huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("Botson-TUI").
				Description("A standalone chat client for a running `botson core`, talking purely over NATS."),
			huh.NewInput().Title("Core host").Value(&m.host),
			huh.NewInput().Title("Core port").Value(&m.port),
			huh.NewInput().
				Title("Your user ID").
				Description("Identifies your sessions to the core -- any string works, it's yours to choose.").
				Value(&m.userID),
		),
	)
	return m
}

func (m connectModel) Init() tea.Cmd {
	return m.form.Init()
}

func (m connectModel) Update(msg tea.Msg) (connectModel, tea.Cmd) {
	form, cmd := m.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.form = f
	}
	return m, cmd
}

func (m connectModel) View() string {
	view := m.form.View()
	if m.errMsg != "" {
		view += "\n" + errorStyle.Render("error: "+m.errMsg)
	}
	return view
}

// Completed reports whether the user has submitted the form.
func (m connectModel) Completed() bool {
	return m.form.State == huh.StateCompleted
}

// Values returns the submitted host, port, and user ID.
func (m connectModel) Values() (host string, port int, userID string) {
	p, _ := strconv.Atoi(m.port)
	return m.host, p, m.userID
}
