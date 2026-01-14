//go:build darwin

package service

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
)

const launchdPlistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>{{.Label}}</string>
    <key>ProgramArguments</key>
    <array>
{{- range .Args}}
        <string>{{.}}</string>
{{- end}}
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>{{.LogPath}}/stdout.log</string>
    <key>StandardErrorPath</key>
    <string>{{.LogPath}}/stderr.log</string>
    <key>WorkingDirectory</key>
    <string>{{.WorkDir}}</string>
</dict>
</plist>
`

type darwinManager struct {
	plistPath string
	label     string
}

func newPlatformManager() (Manager, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	label := fmt.Sprintf("com.glix.%s", ServiceName)
	plistPath := filepath.Join(home, "Library", "LaunchAgents", fmt.Sprintf("%s.plist", label))

	return &darwinManager{
		plistPath: plistPath,
		label:     label,
	}, nil
}

func (m *darwinManager) Install(ctx context.Context, cfg Config) error {
	exePath, err := GetExecutablePath()
	if err != nil {
		return err
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	// Build program arguments
	args := append([]string{exePath}, BuildServiceArgs(cfg)...)

	// Log directory
	logPath := filepath.Join(home, "Library", "Logs", ServiceName)
	if err := os.MkdirAll(logPath, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	// Ensure LaunchAgents directory exists
	dir := filepath.Dir(m.plistPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create LaunchAgents directory: %w", err)
	}

	// Create plist file
	tmpl, err := template.New("plist").Parse(launchdPlistTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse plist template: %w", err)
	}

	f, err := os.Create(m.plistPath)
	if err != nil {
		return fmt.Errorf("failed to create plist file: %w", err)
	}
	defer func() {
		_ = f.Close()
	}()

	data := struct {
		Label   string
		Args    []string
		LogPath string
		WorkDir string
	}{
		Label:   m.label,
		Args:    args,
		LogPath: logPath,
		WorkDir: home,
	}

	if err := tmpl.Execute(f, data); err != nil {
		return fmt.Errorf("failed to write plist file: %w", err)
	}

	return nil
}

func (m *darwinManager) Uninstall(ctx context.Context) error {
	// Unload if loaded
	_ = m.Stop(ctx)

	// Remove plist file
	if err := os.Remove(m.plistPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove plist file: %w", err)
	}

	return nil
}

func (m *darwinManager) Start(ctx context.Context) error {
	cmd := exec.Command("launchctl", "load", m.plistPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to load service: %w: %s", err, string(output))
	}
	return nil
}

func (m *darwinManager) Stop(ctx context.Context) error {
	cmd := exec.Command("launchctl", "unload", m.plistPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to unload service: %w: %s", err, string(output))
	}
	return nil
}

func (m *darwinManager) Status(ctx context.Context) (*Status, error) {
	if !m.IsInstalled() {
		return &Status{
			Running:     false,
			Description: "Service not installed",
		}, nil
	}

	// Check if service is running
	cmd := exec.Command("launchctl", "list", m.label)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return &Status{
			Running:     false,
			Description: "Stopped",
		}, nil
	}

	// Parse output to get PID
	// Format: PID	Status	Label
	lines := strings.Split(string(output), "\n")
	var pid int
	running := false
	desc := "Unknown"

	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 3 && fields[2] == m.label {
			if fields[0] != "-" {
				pid, _ = strconv.Atoi(fields[0])
				running = pid > 0
			}
			if running {
				desc = "Running"
			} else {
				desc = "Stopped"
			}
			break
		}
	}

	// Alternative check using launchctl print
	if !running {
		cmd = exec.Command("launchctl", "print", fmt.Sprintf("gui/%d/%s", os.Getuid(), m.label))
		if err := cmd.Run(); err == nil {
			running = true
			desc = "Running"
		}
	}

	return &Status{
		Running:     running,
		PID:         pid,
		Description: desc,
	}, nil
}

func (m *darwinManager) IsInstalled() bool {
	_, err := os.Stat(m.plistPath)
	return err == nil
}
