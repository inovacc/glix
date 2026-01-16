package client

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/inovacc/glix/internal/server"
)

// DefaultIdleTimeout is the default time the on-demand server stays alive after last activity
const DefaultIdleTimeout = 5 * time.Minute

// DiscoveryConfig holds configuration for server discovery
type DiscoveryConfig struct {
	Address         string
	Port            int
	IdleTimeout     time.Duration
	StartTimeout    time.Duration
	ConnectionRetry int
	RetryDelay      time.Duration
	Logger          *slog.Logger
}

// DefaultDiscoveryConfig returns the default discovery configuration
func DefaultDiscoveryConfig() DiscoveryConfig {
	return DiscoveryConfig{
		Address:         "localhost",
		Port:            server.DefaultPort,
		IdleTimeout:     DefaultIdleTimeout,
		StartTimeout:    30 * time.Second,
		ConnectionRetry: 10,
		RetryDelay:      500 * time.Millisecond,
		Logger:          slog.Default(),
	}
}

// GetClient returns a connected client, starting an on-demand server if needed
func GetClient(ctx context.Context, cfg DiscoveryConfig) (*Client, error) {
	address := fmt.Sprintf("%s:%d", cfg.Address, cfg.Port)

	// First, try to connect to an existing server
	client, err := tryConnect(address, cfg.RetryDelay)
	if err == nil {
		// Server is already running
		if cfg.Logger != nil {
			cfg.Logger.Info("connected to existing server instance", "address", address)
		}

		return client, nil
	}

	// No server running, start an on-demand instance
	if cfg.Logger != nil {
		cfg.Logger.Info("no server found, starting on-demand instance", "address", address)
	}

	if err := startOnDemandServer(ctx, cfg); err != nil {
		return nil, fmt.Errorf("failed to start on-demand server: %w", err)
	}

	// Wait for server to be ready
	client, err = waitForServer(ctx, address, cfg)
	if err != nil {
		return nil, fmt.Errorf("server failed to start: %w", err)
	}

	return client, nil
}

// tryConnect attempts to connect to the server once
func tryConnect(address string, timeout time.Duration) (*Client, error) {
	cfg := Config{
		Address:     address,
		DialTimeout: timeout,
	}

	return New(cfg)
}

// waitForServer waits for the server to become available
func waitForServer(ctx context.Context, address string, cfg DiscoveryConfig) (*Client, error) {
	deadline := time.Now().Add(cfg.StartTimeout)

	for i := 0; i < cfg.ConnectionRetry; i++ {
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timeout waiting for server to start")
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(cfg.RetryDelay):
		}

		client, err := tryConnect(address, cfg.RetryDelay*2)
		if err == nil {
			// Verify server is responsive
			pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			if pingErr := client.Ping(pingCtx); pingErr == nil {
				cancel()
				return client, nil
			}

			cancel()

			_ = client.Close()
		}
	}

	return nil, fmt.Errorf("failed to connect after %d retries", cfg.ConnectionRetry)
}

// startOnDemandServer starts the glix server as a background process with idle timeout
func startOnDemandServer(ctx context.Context, cfg DiscoveryConfig) error {
	// Get the path to the current executable
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	exePath, err = filepath.Abs(exePath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Build command arguments
	args := []string{
		"service", "run",
		"--port", fmt.Sprintf("%d", cfg.Port),
		"--bind", cfg.Address,
		"--idle-timeout", cfg.IdleTimeout.String(),
	}

	// Start the server as a detached process
	cmd := exec.Command(exePath, args...)

	// Detach from parent process
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil

	// Set process attributes for detachment (platform-specific handling in setProcAttr)
	setProcAttr(cmd)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start server process: %w", err)
	}

	// Don't wait for the process - let it run independently
	go func() {
		_ = cmd.Wait()
	}()

	if cfg.Logger != nil {
		cfg.Logger.Info("started on-demand server",
			"pid", cmd.Process.Pid,
			"idle_timeout", cfg.IdleTimeout,
		)
	}

	return nil
}

// IsServerRunning checks if a glix server is running at the given address
func IsServerRunning(address string) bool {
	client, err := tryConnect(address, time.Second)
	if err != nil {
		return false
	}

	defer func() {
		_ = client.Close()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	return client.Ping(ctx) == nil
}
