package cmd

import (
	"fmt"

	"github.com/inovacc/glix/internal/installer"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var rootCmd = &cobra.Command{
	Use:   "glix",
	Short: "Install, update or remove Go modules with ease",
	Long: `glix is a CLI tool that helps manage Go module installations.

You can use it to fetch, install, update, or remove Go packages
from your environment with a clean and idiomatic approach.`,
	Args: cobra.ArbitraryArgs, // <- allows module path as an argument
	RunE: func(cmd *cobra.Command, args []string) error {
		remove, _ := cmd.Flags().GetBool("remove")
		update, _ := cmd.Flags().GetBool("update")

		if remove && update {
			return fmt.Errorf("flags --remove and --update cannot be used together")
		}

		if len(args) == 0 {
			return cmd.Help()
		}

		// Handle remove flag
		if remove {
			return installer.Remover(cmd, args)
		}

		// Handle update flag (not yet implemented)
		if update {
			return fmt.Errorf("update functionality not yet implemented")
		}

		// Default: install
		return installer.Installer(cmd, args)
	},
}

func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	rootCmd.Flags().BoolP("remove", "r", false, "Remove go install module")
	rootCmd.Flags().BoolP("update", "u", false, "Update go install module")

	cobra.CheckErr(viper.BindPFlag("remove", rootCmd.Flags().Lookup("remove")))
	cobra.CheckErr(viper.BindPFlag("update", rootCmd.Flags().Lookup("update")))
}
