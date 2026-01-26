package tui

import (
	"context"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
)

// TUI manages the terminal user interface for glix operations
type TUI struct {
	program *tea.Program
	model   Model
	mu      sync.Mutex
	running bool
	done    chan struct{}
}

// New creates a new TUI instance
func New() *TUI {
	return &TUI{
		model: NewModel(),
		done:  make(chan struct{}),
	}
}

// Start initializes and runs the TUI in the current goroutine
// This function blocks until the TUI exits
func (t *TUI) Start(ctx context.Context) error {
	t.mu.Lock()

	if t.running {
		t.mu.Unlock()
		return nil
	}

	t.running = true
	t.mu.Unlock()

	t.program = tea.NewProgram(t.model)

	// Handle context cancellation
	go func() {
		select {
		case <-ctx.Done():
			t.Stop()
		case <-t.done:
		}
	}()

	_, err := t.program.Run()

	close(t.done)

	return err
}

// Stop gracefully stops the TUI
func (t *TUI) Stop() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.program != nil && t.running {
		t.program.Send(DoneMsg{Success: true})
		t.running = false
	}
}

// SendProgress sends a progress update to the TUI
func (t *TUI) SendProgress(phase, message string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.program != nil && t.running {
		t.program.Send(ProgressMsg{Phase: phase, Message: message})
	}
}

// SendOutput sends an output line to the TUI
func (t *TUI) SendOutput(stream, line string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.program != nil && t.running {
		t.program.Send(OutputMsg{Stream: stream, Line: line})
	}
}

// SetStatus updates the status bar text
func (t *TUI) SetStatus(text string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.program != nil && t.running {
		t.program.Send(StatusMsg{Text: text})
	}
}

// Done signals that the operation has completed
func (t *TUI) Done(err error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.program != nil && t.running {
		t.program.Send(DoneMsg{Success: err == nil, Error: err})
		t.running = false
	}
}

// ProgressHandler returns a handler function compatible with module.ProgressHandler
func (t *TUI) ProgressHandler() func(phase, message string) {
	return func(phase, message string) {
		t.SendProgress(phase, message)
	}
}

// OutputHandler returns a handler function compatible with module output streaming
func (t *TUI) OutputHandler() func(stream, line string) {
	return func(stream, line string) {
		t.SendOutput(stream, line)
	}
}

// Wait blocks until the TUI has finished
func (t *TUI) Wait() {
	<-t.done
}
