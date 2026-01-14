package cmd

import (
	"fmt"

	"github.com/inovacc/glix/internal/service"
	"github.com/spf13/cobra"
)

var serviceStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the glix service",
	Long:  `Start the glix background service.`,
	RunE:  runServiceStart,
}

func init() {
	serviceCmd.AddCommand(serviceStartCmd)
}

func runServiceStart(cmd *cobra.Command, args []string) error {
	mgr, err := service.NewManager()
	if err != nil {
		return fmt.Errorf("failed to create service manager: %w", err)
	}

	if !mgr.IsInstalled() {
		return fmt.Errorf("service is not installed, use 'glix service install' first")
	}

	status, err := mgr.Status(cmd.Context())
	if err != nil {
		return fmt.Errorf("failed to get service status: %w", err)
	}

	if status.Running {
		cmd.Println("Service is already running.")
		return nil
	}

	cmd.Println("Starting glix service...")

	if err := mgr.Start(cmd.Context()); err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}

	cmd.Println("Service started successfully!")

	return nil
}
