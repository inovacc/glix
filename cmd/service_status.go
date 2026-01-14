package cmd

import (
	"fmt"
	"time"

	"github.com/inovacc/glix/internal/client"
	"github.com/inovacc/glix/internal/service"
	"github.com/spf13/cobra"
)

var serviceStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the glix service status",
	Long: `Display the current status of the glix background service.

Shows both the system service status (requires admin) and the
gRPC server status (no admin required).`,
	RunE: runServiceStatus,
}

func init() {
	serviceCmd.AddCommand(serviceStatusCmd)
}

func runServiceStatus(cmd *cobra.Command, args []string) error {
	cmd.Printf("Glix Service Status\n")
	cmd.Printf("-------------------\n")

	// Try to check system service status (may require admin)
	mgr, mgrErr := service.NewManager()
	if mgrErr == nil {
		status, err := mgr.Status(cmd.Context())
		if err == nil {
			cmd.Printf("\nSystem Service:\n")
			cmd.Printf("  Installed: %v\n", mgr.IsInstalled())
			cmd.Printf("  Running:   %v\n", status.Running)
			cmd.Printf("  Status:    %s\n", status.Description)
			if status.PID > 0 {
				cmd.Printf("  PID:       %d\n", status.PID)
			}
		} else {
			cmd.Printf("\nSystem Service: Unable to query (may require admin)\n")
		}
	} else {
		cmd.Printf("\nSystem Service: Unable to access (may require admin)\n")
	}

	// Check gRPC server status (no admin required)
	cmd.Printf("\ngRPC Server:\n")

	cfg := client.DefaultConfig()
	cfg.DialTimeout = 2 * time.Second

	grpcClient, err := client.New(cfg)
	if err != nil {
		cmd.Printf("  Status:  Not running\n")
		cmd.Printf("  Address: %s\n", cfg.Address)
		return nil
	}
	defer func() {
		_ = grpcClient.Close()
	}()

	// Get server status
	status, err := grpcClient.GetStatus(cmd.Context())
	if err != nil {
		cmd.Printf("  Status:  Not responding\n")
		cmd.Printf("  Address: %s\n", cfg.Address)
		return nil
	}

	cmd.Printf("  Status:    Running\n")
	cmd.Printf("  Address:   %s\n", status.GetAddress())
	cmd.Printf("  Namespace: %s\n", status.GetNamespace())
	cmd.Printf("  Database:  %s\n", status.GetDatabasePath())
	cmd.Printf("  Uptime:    %s\n", formatUptime(status.GetUptimeSeconds()))
	cmd.Printf("  Modules:   %d\n", status.GetModuleCount())

	return nil
}

func formatUptime(seconds int64) string {
	d := time.Duration(seconds) * time.Second

	if d < time.Minute {
		return fmt.Sprintf("%ds", seconds)
	}

	if d < time.Hour {
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	}

	hours := int(d.Hours())
	mins := int(d.Minutes()) % 60

	if hours < 24 {
		return fmt.Sprintf("%dh %dm", hours, mins)
	}

	days := hours / 24
	hours = hours % 24

	return fmt.Sprintf("%dd %dh %dm", days, hours, mins)
}
