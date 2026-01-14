//go:build linux

package service

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
)

const systemdUnitTemplate = `[Unit]
Description={{.Description}}
After=network.target

[Service]
Type=simple
ExecStart={{.ExecStart}}
Restart=always
RestartSec=5
User={{.User}}
Environment=HOME={{.Home}}

[Install]
WantedBy=multi-user.target
`

type linuxManager struct {
	unitPath string
}

func newPlatformManager() (Manager, error) {
	// Determine if we should use a system or user unit
	unitPath := fmt.Sprintf("/etc/systemd/system/%s.service", ServiceName)
	if os.Getuid() != 0 {
		// Non-root user, use user unit
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		unitPath = filepath.Join(home, ".config", "systemd", "user", fmt.Sprintf("%s.service", ServiceName))
	}

	return &linuxManager{unitPath: unitPath}, nil
}

func (m *linuxManager) isUserUnit() bool {
	return strings.Contains(m.unitPath, ".config/systemd/user")
}

func (m *linuxManager) systemctl(args ...string) *exec.Cmd {
	if m.isUserUnit() {
		return exec.Command("systemctl", append([]string{"--user"}, args...)...)
	}
	return exec.Command("systemctl", args...)
}

func (m *linuxManager) Install(ctx context.Context, cfg Config) error {
	exePath, err := GetExecutablePath()
	if err != nil {
		return err
	}

	// Build exec start command
	args := BuildServiceArgs(cfg)
	execStart := fmt.Sprintf("%s %s", exePath, strings.Join(args, " "))

	// Get current user info
	user := os.Getenv("USER")
	if user == "" {
		user = "root"
	}
	home := os.Getenv("HOME")
	if home == "" {
		home = "/root"
	}

	// Ensure directory exists
	dir := filepath.Dir(m.unitPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Create unit file
	tmpl, err := template.New("unit").Parse(systemdUnitTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse unit template: %w", err)
	}

	f, err := os.Create(m.unitPath)
	if err != nil {
		return fmt.Errorf("failed to create unit file: %w", err)
	}
	defer func() {
		_ = f.Close()
	}()

	data := struct {
		Description string
		ExecStart   string
		User        string
		Home        string
	}{
		Description: ServiceDescription,
		ExecStart:   execStart,
		User:        user,
		Home:        home,
	}

	if err := tmpl.Execute(f, data); err != nil {
		return fmt.Errorf("failed to write unit file: %w", err)
	}

	// Reload systemd
	cmd := m.systemctl("daemon-reload")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to reload systemd: %w: %s", err, string(output))
	}

	// Enable service
	cmd = m.systemctl("enable", ServiceName)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to enable service: %w: %s", err, string(output))
	}

	return nil
}

func (m *linuxManager) Uninstall(ctx context.Context) error {
	// Stop service if running
	_ = m.Stop(ctx)

	// Disable service
	cmd := m.systemctl("disable", ServiceName)
	_, _ = cmd.CombinedOutput() // Ignore errors

	// Remove unit file
	if err := os.Remove(m.unitPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove unit file: %w", err)
	}

	// Reload systemd
	cmd = m.systemctl("daemon-reload")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to reload systemd: %w: %s", err, string(output))
	}

	return nil
}

func (m *linuxManager) Start(ctx context.Context) error {
	cmd := m.systemctl("start", ServiceName)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to start service: %w: %s", err, string(output))
	}
	return nil
}

func (m *linuxManager) Stop(ctx context.Context) error {
	cmd := m.systemctl("stop", ServiceName)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to stop service: %w: %s", err, string(output))
	}
	return nil
}

func (m *linuxManager) Status(ctx context.Context) (*Status, error) {
	if !m.IsInstalled() {
		return &Status{
			Running:     false,
			Description: "Service not installed",
		}, nil
	}

	cmd := m.systemctl("is-active", ServiceName)
	output, _ := cmd.CombinedOutput()
	isActive := strings.TrimSpace(string(output)) == "active"

	// Get PID if running
	var pid int
	if isActive {
		cmd = m.systemctl("show", "-p", "MainPID", "--value", ServiceName)
		output, err := cmd.CombinedOutput()
		if err == nil {
			_, _ = fmt.Sscanf(strings.TrimSpace(string(output)), "%d", &pid)
		}
	}

	// Get status description
	cmd = m.systemctl("is-active", ServiceName)
	output, _ = cmd.CombinedOutput()
	desc := strings.TrimSpace(string(output))

	return &Status{
		Running:     isActive,
		PID:         pid,
		Description: desc,
	}, nil
}

func (m *linuxManager) IsInstalled() bool {
	_, err := os.Stat(m.unitPath)
	return err == nil
}
