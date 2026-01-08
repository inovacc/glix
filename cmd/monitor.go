package cmd

import (
	"bytes"

	"github.com/inovacc/goinstall/internal/monitor"

	"github.com/spf13/cobra"
)

// monitorCmd represents the monitor command
var monitorCmd = &cobra.Command{
	Use:   "monitor",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	RunE: monitor.Monitor,
}

func init() {
	monitorCmd.SetOut(new(bytes.Buffer))
	monitorCmd.SetErr(new(bytes.Buffer))

	rootCmd.AddCommand(monitorCmd)
}
