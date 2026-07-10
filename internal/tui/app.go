// Package tui is Botson-TUI's bubbletea application: connect to a core,
// pick an agent, create or resume a session, and chat.
package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Savs-Agents/Botson-TUI/internal/config"
	"github.com/Savs-Agents/Botson-TUI/internal/natsapi"
)

type mode int

const (
	modeConnect mode = iota
	modeAgentPicker
	modeSessionPicker
	modeNewSessionCwd
	modeChat
)

// Model is the root bubbletea model, dispatching to whichever screen is
// currently active.
type Model struct {
	mode   mode
	width  int
	height int

	cfg           config.Config
	client        *natsapi.Client
	selectedAgent string
	pendingCwd    string // set by the cwd prompt, consumed once the new session's first turn is sent

	loading     bool
	loadingText string
	spinnerOnly spinnerModel

	connect   connectModel
	agents    agentPickerModel
	sessions  sessionPickerModel
	cwdPrompt cwdPromptModel
	chat      chatModel
}

// New builds the initial Model from a loaded local config.
func New(cfg config.Config) Model {
	m := Model{
		cfg:         cfg,
		mode:        modeConnect,
		connect:     newConnectModel(cfg.Host, cfg.Port, cfg.UserID, cfg.Token),
		spinnerOnly: newSpinnerModel(),
	}
	return m
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.connect.Init(), m.spinnerOnly.Init())
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		if m.mode == modeChat {
			var cmd tea.Cmd
			m.chat, cmd = m.chat.Update(msg)
			return m, cmd
		}
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			if m.client != nil {
				m.client.Close()
			}
			return m, tea.Quit
		}

	case connectedMsg:
		m.client = msg.client
		m.loading = true
		m.loadingText = "Loading agents..."
		return m, listAppsCmd(m.client)

	case connectErrMsg:
		m.loading = false
		m.connect.errMsg = msg.err.Error()
		return m, nil

	case appsLoadedMsg:
		m.loading = false
		m.mode = modeAgentPicker
		m.agents = newAgentPickerModel(msg.apps, m.width, m.height-2)
		return m, nil

	case appsErrMsg:
		m.loading = false
		m.mode = modeConnect
		m.connect.errMsg = msg.err.Error()
		return m, nil

	case sessionsLoadedMsg:
		m.loading = false
		m.mode = modeSessionPicker
		m.sessions = newSessionPickerModel(msg.stats, m.width, m.height-2)
		return m, nil

	case sessionsErrMsg:
		m.loading = false
		m.mode = modeAgentPicker
		return m, nil

	case sessionReadyMsg:
		m.loading = false
		m.mode = modeChat
		m.chat = newChatModel(m.client, m.selectedAgent, m.cfg.UserID, msg.sess.ID, msg.sess.Events, msg.sess.State, m.pendingCwd, m.width, m.height)
		m.pendingCwd = ""
		return m, m.chat.Init()

	case sessionErrMsg:
		m.loading = false
		m.mode = modeSessionPicker
		return m, nil

	case turnDoneMsg:
		var cmd tea.Cmd
		m.chat, cmd = m.chat.applyEvents(msg.events)
		return m, cmd

	case turnErrMsg:
		m.chat = m.chat.applyError(msg.err)
		return m, nil
	}

	if m.loading {
		var cmd tea.Cmd
		m.spinnerOnly, cmd = m.spinnerOnly.Update(msg)
		return m, cmd
	}

	switch m.mode {
	case modeConnect:
		var cmd tea.Cmd
		m.connect, cmd = m.connect.Update(msg)
		if m.connect.Completed() {
			host, port, userID, token := m.connect.Values()
			m.cfg.Host, m.cfg.Port, m.cfg.UserID, m.cfg.Token = host, port, userID, token
			_ = config.Save(m.cfg)
			m.loading = true
			m.loadingText = "Connecting..."
			return m, connectCmd(host, port, token)
		}
		return m, cmd

	case modeAgentPicker:
		var cmd tea.Cmd
		m.agents, cmd = m.agents.Update(msg)
		if sel, ok := m.agents.Selected(); ok {
			m.selectedAgent = sel
			m.cfg.LastAgent = sel
			_ = config.Save(m.cfg)
			m.loading = true
			m.loadingText = "Loading sessions..."
			return m, sessionsListCmd(m.client, sel, m.cfg.UserID)
		}
		return m, cmd

	case modeSessionPicker:
		var cmd tea.Cmd
		m.sessions, cmd = m.sessions.Update(msg)
		if isNew, sid, ok := m.sessions.Selected(); ok {
			if isNew {
				m.mode = modeNewSessionCwd
				m.cwdPrompt = newCwdPromptModel()
				return m, m.cwdPrompt.Init()
			}
			m.loading = true
			m.loadingText = "Loading session..."
			return m, getSessionCmd(m.client, m.selectedAgent, m.cfg.UserID, sid)
		}
		return m, cmd

	case modeNewSessionCwd:
		var cmd tea.Cmd
		m.cwdPrompt, cmd = m.cwdPrompt.Update(msg)
		if m.cwdPrompt.Completed() {
			m.pendingCwd = strings.TrimSpace(m.cwdPrompt.cwd)
			m.loading = true
			m.loadingText = "Creating session..."
			return m, createSessionCmd(m.client, m.selectedAgent, m.cfg.UserID)
		}
		return m, cmd

	case modeChat:
		var cmd tea.Cmd
		m.chat, cmd = m.chat.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) View() string {
	if m.loading {
		return lipgloss.NewStyle().Padding(1, 2).Render(m.spinnerOnly.View() + " " + m.loadingText)
	}

	switch m.mode {
	case modeConnect:
		return lipgloss.NewStyle().Padding(1, 2).Render(m.connect.View())
	case modeAgentPicker:
		return m.agents.View()
	case modeSessionPicker:
		return m.sessions.View()
	case modeNewSessionCwd:
		return lipgloss.NewStyle().Padding(1, 2).Render(m.cwdPrompt.View())
	case modeChat:
		return m.chat.View()
	}
	return ""
}
