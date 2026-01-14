package cmd

import (
	"github.com/spf13/cobra"
)

var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: "Manage the glix background service",
	Long: `Manage the glix gRPC background service.

The service allows multiple glix clients to share a common database
and enables remote module management.

Commands:
  install   - Install the service on the system
  uninstall - Remove the service from the system
  start     - Start the service
  stop      - Stop the service
  status    - Show service status
  run       - Run the server directly (used by service managers)`,
}

func init() {
	rootCmd.AddCommand(serviceCmd)
}
