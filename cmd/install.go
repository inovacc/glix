package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/inovacc/glix/internal/client"
	"github.com/inovacc/glix/internal/module"
	"github.com/inovacc/glix/internal/tui"
	"github.com/spf13/cobra"
)

// installCmd represents the install command
var installCmd = &cobra.Command{
	Use:   "install [module]",
	Short: "Install a Go module",
	Long: `Install a Go module from a repository and track it in the database.

The module can be specified as a full import path or a GitHub URL.
glix will automatically detect CLI binaries in the repository if the
root is not installable.

Examples:
  glix install github.com/inovacc/twig
  glix install https://github.com/inovacc/twig
  glix install github.com/inovacc/twig@latest
  glix install github.com/inovacc/twig@v1.0.0`,
	Args: cobra.ExactArgs(1),
	RunE: runInstall,
}

func init() {
	rootCmd.AddCommand(installCmd)
}

func runInstall(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// Parse module path and version
	modulePath, version := parseModulePath(args[0])

	if IsTUIEnabled() {
		return runInstallWithTUI(ctx, cmd, modulePath, version)
	}

	return runInstallPlainText(ctx, cmd, modulePath, version)
}

func runInstallWithTUI(ctx context.Context, cmd *cobra.Command, modulePath, version string) error {
	// Create TUI instance
	t := tui.New()

	// Create a context that we can cancel when TUI exits
	tuiCtx, tuiCancel := context.WithCancel(ctx)
	defer tuiCancel()

	// Channel to communicate errors from the installation goroutine
	errCh := make(chan error, 1)

	// Run installation in background
	go func() {
		errCh <- doInstall(tuiCtx, cmd, modulePath, version, t.ProgressHandler(), t.OutputHandler(), t.SetStatus)
	}()

	// Start TUI - this blocks until done
	go func() {
		// Wait for installation to complete
		err := <-errCh
		t.Done(err)
	}()

	// Run TUI
	if err := t.Start(tuiCtx); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	return nil
}

func runInstallPlainText(ctx context.Context, cmd *cobra.Command, modulePath, version string) error {
	cmd.Printf("Installing module: %s", modulePath)
	if version != "" {
		cmd.Printf("@%s", version)
	}
	cmd.Println()

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
		cmd.Printf("Status: %s\n", text)
	}

	return doInstall(ctx, cmd, modulePath, version, progressHandler, outputHandler, statusHandler)
}

func doInstall(
	ctx context.Context,
	cmd *cobra.Command,
	modulePath, version string,
	progressHandler func(phase, message string),
	outputHandler func(stream, line string),
	statusHandler func(text string),
) error {
	statusHandler(fmt.Sprintf("Installing %s", modulePath))

	// Connect to server first (starts on-demand server if needed)
	cfg := client.DefaultDiscoveryConfig()
	grpcClient, err := client.GetClient(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer func() {
		_ = grpcClient.Close()
	}()

	// Create a unique working directory for this install
	cacheDir, err := module.GetApplicationCacheDirectory()
	if err != nil {
		return fmt.Errorf("failed to get cache directory: %w", err)
	}

	workDir := filepath.Join(cacheDir, fmt.Sprintf("install-%d", time.Now().UnixNano()))
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return fmt.Errorf("failed to create working directory: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(workDir)
	}()

	// Create module instance
	m, err := module.NewModule(ctx, "go", workDir)
	if err != nil {
		return fmt.Errorf("failed to create module: %w", err)
	}

	// Set progress handler to show what's happening
	m.SetProgressHandler(progressHandler)

	// Build full module path with version if specified
	fullPath := modulePath
	if version != "" && version != "latest" {
		fullPath = fmt.Sprintf("%s@%s", modulePath, version)
	}

	// Fetch module info (CLI performs this locally)
	if err := m.FetchModuleInfo(fullPath); err != nil {
		return fmt.Errorf("failed to fetch module info: %w", err)
	}

	progressHandler("install", fmt.Sprintf("Installing %s@%s...", m.Name, m.Version))
	statusHandler(fmt.Sprintf("Installing %s@%s", m.Name, m.Version))

	// Install module locally with streaming output
	if err := m.InstallModuleWithStreaming(ctx, outputHandler); err != nil {
		return fmt.Errorf("installation failed: %w", err)
	}

	// Store module info in database via server
	progressHandler("store", "Saving to database...")
	if err := grpcClient.StoreModule(ctx, m); err != nil {
		progressHandler("warning", fmt.Sprintf("failed to store module in database: %v", err))
	}

	progressHandler("complete", fmt.Sprintf("Module %s installed successfully", m.Name))
	statusHandler(fmt.Sprintf("Installed %s@%s", m.Name, m.Version))

	return nil
}

// parseModulePath extracts the module path and version from the input
func parseModulePath(input string) (string, string) {
	// Remove common URL prefixes
	input = strings.TrimPrefix(input, "https://")
	input = strings.TrimPrefix(input, "http://")
	input = strings.TrimPrefix(input, "git://")
	input = strings.TrimPrefix(input, "ssh://")
	input = strings.TrimSuffix(input, ".git")

	// Check for version suffix
	if idx := strings.LastIndex(input, "@"); idx != -1 {
		return input[:idx], input[idx+1:]
	}

	return input, ""
}
