package module

import (
	"context"
	"testing"
)

func TestDiscoverFromCmdDir(t *testing.T) {
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := NewModule(context.TODO(), "go")
			if err != nil {
				t.Fatalf("NewModule() error = %v", err)
			}

			paths := m.discoverFromCmdDir(context.TODO(), tt.rootModule)

			if len(paths) != len(tt.wantPaths) {
				t.Errorf("discoverFromCmdDir() got %d paths, want %d", len(paths), len(tt.wantPaths))
			}

			// Check if expected paths are in the results
			for _, wantPath := range tt.wantPaths {
				found := false
				for _, path := range paths {
					if path == wantPath {
						found = true
						break
					}
				}
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
			m, err := NewModule(context.TODO(), "go")
			if err != nil {
				t.Fatalf("NewModule() error = %v", err)
			}

			paths := m.discoverFromCliDir(context.TODO(), tt.rootModule)

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
			m, err := NewModule(context.TODO(), "go")
			if err != nil {
				t.Fatalf("NewModule() error = %v", err)
			}

			hasMain := m.hasPackageMain(context.TODO(), tt.path)

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
			m, err := NewModule(context.TODO(), "go")
			if err != nil {
				t.Fatalf("NewModule() error = %v", err)
			}

			paths, found, err := m.DiscoverCLIPaths(context.TODO(), tt.rootModule)
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

	// Test: goinstall github.com/inovacc/brdoc
	// Should auto-discover: github.com/inovacc/brdoc/cmd/brdoc

	m, err := NewModule(context.TODO(), "go")
	if err != nil {
		t.Fatalf("NewModule() error = %v", err)
	}

	err = m.FetchModuleInfo("github.com/inovacc/brdoc")
	if err != nil {
		t.Fatalf("FetchModuleInfo() error = %v", err)
	}

	discovered := m.GetDiscoveredPaths()
	if len(discovered) == 0 {
		t.Fatal("Expected to discover CLI paths")
	}

	expectedPath := "github.com/inovacc/brdoc/cmd/brdoc"
	found := false
	for _, path := range discovered {
		if path == expectedPath {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Expected to find %s in discovered paths, got: %v", expectedPath, discovered)
	}
}

func TestPromptCLISelection(t *testing.T) {
	tests := []struct {
		name      string
		paths     []string
		wantErr   bool
		wantPaths int
	}{
		{
			name:      "empty paths",
			paths:     []string{},
			wantErr:   true,
			wantPaths: 0,
		},
		{
			name:      "single path - auto select",
			paths:     []string{"github.com/user/repo/cmd/tool"},
			wantErr:   false,
			wantPaths: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip the multiple paths test since it requires user input
			if len(tt.paths) > 1 {
				t.Skip("Skipping test that requires user input")
			}

			selected, err := PromptCLISelection(tt.paths)

			if (err != nil) != tt.wantErr {
				t.Errorf("PromptCLISelection() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && len(selected) != tt.wantPaths {
				t.Errorf("PromptCLISelection() got %d paths, want %d", len(selected), tt.wantPaths)
			}
		})
	}
}
