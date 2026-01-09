package module

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/inovacc/goinstall/pkg/exec"
)

// DiscoverCLIPaths attempts to find installable CLI paths when the root module fails
// Returns: list of candidate paths, whether discovery was needed, error
func (m *Module) DiscoverCLIPaths(ctx context.Context, dir, rootModule string) ([]string, bool, error) {
	var candidates []string

	// Method 1: Check for cmd/ directory
	cmdPaths := m.discoverFromCmdDir(ctx, dir, rootModule)
	candidates = append(candidates, cmdPaths...)

	// Method 2: Check for cli/ directory
	cliPaths := m.discoverFromCliDir(ctx, dir, rootModule)
	candidates = append(candidates, cliPaths...)

	// Method 3: Parse .goreleaser.yaml if exists
	grPaths := m.discoverFromGoReleaser(ctx, rootModule)
	candidates = append(candidates, grPaths...)

	// Remove duplicates
	seen := make(map[string]bool)

	var unique []string

	for _, path := range candidates {
		if !seen[path] {
			seen[path] = true
			unique = append(unique, path)
		}
	}

	return unique, len(unique) > 0, nil
}

// discoverFromCmdDir checks for cmd/* subdirectories
func (m *Module) discoverFromCmdDir(ctx context.Context, dir, rootModule string) []string {
	var paths []string

	// Try: go list -json rootModule/cmd/...
	cmd := exec.CommandContext(ctx, m.goBinPath, "list", "-json", fmt.Sprintf("%s/cmd/...", rootModule))
	cmd.Dir = dir

	var out bytes.Buffer

	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return paths // cmd/ doesn't exist
	}

	// Parse JSON output (one JSON object per line)
	decoder := json.NewDecoder(&out)

	for {
		var pkg struct {
			ImportPath string `json:"ImportPath"`
			Name       string `json:"Name"`
		}

		if err := decoder.Decode(&pkg); err != nil {
			break
		}

		// Only include if it's a command (has package main)
		if pkg.Name == "main" {
			paths = append(paths, pkg.ImportPath)
		}
	}

	return paths
}

// discoverFromCliDir checks for cli/* subdirectories
func (m *Module) discoverFromCliDir(ctx context.Context, dir, rootModule string) []string {
	var paths []string

	cmd := exec.CommandContext(ctx, m.goBinPath, "list", "-json", fmt.Sprintf("%s/cli/...", rootModule))
	cmd.Dir = dir

	var out bytes.Buffer

	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return paths
	}

	decoder := json.NewDecoder(&out)

	for {
		var pkg struct {
			ImportPath string `json:"ImportPath"`
			Name       string `json:"Name"`
		}

		if err := decoder.Decode(&pkg); err != nil {
			break
		}

		if pkg.Name == "main" {
			paths = append(paths, pkg.ImportPath)
		}
	}

	return paths
}

// discoverFromGoReleaser parses .goreleaser.yaml for build targets
func (m *Module) discoverFromGoReleaser(ctx context.Context, rootModule string) []string {
	var paths []string

	// Use go list to get module cache location
	cmd := exec.CommandContext(ctx, m.goBinPath, "list", "-m", "-json", fmt.Sprintf("%s@latest", rootModule))

	var out bytes.Buffer

	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return paths
	}

	var modInfo struct {
		Dir string `json:"Dir"`
	}

	if err := json.NewDecoder(&out).Decode(&modInfo); err != nil {
		return paths
	}

	// Read .goreleaser.yaml from module directory
	goreleaserPath := filepath.Join(modInfo.Dir, ".goreleaser.yaml")

	data, err := os.ReadFile(goreleaserPath)
	if err != nil {
		// Try .goreleaser.yml
		goreleaserPath = filepath.Join(modInfo.Dir, ".goreleaser.yml")

		data, err = os.ReadFile(goreleaserPath)
		if err != nil {
			return paths // No goreleaser config
		}
	}

	// Simple YAML parsing for builds section
	// Format: builds[].main or builds[].dir
	lines := strings.SplitSeq(string(data), "\n")
	for line := range lines {
		line = strings.TrimSpace(line)

		// Look for main: ./cmd/toolname
		if after, ok := strings.CutPrefix(line, "main:"); ok {
			mainPath := strings.TrimSpace(after)
			mainPath = strings.Trim(mainPath, `"'`)
			mainPath = strings.TrimPrefix(mainPath, "./")

			// Convert to full module path
			fullPath := filepath.Join(rootModule, mainPath)
			fullPath = strings.ReplaceAll(fullPath, "\\", "/")
			paths = append(paths, fullPath)
		}
	}

	return paths
}

// hasPackageMain verifies a path contains package main
func (m *Module) hasPackageMain(ctx context.Context, dir, path string) bool {
	cmd := exec.CommandContext(ctx, m.goBinPath, "list", "-json", path)
	cmd.Dir = dir

	var out bytes.Buffer

	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return false
	}

	var pkg struct {
		Name string `json:"Name"`
	}

	if err := json.NewDecoder(&out).Decode(&pkg); err != nil {
		return false
	}

	return pkg.Name == "main"
}
