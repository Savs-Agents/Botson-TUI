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

// argsPreviewMaxLen bounds how much of a tool call's args/error text shows
// inline in its status line -- enough to be useful, short enough that one
// call never dominates the log.
const argsPreviewMaxLen = 80

// toolCallView is the single, in-place-updating status line for one tool
// call across its full lifecycle: called -> awaiting approval/deferred ->
// approved/denied -> done/error. Replacing what used to be up to four
// separate appended lines per call (call, approve prompt, approved/denied,
// result) with one line that mutates in place has a second benefit beyond
// decluttering: the line's position in m.lines is fixed the moment the call
// is first seen (from the model-response event, a real ordered slice), so a
// later confirmation or result event arriving in ADK's own map-scrambled
// order (see AGENTS.md "HITL confirmation wire protocol" in Botson-ADKv2)
// only updates that fixed slot -- there's no append-in-arrival-order step
// left for the scrambling to visibly corrupt.
type toolCallView struct {
	name        string
	argsPreview string
	phase       string // "called", "awaiting approval", "deferred", "approved", "denied", "done", "error"
	detail      string // set only for phase == "error": the truncated failure message
	lineIndex   int
}

func newToolCallView(name, argsPreview string) *toolCallView {
	return &toolCallView{name: name, argsPreview: argsPreview, phase: "called"}
}

func (v *toolCallView) render() string {
	glyph, style := "▸", dimStyle
	switch v.phase {
	case "awaiting approval":
		glyph, style = "?", confirmStyle
	case "denied", "error":
		glyph, style = "✗", errorStyle
	case "done", "approved", "deferred", "auto-approved":
		glyph, style = "✓", dimStyle
	}
	text := fmt.Sprintf("%s %-15s %s", glyph, v.name, v.phase)
	switch {
	case v.phase == "error" && v.detail != "":
		text += " · " + v.detail
	case v.argsPreview != "":
		text += " · " + v.argsPreview
	}
	return style.Render(text)
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "..."
}

// pendingConfirmation tracks an in-flight human-in-the-loop approval --
// see AGENTS.md "HITL confirmation wire protocol" in Botson-ADKv2 for the
// full event sequence this is built against.
type pendingConfirmation struct {
	confirmCallID  string // the adk_request_confirmation wrapper's own call id -- what the FunctionResponse answer must reuse
	originalCallID string // the real tool call's id -- keys into chatModel.toolCalls so the answer can update its status line
	toolName       string
}

// chatModel is the main conversation view: history, input, and whatever
// HITL prompt might be blocking it.
type chatModel struct {
	client    *natsapi.Client
	app       string
	user      string
	sessionID string

	lines    []string
	viewport viewport.Model
	input    textarea.Model
	spinner  spinner.Model
	help     help.Model
	md       *glamour.TermRenderer
	waiting  bool

	// toolCalls holds every tool call's status line seen so far this
	// session, keyed by the call's own (stable, original) FunctionCall.ID
	// -- see toolCallView's doc for why this replaces the old append-only
	// per-stage log lines.
	toolCalls map[string]*toolCallView

	// wrapperOrig maps an adk_request_confirmation wrapper's own call id to
	// the original tool call id it's confirming -- needed because a
	// confirmation's answer (whether live or replayed from history) only
	// carries the wrapper's id, not the original call's.
	wrapperOrig map[string]string

	// autoMode mirrors the session's own natsapi.AutoModeStateKey flag --
	// when true, this client answers every confirmation itself (marked
	// natsapi.AutoModeResponseKey so history shows it wasn't a human's own
	// y/n) instead of queuing it. The core's own background automode
	// worker (Botson-ADKv2's internal/automode) does the same thing
	// unattended, as a fallback once this client disconnects -- so a
	// pending confirmation still gets answered even if you close the TUI
	// mid-task; this just answers it faster while you're still connected.
	autoMode bool

	// confirmQueue holds every HITL confirmation still awaiting an answer
	// from the current turn -- a model can call the same gated tool (or
	// several) in parallel, and ADK batches all of their
	// adk_request_confirmation wrappers into one event, not one event
	// each (see internal/llminternal/base_flow.go's handleFunctionCalls
	// in the ADK module: parallel FunctionCalls run concurrently and get
	// merged into a single confirmation-request event). Answered one at a
	// time (front of queue first); confirmResults accumulates the
	// FunctionResponse for each answer and is only sent, as a single
	// batched turn, once the queue is empty -- every gated call from that
	// turn needs a response before the run can proceed, so answering only
	// the first and sending immediately would leave the rest hanging
	// until the backend's context deadline exceeded.
	//
	// Ordering-only deferred confirmations (Deferred() true -- injected by
	// the core's toolorder plugin for a non-gated call sequenced behind
	// this turn's pending approvals) never enter confirmQueue: their
	// approved FunctionResponse goes straight into confirmResults, riding
	// out with the human's answers.
	confirmQueue   []pendingConfirmation
	confirmResults []natsapi.Part

	// pendingStateDelta, if non-nil, rides along with the next RunTurn
	// call only (then is cleared) -- used to set "botson:cwd" on a freshly
	// created session's first turn. Always nil when resuming a session.
	pendingStateDelta map[string]any
}

func newChatModel(client *natsapi.Client, app, user, sessionID string, history []natsapi.Event, state map[string]any, initialCwd string, width, height int) chatModel {
	ta := textarea.New()
	ta.Placeholder = "Type a message..."
	ta.ShowLineNumbers = false
	ta.SetHeight(3)
	ta.Focus()

	vp := viewport.New(width, max(height-6, 3))

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))

	md, _ := glamour.NewTermRenderer(glamour.WithAutoStyle(), glamour.WithWordWrap(max(width-4, 20)))

	autoMode, _ := state[natsapi.AutoModeStateKey].(bool)

	m := chatModel{
		client:      client,
		app:         app,
		user:        user,
		sessionID:   sessionID,
		viewport:    vp,
		input:       ta,
		spinner:     sp,
		help:        help.New(),
		md:          md,
		toolCalls:   make(map[string]*toolCallView),
		wrapperOrig: make(map[string]string),
		autoMode:    autoMode,
	}
	if initialCwd != "" {
		m.pendingStateDelta = map[string]any{"botson:cwd": initialCwd}
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

	case autoModeErrMsg:
		m.appendLine(errorStyle.Render("auto mode: " + msg.err.Error()))
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+a" {
			return m.toggleAutoMode()
		}
		if len(m.confirmQueue) > 0 {
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
			stateDelta := m.pendingStateDelta
			m.pendingStateDelta = nil
			return m, tea.Batch(m.spinner.Tick, runTurnCmd(m.client, m.app, m.user, m.sessionID, natsapi.Content{
				Role:  "user",
				Parts: []natsapi.Part{{Text: text}},
			}, stateDelta))
		}
		// PageUp/PageDown are the only keys that scroll history -- every
		// other key goes to the input box (see the isKey guard below),
		// since the viewport's own default bindings are plain letters
		// (u/d/j/k/h/l/space/f/b) that would otherwise fight with typing.
		if msg.Type == tea.KeyPgUp || msg.Type == tea.KeyPgDown {
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}
	}

	if m.waiting {
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	} else if len(m.confirmQueue) == 0 {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		cmds = append(cmds, cmd)
	}

	// The viewport's own key bindings are a vim-style pager (u/d/j/k/h/l/
	// space/f/b/pgup/pgdown) that would otherwise steal those letters
	// from the input box, so keyboard input goes to the textarea only --
	// the viewport still scrolls via mouse wheel (see tea.WithMouseCellMotion
	// in main.go) and auto-follows new messages via GotoBottom.
	if _, isKey := msg.(tea.KeyMsg); !isKey {
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// toggleAutoMode flips autoMode locally and persists it on the core via
// SubjectSessionsSetAutoMode, so the core's own background automode worker
// (and any other connected client) picks up the same setting.
func (m chatModel) toggleAutoMode() (chatModel, tea.Cmd) {
	m.autoMode = !m.autoMode
	label := "OFF"
	if m.autoMode {
		label = "ON"
	}
	m.appendLine(dimStyle.Render(fmt.Sprintf("[auto mode: %s]", label)))
	return m, setAutoModeCmd(m.client, m.app, m.user, m.sessionID, m.autoMode)
}

// answerConfirm records an answer for the confirmation at the front of
// confirmQueue, updates that call's status line in place, and pops it. If
// more are still queued (a parallel turn with several gated calls), it
// waits for the next y/n instead of sending anything yet -- every one of
// them needs an answer before the run can proceed, so the batched
// FunctionResponses only go out once the queue is empty.
func (m chatModel) answerConfirm(confirmed bool) (chatModel, tea.Cmd) {
	pc := m.confirmQueue[0]
	m.confirmQueue = m.confirmQueue[1:]

	if tv, ok := m.toolCalls[pc.originalCallID]; ok {
		if confirmed {
			tv.phase = "approved"
		} else {
			tv.phase = "denied"
		}
		m.lines[tv.lineIndex] = tv.render()
	}
	m.refreshViewport()

	m.confirmResults = append(m.confirmResults, natsapi.Part{
		FunctionResponse: &natsapi.FunctionResponse{
			ID:       pc.confirmCallID,
			Name:     natsapi.ConfirmationFunctionName,
			Response: map[string]any{"confirmed": confirmed},
		},
	})

	if len(m.confirmQueue) > 0 {
		return m, nil
	}

	return m.sendConfirmResults()
}

// sendConfirmResults flushes the accumulated confirmation answers (human
// and auto-approved deferred ones alike) back to the core as one user turn.
func (m chatModel) sendConfirmResults() (chatModel, tea.Cmd) {
	parts := m.confirmResults
	m.confirmResults = nil
	m.waiting = true

	resp := natsapi.Content{Role: "user", Parts: parts}
	// pendingStateDelta (if any) was already sent and cleared on the turn
	// that produced these pending confirmations -- never resent here.
	return m, tea.Batch(m.spinner.Tick, runTurnCmd(m.client, m.app, m.user, m.sessionID, resp, nil))
}

// applyEvents renders a turn's events into the chat and clears the waiting
// spinner. If the turn left auto-approved deferred confirmations pending
// with no human prompt alongside them to trigger answerConfirm's send, they
// are flushed immediately (a deferred confirmation normally co-occurs with
// at least one real prompt in the same event, but the run would stall
// forever if one ever arrived alone).
func (m chatModel) applyEvents(events []natsapi.Event) (chatModel, tea.Cmd) {
	m.waiting = false
	m.processEvents(events)
	m.refreshViewport()
	if len(m.confirmQueue) == 0 && len(m.confirmResults) > 0 {
		return m.sendConfirmResults()
	}
	return m, nil
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
				wrapperID := part.FunctionCall.ID
				var originalID, originalName string
				if args.OriginalFunctionCall != nil {
					originalID = args.OriginalFunctionCall.ID
					originalName = args.OriginalFunctionCall.Name
				}
				m.wrapperOrig[wrapperID] = originalID

				tv, ok := m.toolCalls[originalID]
				if !ok {
					// The plain FunctionCall this confirms should always
					// have been seen already (it precedes its wrapper in
					// every real turn) -- this fallback only guards
					// against an unexpected/partial event feed.
					tv = newToolCallView(originalName, "")
					m.lines = append(m.lines, tv.render())
					tv.lineIndex = len(m.lines) - 1
					m.toolCalls[originalID] = tv
				}

				switch {
				case args.Deferred():
					// An ordering-only deferral (the core sequenced a
					// non-gated call behind this turn's pending
					// approvals) carries no human decision: answer it
					// approved immediately, batched with the real
					// answers, and never show a y/n prompt for it.
					m.confirmResults = append(m.confirmResults, natsapi.Part{
						FunctionResponse: &natsapi.FunctionResponse{
							ID:       wrapperID,
							Name:     natsapi.ConfirmationFunctionName,
							Response: map[string]any{"confirmed": true},
						},
					})
					tv.phase = "deferred"

				case m.autoMode:
					// Auto mode is on for this session: answer a genuine
					// HITL confirmation ourselves instead of prompting,
					// marked AutoModeResponseKey so history (and the
					// core's own background automode worker, which would
					// otherwise answer it too once we go quiet) can tell
					// this was an unattended approval, not a human's y/n.
					m.confirmResults = append(m.confirmResults, natsapi.Part{
						FunctionResponse: &natsapi.FunctionResponse{
							ID:   wrapperID,
							Name: natsapi.ConfirmationFunctionName,
							Response: map[string]any{
								"confirmed":                 true,
								natsapi.AutoModeResponseKey: true,
							},
						},
					})
					tv.phase = "auto-approved"

				default:
					// Queued, not answered immediately -- a model can call
					// the same (or several) gated tools in parallel, and
					// ADK batches every one of that turn's
					// adk_request_confirmation wrappers into this single
					// event rather than one event each.
					m.confirmQueue = append(m.confirmQueue, pendingConfirmation{
						confirmCallID:  wrapperID,
						originalCallID: originalID,
						toolName:       tv.name,
					})
					tv.phase = "awaiting approval"
				}
				m.lines[tv.lineIndex] = tv.render()

			case part.FunctionCall != nil:
				if _, seen := m.toolCalls[part.FunctionCall.ID]; !seen {
					argsPreview := ""
					if len(part.FunctionCall.Args) > 0 {
						if b, err := json.Marshal(part.FunctionCall.Args); err == nil {
							argsPreview = truncate(string(b), argsPreviewMaxLen)
						}
					}
					tv := newToolCallView(part.FunctionCall.Name, argsPreview)
					m.lines = append(m.lines, tv.render())
					tv.lineIndex = len(m.lines) - 1
					m.toolCalls[part.FunctionCall.ID] = tv
				}

			case part.FunctionResponse != nil && part.FunctionResponse.Name == natsapi.ConfirmationFunctionName:
				// An already-recorded answer to a confirmation (seen when
				// replaying a resumed session's history): reflect that
				// answer on the call's status line, and drop any
				// queue/auto entry that thinks it's still unanswered
				// instead of prompting for (or re-sending) it again.
				m.resolveConfirmation(part.FunctionResponse.ID)
				if originalID, ok := m.wrapperOrig[part.FunctionResponse.ID]; ok {
					if tv, ok := m.toolCalls[originalID]; ok {
						confirmed, _ := part.FunctionResponse.Response["confirmed"].(bool)
						auto, _ := part.FunctionResponse.Response[natsapi.AutoModeResponseKey].(bool)
						switch {
						case !confirmed:
							tv.phase = "denied"
						case auto:
							tv.phase = "auto-approved"
						default:
							tv.phase = "approved"
						}
						m.lines[tv.lineIndex] = tv.render()
					}
				}

			case part.FunctionResponse != nil:
				if isConfirmationBookkeeping(part.FunctionResponse.Response) {
					continue
				}
				tv, ok := m.toolCalls[part.FunctionResponse.ID]
				if !ok {
					// The plain FunctionCall this answers should always
					// have been seen already -- this fallback only guards
					// against an unexpected/partial event feed.
					tv = newToolCallView(part.FunctionResponse.Name, "")
					m.lines = append(m.lines, tv.render())
					tv.lineIndex = len(m.lines) - 1
					m.toolCalls[part.FunctionResponse.ID] = tv
				}
				if errMsg, ok := part.FunctionResponse.Response["error"].(string); ok {
					tv.phase = "error"
					tv.detail = truncate(errMsg, argsPreviewMaxLen)
				} else {
					tv.phase = "done"
				}
				m.lines[tv.lineIndex] = tv.render()
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

// resolveConfirmation removes any pending prompt or accumulated answer for
// the given adk_request_confirmation call id -- called when history replay
// shows that id was already answered in a previous run of the TUI.
func (m *chatModel) resolveConfirmation(confirmCallID string) {
	queue := m.confirmQueue[:0]
	for _, pc := range m.confirmQueue {
		if pc.confirmCallID != confirmCallID {
			queue = append(queue, pc)
		}
	}
	m.confirmQueue = queue

	results := m.confirmResults[:0]
	for _, part := range m.confirmResults {
		if part.FunctionResponse == nil || part.FunctionResponse.ID != confirmCallID {
			results = append(results, part)
		}
	}
	m.confirmResults = results
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
	if m.autoMode {
		b.WriteString("  " + confirmStyle.Render("[auto mode: ON]"))
	}
	b.WriteString("\n")
	b.WriteString(m.viewport.View())
	b.WriteString("\n")
	switch {
	case m.waiting:
		b.WriteString(m.spinner.View())
		b.WriteString(" thinking...")
	case len(m.confirmQueue) > 0:
		msg := fmt.Sprintf("Waiting for your approval: %s", m.confirmQueue[0].toolName)
		if n := len(m.confirmQueue); n > 1 {
			msg = fmt.Sprintf("%s (%d more pending)", msg, n-1)
		}
		b.WriteString(confirmStyle.Render(msg))
	default:
		b.WriteString(m.input.View())
	}
	b.WriteString("\n")
	if len(m.confirmQueue) > 0 {
		b.WriteString(m.help.ShortHelpView([]key.Binding{keys.Approve, keys.Deny, keys.AutoMode, keys.Scroll, keys.Quit}))
	} else {
		b.WriteString(m.help.ShortHelpView([]key.Binding{keys.Send, keys.AutoMode, keys.Scroll, keys.Quit}))
	}
	return b.String()
}

func shortID(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}
