package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Savs-Agents/Botson-TUI/internal/natsapi"
)

type sessionItem struct {
	isNew bool
	stat  natsapi.SessionStat
}

func (s sessionItem) Title() string {
	if s.isNew {
		return "+ New session"
	}
	if s.stat.DisplayName != "" {
		return s.stat.DisplayName
	}
	return s.stat.ID
}

func (s sessionItem) Description() string {
	if s.isNew {
		return "Start a fresh conversation"
	}
	return fmt.Sprintf("%d events -- updated %s", s.stat.EventCount, s.stat.LastUpdated().Format("2006-01-02 15:04"))
}

func (s sessionItem) FilterValue() string { return s.Title() }

// sessionPickerModel lets the user resume an existing session (from
// botson.sessions.list) or start a new one.
type sessionPickerModel struct {
	list      list.Model
	chosen    bool
	isNew     bool
	sessionID string
}

func newSessionPickerModel(stats []natsapi.SessionStat, width, height int) sessionPickerModel {
	items := make([]list.Item, 0, len(stats)+1)
	items = append(items, sessionItem{isNew: true})
	for _, s := range stats {
		items = append(items, sessionItem{stat: s})
	}
	l := list.New(items, list.NewDefaultDelegate(), width, height)
	l.Title = "Select or create a session"
	l.SetShowStatusBar(false)
	return sessionPickerModel{list: l}
}

func (m sessionPickerModel) Update(msg tea.Msg) (sessionPickerModel, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.Type == tea.KeyEnter && !m.list.SettingFilter() {
		if item, ok := m.list.SelectedItem().(sessionItem); ok {
			m.chosen = true
			m.isNew = item.isNew
			m.sessionID = item.stat.ID
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// Selected reports the chosen session, once the user has picked one.
func (m sessionPickerModel) Selected() (isNew bool, sessionID string, ok bool) {
	return m.isNew, m.sessionID, m.chosen
}

func (m sessionPickerModel) View() string {
	return m.list.View()
}
