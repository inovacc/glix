//go:build windows

package client

import (
	"os/exec"
	"syscall"
)

// setProcAttr sets Windows-specific process attributes for detachment
func setProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}
