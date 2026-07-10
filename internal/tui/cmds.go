package tui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"

	"github.com/Savs-Agents/Botson-TUI/internal/natsapi"
)

// Messages carrying the results of async NATS calls back into the
// bubbletea update loop.
type (
	connectedMsg      struct{ client *natsapi.Client }
	connectErrMsg     struct{ err error }
	appsLoadedMsg     struct{ apps []string }
	appsErrMsg        struct{ err error }
	sessionsLoadedMsg struct{ stats []natsapi.SessionStat }
	sessionsErrMsg    struct{ err error }
	sessionReadyMsg   struct{ sess *natsapi.Session }
	sessionErrMsg     struct{ err error }
	turnDoneMsg       struct{ events []natsapi.Event }
	turnErrMsg        struct{ err error }
	autoModeErrMsg    struct{ err error }
)

func connectCmd(host string, port int, token string) tea.Cmd {
	return func() tea.Msg {
		c, err := natsapi.Connect(host, port, token)
		if err != nil {
			return connectErrMsg{err}
		}
		return connectedMsg{c}
	}
}

func listAppsCmd(c *natsapi.Client) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		apps, err := c.ListApps(ctx)
		if err != nil {
			return appsErrMsg{err}
		}
		return appsLoadedMsg{apps}
	}
}

func sessionsListCmd(c *natsapi.Client, agent, user string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		stats, err := c.SessionsList(ctx, agent, user)
		if err != nil {
			return sessionsErrMsg{err}
		}
		return sessionsLoadedMsg{stats}
	}
}

func createSessionCmd(c *natsapi.Client, agent, user string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		sess, err := c.CreateSession(ctx, agent, user, uuid.NewString())
		if err != nil {
			return sessionErrMsg{err}
		}
		return sessionReadyMsg{sess}
	}
}

func getSessionCmd(c *natsapi.Client, agent, user, sessionID string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		sess, err := c.GetSession(ctx, agent, user, sessionID)
		if err != nil {
			return sessionErrMsg{err}
		}
		return sessionReadyMsg{sess}
	}
}

func setAutoModeCmd(c *natsapi.Client, app, user, sessionID string, enabled bool) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := c.SetSessionAutoMode(ctx, app, user, sessionID, enabled); err != nil {
			return autoModeErrMsg{err}
		}
		return nil
	}
}

func runTurnCmd(c *natsapi.Client, app, user, sessionID string, newMessage natsapi.Content, stateDelta map[string]any) tea.Cmd {
	return func() tea.Msg {
		// Must stay above the core's own gateway RequestTimeout (see
		// Botson-ADKv2's cmd/botson-core/cmd_core.go, currently 8m) or
		// this deadline fires first, cutting off a real multi-tool-call
		// turn before the core's own timeout would have -- masking the
		// real error with a generic context-deadline one instead.
		ctx, cancel := context.WithTimeout(context.Background(), 9*time.Minute)
		defer cancel()
		events, err := c.RunTurn(ctx, app, user, sessionID, newMessage, stateDelta)
		if err != nil {
			return turnErrMsg{err}
		}
		return turnDoneMsg{events}
	}
}
