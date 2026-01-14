package cmd

import (
	"fmt"

	"github.com/inovacc/glix/internal/service"
	"github.com/spf13/cobra"
)

var serviceUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove the glix service from the system",
	Long: `Remove the glix service from the system.

This will stop the service if running and unregister it from the system.`,
	RunE: runServiceUninstall,
}

func init() {
	serviceCmd.AddCommand(serviceUninstallCmd)
}

func runServiceUninstall(cmd *cobra.Command, args []string) error {
	mgr, err := service.NewManager()
	if err != nil {
		return fmt.Errorf("failed to create service manager: %w", err)
	}

	if !mgr.IsInstalled() {
		cmd.Println("Service is not installed.")
		return nil
	}

	cmd.Println("Uninstalling glix service...")

	if err := mgr.Uninstall(cmd.Context()); err != nil {
		return fmt.Errorf("failed to uninstall service: %w", err)
	}

	cmd.Println("Service uninstalled successfully!")

	return nil
}
