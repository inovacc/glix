package cmd

import (
	"fmt"
	"strings"

	"github.com/inovacc/glix/internal/client"
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

The installation uses a gRPC client that connects to an on-demand
server (starts automatically if not running).

Examples:
  glix install github.com/inovacc/twig
  glix install https://github.com/inovacc/twig
  glix install github.com/inovacc/twig@latest
  glix install github.com/inovacc/twig@v1.0.0`,
	Args: cobra.ExactArgs(1),
	RunE: runInstall,
}

var installForce bool

func init() {
	rootCmd.AddCommand(installCmd)

	installCmd.Flags().BoolVarP(&installForce, "force", "f", false, "Force installation even if already installed")
}

func runInstall(cmd *cobra.Command, args []string) error {
	// Parse module path and version
	modulePath, version := parseModulePath(args[0])

	// Use the gRPC client (on-demand server starts automatically)
	cfg := client.DefaultDiscoveryConfig()
	grpcClient, err := client.GetClient(cmd.Context(), cfg)
	if err != nil {
		return fmt.Errorf("failed to connect to gRPC server: %w", err)
	}
	defer func() {
		_ = grpcClient.Close()
	}()

	cmd.Printf("Installing module: %s", modulePath)
	if version != "" {
		cmd.Printf("@%s", version)
	}
	cmd.Println()

	// Output handler for streaming
	outputHandler := func(stream string, line string) {
		if stream == "stderr" {
			_, _ = fmt.Fprintln(cmd.ErrOrStderr(), line)
		} else {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), line)
		}
	}

	// Progress handler
	progressHandler := func(phase, message string, percent int32) {
		if percent >= 0 {
			cmd.Printf("[%d%%] %s: %s\n", percent, phase, message)
		} else {
			cmd.Printf("[...] %s: %s\n", phase, message)
		}
	}

	// Install with streaming
	result, err := grpcClient.InstallWithStreaming(cmd.Context(), modulePath, version, installForce, outputHandler, progressHandler)
	if err != nil {
		return fmt.Errorf("installation failed: %w", err)
	}

	if !result.GetSuccess() {
		return fmt.Errorf("installation failed: %s", result.GetErrorMessage())
	}

	cmd.Println()
	cmd.Println("Module installed successfully:", result.GetModule().GetName())
	cmd.Printf("Show report using: %s report %s\n", cmd.Root().Name(), result.GetModule().GetName())

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
