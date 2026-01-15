package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/inovacc/glix/internal/client"
	"github.com/inovacc/glix/internal/module"
	"github.com/spf13/cobra"
	"golang.org/x/mod/semver"
)

// updateCmd represents the update command
var updateCmd = &cobra.Command{
	Use:   "update [module]",
	Short: "Update an installed Go module to the latest version",
	Long: `Update a previously installed Go module to its latest version.
This will fetch the latest version from the Go proxy, install it,
and update the database entry.

Example:
  glix update github.com/inovacc/twig
  glix update twig`,
	Args: cobra.ExactArgs(1),
	RunE: runUpdate,
}

func init() {
	rootCmd.AddCommand(updateCmd)
}

func runUpdate(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// Parse module path (strip URL prefixes if any)
	modulePath, _ := parseModulePath(args[0])

	// Connect to server to get current module info
	cfg := client.DefaultDiscoveryConfig()
	grpcClient, err := client.GetClient(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer func() {
		_ = grpcClient.Close()
	}()

	// Get currently installed module
	resp, err := grpcClient.GetModule(ctx, modulePath, "")
	if err != nil {
		return fmt.Errorf("failed to query module: %w", err)
	}

	if !resp.GetFound() {
		return fmt.Errorf("module %q is not installed, use 'glix install %s' first", modulePath, modulePath)
	}

	installedModule := resp.GetModule()
	installedVersion := installedModule.GetVersion()

	cmd.Printf("Checking for updates: %s@%s\n", modulePath, installedVersion)

	// Create a unique working directory for this update
	cacheDir, err := module.GetApplicationCacheDirectory()
	if err != nil {
		return fmt.Errorf("failed to get cache directory: %w", err)
	}

	workDir := filepath.Join(cacheDir, fmt.Sprintf("update-%d", time.Now().UnixNano()))
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return fmt.Errorf("failed to create working directory: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(workDir)
	}()

	// Create module instance to fetch latest version info
	m, err := module.NewModule(ctx, "go", workDir)
	if err != nil {
		return fmt.Errorf("failed to create module: %w", err)
	}

	// Fetch latest module info
	cmd.Println("[...] Fetching latest version information...")
	if err := m.FetchModuleInfo(modulePath); err != nil {
		return fmt.Errorf("failed to fetch module info: %w", err)
	}

	latestVersion := m.Version

	// Compare versions
	if !isNewerVersion(latestVersion, installedVersion) {
		cmd.Printf("Already at latest version: %s@%s\n", modulePath, installedVersion)
		return nil
	}

	cmd.Printf("[...] Updating %s: %s -> %s\n", modulePath, installedVersion, latestVersion)

	// Output handler for streaming installation output
	outputHandler := func(stream string, line string) {
		if stream == "stderr" {
			_, _ = fmt.Fprintln(cmd.ErrOrStderr(), line)
		} else {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), line)
		}
	}

	// Install the new version locally with streaming output
	if err := m.InstallModuleWithStreaming(ctx, outputHandler); err != nil {
		return fmt.Errorf("update failed: %w", err)
	}

	// Store updated module info in database via server
	if err := grpcClient.StoreModule(ctx, m); err != nil {
		cmd.PrintErrf("Warning: failed to update module in database: %v\n", err)
	}

	cmd.Println()
	cmd.Printf("Module updated successfully: %s\n", m.Name)
	cmd.Printf("  Previous: %s\n", installedVersion)
	cmd.Printf("  Current:  %s\n", latestVersion)

	return nil
}

// isNewerVersion compares two versions and returns true if newVer is newer than oldVer
func isNewerVersion(newVer, oldVer string) bool {
	// Ensure versions have 'v' prefix for semver comparison
	if newVer != "" && newVer[0] != 'v' {
		newVer = "v" + newVer
	}
	if oldVer != "" && oldVer[0] != 'v' {
		oldVer = "v" + oldVer
	}

	// If versions are identical, no update needed
	if newVer == oldVer {
		return false
	}

	// Try semver comparison first
	cmp := semver.Compare(newVer, oldVer)
	if cmp != 0 {
		return cmp > 0
	}

	// For pseudo-versions or non-standard versions, do string comparison
	// Pseudo-versions like v0.0.0-20260108194045-146fb9cee2cb contain timestamps
	return newVer > oldVer
}
