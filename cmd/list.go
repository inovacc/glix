package cmd

import (
	"fmt"
	"time"

	"github.com/inovacc/glix/internal/client"
	"github.com/spf13/cobra"
)

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all installed modules",
	Long: `Display a list of all Go modules installed via glix.

Shows module names, versions, and installation times.

Examples:
  glix list
  glix list --filter cobra
  glix list --limit 10`,
	RunE: runList,
}

var (
	listLimit  int32
	listOffset int32
	listFilter string
)

func init() {
	rootCmd.AddCommand(listCmd)

	listCmd.Flags().Int32VarP(&listLimit, "limit", "l", 0, "Maximum number of modules to show (0 = all)")
	listCmd.Flags().Int32VarP(&listOffset, "offset", "o", 0, "Number of modules to skip")
	listCmd.Flags().StringVarP(&listFilter, "filter", "f", "", "Filter modules by name")
}

func runList(cmd *cobra.Command, args []string) error {
	// Try to use the gRPC client
	cfg := client.DefaultDiscoveryConfig()

	grpcClient, err := client.GetClient(cmd.Context(), cfg)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}

	defer func() {
		_ = grpcClient.Close()
	}()

	// List modules
	resp, err := grpcClient.ListModules(cmd.Context(), listLimit, listOffset, listFilter)
	if err != nil {
		return fmt.Errorf("failed to list modules: %w", err)
	}

	modules := resp.GetModules()
	if len(modules) == 0 {
		cmd.Println("No modules installed")

		if listFilter != "" {
			cmd.Printf("(filter: %q)\n", listFilter)
		}

		return nil
	}

	cmd.Println()
	cmd.Printf("Installed modules (%d):\n", resp.GetTotalCount())
	cmd.Println()

	for _, mod := range modules {
		// Format installation time
		installedAt := ""

		if mod.GetTimestampUnixNano() > 0 {
			t := time.Unix(0, mod.GetTimestampUnixNano())
			installedAt = t.Format("2006-01-02 15:04")
		}

		// Count dependencies
		depCount := len(mod.GetDependencies())

		cmd.Printf("  %s@%s\n", mod.GetName(), mod.GetVersion())

		if installedAt != "" {
			cmd.Printf("    Installed: %s | Dependencies: %d\n", installedAt, depCount)
		}
	}

	cmd.Println()

	// Show pagination info if applicable
	if listLimit > 0 && resp.GetTotalCount() > int64(len(modules)) {
		cmd.Printf("Showing %d of %d modules\n", len(modules), resp.GetTotalCount())
	}

	return nil
}
