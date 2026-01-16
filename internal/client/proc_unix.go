//go:build !windows

package client

import (
	"os/exec"
	"syscall"
)

// setProcAttr sets Unix-specific process attributes for detachment
func setProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
		Pgid:    0,
	}
}
