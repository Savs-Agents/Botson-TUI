package tui

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"

	"github.com/Savs-Agents/Botson-TUI/internal/natsapi"
)

// pendingConfirmation tracks an in-flight human-in-the-loop approval --
// see AGENTS.md "HITL confirmation wire protocol" in Botson-ADKv2 for the
// full event sequence this is built against.
type pendingConfirmation struct {
	confirmCallID string
	toolName      string
	hint          string
}

// chatModel is the main conversation view: history, input, and whatever
// HITL prompt might be blocking it.
type chatModel struct {
	client    *natsapi.Client
	app       string
	user      string
	sessionID string

	lines          []string
	viewport       viewport.Model
	input          textarea.Model
	spinner        spinner.Model
	help           help.Model
	md             *glamour.TermRenderer
	waiting        bool
	pendingConfirm *pendingConfirmation
}

func newChatModel(client *natsapi.Client, app, user, sessionID string, history []natsapi.Event, width, height int) chatModel {
	ta := textarea.New()
	ta.Placeholder = "Type a message..."
	ta.ShowLineNumbers = false
	ta.SetHeight(3)
	ta.Focus()

	vp := viewport.New(width, max(height-6, 3))

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))

	md, _ := glamour.NewTermRenderer(glamour.WithAutoStyle(), glamour.WithWordWrap(max(width-4, 20)))

	m := chatModel{
		client:    client,
		app:       app,
		user:      user,
		sessionID: sessionID,
		viewport:  vp,
		input:     ta,
		spinner:   sp,
		help:      help.New(),
		md:        md,
	}
	m.setWidth(width)
	m.processEvents(history)
	return m
}

func (m *chatModel) setWidth(width int) {
	m.viewport.Width = width
	m.input.SetWidth(width)
}

func (m chatModel) Init() tea.Cmd {
	return nil
}

func (m chatModel) Update(msg tea.Msg) (chatModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.setWidth(msg.Width)
		m.viewport.Height = max(msg.Height-6, 3)
		m.refreshViewport()
		return m, nil

	case tea.KeyMsg:
		if m.pendingConfirm != nil {
			switch msg.String() {
			case "y", "Y":
				return m.answerConfirm(true)
			case "n", "N":
				return m.answerConfirm(false)
			}
			return m, nil
		}
		if msg.Type == tea.KeyEnter && !m.waiting {
			text := strings.TrimSpace(m.input.Value())
			if text == "" {
				return m, nil
			}
			m.input.Reset()
			m.appendLine(userStyle.Render("you") + "\n" + text)
			m.waiting = true
			return m, tea.Batch(m.spinner.Tick, runTurnCmd(m.client, m.app, m.user, m.sessionID, natsapi.Content{
				Role:  "user",
				Parts: []natsapi.Part{{Text: text}},
			}))
		}
	}

	if m.waiting {
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	} else if m.pendingConfirm == nil {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		cmds = append(cmds, cmd)
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m chatModel) answerConfirm(confirmed bool) (chatModel, tea.Cmd) {
	pc := m.pendingConfirm
	m.pendingConfirm = nil
	label := "denied"
	if confirmed {
		label = "approved"
	}
	m.appendLine(dimStyle.Render(fmt.Sprintf("[%s: %s]", label, pc.toolName)))
	m.waiting = true

	resp := natsapi.Content{
		Role: "user",
		Parts: []natsapi.Part{{
			FunctionResponse: &natsapi.FunctionResponse{
				ID:       pc.confirmCallID,
				Name:     natsapi.ConfirmationFunctionName,
				Response: map[string]any{"confirmed": confirmed},
			},
		}},
	}
	return m, tea.Batch(m.spinner.Tick, runTurnCmd(m.client, m.app, m.user, m.sessionID, resp))
}

// applyEvents renders a turn's (or a resumed session's) events into the
// chat and clears the waiting spinner.
func (m chatModel) applyEvents(events []natsapi.Event) chatModel {
	m.waiting = false
	m.processEvents(events)
	m.refreshViewport()
	return m
}

func (m chatModel) applyError(err error) chatModel {
	m.waiting = false
	m.appendLine(errorStyle.Render("error: " + err.Error()))
	m.refreshViewport()
	return m
}

func (m *chatModel) processEvents(events []natsapi.Event) {
	for _, ev := range events {
		if ev.Content == nil {
			continue
		}
		for _, part := range ev.Content.Parts {
			switch {
			case part.Text != "":
				m.lines = append(m.lines, m.renderMessage(ev.Author, part.Text))

			case part.FunctionCall != nil && part.FunctionCall.Name == natsapi.ConfirmationFunctionName:
				args := decodeConfirmationArgs(part.FunctionCall.Args)
				toolName := ""
				if args.OriginalFunctionCall != nil {
					toolName = args.OriginalFunctionCall.Name
				}
				m.pendingConfirm = &pendingConfirmation{
					confirmCallID: part.FunctionCall.ID,
					toolName:      toolName,
					hint:          args.ToolConfirmation.Hint,
				}
				m.lines = append(m.lines, confirmStyle.Render(fmt.Sprintf("! %s\n  Approve? (y/n)", args.ToolConfirmation.Hint)))

			case part.FunctionCall != nil:
				m.lines = append(m.lines, dimStyle.Render(fmt.Sprintf("[tool call: %s]", part.FunctionCall.Name)))

			case part.FunctionResponse != nil:
				if isConfirmationBookkeeping(part.FunctionResponse.Response) {
					continue
				}
				m.lines = append(m.lines, dimStyle.Render(fmt.Sprintf("[tool result: %s]", part.FunctionResponse.Name)))
			}
		}
	}
	m.refreshViewport()
}

func (m chatModel) renderMessage(author, text string) string {
	if author == "" || author == "user" {
		return userStyle.Render("you") + "\n" + text
	}
	body := text
	if m.md != nil {
		if out, err := m.md.Render(text); err == nil {
			body = strings.TrimRight(out, "\n")
		}
	}
	return assistantStyle.Render(author) + "\n" + body
}

func (m *chatModel) appendLine(line string) {
	m.lines = append(m.lines, line)
	m.refreshViewport()
}

func (m *chatModel) refreshViewport() {
	m.viewport.SetContent(strings.Join(m.lines, "\n\n"))
	m.viewport.GotoBottom()
}

func decodeConfirmationArgs(args map[string]any) natsapi.ConfirmationArgs {
	var out natsapi.ConfirmationArgs
	data, err := json.Marshal(args)
	if err != nil {
		return out
	}
	_ = json.Unmarshal(data, &out)
	return out
}

// isConfirmationBookkeeping reports whether resp is the immediate,
// synthetic "blocked pending confirmation" functionResponse ADK inserts
// before the human has answered -- not a real tool result. See AGENTS.md
// step 2 of the HITL sequence.
func isConfirmationBookkeeping(resp map[string]any) bool {
	e, ok := resp["error"].(string)
	return ok && strings.Contains(e, "requires confirmation")
}

func (m chatModel) View() string {
	var b strings.Builder
	b.WriteString(headerStyle.Render(fmt.Sprintf("%s -- session %s", m.app, shortID(m.sessionID))))
	b.WriteString("\n")
	b.WriteString(m.viewport.View())
	b.WriteString("\n")
	switch {
	case m.waiting:
		b.WriteString(m.spinner.View())
		b.WriteString(" thinking...")
	case m.pendingConfirm != nil:
		b.WriteString(confirmStyle.Render("Waiting for your approval"))
	default:
		b.WriteString(m.input.View())
	}
	b.WriteString("\n")
	if m.pendingConfirm != nil {
		b.WriteString(m.help.ShortHelpView([]key.Binding{keys.Approve, keys.Deny, keys.Quit}))
	} else {
		b.WriteString(m.help.ShortHelpView([]key.Binding{keys.Send, keys.Quit}))
	}
	return b.String()
}

func shortID(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}
