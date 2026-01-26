//go:build ignore

// Command cmdtree generates a tree visualization of all Cobra commands.
// Run with: go run scripts/cmdtree/cmdtree.go
// Or with: task cmdtree
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/inovacc/clonr/cmd"
	"github.com/spf13/cobra"
)

// ASCII tree characters for consistent width across all terminals
const (
	treeMiddle = "+-- "
	treeLast   = "\\-- "
	treeIndent = "|   "
	treeSpace  = "    "
)

var (
	outputFile  string
	showHidden  bool
	maxDescLen  int
	includeHelp bool
	commentCol  int
)

func init() {
	flag.StringVar(&outputFile, "output", "", "Output file (default: stdout)")
	flag.StringVar(&outputFile, "o", "", "Output file (shorthand)")
	flag.BoolVar(&showHidden, "hidden", false, "Show hidden commands")
	flag.IntVar(&maxDescLen, "desc-len", 40, "Maximum description length")
	flag.BoolVar(&includeHelp, "include-help", false, "Include help/completion commands")
	flag.IntVar(&commentCol, "comment-col", 45, "Column position for comments")
}

func main() {
	flag.Parse()

	var w io.Writer = os.Stdout

	if outputFile != "" {
		f, err := os.Create(outputFile)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error creating file: %v\n", err)
			os.Exit(1)
		}

		defer func() {
			_ = f.Close()
		}()

		w = f
	}

	rootCmd := cmd.GetRootCmd()

	_, _ = fmt.Fprintln(w, "# Command Tree")
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, "```")
	_, _ = fmt.Fprintln(w, rootCmd.Use)
	printCommands(w, rootCmd.Commands(), "")
	_, _ = fmt.Fprintln(w, "```")

	if outputFile != "" {
		_, _ = fmt.Fprintf(os.Stderr, "Command tree written to %s\n", outputFile)
	}
}

func printCommands(w io.Writer, commands []*cobra.Command, prefix string) {
	// Filter commands
	var visible []*cobra.Command

	for _, c := range commands {
		// Skip help and completion unless requested
		if !includeHelp && (c.Name() == "help" || c.Name() == "completion") {
			continue
		}

		// Skip hidden commands unless requested
		if !showHidden && c.Hidden {
			continue
		}

		visible = append(visible, c)
	}

	for i, c := range visible {
		isLast := i == len(visible)-1

		connector := treeMiddle
		if isLast {
			connector = treeLast
		}

		// Build description
		desc := c.Short
		if desc == "" {
			desc = c.Long
		}

		if len(desc) > maxDescLen {
			desc = desc[:maxDescLen-3] + "..."
		}

		// Build the command part (prefix + connector + name)
		cmdPart := prefix + connector + c.Name()

		// Calculate padding (ASCII chars = 1 byte = 1 column)
		padding := commentCol - len(cmdPart)
		if padding < 2 {
			padding = 2
		}

		// Print command with aligned comment
		_, _ = fmt.Fprintf(w, "%s%s# %s\n", cmdPart, strings.Repeat(" ", padding), desc)

		// Print subcommands
		if len(c.Commands()) > 0 {
			newPrefix := prefix + treeIndent
			if isLast {
				newPrefix = prefix + treeSpace
			}

			printCommands(w, c.Commands(), newPrefix)
		}
	}
}
