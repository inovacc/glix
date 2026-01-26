package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var noTUI bool

var rootCmd = &cobra.Command{
	Use:   "glix [module]",
	Short: "Install, update or remove Go modules with ease",
	Long: `glix is a CLI tool that helps manage Go module installations.

You can use it to fetch, install, update, or remove Go packages
from your environment with a clean and idiomatic approach.

Usage:
  glix install <module>  - Install a Go module
  glix remove <module>   - Remove an installed module
  glix update <module>   - Update a module to latest version
  glix service <cmd>     - Manage the glix background service
  glix <module>          - Shorthand for install`,
	Args: cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}

		// Direct invocation acts as shorthand for installation
		// Reuse the install command logic
		return runInstall(cmd, args)
	},
}

func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

// GetRootCmd returns the root command for introspection purposes.
func GetRootCmd() *cobra.Command {
	return rootCmd
}

func init() {
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.PersistentFlags().BoolVar(&noTUI, "no-tui", false,
		"Disable TUI, use plain text output")
}

// IsTUIEnabled returns whether the TUI should be used
// Returns false if --no-tui flag is set or if not running in a terminal
func IsTUIEnabled() bool {
	if noTUI {
		return false
	}
	// Also disable TUI if not running in a terminal
	return term.IsTerminal(int(os.Stdout.Fd()))
}
