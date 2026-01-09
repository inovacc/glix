package module

import (
	"context"
	"fmt"
	"os"
	"slices"
	"testing"

	"github.com/inovacc/glix/pkg/exec"
)

func TestDiscoverFromCmdDir(t *testing.T) {
	exec.SetCommandDebug(true)

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tests := []struct {
		name       string
		rootModule string
		wantPaths  []string
		wantErr    bool
	}{
		{
			name:       "single CLI in cmd - brdoc",
			rootModule: "github.com/inovacc/brdoc",
			wantPaths:  []string{"github.com/inovacc/brdoc/cmd/brdoc"},
			wantErr:    false,
		},
		{
			name:       "module without cmd directory",
			rootModule: "github.com/pkg/errors",
			wantPaths:  []string{},
			wantErr:    false,
		},
		{
			name:       "module without cmd directory",
			rootModule: "github.com/goreleaser/goreleaser",
			wantPaths:  []string{},
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp directory
			tmpDir, err := os.MkdirTemp("", "discovery-test")
			if err != nil {
				t.Fatalf("MkdirTemp() error = %v", err)
			}
			defer func(path string) {
				if err := os.RemoveAll(path); err != nil {
					t.Fatalf("RemoveAll() error = %v", err)
				}
			}(tmpDir)

			m, err := NewModule(context.TODO(), "go", tmpDir)
			if err != nil {
				t.Fatalf("NewModule() error = %v", err)
			}

			// Setup temp module and get the module for testing
			ctx := context.TODO()
			if err := m.setupTempModule(ctx); err != nil {
				t.Fatalf("setupTempModule() error = %v", err)
			}

			_ = m.getModule(ctx, fmt.Sprintf("%s@latest", tt.rootModule))

			paths := m.discoverFromCmdDir(ctx, tmpDir, tt.rootModule)

			if len(paths) != len(tt.wantPaths) {
				t.Errorf("discoverFromCmdDir() got %d paths, want %d", len(paths), len(tt.wantPaths))
			}

			// Check if expected paths are in the results
			for _, wantPath := range tt.wantPaths {
				found := slices.Contains(paths, wantPath)

				if !found {
					t.Errorf("discoverFromCmdDir() missing expected path %s", wantPath)
				}
			}
		})
	}
}

func TestDiscoverFromCliDir(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tests := []struct {
		name       string
		rootModule string
		wantPaths  []string
	}{
		{
			name:       "module without cli directory",
			rootModule: "github.com/pkg/errors",
			wantPaths:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp directory
			tmpDir, err := os.MkdirTemp("", "discovery-test")
			if err != nil {
				t.Fatalf("MkdirTemp() error = %v", err)
			}
			defer func(path string) {
				if err := os.RemoveAll(path); err != nil {
					t.Fatalf("RemoveAll() error = %v", err)
				}
			}(tmpDir)

			m, err := NewModule(context.TODO(), "go", tmpDir)
			if err != nil {
				t.Fatalf("NewModule() error = %v", err)
			}

			// Setup temp module
			ctx := context.TODO()
			if err := m.setupTempModule(ctx); err != nil {
				t.Fatalf("setupTempModule() error = %v", err)
			}

			_ = m.getModule(ctx, fmt.Sprintf("%s@latest", tt.rootModule))

			paths := m.discoverFromCliDir(ctx, tmpDir, tt.rootModule)

			if len(paths) != len(tt.wantPaths) {
				t.Errorf("discoverFromCliDir() got %d paths, want %d", len(paths), len(tt.wantPaths))
			}
		})
	}
}

func TestHasPackageMain(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tests := []struct {
		name     string
		path     string
		wantMain bool
	}{
		{
			name:     "actual main package",
			path:     "github.com/inovacc/brdoc/cmd/brdoc",
			wantMain: true,
		},
		{
			name:     "library package",
			path:     "github.com/pkg/errors",
			wantMain: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp directory
			tmpDir, err := os.MkdirTemp("", "discovery-test")
			if err != nil {
				t.Fatalf("MkdirTemp() error = %v", err)
			}
			defer func(path string) {
				if err := os.RemoveAll(path); err != nil {
					t.Fatalf("RemoveAll() error = %v", err)
				}
			}(tmpDir)

			m, err := NewModule(context.TODO(), "go", tmpDir)
			if err != nil {
				t.Fatalf("NewModule() error = %v", err)
			}

			// Setup temp module
			ctx := context.TODO()
			if err := m.setupTempModule(ctx); err != nil {
				t.Fatalf("setupTempModule() error = %v", err)
			}

			_ = m.getModule(ctx, fmt.Sprintf("%s@latest", tt.path))

			hasMain := m.hasPackageMain(ctx, tt.path)

			if hasMain != tt.wantMain {
				t.Errorf("hasPackageMain() = %v, want %v", hasMain, tt.wantMain)
			}
		})
	}
}

func TestDiscoverCLIPaths(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tests := []struct {
		name       string
		rootModule string
		wantFound  bool
		minPaths   int
	}{
		{
			name:       "brdoc - should find cmd/brdoc",
			rootModule: "github.com/inovacc/brdoc",
			wantFound:  true,
			minPaths:   1,
		},
		{
			name:       "library package - no CLIs",
			rootModule: "github.com/pkg/errors",
			wantFound:  false,
			minPaths:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp directory for discovery
			tmpDir, err := os.MkdirTemp("", "discovery-test")
			if err != nil {
				t.Fatalf("MkdirTemp() error = %v", err)
			}
			defer func(path string) {
				if err := os.RemoveAll(path); err != nil {
					t.Fatalf("RemoveAll() error = %v", err)
				}
			}(tmpDir)

			m, err := NewModule(context.TODO(), "go", tmpDir)
			if err != nil {
				t.Fatalf("NewModule() error = %v", err)
			}

			// Setup temp module
			ctx := context.TODO()
			if err := m.setupTempModule(ctx); err != nil {
				t.Fatalf("setupTempModule() error = %v", err)
			}

			// Get module for discovery
			if err := m.getModule(ctx, fmt.Sprintf("%s@latest", tt.rootModule)); err != nil {
				t.Logf("getModule() error (may be expected): %v", err)
			}

			paths, found, err := m.DiscoverCLIPaths(ctx, tt.rootModule)
			if err != nil {
				t.Fatalf("DiscoverCLIPaths() error = %v", err)
			}

			if found != tt.wantFound {
				t.Errorf("DiscoverCLIPaths() found = %v, want %v", found, tt.wantFound)
			}

			if len(paths) < tt.minPaths {
				t.Errorf("DiscoverCLIPaths() got %d paths, want at least %d", len(paths), tt.minPaths)
			}
		})
	}
}

func TestSmartDetection_BrdocExample(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Test: glix github.com/inovacc/brdoc
	// Should auto-discover: github.com/inovacc/brdoc/cmd/brdoc

	m, err := NewModule(context.TODO(), "go", "")
	if err != nil {
		t.Fatalf("NewModule() error = %v", err)
	}

	err = m.FetchModuleInfo("github.com/inovacc/brdoc")
	if err != nil {
		t.Fatalf("FetchModuleInfo() error = %v", err)
	}

	// After discovery, the module name should be updated to the discovered CLI path
	expectedPath := "github.com/inovacc/brdoc/cmd/brdoc"
	if m.Name != expectedPath {
		t.Errorf("Expected module name to be %s, got %s", expectedPath, m.Name)
	}
}
