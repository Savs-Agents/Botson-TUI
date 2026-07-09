package natsapi

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// LocalToken best-effort reads a Botson-ADKv2 core's own
// ~/.botson/config.json off this same machine and returns its
// nats_auth_token, for zero-configuration pairing with a local core. Not
// a shared type with the core (separate Go module) -- just the one field
// this needs. Returns ("", false) on any error: no file, wrong machine,
// unreadable, etc. -- callers should fall back to asking the user.
func LocalToken() (string, bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", false
	}

	data, err := os.ReadFile(filepath.Join(home, ".botson", "config.json"))
	if err != nil {
		return "", false
	}

	var cfg struct {
		NatsAuthToken string `json:"nats_auth_token"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return "", false
	}
	if cfg.NatsAuthToken == "" {
		return "", false
	}
	return cfg.NatsAuthToken, true
}
