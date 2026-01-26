package cmd

import (
	"fmt"
	"time"

	"github.com/inovacc/glix/internal/client"
	"github.com/spf13/cobra"
)

// reportCmd represents the report command
var reportCmd = &cobra.Command{
	Use:   "report [module]",
	Short: "Show details about an installed module",
	Long: `Display detailed information about an installed Go module.

Shows the module name, version, installation time, and dependencies.

Examples:
  glix report github.com/inovacc/twig
  glix report github.com/spf13/cobra`,
	Args: cobra.ExactArgs(1),
	RunE: runReport,
}

var reportVersion string

func init() {
	rootCmd.AddCommand(reportCmd)

	reportCmd.Flags().StringVarP(&reportVersion, "version", "v", "", "Specific version to show (default: latest)")
}

func runReport(cmd *cobra.Command, args []string) error {
	moduleName := args[0]

	// Try to use the gRPC client
	cfg := client.DefaultDiscoveryConfig()

	grpcClient, err := client.GetClient(cmd.Context(), cfg)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}

	defer func() {
		_ = grpcClient.Close()
	}()

	// Get module info
	resp, err := grpcClient.GetModule(cmd.Context(), moduleName, reportVersion)
	if err != nil {
		return fmt.Errorf("failed to get module: %w", err)
	}

	if !resp.GetFound() {
		cmd.Printf("Module %q not found in database\n", moduleName)

		if reportVersion != "" {
			cmd.Printf("Try without --version flag to see all installed versions\n")
		}

		return nil
	}

	mod := resp.GetModule()

	// Display module information
	cmd.Println()
	cmd.Printf("Module: %s\n", mod.GetName())
	cmd.Printf("Version: %s\n", mod.GetVersion())

	if mod.GetTimestampUnixNano() > 0 {
		installedAt := time.Unix(0, mod.GetTimestampUnixNano())
		cmd.Printf("Installed: %s\n", installedAt.Format(time.RFC3339))
	}

	if mod.GetHash() != "" {
		cmd.Printf("Hash: %s\n", mod.GetHash())
	}

	if len(mod.GetVersions()) > 0 {
		cmd.Printf("Available versions: %d\n", len(mod.GetVersions()))
		// Show up to 5 most recent versions
		versions := mod.GetVersions()

		showCount := min(len(versions), 5)

		cmd.Printf("Latest versions: %v\n", versions[:showCount])
	}

	// Show dependencies
	deps := mod.GetDependencies()
	if len(deps) > 0 {
		cmd.Printf("\nDependencies (%d):\n", len(deps))

		for _, dep := range deps {
			cmd.Printf("  - %s@%s\n", dep.GetName(), dep.GetVersion())
		}
	} else {
		cmd.Println("\nNo dependencies recorded")
	}

	cmd.Println()

	return nil
}
