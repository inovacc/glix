package module

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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
