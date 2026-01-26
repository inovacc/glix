package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/inovacc/glix/internal/client"
	"github.com/inovacc/glix/internal/tui"
	"github.com/spf13/cobra"
)

// removeCmd represents the remove command
var removeCmd = &cobra.Command{
	Use:   "remove [module]",
	Short: "Remove an installed Go module",
	Long: `Remove a previously installed Go module by deleting its binary
from GOBIN and removing its entry from the database.

Example:
  glix remove github.com/inovacc/twig
  glix remove github.com/inovacc/twig@v1.0.0`,
	Args: cobra.ExactArgs(1),
	RunE: runRemove,
}

func init() {
	rootCmd.AddCommand(removeCmd)
}

func runRemove(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	input := args[0]

	// Parse module path and version
	modulePath, version := parseModulePath(input)

	if IsTUIEnabled() {
		return runRemoveWithTUI(ctx, modulePath, version)
	}

	return runRemovePlainText(ctx, cmd, modulePath, version)
}

func runRemoveWithTUI(ctx context.Context, modulePath, version string) error {
	// Create TUI instance
	t := tui.New()

	// Create a context that we can cancel when TUI exits
	tuiCtx, tuiCancel := context.WithCancel(ctx)
	defer tuiCancel()

	// Channel to communicate errors from the remove goroutine
	errCh := make(chan error, 1)

	// Run remove in background
	go func() {
		errCh <- doRemove(tuiCtx, modulePath, version, t.ProgressHandler(), t.SetStatus)
	}()

	// Wait for remove to complete
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

func runRemovePlainText(ctx context.Context, cmd *cobra.Command, modulePath, version string) error {
	cmd.Printf("Removing module: %s", modulePath)

	if version != "" {
		cmd.Printf("@%s", version)
	}

	cmd.Println()

	progressHandler := func(phase, message string) {
		cmd.Printf("[%s] %s\n", phase, message)
	}

	statusHandler := func(text string) {
		// In plain text mode, we don't need a separate status line
	}

	return doRemove(ctx, modulePath, version, progressHandler, statusHandler)
}

func doRemove(
	ctx context.Context,
	modulePath, version string,
	progressHandler func(phase, message string),
	statusHandler func(text string),
) error {
	statusHandler(fmt.Sprintf("Removing %s", modulePath))

	// Try to remove binary from GOBIN
	progressHandler("binary", "Removing binary from GOBIN...")

	binaryName := filepath.Base(modulePath)
	if idx := strings.LastIndex(binaryName, "/"); idx != -1 {
		binaryName = binaryName[idx+1:]
	}

	gobin := os.Getenv("GOBIN")
	if gobin == "" {
		gopath := os.Getenv("GOPATH")
		if gopath == "" {
			home, _ := os.UserHomeDir()
			gopath = filepath.Join(home, "go")
		}

		gobin = filepath.Join(gopath, "bin")
	}

	// Try common binary extensions
	binaryRemoved := false

	extensions := []string{"", ".exe"}
	for _, ext := range extensions {
		binaryPath := filepath.Join(gobin, binaryName+ext)
		if _, err := os.Stat(binaryPath); err == nil {
			if err := os.Remove(binaryPath); err != nil {
				progressHandler("warning", fmt.Sprintf("failed to remove binary %s: %v", binaryPath, err))
			} else {
				progressHandler("binary", fmt.Sprintf("Removed: %s", binaryPath))

				binaryRemoved = true
			}

			break
		}
	}

	if !binaryRemoved {
		progressHandler("binary", "Binary not found in GOBIN")
	}

	// Try to use the gRPC client to remove from database
	progressHandler("database", "Connecting to server...")

	cfg := client.DefaultDiscoveryConfig()

	grpcClient, err := client.GetClient(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}

	defer func() {
		_ = grpcClient.Close()
	}()

	// Remove from database
	progressHandler("database", "Removing from database...")

	resp, err := grpcClient.Remove(ctx, modulePath, version)
	if err != nil {
		return fmt.Errorf("failed to remove module from database: %w", err)
	}

	if !resp.GetSuccess() {
		return fmt.Errorf("failed to remove module: %s", resp.GetErrorMessage())
	}

	progressHandler("complete", "Module removed successfully")
	statusHandler(fmt.Sprintf("Removed %s", modulePath))

	return nil
}
