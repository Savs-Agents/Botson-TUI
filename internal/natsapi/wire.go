package natsapi

import "time"

// The types below are a hand-rolled, minimal mirror of the JSON wire shapes
// ADK's REST API (fronted here by NATS-ADK-Proxy) actually sends and
// expects -- see google.golang.org/adk/v2/server/adkrest/internal/models
// for the authoritative source. Botson-TUI deliberately doesn't import
// ADK's Go SDK or genai types: it's meant to be a plain NATS/JSON consumer,
// same as any other language could be.

// FunctionCall mirrors genai.FunctionCall.
type FunctionCall struct {
	ID   string         `json:"id,omitempty"`
	Name string         `json:"name,omitempty"`
	Args map[string]any `json:"args,omitempty"`
}

// FunctionResponse mirrors genai.FunctionResponse.
type FunctionResponse struct {
	ID       string         `json:"id,omitempty"`
	Name     string         `json:"name,omitempty"`
	Response map[string]any `json:"response,omitempty"`
}

// Part mirrors genai.Part -- exactly one of these fields is normally set.
type Part struct {
	Text             string            `json:"text,omitempty"`
	FunctionCall     *FunctionCall     `json:"functionCall,omitempty"`
	FunctionResponse *FunctionResponse `json:"functionResponse,omitempty"`
}

// Content mirrors genai.Content -- a single message: who said it (Role)
// and what it contains (Parts).
type Content struct {
	Role  string `json:"role,omitempty"`
	Parts []Part `json:"parts,omitempty"`
}

// Event mirrors one event in a session's history, as returned embedded in
// a Session or as an element of a run's response array. Only the fields
// Botson-TUI actually renders are kept.
type Event struct {
	ID      string   `json:"id"`
	Author  string   `json:"author"`
	Content *Content `json:"content"`
}

// Session mirrors the full session object returned by create/get.
type Session struct {
	ID             string         `json:"id"`
	AppName        string         `json:"appName"`
	UserID         string         `json:"userId"`
	LastUpdateTime int64          `json:"lastUpdateTime"`
	Events         []Event        `json:"events"`
	State          map[string]any `json:"state"`
}

// RunRequest is the /api/run request body.
type RunRequest struct {
	AppName    string  `json:"appName"`
	UserID     string  `json:"userId"`
	SessionID  string  `json:"sessionId"`
	NewMessage Content `json:"newMessage"`

	// StateDelta is upstream ADK's own mechanism for merging values into a
	// session's state as part of a turn -- used here to set "botson:cwd"
	// (see docs/nats-api.md in Botson-ADKv2) on a freshly created
	// session's first turn. nil on every other turn.
	StateDelta map[string]any `json:"stateDelta,omitempty"`
}

// SessionStat mirrors Botson-ADKv2's internal/management.SessionStat, the
// dashboard-shaped summary botson.sessions.list returns.
type SessionStat struct {
	ID             string `json:"id"`
	AgentName      string `json:"agentName"`
	UserID         string `json:"userId"`
	DisplayName    string `json:"displayName"`
	LastUpdateTime int64  `json:"lastUpdateTime"`
	EventCount     int    `json:"eventCount"`
}

// LastUpdated converts LastUpdateTime (unix millis, per ADK's own
// convention) to a time.Time for display.
func (s SessionStat) LastUpdated() time.Time {
	return time.UnixMilli(s.LastUpdateTime)
}

const (
	// ConfirmationFunctionName is the synthetic functionCall name ADK
	// wraps a gated tool call in when it requires human approval.
	ConfirmationFunctionName = "adk_request_confirmation"
)

// ConfirmationArgs is the shape of a adk_request_confirmation functionCall's
// Args map, decoded on demand (Args is untyped JSON on the wire).
type ConfirmationArgs struct {
	OriginalFunctionCall *FunctionCall `json:"originalFunctionCall"`
	ToolConfirmation     struct {
		Hint string `json:"hint"`
	} `json:"toolConfirmation"`
}
