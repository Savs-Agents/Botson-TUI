// Package natsapi is Botson-TUI's connection to a running botson core:
// the standard adk.* surface (via NATS-ADK-Proxy's client package) plus
// the one botson.* subject this app needs (botson.sessions.list).
package natsapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/nats-io/nats.go"

	adkclient "github.com/Savs-Agents/NATS-ADK-Proxy/client"
)

// Client is a live connection to one botson core.
type Client struct {
	nc  *nats.Conn
	adk *adkclient.Client
}

// Connect dials the core's embedded NATS server at host:port. token
// authenticates the connection -- required, since the core's embedded
// NATS server rejects unauthenticated connections (see LocalToken for
// zero-config pairing with a core on this same machine).
func Connect(host string, port int, token string) (*Client, error) {
	natsURL := fmt.Sprintf("nats://%s:%d", host, port)
	nc, err := nats.Connect(natsURL, nats.Timeout(5*time.Second), nats.Token(token))
	if err != nil {
		return nil, fmt.Errorf("connect to %s: %w", natsURL, err)
	}
	return &Client{
		nc: nc,
		// Must stay above the core's own gateway RequestTimeout (see
		// Botson-ADKv2's cmd/botson-core/cmd_core.go, currently
		// procutil.DefaultTimeout + 90s = 3m30s) or this client gives up
		// before the core's own request-to-backend deadline would have,
		// masking the real error with a generic NATS timeout instead.
		adk: adkclient.New(nc, adkclient.WithTimeout(4*time.Minute)),
	}, nil
}

// Close releases the underlying NATS connection.
func (c *Client) Close() {
	c.nc.Close()
}

// errEnvelope is the shape every botson.* failure reply takes.
type errEnvelope struct {
	Error string `json:"error"`
}

// ListApps returns the available agent names.
func (c *Client) ListApps(ctx context.Context) ([]string, error) {
	resp, err := c.adk.Do(ctx, http.MethodGet, "/api/list-apps", nil, nil)
	if err != nil {
		return nil, err
	}
	if err := restError(resp); err != nil {
		return nil, err
	}
	var apps []string
	if err := json.Unmarshal(resp.Body, &apps); err != nil {
		return nil, fmt.Errorf("decode list-apps response: %w", err)
	}
	return apps, nil
}

func sessionPath(app, user, sessionID string) string {
	return fmt.Sprintf("/api/apps/%s/users/%s/sessions/%s",
		url.PathEscape(app), url.PathEscape(user), url.PathEscape(sessionID))
}

// CreateSession creates a new, empty session under (app, user, sessionID).
func (c *Client) CreateSession(ctx context.Context, app, user, sessionID string) (*Session, error) {
	resp, err := c.adk.Do(ctx, http.MethodPost, sessionPath(app, user, sessionID), nil, nil)
	if err != nil {
		return nil, err
	}
	if err := restError(resp); err != nil {
		return nil, err
	}
	var sess Session
	if err := json.Unmarshal(resp.Body, &sess); err != nil {
		return nil, fmt.Errorf("decode create-session response: %w", err)
	}
	return &sess, nil
}

// GetSession fetches a session's full state and event history.
func (c *Client) GetSession(ctx context.Context, app, user, sessionID string) (*Session, error) {
	resp, err := c.adk.Do(ctx, http.MethodGet, sessionPath(app, user, sessionID), nil, nil)
	if err != nil {
		return nil, err
	}
	if err := restError(resp); err != nil {
		return nil, err
	}
	var sess Session
	if err := json.Unmarshal(resp.Body, &sess); err != nil {
		return nil, fmt.Errorf("decode get-session response: %w", err)
	}
	return &sess, nil
}

// RunTurn sends one turn (newMessage) and returns the events it produced.
// stateDelta is nil on every turn except optionally the first one of a
// freshly created session (see docs/nats-api.md's "setting a session's
// working directory").
func (c *Client) RunTurn(ctx context.Context, app, user, sessionID string, newMessage Content, stateDelta map[string]any) ([]Event, error) {
	body, err := json.Marshal(RunRequest{
		AppName:    app,
		UserID:     user,
		SessionID:  sessionID,
		NewMessage: newMessage,
		StateDelta: stateDelta,
	})
	if err != nil {
		return nil, err
	}
	header := http.Header{"Content-Type": []string{"application/json"}}
	resp, err := c.adk.Do(ctx, http.MethodPost, "/api/run", header, body)
	if err != nil {
		return nil, err
	}
	if err := restError(resp); err != nil {
		return nil, err
	}
	var events []Event
	if err := json.Unmarshal(resp.Body, &events); err != nil {
		return nil, fmt.Errorf("decode run response: %w", err)
	}
	return events, nil
}

// SessionsList calls botson.sessions.list, filtered by agent and/or user
// (either may be empty to mean "all").
func (c *Client) SessionsList(ctx context.Context, agent, user string) ([]SessionStat, error) {
	req, err := json.Marshal(struct {
		Agent string `json:"agent,omitempty"`
		User  string `json:"user,omitempty"`
	}{Agent: agent, User: user})
	if err != nil {
		return nil, err
	}

	msg, err := c.nc.RequestWithContext(ctx, "botson.sessions.list", req)
	if err != nil {
		return nil, fmt.Errorf("botson.sessions.list: %w", err)
	}

	var env errEnvelope
	if err := json.Unmarshal(msg.Data, &env); err == nil && env.Error != "" {
		return nil, fmt.Errorf("botson.sessions.list: %s", env.Error)
	}

	var stats []SessionStat
	if err := json.Unmarshal(msg.Data, &stats); err != nil {
		return nil, fmt.Errorf("decode botson.sessions.list response: %w", err)
	}
	return stats, nil
}

// restError reports a non-2xx REST response as an error.
func restError(resp *adkclient.Response) error {
	if resp.Status >= 200 && resp.Status < 300 {
		return nil
	}
	return fmt.Errorf("request failed: status %d: %s", resp.Status, string(resp.Body))
}
