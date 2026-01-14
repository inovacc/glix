package cmd

import (
	"github.com/inovacc/glix/internal/installer"
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
  glix remove twig`,
	Args: cobra.ExactArgs(1),
	RunE: installer.Remover,
}

func init() {
	rootCmd.AddCommand(removeCmd)
}
