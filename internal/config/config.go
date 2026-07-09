// Package config persists Botson-TUI's own local settings -- connection
// details and a self-chosen user identity -- separate from anything the
// core it connects to knows about.
package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// Config is Botson-TUI's local, on-disk state. It has nothing to do with
// the core's own ~/.botson config -- this only remembers how to reach a
// core and which user identity to present to it.
type Config struct {
	Host      string `json:"host"`
	Port      int    `json:"port"`
	UserID    string `json:"userId"`
	LastAgent string `json:"lastAgent,omitempty"`
}

func defaultConfig() Config {
	return Config{
		Host: "127.0.0.1",
		Port: 4222,
		// No colon here (unlike "tui:<id>") -- Botson-ADKv2's local
		// artifact store joins the raw user ID into a filesystem path
		// (internal/artifact/local.go), and ":" isn't a legal path
		// character on Windows. A colon-containing user ID (e.g. the
		// "web:<account-id>" pattern the core's own docs suggest) makes
		// every turn fail with "failed to list artifacts" once a Windows
		// core is involved. Using "-" here sidesteps it for our default;
		// worth fixing upstream in Botson-ADKv2 too.
		UserID: "tui-" + randHex(4),
	}
}

func randHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "anon"
	}
	return hex.EncodeToString(b)
}

// Path returns the on-disk location of the config file, creating its
// parent directory if necessary.
func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir = filepath.Join(dir, "botson-tui")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// Load reads the config file, returning a fresh default Config (with a
// newly generated UserID) if none exists yet.
func Load() (Config, error) {
	path, err := Path()
	if err != nil {
		return Config{}, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return defaultConfig(), nil
	}
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Save writes the config file.
func Save(cfg Config) error {
	path, err := Path()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
