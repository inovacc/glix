package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// Config holds the service configuration
type Config struct {
	Namespace    string
	DatabasePath string
	Port         int
	BindAddress  string
}

// Status represents the service status
type Status struct {
	Running     bool
	PID         int
	Description string
}

// Manager defines the interface for platform-specific service management
type Manager interface {
	// Install installs the service on the system
	Install(ctx context.Context, cfg Config) error

	// Uninstall removes the service from the system
	Uninstall(ctx context.Context) error

	// Start starts the service
	Start(ctx context.Context) error

	// Stop stops the service
	Stop(ctx context.Context) error

	// Status returns the current service status
	Status(ctx context.Context) (*Status, error)

	// IsInstalled checks if the service is installed
	IsInstalled() bool
}

// ServiceName is the name used for the system service
const ServiceName = "glix"

// ServiceDisplayName is the display name for the service
const ServiceDisplayName = "Glix Module Manager"

// ServiceDescription is the description for the service
const ServiceDescription = "Glix gRPC service for managing Go module installations"

// NewManager creates a platform-specific service manager
func NewManager() (Manager, error) {
	return newPlatformManager()
}

// GetExecutablePath returns the path to the current executable
func GetExecutablePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to get executable path: %w", err)
	}

	return filepath.Abs(exe)
}

// BuildServiceArgs builds the command line arguments for the service
func BuildServiceArgs(cfg Config) []string {
	args := []string{"service", "run"}

	if cfg.Namespace != "" {
		args = append(args, "--namespace", cfg.Namespace)
	}

	if cfg.DatabasePath != "" {
		args = append(args, "--database", cfg.DatabasePath)
	}

	if cfg.Port != 0 {
		args = append(args, "--port", fmt.Sprintf("%d", cfg.Port))
	}

	if cfg.BindAddress != "" {
		args = append(args, "--bind", cfg.BindAddress)
	}

	return args
}
