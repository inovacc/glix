package tui

import "github.com/charmbracelet/lipgloss"

var (
	// PhaseStyle is used for the current phase label (bold cyan)
	PhaseStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("86"))

	// MessageStyle is used for progress messages (light gray)
	MessageStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	// ErrorStyle is used for error messages (red)
	ErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	// SuccessStyle is used for success messages (green)
	SuccessStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("46"))

	// WarningStyle is used for warning messages (yellow)
	WarningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("226"))

	// StatusStyle is used for the status bar (dark background)
	StatusStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("236")).
			Foreground(lipgloss.Color("252")).
			Padding(0, 1)

	// LogStyle is used for log lines (dim)
	LogStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245"))

	// StderrStyle is used for stderr output (orange/yellow)
	StderrStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214"))
)
