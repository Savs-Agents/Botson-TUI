package tui

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

type agentItem string

func (a agentItem) Title() string       { return string(a) }
func (a agentItem) Description() string { return "" }
func (a agentItem) FilterValue() string { return string(a) }

// agentPickerModel lets the user choose which agent to talk to, from the
// core's own list-apps response.
type agentPickerModel struct {
	list     list.Model
	selected string
	chosen   bool
}

func newAgentPickerModel(apps []string, width, height int) agentPickerModel {
	items := make([]list.Item, len(apps))
	for i, a := range apps {
		items[i] = agentItem(a)
	}
	l := list.New(items, list.NewDefaultDelegate(), width, height)
	l.Title = "Select an agent"
	l.SetShowStatusBar(false)
	return agentPickerModel{list: l}
}

func (m agentPickerModel) Update(msg tea.Msg) (agentPickerModel, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.Type == tea.KeyEnter && !m.list.SettingFilter() {
		if item, ok := m.list.SelectedItem().(agentItem); ok {
			m.selected = string(item)
			m.chosen = true
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// Selected reports the chosen agent name, once the user has picked one.
func (m agentPickerModel) Selected() (string, bool) {
	return m.selected, m.chosen
}

func (m agentPickerModel) View() string {
	return m.list.View()
}
