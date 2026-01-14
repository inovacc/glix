package module

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	osExec "os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/inovacc/glix/pkg/exec"
)

// OutputHandler is called for each line of output from a command
type OutputHandler func(stream string, line string)

// ExecuteWithStreaming runs a command and streams its output to the handler
func ExecuteWithStreaming(ctx context.Context, handler OutputHandler, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		streamLines(stdoutPipe, "stdout", handler)
	}()

	go func() {
		defer wg.Done()
		streamLines(stderrPipe, "stderr", handler)
	}()

	wg.Wait()

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("command failed: %w", err)
	}

	return nil
}

func streamLines(r io.Reader, stream string, handler OutputHandler) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		if handler != nil {
			handler(stream, scanner.Text())
		}
	}
}

// DefaultOutputHandler prints output to stdout/stderr
func DefaultOutputHandler(stream string, line string) {
	if stream == "stderr" {
		_, _ = fmt.Fprintln(os.Stderr, line)
	} else {
		_, _ = fmt.Fprintln(os.Stdout, line)
	}
}

// InstallModuleWithStreaming installs a module with real-time output streaming
func (m *Module) InstallModuleWithStreaming(ctx context.Context, handler OutputHandler) error {
	// Download the module to check for .goreleaser.yaml
	moduleDir, err := m.getModuleSourceDir(ctx)
	if err != nil {
		return fmt.Errorf("failed to get module source: %w", err)
	}

	// Check if the module has a .goreleaser.yaml file
	hasGR, configPath, err := m.hasGoReleaserConfig(ctx, moduleDir)
	if err != nil {
		return fmt.Errorf("failed to check for goreleaser config: %w", err)
	}

	if hasGR {
		if handler != nil {
			handler("stdout", fmt.Sprintf("Found GoReleaser config: %s", configPath))
		}
		return m.installViaGoReleaserWithStreaming(ctx, moduleDir, handler)
	}

	// Standard go install with streaming
	modulePath := fmt.Sprintf("%s@%s", m.Name, m.Version)

	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		home, _ := os.UserHomeDir()
		gopath = fmt.Sprintf("%s/go", home)
	}

	// Set GOBIN environment variable
	gobin := fmt.Sprintf("%s/bin", gopath)

	cmd := exec.CommandContext(ctx, m.goBinPath, "install", modulePath)
	cmd.Env = append(os.Environ(), fmt.Sprintf("GOBIN=%s", gobin))

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start go install: %w", err)
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		streamLines(stdoutPipe, "stdout", handler)
	}()

	go func() {
		defer wg.Done()
		streamLines(stderrPipe, "stderr", handler)
	}()

	wg.Wait()

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("go install failed: %w", err)
	}

	return nil
}

// installViaGoReleaserWithStreaming builds and installs using GoReleaser with output streaming
func (m *Module) installViaGoReleaserWithStreaming(ctx context.Context, moduleDir string, handler OutputHandler) error {
	// Check if goreleaser is installed
	if _, err := osExec.LookPath("goreleaser"); err != nil {
		if handler != nil {
			handler("stdout", "GoReleaser not found, installing...")
		}
		if err := ExecuteWithStreaming(ctx, handler, m.goBinPath, "install", "github.com/goreleaser/goreleaser/v2@latest"); err != nil {
			return fmt.Errorf("failed to install goreleaser: %w", err)
		}
	}

	// Create a temporary build directory to avoid polluting the cache
	cacheDir, err := GetApplicationCacheDirectory()
	if err != nil {
		return fmt.Errorf("failed to get cache directory: %w", err)
	}

	buildDir := filepath.Join(cacheDir, "build")
	if err := copyDir(moduleDir, buildDir); err != nil {
		return fmt.Errorf("failed to copy module source: %w", err)
	}

	defer func() {
		_ = os.RemoveAll(buildDir)
	}()

	if handler != nil {
		handler("stdout", "Building with GoReleaser...")
	}

	// Build with goreleaser in the build directory
	cmd := exec.CommandContext(ctx, "goreleaser", "build", "--snapshot", "--clean")
	cmd.Dir = buildDir

	// Set environment variables
	env := os.Environ()
	parts := strings.Split(m.Name, "/")
	if len(parts) >= 2 {
		owner := parts[len(parts)-2]
		env = append(env, fmt.Sprintf("GITHUB_OWNER=%s", owner))
	}
	cmd.Env = env

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start goreleaser: %w", err)
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		streamLines(stdoutPipe, "stdout", handler)
	}()

	go func() {
		defer wg.Done()
		streamLines(stderrPipe, "stderr", handler)
	}()

	wg.Wait()

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("goreleaser build failed: %w", err)
	}

	if handler != nil {
		handler("stdout", "Build completed successfully")
	}

	// Find the built binary in the dist directory
	distDir := filepath.Join(buildDir, "dist")
	binaryPath, err := m.findBuiltBinary(distDir)
	if err != nil {
		return fmt.Errorf("failed to find built binary: %w", err)
	}

	// Copy binary to GOBIN
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		home, _ := os.UserHomeDir()
		gopath = filepath.Join(home, "go")
	}

	gobin := filepath.Join(gopath, "bin")

	// Ensure GOBIN directory exists
	if err := os.MkdirAll(gobin, 0755); err != nil {
		return fmt.Errorf("failed to create GOBIN directory: %w", err)
	}

	// Determine binary name from the module name
	binaryName := filepath.Base(m.Name)
	if runtime.GOOS == "windows" && !strings.HasSuffix(binaryName, ".exe") {
		binaryName += ".exe"
	}

	destPath := filepath.Join(gobin, binaryName)

	// Copy the binary to GOBIN
	if err := copyFile(binaryPath, destPath); err != nil {
		return fmt.Errorf("failed to copy binary to GOBIN: %w", err)
	}

	// Make it executable (Unix only)
	if runtime.GOOS != "windows" {
		if err := os.Chmod(destPath, 0755); err != nil {
			return fmt.Errorf("failed to make binary executable: %w", err)
		}
	}

	if handler != nil {
		handler("stdout", fmt.Sprintf("Binary installed to: %s", destPath))
	}

	return nil
}
