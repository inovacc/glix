//go:build windows

package service

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

type windowsManager struct{}

func newPlatformManager() (Manager, error) {
	return &windowsManager{}, nil
}

func (m *windowsManager) Install(ctx context.Context, cfg Config) error {
	_ = ctx // unused

	exePath, err := GetExecutablePath()
	if err != nil {
		return err
	}

	scm, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("failed to connect to service manager: %w", err)
	}

	defer func() {
		_ = scm.Disconnect()
	}()

	// Check if a service already exists
	s, err := scm.OpenService(ServiceName)
	if err == nil {
		_ = s.Close()
		return fmt.Errorf("service %s already exists", ServiceName)
	}

	// Build the service command line
	args := BuildServiceArgs(cfg)
	binPath := fmt.Sprintf("%s %s", exePath, strings.Join(args, " "))

	// Create the service
	s, err = scm.CreateService(
		ServiceName,
		exePath,
		mgr.Config{
			DisplayName:  ServiceDisplayName,
			Description:  ServiceDescription,
			StartType:    mgr.StartAutomatic,
			ErrorControl: mgr.ErrorNormal,
		},
		args...,
	)
	if err != nil {
		return fmt.Errorf("failed to create service: %w (binPath: %s)", err, binPath)
	}

	defer func() {
		_ = s.Close()
	}()

	// Set recovery actions
	recoveryActions := []mgr.RecoveryAction{
		{Type: mgr.ServiceRestart, Delay: 5 * time.Second},
		{Type: mgr.ServiceRestart, Delay: 30 * time.Second},
		{Type: mgr.ServiceRestart, Delay: 60 * time.Second},
	}
	if err := s.SetRecoveryActions(recoveryActions, 86400); err != nil {
		// Non-fatal, just log
		_, _ = fmt.Fprintf(os.Stderr, "warning: failed to set recovery actions: %v\n", err)
	}

	return nil
}

func (m *windowsManager) Uninstall(ctx context.Context) error {
	_ = ctx // unused

	scm, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("failed to connect to service manager: %w", err)
	}

	defer func() {
		_ = scm.Disconnect()
	}()

	s, err := scm.OpenService(ServiceName)
	if err != nil {
		return fmt.Errorf("service %s not found: %w", ServiceName, err)
	}

	defer func() {
		_ = s.Close()
	}()

	// Stop the service if running
	status, err := s.Query()
	if err == nil && status.State != svc.Stopped {
		_, _ = s.Control(svc.Stop)
		// Wait for stop
		for range 30 {
			status, err = s.Query()
			if err != nil || status.State == svc.Stopped {
				break
			}

			time.Sleep(time.Second)
		}
	}

	// Delete the service
	if err := s.Delete(); err != nil {
		return fmt.Errorf("failed to delete service: %w", err)
	}

	return nil
}

func (m *windowsManager) Start(ctx context.Context) error {
	_ = ctx // unused

	scm, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("failed to connect to service manager: %w", err)
	}

	defer func() {
		_ = scm.Disconnect()
	}()

	s, err := scm.OpenService(ServiceName)
	if err != nil {
		return fmt.Errorf("service %s not found: %w", ServiceName, err)
	}

	defer func() {
		_ = s.Close()
	}()

	if err := s.Start(); err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}

	// Wait for service to start
	for range 30 {
		status, err := s.Query()
		if err != nil {
			return fmt.Errorf("failed to query service status: %w", err)
		}

		if status.State == svc.Running {
			return nil
		}

		if status.State == svc.Stopped {
			return fmt.Errorf("service stopped unexpectedly")
		}

		time.Sleep(time.Second)
	}

	return fmt.Errorf("timeout waiting for service to start")
}

func (m *windowsManager) Stop(ctx context.Context) error {
	_ = ctx // unused

	scm, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("failed to connect to service manager: %w", err)
	}

	defer func() {
		_ = scm.Disconnect()
	}()

	s, err := scm.OpenService(ServiceName)
	if err != nil {
		return fmt.Errorf("service %s not found: %w", ServiceName, err)
	}

	defer func() {
		_ = s.Close()
	}()

	status, err := s.Query()
	if err != nil {
		return fmt.Errorf("failed to query service status: %w", err)
	}

	if status.State == svc.Stopped {
		return nil // Already stopped
	}

	_, err = s.Control(svc.Stop)
	if err != nil {
		return fmt.Errorf("failed to stop service: %w", err)
	}

	// Wait for service to stop
	for range 30 {
		status, err := s.Query()
		if err != nil {
			return fmt.Errorf("failed to query service status: %w", err)
		}

		if status.State == svc.Stopped {
			return nil
		}

		time.Sleep(time.Second)
	}

	return fmt.Errorf("timeout waiting for service to stop")
}

func (m *windowsManager) Status(ctx context.Context) (*Status, error) {
	_ = ctx // unused

	scm, err := mgr.Connect()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to service manager: %w", err)
	}

	defer func() {
		_ = scm.Disconnect()
	}()

	s, err := scm.OpenService(ServiceName)
	if err != nil {
		return &Status{
			Running:     false,
			Description: "Service not installed",
		}, nil
	}

	defer func() {
		_ = s.Close()
	}()

	status, err := s.Query()
	if err != nil {
		return nil, fmt.Errorf("failed to query service status: %w", err)
	}

	var desc string

	switch status.State {
	case svc.Stopped:
		desc = "Stopped"
	case svc.StartPending:
		desc = "Starting"
	case svc.StopPending:
		desc = "Stopping"
	case svc.Running:
		desc = "Running"
	case svc.ContinuePending:
		desc = "Continuing"
	case svc.PausePending:
		desc = "Pausing"
	case svc.Paused:
		desc = "Paused"
	default:
		desc = "Unknown"
	}

	return &Status{
		Running:     status.State == svc.Running,
		PID:         int(status.ProcessId),
		Description: desc,
	}, nil
}

func (m *windowsManager) IsInstalled() bool {
	scm, err := mgr.Connect()
	if err != nil {
		return false
	}

	defer func() {
		_ = scm.Disconnect()
	}()

	s, err := scm.OpenService(ServiceName)
	if err != nil {
		return false
	}

	_ = s.Close()

	return true
}
