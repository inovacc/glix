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
	"github.com/inovacc/glix/internal/tui"
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

	if IsTUIEnabled() {
		return runMonitorWithTUI(ctx)
	}

	return runMonitorPlainText(ctx, cmd)
}

func runMonitorWithTUI(ctx context.Context) error {
	// Create TUI instance
	t := tui.New()

	// Create a context that we can cancel when TUI exits
	tuiCtx, tuiCancel := context.WithCancel(ctx)
	defer tuiCancel()

	// Channel to communicate errors
	errCh := make(chan error, 1)

	// Run monitor in background
	go func() {
		errCh <- doMonitor(tuiCtx, t.ProgressHandler(), t.OutputHandler(), t.SetStatus)
	}()

	// Wait for completion
	go func() {
		err := <-errCh
		t.Done(err)
	}()

	// Run TUI
	if err := t.Start(tuiCtx); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	return nil
}

func runMonitorPlainText(ctx context.Context, cmd *cobra.Command) error {
	progressHandler := func(phase, message string) {
		cmd.Printf("[%s] %s\n", phase, message)
	}

	outputHandler := func(stream, line string) {
		if stream == "stderr" {
			_, _ = fmt.Fprintln(cmd.ErrOrStderr(), line)
		} else {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), line)
		}
	}

	statusHandler := func(text string) {
		// In plain text mode, status is shown via progress
	}

	return doMonitor(ctx, progressHandler, outputHandler, statusHandler)
}

func doMonitor(
	ctx context.Context,
	progressHandler func(phase, message string),
	outputHandler func(stream, line string),
	statusHandler func(text string),
) error {
	statusHandler("Checking for updates...")

	// Connect to server
	progressHandler("connect", "Connecting to server...")

	cfg := client.DefaultDiscoveryConfig()

	grpcClient, err := client.GetClient(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}

	defer func() {
		_ = grpcClient.Close()
	}()

	// List all installed modules
	progressHandler("list", "Listing installed modules...")

	resp, err := grpcClient.ListModules(ctx, 0, 0, "")
	if err != nil {
		return fmt.Errorf("failed to list modules: %w", err)
	}

	modules := resp.GetModules()
	if len(modules) == 0 {
		progressHandler("complete", "No modules installed")
		statusHandler("No modules installed")

		return nil
	}

	progressHandler("check", fmt.Sprintf("Checking %d module(s) for updates...", len(modules)))
	statusHandler(fmt.Sprintf("Checking %d modules...", len(modules)))

	// Check each module for updates concurrently
	statuses := make([]moduleStatus, len(modules))

	var (
		wg sync.WaitGroup
		mu sync.Mutex
	)

	checked := 0

	for i, mod := range modules {
		wg.Add(1)

		go func(idx int, modName, modVersion string) {
			defer wg.Done()

			statuses[idx] = checkModuleUpdate(ctx, modName, modVersion)

			mu.Lock()

			checked++
			progressHandler("check", fmt.Sprintf("Checked %d/%d: %s", checked, len(modules), modName))
			mu.Unlock()
		}(i, mod.GetName(), mod.GetVersion())
	}

	wg.Wait()

	// Categorize results
	var (
		updatesAvailable []moduleStatus
		upToDate         []moduleStatus
		errors           []moduleStatus
	)

	for _, status := range statuses {
		if status.Error != nil {
			errors = append(errors, status)
		} else if status.HasUpdate {
			updatesAvailable = append(updatesAvailable, status)
		} else {
			upToDate = append(upToDate, status)
		}
	}

	// Report results
	if len(updatesAvailable) > 0 {
		progressHandler("result", fmt.Sprintf("%d update(s) available:", len(updatesAvailable)))

		for _, s := range updatesAvailable {
			outputHandler("stdout", fmt.Sprintf("  %s: %s -> %s", s.Name, s.InstalledVersion, s.LatestVersion))
		}
	}

	if len(upToDate) > 0 {
		progressHandler("result", fmt.Sprintf("%d module(s) up to date", len(upToDate)))
	}

	if len(errors) > 0 {
		progressHandler("result", fmt.Sprintf("%d error(s):", len(errors)))

		for _, s := range errors {
			outputHandler("stderr", fmt.Sprintf("  %s: %v", s.Name, s.Error))
		}
	}

	// Summary
	summary := fmt.Sprintf("Summary: %d up to date, %d update(s) available, %d error(s)",
		len(upToDate), len(updatesAvailable), len(errors))
	progressHandler("summary", summary)
	statusHandler(summary)

	// If --update flag is set, update all outdated modules
	if monitorUpdateAll && len(updatesAvailable) > 0 {
		progressHandler("update", "Updating outdated modules...")

		for _, s := range updatesAvailable {
			progressHandler("update", fmt.Sprintf("Updating %s: %s -> %s", s.Name, s.InstalledVersion, s.LatestVersion))
			statusHandler(fmt.Sprintf("Updating %s...", s.Name))

			if err := updateModuleCore(ctx, grpcClient, s.Name); err != nil {
				progressHandler("error", fmt.Sprintf("Failed to update %s: %v", s.Name, err))
			} else {
				progressHandler("update", fmt.Sprintf("Updated %s to %s", s.Name, s.LatestVersion))
			}
		}

		progressHandler("complete", "All updates complete")
		statusHandler("Updates complete")
	} else {
		progressHandler("complete", "Check complete")
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

// updateModuleCore updates a single module (core logic without TUI)
func updateModuleCore(ctx context.Context, grpcClient *client.Client, moduleName string) error {
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
