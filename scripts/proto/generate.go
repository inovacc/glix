package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	protoDir := "proto/v1"
	outDir := "pkg/api/v1"

	fmt.Println("Generating protobuf code...")

	// Create an output directory if it doesn't exist
	if err := os.MkdirAll(outDir, 0755); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Failed to create output directory: %v\n", err)

		os.Exit(1)
	}

	// Proto files to generate
	protoFiles, err := filepath.Glob(filepath.Join(protoDir, "*.proto"))
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Failed to find proto files: %v\n", err)
	}

	// Build protoc command
	args := []string{
		"--go_out=.",
		"--go_opt=module=github.com/inovacc/glix",
		"--go-grpc_out=.",
		"--go-grpc_opt=module=github.com/inovacc/glix",
		"--proto_path=.",
	}
	args = append(args, protoFiles...)

	cmd := exec.Command("protoc", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "")
		_, _ = fmt.Fprintln(os.Stderr, "ERROR: Proto generation failed")
		_, _ = fmt.Fprintln(os.Stderr, "")
		_, _ = fmt.Fprintln(os.Stderr, "Make sure you have protoc installed:")
		_, _ = fmt.Fprintln(os.Stderr, "  Download from: https://github.com/protocolbuffers/protobuf/releases")
		_, _ = fmt.Fprintln(os.Stderr, "")
		_, _ = fmt.Fprintln(os.Stderr, "And install Go plugins:")
		_, _ = fmt.Fprintln(os.Stderr, "  go install google.golang.org/protobuf/cmd/protoc-gen-go@latest")
		_, _ = fmt.Fprintln(os.Stderr, "  go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest")

		os.Exit(1)
	}

	fmt.Println("")
	fmt.Printf("Proto files generated successfully in %s\n", outDir)
}
