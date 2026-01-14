package cmd

import (
	"fmt"

	"github.com/inovacc/glix/internal/module"
	"github.com/inovacc/glix/internal/server"
	"github.com/inovacc/glix/internal/service"
	"github.com/spf13/cobra"
)

var serviceInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install the glix service on the system",
	Long: `Install the glix service on the system.

This will register glix as a system service that starts automatically
on boot and runs the gRPC server in the background.

Platform-specific behavior:
  - Windows: Registers as a Windows Service
  - Linux: Creates a systemd unit file
  - macOS: Creates a launchd plist file`,
	RunE: runServiceInstall,
}

var (
	installNamespace    string
	installDatabasePath string
	installPort         int
	installBindAddress  string
)

func init() {
	serviceCmd.AddCommand(serviceInstallCmd)

	serviceInstallCmd.Flags().StringVar(&installNamespace, "namespace", "", "Namespace for the service (defaults to hostname)")
	serviceInstallCmd.Flags().StringVar(&installDatabasePath, "database", "", "Path to the database file")
	serviceInstallCmd.Flags().IntVar(&installPort, "port", server.DefaultPort, "Port for the gRPC server")
	serviceInstallCmd.Flags().StringVar(&installBindAddress, "bind", "localhost", "Address to bind the server to")
}

func runServiceInstall(cmd *cobra.Command, args []string) error {
	mgr, err := service.NewManager()
	if err != nil {
		return fmt.Errorf("failed to create service manager: %w", err)
	}

	if mgr.IsInstalled() {
		return fmt.Errorf("service is already installed, use 'glix service uninstall' first")
	}

	// Use default database path if not specified
	dbPath := installDatabasePath
	if dbPath == "" {
		dbPath = module.GetDatabaseDirectory()
	}

	cfg := service.Config{
		Namespace:    installNamespace,
		DatabasePath: dbPath,
		Port:         installPort,
		BindAddress:  installBindAddress,
	}

	cmd.Printf("Installing glix service...\n")
	cmd.Printf("  Namespace:    %s\n", cfg.Namespace)
	cmd.Printf("  Database:     %s\n", cfg.DatabasePath)
	cmd.Printf("  Port:         %d\n", cfg.Port)
	cmd.Printf("  Bind Address: %s\n", cfg.BindAddress)

	if err := mgr.Install(cmd.Context(), cfg); err != nil {
		return fmt.Errorf("failed to install service: %w", err)
	}

	cmd.Println("\nService installed successfully!")
	cmd.Println("Use 'glix service start' to start the service.")

	return nil
}
