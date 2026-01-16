package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

const (
	defaultMaxLogs = 15
)

// Model represents the Bubble Tea model for the TUI
type Model struct {
	spinner spinner.Model
	phase   string
	message string
	logs    []logEntry
	maxLogs int
	status  string
	done    bool
	err     error
	width   int
	height  int
}

type logEntry struct {
	text     string
	isStderr bool
}

// NewModel creates a new TUI model
func NewModel() Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = PhaseStyle

	return Model{
		spinner: s,
		maxLogs: defaultMaxLogs,
		logs:    make([]logEntry, 0),
		status:  "Initializing...",
	}
}

// Init implements tea.Model
func (m Model) Init() tea.Cmd {
	return m.spinner.Tick
}

// Update implements tea.Model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case ProgressMsg:
		m.phase = msg.Phase
		m.message = msg.Message
		m.addLog(fmt.Sprintf("[%s] %s", msg.Phase, msg.Message), false)

	case OutputMsg:
		m.addLog(msg.Line, msg.Stream == "stderr")

	case StatusMsg:
		m.status = msg.Text

	case DoneMsg:
		m.done = true
		m.err = msg.Error
		if msg.Error != nil {
			m.status = fmt.Sprintf("Error: %v", msg.Error)
		} else {
			m.status = "Completed successfully"
		}
		return m, tea.Quit

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

// addLog adds a log entry and maintains the max log size
func (m *Model) addLog(text string, isStderr bool) {
	m.logs = append(m.logs, logEntry{text: text, isStderr: isStderr})
	if len(m.logs) > m.maxLogs {
		m.logs = m.logs[1:]
	}
}

// View implements tea.Model
func (m Model) View() string {
	var b strings.Builder

	// Header with spinner (or done indicator) and current phase
	if !m.done {
		b.WriteString(m.spinner.View())
		b.WriteString(" ")
	} else if m.err != nil {
		b.WriteString(ErrorStyle.Render("x"))
		b.WriteString(" ")
	} else {
		b.WriteString(SuccessStyle.Render("*"))
		b.WriteString(" ")
	}

	if m.phase != "" {
		b.WriteString(PhaseStyle.Render(m.phase))
		b.WriteString(" ")
	}
	b.WriteString(MessageStyle.Render(m.message))
	b.WriteString("\n\n")

	// Log view
	for _, entry := range m.logs {
		b.WriteString("  ")
		if entry.isStderr {
			b.WriteString(StderrStyle.Render(entry.text))
		} else {
			b.WriteString(LogStyle.Render(entry.text))
		}
		b.WriteString("\n")
	}

	// Padding to push status bar to consistent position
	logLines := len(m.logs)
	if logLines < m.maxLogs {
		for i := 0; i < m.maxLogs-logLines; i++ {
			b.WriteString("\n")
		}
	}

	// Status bar
	b.WriteString("\n")
	if m.err != nil {
		b.WriteString(ErrorStyle.Render(m.status))
	} else if m.done {
		b.WriteString(SuccessStyle.Render(m.status))
	} else {
		b.WriteString(StatusStyle.Render(m.status))
	}
	b.WriteString("\n")

	return b.String()
}
