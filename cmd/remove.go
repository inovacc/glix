package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/inovacc/glix/internal/client"
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
	input := args[0]

	// Parse module path and version
	modulePath, version := parseModulePath(input)

	cmd.Printf("Removing module: %s", modulePath)
	if version != "" {
		cmd.Printf("@%s", version)
	}
	cmd.Println()

	// Try to remove binary from GOBIN
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
	extensions := []string{"", ".exe"}
	for _, ext := range extensions {
		binaryPath := filepath.Join(gobin, binaryName+ext)
		if _, err := os.Stat(binaryPath); err == nil {
			if err := os.Remove(binaryPath); err != nil {
				cmd.PrintErrf("Warning: failed to remove binary %s: %v\n", binaryPath, err)
			} else {
				cmd.Printf("Binary removed: %s\n", binaryPath)
			}
			break
		}
	}

	// Try to use the gRPC client to remove from database
	cfg := client.DefaultDiscoveryConfig()
	grpcClient, err := client.GetClient(cmd.Context(), cfg)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer func() {
		_ = grpcClient.Close()
	}()

	// Remove from database
	resp, err := grpcClient.Remove(cmd.Context(), modulePath, version)
	if err != nil {
		return fmt.Errorf("failed to remove module from database: %w", err)
	}

	if !resp.GetSuccess() {
		return fmt.Errorf("failed to remove module: %s", resp.GetErrorMessage())
	}

	cmd.Println("Module removed from database")

	return nil
}
