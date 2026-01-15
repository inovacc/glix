package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/inovacc/glix/internal/client"
	"github.com/inovacc/glix/internal/module"
	"github.com/spf13/cobra"
)

var (
	monitorUpdateAll bool
)

// monitorCmd represents the monitor command
var monitorCmd = &cobra.Command{
	Use:   "monitor",
	Short: "Check all installed modules for available updates",
	Long: `Monitor checks all installed Go modules for available updates.

It compares the installed version of each module against the latest
available version from the Go proxy and reports any that can be updated.

Examples:
  glix monitor              # Check for updates
  glix monitor --update     # Check and update all outdated modules`,
	RunE: runMonitor,
}

func init() {
	monitorCmd.Flags().BoolVarP(&monitorUpdateAll, "update", "u", false, "Automatically update all outdated modules")
	rootCmd.AddCommand(monitorCmd)
}

// moduleStatus represents the update status of a module
type moduleStatus struct {
	Name             string
	InstalledVersion string
	LatestVersion    string
	HasUpdate        bool
	Error            error
}

func runMonitor(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	// Connect to server
	cfg := client.DefaultDiscoveryConfig()
	grpcClient, err := client.GetClient(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer func() {
		_ = grpcClient.Close()
	}()

	// List all installed modules
	resp, err := grpcClient.ListModules(ctx, 0, 0, "")
	if err != nil {
		return fmt.Errorf("failed to list modules: %w", err)
	}

	modules := resp.GetModules()
	if len(modules) == 0 {
		cmd.Println("No modules installed")
		return nil
	}

	cmd.Printf("Checking %d installed module(s) for updates...\n\n", len(modules))

	// Check each module for updates concurrently
	statuses := make([]moduleStatus, len(modules))
	var wg sync.WaitGroup

	for i, mod := range modules {
		wg.Add(1)
		go func(idx int, modName, modVersion string) {
			defer wg.Done()
			statuses[idx] = checkModuleUpdate(ctx, modName, modVersion)
		}(i, mod.GetName(), mod.GetVersion())
	}

	wg.Wait()

	// Display results
	var updatesAvailable []moduleStatus
	var upToDate []moduleStatus
	var errors []moduleStatus

	for _, status := range statuses {
		if status.Error != nil {
			errors = append(errors, status)
		} else if status.HasUpdate {
			updatesAvailable = append(updatesAvailable, status)
		} else {
			upToDate = append(upToDate, status)
		}
	}

	// Show modules with updates available
	if len(updatesAvailable) > 0 {
		cmd.Println("Updates available:")
		for _, s := range updatesAvailable {
			cmd.Printf("  %s\n", s.Name)
			cmd.Printf("    Installed: %s\n", s.InstalledVersion)
			cmd.Printf("    Latest:    %s\n", s.LatestVersion)
		}
		cmd.Println()
	}

	// Show up-to-date modules
	if len(upToDate) > 0 {
		cmd.Println("Up to date:")
		for _, s := range upToDate {
			cmd.Printf("  %s@%s\n", s.Name, s.InstalledVersion)
		}
		cmd.Println()
	}

	// Show errors
	if len(errors) > 0 {
		cmd.Println("Errors checking:")
		for _, s := range errors {
			cmd.Printf("  %s: %v\n", s.Name, s.Error)
		}
		cmd.Println()
	}

	// Summary
	cmd.Printf("Summary: %d up to date, %d update(s) available, %d error(s)\n",
		len(upToDate), len(updatesAvailable), len(errors))

	// If --update flag is set, update all outdated modules
	if monitorUpdateAll && len(updatesAvailable) > 0 {
		cmd.Println()
		cmd.Println("Updating outdated modules...")
		cmd.Println()

		for _, s := range updatesAvailable {
			cmd.Printf("Updating %s: %s -> %s\n", s.Name, s.InstalledVersion, s.LatestVersion)

			if err := updateModule(cmd, grpcClient, s.Name); err != nil {
				cmd.PrintErrf("  Failed: %v\n", err)
			} else {
				cmd.Println("  Done")
			}
		}
	}

	return nil
}

// checkModuleUpdate checks if a module has an available update
func checkModuleUpdate(ctx context.Context, moduleName, installedVersion string) moduleStatus {
	status := moduleStatus{
		Name:             moduleName,
		InstalledVersion: installedVersion,
	}

	// Create a unique working directory
	cacheDir, err := module.GetApplicationCacheDirectory()
	if err != nil {
		status.Error = err
		return status
	}

	workDir := filepath.Join(cacheDir, fmt.Sprintf("monitor-%d", time.Now().UnixNano()))
	if err := os.MkdirAll(workDir, 0755); err != nil {
		status.Error = err
		return status
	}
	defer func() {
		_ = os.RemoveAll(workDir)
	}()

	// Create module instance to fetch latest version
	m, err := module.NewModule(ctx, "go", workDir)
	if err != nil {
		status.Error = err
		return status
	}

	// Fetch latest version info
	if err := m.FetchModuleInfo(moduleName); err != nil {
		status.Error = err
		return status
	}

	status.LatestVersion = m.Version
	status.HasUpdate = isNewerVersion(m.Version, installedVersion)

	return status
}

// updateModule updates a single module
func updateModule(cmd *cobra.Command, grpcClient *client.Client, moduleName string) error {
	ctx := cmd.Context()

	// Create a unique working directory
	cacheDir, err := module.GetApplicationCacheDirectory()
	if err != nil {
		return err
	}

	workDir := filepath.Join(cacheDir, fmt.Sprintf("update-%d", time.Now().UnixNano()))
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return err
	}
	defer func() {
		_ = os.RemoveAll(workDir)
	}()

	// Create module instance
	m, err := module.NewModule(ctx, "go", workDir)
	if err != nil {
		return err
	}

	// Fetch latest module info
	if err := m.FetchModuleInfo(moduleName); err != nil {
		return err
	}

	// Output handler (suppress output during batch update)
	outputHandler := func(stream string, line string) {
		// Silent update - could add verbose flag later
	}

	// Install the new version
	if err := m.InstallModuleWithStreaming(ctx, outputHandler); err != nil {
		return err
	}

	// Store updated module info
	return grpcClient.StoreModule(ctx, m)
}
