// Command botson-tui is a standalone terminal chat client for a running
// `botson core`, talking to it purely over NATS -- see
// https://github.com/xSaVageAU/Botson-ADKv2/blob/core-rebuild/docs/nats-api.md
// for the wire contract this app is built against.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log"

	"github.com/Savs-Agents/Botson-TUI/internal/config"
	"github.com/Savs-Agents/Botson-TUI/internal/tui"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "botson-tui:", err)
		os.Exit(1)
	}
}

func run() error {
	logFile, err := setupLogging()
	if err != nil {
		return fmt.Errorf("set up logging: %w", err)
	}
	defer logFile.Close()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	p := tea.NewProgram(tui.New(cfg), tea.WithAltScreen())
	_, err = p.Run()
	return err
}

// setupLogging points charmbracelet/log at a file instead of stdout/stderr
// -- bubbletea owns the terminal while the program runs, so nothing else
// may write there.
func setupLogging() (*os.File, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	dir = filepath.Join(dir, "botson-tui")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	f, err := os.OpenFile(filepath.Join(dir, "tui.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	log.SetOutput(f)
	log.SetReportTimestamp(true)
	return f, nil
}
