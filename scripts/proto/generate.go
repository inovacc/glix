package main

import (
	"fmt"
	"os"
	"os/exec"
)

func main() {
	_, _ = fmt.Fprintln(os.Stdout, "Generating protobuf code with buf...")

	// Run buf generate
	cmd := exec.Command("buf", "generate")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "")
		_, _ = fmt.Fprintln(os.Stderr, "ERROR: Proto generation failed")
		_, _ = fmt.Fprintln(os.Stderr, "")
		_, _ = fmt.Fprintln(os.Stderr, "Make sure you have buf installed:")
		_, _ = fmt.Fprintln(os.Stderr, "  go install github.com/bufbuild/buf/cmd/buf@latest")

		os.Exit(1)
	}

	_, _ = fmt.Fprintln(os.Stdout, "")
	_, _ = fmt.Fprintln(os.Stdout, "Proto files generated successfully in pkg/api")
}
