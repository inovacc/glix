package module

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/inovacc/glix/internal/database"
	pb "github.com/inovacc/glix/pkg/api/v1"
	"github.com/inovacc/glix/pkg/exec"
	"golang.org/x/mod/semver"
)

// dummyModuleName is the name used for temporary Go modules created during
// version resolution and dependency extraction. Using a dummy module allows
// us to run go commands like `go list -m -versions` and `go get` without
// polluting the user's workspace.
const dummyModuleName = "dummy"

// ProgressHandler is called to report progress during module operations
type ProgressHandler func(phase, message string)

type Module struct {
	ctx             context.Context
	goBinPath       string
	workingDir      string
	timeout         time.Duration
	goListPackage   []GoListPackage
	progressHandler ProgressHandler
	Time            time.Time    `json:"time"`
	Name            string       `json:"name"`
	RootModule      string       `json:"root_module"` // The actual Go module path (e.g., github.com/sqlc-dev/sqlc)
	Hash            string       `json:"hash"`
	Version         string       `json:"version"`
	Versions        []string     `json:"versions"`
	Dependencies    []Dependency `json:"dependencies"`
}

type Dependency struct {
	Name         string       `json:"name"`
	Hash         string       `json:"hash"`
	Version      string       `json:"version"`
	Versions     []string     `json:"versions"`
	Dependencies []Dependency `json:"dependencies,omitempty"`
}

type ListResp struct {
	Time     time.Time `json:"time"`
	Path     string    `json:"path"`
	Version  string    `json:"version"`
	Versions []string  `json:"versions,omitempty"`
}

func (l *ListResp) EmptyVersions() bool {
	return len(l.Versions) == 0
}

func NewModule(ctx context.Context, goBinPath, workingDir string) (*Module, error) {
	if err := validGoBinary(goBinPath); err != nil {
		return nil, err
	}

	return &Module{
		ctx:           ctx,
		goBinPath:     goBinPath,
		workingDir:    workingDir,
		goListPackage: make([]GoListPackage, 0),
		Dependencies:  make([]Dependency, 0),
	}, nil
}

// SetProgressHandler sets a callback for progress updates
func (m *Module) SetProgressHandler(handler ProgressHandler) {
	m.progressHandler = handler
}

// progress reports progress if a handler is set
func (m *Module) progress(phase, message string) {
	if m.progressHandler != nil {
		m.progressHandler(phase, message)
	}
}

// setupTempModule initializes a temporary Go module in the working directory.
// This creates a minimal go.mod file that allows running go commands for
// module resolution and dependency extraction.
func (m *Module) setupTempModule(ctx context.Context) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	absCwd, err := filepath.Abs(cwd)
	if err != nil {
		return err
	}

	absWorkingDir, err := filepath.Abs(m.workingDir)
	if err != nil {
		return err
	}

	if absWorkingDir != absCwd {
		cmd := exec.CommandContext(ctx, m.goBinPath, "mod", "init", dummyModuleName)
		cmd.Dir = m.workingDir

		return cmd.Run()
	}

	return nil
}

func (m *Module) FetchModuleInfo(module string) error {
	module = m.normalizeModulePath(module)

	ctx, cancel := context.WithTimeout(m.ctx, m.getTimeout())
	defer cancel()

	module, version := m.splitModuleVersion(module)
	m.Name = module

	// Setup dummy mod early for discovery
	m.progress("init", "Initializing workspace...")
	if err := m.setupTempModule(ctx); err != nil {
		return err
	}

	// Get versions from upstream (this also resolves the root module)
	m.progress("versions", "Fetching available versions...")
	result, err := m.fetchModuleVersions(ctx, module)
	if err != nil {
		return err
	}

	lr := result.ListResp
	rootModule := result.RootModule
	m.RootModule = rootModule // Store the root module for later use (e.g., go mod download)

	// Download the module first to check if it's installable
	m.progress("download", "Downloading module...")
	if err := m.getModule(ctx, fmt.Sprintf("%s@latest", module)); err != nil {
		return fmt.Errorf("failed to download module: %w", err)
	}

	// Check if the resolved module is installable (has package main)
	// If not, trigger smart detection to find CLI paths using the ROOT module
	m.progress("check", "Checking if module is installable...")
	if !m.hasPackageMain(ctx, module) {
		m.progress("discover", "Searching for CLI binaries...")
		fmt.Printf("Module %q found but is not installable (no main package), searching for CLIs...\n", module)

		// Use root module for discovery, not the user-provided path
		discovered, found, discErr := m.DiscoverCLIPaths(ctx, rootModule)
		if discErr == nil && found && len(discovered) > 0 {
			// Auto-select the first discovered CLI
			selectedCLI := discovered[0]

			if len(discovered) > 1 {
				fmt.Printf("Found %d installable CLIs, auto-selecting: %s\n", len(discovered), selectedCLI)
			} else {
				fmt.Printf("Found installable CLI: %s\n", selectedCLI)
			}

			module = selectedCLI
			m.Name = selectedCLI
		} else {
			return fmt.Errorf("module %q is not installable and no CLI paths were discovered", module)
		}
	}

	if version == "latest" {
		version = lr.Version
	}

	m.Versions = lr.Versions
	m.Version = m.pickVersion(version, lr.Versions)
	m.Time = time.Now()
	m.Hash = m.hashModule(fmt.Sprintf("%s@%s", module, version))

	// Install the target module in dummy with a specific version if different from the latest
	// (we already downloaded @latest above for the installability check)
	if version != "latest" && version != lr.Version {
		m.progress("download", fmt.Sprintf("Downloading %s...", version))
		if err := m.getModule(ctx, fmt.Sprintf("%s@%s", module, version)); err != nil {
			return err
		}
	}

	// Extract dependencies
	m.progress("deps", "Resolving dependencies...")
	m.Dependencies, err = m.extractDependencies(ctx, module)
	m.progress("done", "Module info fetched successfully")

	return err
}

// getModuleSourceDir downloads the module and returns its source directory
func (m *Module) getModuleSourceDir(ctx context.Context) (string, error) {
	// Use go mod download to get the module (must use root module path, not package path)
	modulePath := m.RootModule
	if modulePath == "" {
		modulePath = m.Name // Fallback for backwards compatibility
	}

	cmd := exec.CommandContext(ctx, m.goBinPath, "mod", "download", "-json", fmt.Sprintf("%s@%s", modulePath, m.Version))

	var out bytes.Buffer

	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("go mod download failed: %w", err)
	}

	var result struct {
		Dir string `json:"Dir"`
	}

	if err := json.NewDecoder(&out).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode download result: %w", err)
	}

	if result.Dir == "" {
		return "", fmt.Errorf("module directory not found in download result")
	}

	return result.Dir, nil
}

func (m *Module) ToJSON() ([]byte, error) {
	return json.MarshalIndent(m, "", "  ")
}

func (m *Module) SaveToFile(path string) error {
	data, err := m.ToJSON()
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func (m *Module) Report(db *database.Storage) error {
	// Convert Module struct to Protocol Buffer
	moduleProto := &pb.ModuleProto{
		Name:              m.Name,
		Version:           m.Version,
		Versions:          m.Versions,
		Dependencies:      convertDependenciesToProto(m.Dependencies),
		Hash:              m.Hash,
		TimestampUnixNano: m.Time.UnixNano(),
	}

	// Upsert module
	if err := db.UpsertModule(moduleProto); err != nil {
		return fmt.Errorf("failed to upsert module: %w", err)
	}

	// Upsert dependencies as a single entry
	if len(m.Dependencies) > 0 {
		depsProto := &pb.DependenciesProto{
			Dependencies: moduleProto.GetDependencies(),
		}

		if err := db.UpsertDependencies(m.Name, depsProto); err != nil {
			return fmt.Errorf("failed to upsert dependencies: %w", err)
		}
	}

	return nil
}

// convertDependenciesToProto converts []Dependency to []*pb.DependencyProto
func convertDependenciesToProto(deps []Dependency) []*pb.DependencyProto {
	if len(deps) == 0 {
		return nil
	}

	result := make([]*pb.DependencyProto, 0, len(deps))
	for _, dep := range deps {
		result = append(result, &pb.DependencyProto{
			Name:         dep.Name,
			Version:      dep.Version,
			Versions:     dep.Versions,
			Hash:         dep.Hash,
			Dependencies: convertDependenciesToProto(dep.Dependencies),
		})
	}

	return result
}

func LoadModuleFromFile(path string) (*Module, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var mod Module
	if err := json.Unmarshal(data, &mod); err != nil {
		return nil, err
	}

	return &mod, nil
}

func (m *Module) dependency(module string) (*Dependency, error) {
	tmpDir, err := os.MkdirTemp("", "go-list")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}

	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	ctx, cancel := context.WithTimeout(m.ctx, m.getTimeout())
	defer cancel()

	name, suffix := m.splitModuleVersion(module)

	result, err := m.fetchModuleVersions(ctx, name)
	if err != nil {
		return nil, err
	}

	version := m.pickVersion(suffix, result.ListResp.Versions)

	return &Dependency{
		Name:     name,
		Hash:     m.hashModule(fmt.Sprintf("%s@%s", name, version)),
		Version:  version,
		Versions: result.ListResp.Versions,
	}, nil
}

// tryFetchVersions attempts a single version fetch for a specific module path
func (m *Module) tryFetchVersions(ctx context.Context, module string) (*ListResp, error) {
	cmd := exec.CommandContext(ctx, m.goBinPath, "list", "-m", "-versions", "-json", fmt.Sprintf("%s@latest", module))
	cmd.Dir = m.workingDir

	var (
		lr  ListResp
		out bytes.Buffer
	)

	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return nil, err
	}

	if err := json.NewDecoder(&out).Decode(&lr); err != nil {
		return nil, err
	}

	// For modules with tagged versions, sort them by semver
	if !lr.EmptyVersions() {
		sort.Slice(lr.Versions, func(i, j int) bool {
			return semver.Compare(lr.Versions[i], lr.Versions[j]) > 0
		})

		return &lr, nil
	}

	// For modules without tags, use pseudo-version if available
	if lr.Version != "" {
		lr.Versions = []string{lr.Version}
		return &lr, nil
	}

	return nil, fmt.Errorf("no versions found")
}

// fetchModuleVersionsResult contains both the version info and the resolved root module path
type fetchModuleVersionsResult struct {
	ListResp   *ListResp
	RootModule string
}

func (m *Module) fetchModuleVersions(ctx context.Context, module string) (*fetchModuleVersionsResult, error) {
	original := module
	attempts := 0

	const maxAttempts = 5

	// PHASE 1: Try original path with backwards traversal
	for {
		cmd := exec.CommandContext(ctx, m.goBinPath, "list", "-m", "-versions", "-json", fmt.Sprintf("%s@latest", module))
		cmd.Dir = m.workingDir

		var (
			lr  ListResp
			out bytes.Buffer
		)

		cmd.Stdout = &out

		if err := cmd.Run(); err == nil {
			if err := json.NewDecoder(&out).Decode(&lr); err != nil {
				return nil, fmt.Errorf("decoding list response failed: %w", err)
			}

			// For modules with tagged versions
			if !lr.EmptyVersions() {
				sort.Slice(lr.Versions, func(i, j int) bool {
					return semver.Compare(lr.Versions[i], lr.Versions[j]) > 0
				})

				return &fetchModuleVersionsResult{ListResp: &lr, RootModule: module}, nil
			}

			// For modules without tags, use pseudo-version if available
			if lr.Version != "" {
				lr.Versions = []string{lr.Version}
				return &fetchModuleVersionsResult{ListResp: &lr, RootModule: module}, nil
			}
		}

		// Step back one path segment
		lastSlash := strings.LastIndex(module, "/")
		if lastSlash == -1 || attempts >= maxAttempts {
			break
		}

		module = module[:lastSlash]
		attempts++
	}

	// PHASE 2: Smart Detection - Discover CLI paths
	// Only trigger discovery for the original user input, not for dependencies
	// Check if the original path looks like a root module (not a deep import path)
	if strings.Count(original, "/") <= 2 || strings.Contains(original, "/cmd/") || strings.Contains(original, "/cli/") {
		fmt.Printf("Path %q not found, searching for installable CLIs...\n", original)

		discovered, found, err := m.DiscoverCLIPaths(ctx, original)
		if err != nil || !found {
			return nil, fmt.Errorf("failed to resolve module versions for %q (initially %q)", module, original)
		}

		if len(discovered) > 1 {
			fmt.Printf("Found %d installable CLIs, using first: %s\n", len(discovered), discovered[0])
		} else {
			fmt.Printf("Found installable CLI: %s\n", discovered[0])
		}

		// Try first discovered path to get versions
		if len(discovered) > 0 {
			lr, err := m.tryFetchVersions(ctx, discovered[0])
			if err != nil {
				return nil, err
			}

			return &fetchModuleVersionsResult{ListResp: lr, RootModule: module}, nil
		}
	}

	return nil, fmt.Errorf("failed to resolve module versions for %q (initially %q)", module, original)
}

func (m *Module) getModule(ctx context.Context, moduleWithVersion string) error {
	cmd := exec.CommandContext(ctx, m.goBinPath, "get", moduleWithVersion)
	cmd.Dir = m.workingDir

	return cmd.Run()
}

func (m *Module) getLatestModule(ctx context.Context, moduleName string) error {
	cmd := exec.CommandContext(ctx, m.goBinPath, "get", fmt.Sprintf("%s@latest", moduleName))
	cmd.Dir = m.workingDir

	return cmd.Run()
}

func (m *Module) extractDependencies(ctx context.Context, self string) ([]Dependency, error) {
	cmd := exec.CommandContext(ctx, m.goBinPath, "list", "-m", "all")
	cmd.Dir = m.workingDir

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go list -m all failed: %w", err)
	}

	seen := make(map[string]struct{}) // module name deduplication

	var deps []Dependency

	lines := strings.SplitSeq(string(out), "\n")
	for line := range lines {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}

		name := fields[0]
		if name == dummyModuleName || name == self {
			continue
		}

		if _, ok := seen[name]; ok {
			continue
		}

		seen[name] = struct{}{}

		dep, err := m.dependency(name)
		if err == nil {
			deps = append(deps, *dep)
		}
	}

	return deps, nil
}

func (m *Module) getTimeout() time.Duration {
	if m.timeout == 0 {
		return 5 * time.Minute // Increased timeout for large modules
	}

	return m.timeout
}

func (m *Module) splitModuleVersion(full string) (string, string) {
	parts := strings.SplitN(full, "@", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}

	return full, "latest"
}

func (m *Module) hashModule(input string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(input)))
}

func (m *Module) pickVersion(preferred string, versions []string) string {
	if len(versions) > 0 {
		return versions[0]
	}

	if preferred != "" {
		return preferred
	}

	return ""
}

func (m *Module) normalizeModulePath(input string) string {
	// Strip known prefixes
	prefixes := []string{
		"https://", "http://", "git://", "ssh://", "git@", "ssh@", "www.",
	}
	for _, p := range prefixes {
		if after, ok := strings.CutPrefix(input, p); ok {
			input = after
			break
		}
	}

	// Handle ssh-style git@github.com:user/repo.git
	if strings.Contains(input, ":") && strings.Contains(input, "@") {
		parts := strings.SplitN(input, ":", 2)
		if len(parts) == 2 {
			input = strings.ReplaceAll(parts[1], "\\", "/")
		}
	}

	// Trim trailing `.git`
	input = strings.ReplaceAll(input, ".git", "")

	// Final path cleanup
	return strings.Trim(input, "/")
}
