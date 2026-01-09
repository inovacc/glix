package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
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
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("update functionality not yet implemented")
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
}
