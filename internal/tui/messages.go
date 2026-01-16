package tui

// ProgressMsg represents a progress update from module operations
type ProgressMsg struct {
	Phase   string
	Message string
}

// OutputMsg represents output from go install or build commands
type OutputMsg struct {
	Stream string // "stdout" or "stderr"
	Line   string
}

// StatusMsg updates the status bar text
type StatusMsg struct {
	Text string
}

// DoneMsg signals that the operation has completed
type DoneMsg struct {
	Success bool
	Error   error
}
