package module

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/inovacc/goinstall/pkg/exec"
)

// hasGoReleaserConfig checks if the module has a .goreleaser.yaml or .goreleaser.yml file
func (m *Module) hasGoReleaserConfig(ctx context.Context, moduleDir string) (bool, string, error) {
	configs := []string{".goreleaser.yaml", ".goreleaser.yml"}

	_ = ctx // context is not used in this function but included for consistency

	for _, config := range configs {
		configPath := filepath.Join(moduleDir, config)
		if _, err := os.Stat(configPath); err == nil {
			return true, configPath, nil
		}
	}

	return false, "", nil
}

// ensureGoReleaserInstalled checks if goreleaser is installed, and installs it if not
func (m *Module) ensureGoReleaserInstalled(ctx context.Context) error {
	// Check if goreleaser is already installed
	cmd := exec.CommandContext(ctx, "goreleaser", "--version")
	if err := cmd.Run(); err == nil {
		return nil // Already installed
	}

	fmt.Println("GoReleaser not found, installing...")

	// Install goreleaser
	installCmd := exec.CommandContext(ctx, m.goBinPath, "install", "github.com/goreleaser/goreleaser/v2@latest")

	if err := installCmd.Run(); err != nil {
		return fmt.Errorf("failed to install goreleaser: %w", err)
	}

	fmt.Println("GoReleaser installed successfully")

	return nil
}

// buildWithGoReleaser builds the module using goreleaser
func (m *Module) buildWithGoReleaser(ctx context.Context, moduleDir string) (string, error) {
	fmt.Println("Building with GoReleaser...")

	// Run goreleaser build --snapshot --clean
	cmd := exec.CommandContext(ctx, "goreleaser", "build", "--snapshot", "--clean")
	cmd.Dir = moduleDir

	// Set default environment variables that might be needed by .goreleaser.yaml
	env := os.Environ()

	// Extract owner from module name (e.g., github.com/owner/repo -> owner)
	parts := strings.Split(m.Name, "/")
	if len(parts) >= 2 {
		owner := parts[len(parts)-2]
		env = append(env, fmt.Sprintf("GITHUB_OWNER=%s", owner))
	}

	cmd.Env = env

	var out strings.Builder

	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("goreleaser build failed: %w\nOutput: %s", err, out.String())
	}

	fmt.Println("Build completed successfully")

	return out.String(), nil
}

// findBuiltBinary finds the built binary in the dist directory
func (m *Module) findBuiltBinary(distDir string) (string, error) {
	// Determine the expected binary pattern based on OS/ARCH
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	// Common patterns for goreleaser output
	patterns := []string{
		fmt.Sprintf("*_%s_%s*", goos, goarch),
		fmt.Sprintf("*_%s_%s_*", goos, goarch),
	}

	var foundBinary string

	if err := filepath.Walk(distDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// Skip checksum files, archives, etc.
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".txt" || ext == ".md" || ext == ".tar" || ext == ".gz" || ext == ".zip" {
			return nil
		}

		// Check if the path matches our platform
		fileName := info.Name()
		for _, pattern := range patterns {
			matched, _ := filepath.Match(pattern, fileName)
			if matched {
				foundBinary = path
				return filepath.SkipAll
			}
		}

		// On Windows, also check for .exe files
		if goos == "windows" && ext == ".exe" {
			foundBinary = path
			return filepath.SkipAll
		}

		// On Unix, check for executable files without extension
		if goos != "windows" && ext == "" && info.Mode()&0111 != 0 {
			foundBinary = path
			return filepath.SkipAll
		}

		return nil
	}); err != nil {
		return "", fmt.Errorf("error searching for binary: %w", err)
	}

	if foundBinary == "" {
		return "", fmt.Errorf("no binary found for %s/%s in dist directory", goos, goarch)
	}

	return foundBinary, nil
}

// installViaGoReleaser installs a module by building it with goreleaser
func (m *Module) installViaGoReleaser(ctx context.Context, moduleDir string) error {
	// Ensure goreleaser is installed
	if err := m.ensureGoReleaserInstalled(ctx); err != nil {
		return err
	}

	// Copy module to a temporary writable directory (module cache is read-only)
	tmpDir, err := os.MkdirTemp("", "goinstall-build-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}

	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	buildDir := filepath.Join(tmpDir, "build")
	if err := copyDir(moduleDir, buildDir); err != nil {
		return fmt.Errorf("failed to copy module source: %w", err)
	}

	// Build with goreleaser
	if _, err := m.buildWithGoReleaser(ctx, buildDir); err != nil {
		return err
	}

	// Find the built binary
	distDir := filepath.Join(buildDir, "dist")

	binaryPath, err := m.findBuiltBinary(distDir)
	if err != nil {
		return err
	}

	fmt.Printf("Found binary: %s\n", filepath.Base(binaryPath))

	// Get GOBIN path
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		gopath = filepath.Join(os.Getenv("HOME"), "go")
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

	fmt.Printf("Binary installed to: %s\n", destPath)

	return nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}

	defer func() {
		_ = sourceFile.Close()
	}()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}

	defer func() {
		_ = destFile.Close()
	}()

	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return err
	}

	return destFile.Sync()
}

// copyDir recursively copies a directory from src to dst
func copyDir(src, dst string) error {
	// Get source directory info
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("failed to stat source directory: %w", err)
	}

	// Create destination directory
	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Read source directory entries
	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("failed to read source directory: %w", err)
	}

	// Copy each entry
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			// Recursively copy subdirectory
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			// Copy file
			if err := copyFile(srcPath, dstPath); err != nil {
				return fmt.Errorf("failed to copy file %s: %w", entry.Name(), err)
			}
		}
	}

	return nil
}
