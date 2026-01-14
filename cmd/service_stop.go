package cmd

import (
	"fmt"

	"github.com/inovacc/glix/internal/service"
	"github.com/spf13/cobra"
)

var serviceStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the glix service",
	Long:  `Stop the glix background service.`,
	RunE:  runServiceStop,
}

func init() {
	serviceCmd.AddCommand(serviceStopCmd)
}

func runServiceStop(cmd *cobra.Command, args []string) error {
	mgr, err := service.NewManager()
	if err != nil {
		return fmt.Errorf("failed to create service manager: %w", err)
	}

	if !mgr.IsInstalled() {
		return fmt.Errorf("service is not installed")
	}

	status, err := mgr.Status(cmd.Context())
	if err != nil {
		return fmt.Errorf("failed to get service status: %w", err)
	}

	if !status.Running {
		cmd.Println("Service is not running.")
		return nil
	}

	cmd.Println("Stopping glix service...")

	if err := mgr.Stop(cmd.Context()); err != nil {
		return fmt.Errorf("failed to stop service: %w", err)
	}

	cmd.Println("Service stopped successfully!")

	return nil
}
