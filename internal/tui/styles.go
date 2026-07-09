package tui

import "github.com/charmbracelet/lipgloss"

var (
	accent = lipgloss.AdaptiveColor{Light: "#6633CC", Dark: "#B39DFF"}
	subtle = lipgloss.AdaptiveColor{Light: "#888888", Dark: "#666666"}
	warn   = lipgloss.AdaptiveColor{Light: "#B36B00", Dark: "#E5C07B"}
	bad    = lipgloss.AdaptiveColor{Light: "#B00020", Dark: "#E06C75"}

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(accent).
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(subtle).
			PaddingBottom(0)

	userStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#005FAF", Dark: "#7FB4FF"})
	assistantStyle = lipgloss.NewStyle().Bold(true).Foreground(accent)
	dimStyle       = lipgloss.NewStyle().Foreground(subtle).Italic(true)
	errorStyle     = lipgloss.NewStyle().Bold(true).Foreground(bad)
	confirmStyle   = lipgloss.NewStyle().Bold(true).Foreground(warn)
)
