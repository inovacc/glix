package exec

import (
	"context"
	"fmt"
	"os/exec"
)

var debug bool

type ExitError = exec.ExitError

func SetCommandDebug(v bool) {
	debug = v
}

// Command
func Command(name string, arg ...string) *exec.Cmd {
	if debug {
		fmt.Printf("Executing: %s > Args: %v\n", name, arg)
	}

	return exec.Command(name, arg...)
}

// CommandContext
func CommandContext(ctx context.Context, name string, arg ...string) *exec.Cmd {
	if debug {
		fmt.Printf("Executing: %s > Args: %v\n", name, arg)
	}

	return exec.CommandContext(ctx, name, arg...)
}
