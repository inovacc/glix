package cmd

import (
	"github.com/inovacc/glix/internal/installer"
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
	RunE: installer.Installer,
}

func init() {
	rootCmd.AddCommand(installCmd)

	installCmd.Flags().BoolP("force", "f", false, "Force installation even if already installed")
}
