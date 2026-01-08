package module

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/inovacc/goinstall/internal/database"
	"github.com/spf13/afero"
	"golang.org/x/mod/semver"
)

const dummyModuleName = "dummy"

type Module struct {
	ctx          context.Context
	fs           afero.Fs
	goBinPath    string
	timeout      time.Duration
	Time         time.Time    `json:"time"`
	Name         string       `json:"name"`
	Hash         string       `json:"hash"`
	Version      string       `json:"version"`
	Versions     []string     `json:"versions"`
	Dependencies []Dependency `json:"dependencies"`
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

func NewModule(ctx context.Context, afs afero.Fs, goBinPath string) (*Module, error) {
	if err := validGoBinary(goBinPath); err != nil {
		return nil, err
	}
	return &Module{
		ctx:          ctx,
		fs:           afs,
		goBinPath:    goBinPath,
		Dependencies: make([]Dependency, 0),
	}, nil
}

func (m *Module) FetchModuleInfo(module string) error {
	tmpDir, err := afero.TempDir(m.fs, "", "go-list")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer func(fs afero.Fs, path string) {
		_ = m.fs.RemoveAll(tmpDir)
	}(m.fs, tmpDir)

	module = m.normalizeModulePath(module)

	ctx, cancel := context.WithTimeout(m.ctx, m.getTimeout())
	defer cancel()

	module, version := m.splitModuleVersion(module)
	m.Name = module

	// Get versions from upstream
	lr, err := m.fetchModuleVersions(ctx, tmpDir, module)
	if err != nil {
		return err
	}

	if version == "latest" {
		version = lr.Version
	}

	m.Versions = lr.Versions
	m.Version = m.pickVersion(version, lr.Versions)
	m.Time = time.Now()
	m.Hash = m.hashModule(fmt.Sprintf("%s@%s", module, version))

	// Setup dummy mod
	if err := m.setupTempModule(ctx, tmpDir); err != nil {
		return err
	}

	// Install target module in dummy
	if err := m.getModule(ctx, tmpDir, fmt.Sprintf("%s@%s", module, version)); err != nil {
		return err
	}

	// Extract dependencies
	m.Dependencies, err = m.extractDependencies(ctx, tmpDir, module)
	return err
}

func (m *Module) InstallModule(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, m.goBinPath, "install", fmt.Sprintf("%s@%s", m.Name, m.Version))

	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		gopath = filepath.Join(os.Getenv("HOME"), "go")
	}
	cmd.Env = append(os.Environ(), fmt.Sprintf("GOBIN=%s/bin", gopath))

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go install failed: %w", err)
	}
	return nil
}

func (m *Module) ToJSON() ([]byte, error) {
	return json.MarshalIndent(m, "", "  ")
}

func (m *Module) SaveToFile(path string) error {
	data, err := m.ToJSON()
	if err != nil {
		return err
	}
	return afero.WriteFile(m.fs, path, data, 0644)
}

func (m *Module) Report(db *database.Database) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
				log.Println("rollback failed:", err)
			}
		}
	}()

	// Serialize versions and dependencies to JSON
	versionsJSON, err := json.Marshal(m.Versions)
	if err != nil {
		return fmt.Errorf("failed to marshal versions: %w", err)
	}

	depsJSON, err := json.Marshal(m.Dependencies)
	if err != nil {
		return fmt.Errorf("failed to marshal dependencies: %w", err)
	}

	query := `
		INSERT INTO modules (name, version, versions, dependencies, hash, time)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(name, version) DO UPDATE
		SET hash = excluded.hash,
			time = excluded.time,
			versions = excluded.versions,
			dependencies = excluded.dependencies
		`
	if _, err := tx.Exec(query, m.Name, m.Version, versionsJSON, depsJSON, m.Hash, m.Time); err != nil {
		return fmt.Errorf("failed to insert module: %w", err)
	}

	depStmt := `
		INSERT INTO dependencies (module_name, dep_name, dep_version, dep_hash)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(module_name, dep_name) DO UPDATE
		SET dep_version = excluded.dep_version,
			dep_hash = excluded.dep_hash
		`

	for _, d := range m.Dependencies {
		if _, err := tx.Exec(depStmt, m.Name, d.Name, d.Version, d.Hash); err != nil {
			return fmt.Errorf("failed to insert dependency: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	committed = true
	return nil
}

func LoadModuleFromFile(fs afero.Fs, path string) (*Module, error) {
	data, err := afero.ReadFile(fs, path)
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
	tmpDir, err := afero.TempDir(m.fs, "", "go-list")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer func(fs afero.Fs, path string) {
		_ = m.fs.RemoveAll(tmpDir)
	}(m.fs, tmpDir)

	ctx, cancel := context.WithTimeout(m.ctx, m.getTimeout())
	defer cancel()

	name, suffix := m.splitModuleVersion(module)

	lr, err := m.fetchModuleVersions(ctx, tmpDir, name)
	if err != nil {
		return nil, err
	}

	version := m.pickVersion(suffix, lr.Versions)
	return &Dependency{
		Name:     name,
		Hash:     m.hashModule(fmt.Sprintf("%s@%s", name, version)),
		Version:  version,
		Versions: lr.Versions,
	}, nil
}

func (m *Module) fetchModuleVersions(ctx context.Context, dir, module string) (*ListResp, error) {
	original := module
	attempts := 0
	const maxAttempts = 5

	for {
		cmd := exec.CommandContext(ctx, m.goBinPath, "list", "-m", "-versions", "-json", fmt.Sprintf("%s@latest", module))
		cmd.Dir = dir

		var lr ListResp
		var out bytes.Buffer
		cmd.Stdout = &out

		if err := cmd.Run(); err == nil {
			if err := json.NewDecoder(&out).Decode(&lr); err != nil {
				return nil, fmt.Errorf("decoding list response failed: %w", err)
			}

			if len(lr.Versions) > 0 {
				sort.Slice(lr.Versions, func(i, j int) bool {
					return semver.Compare(lr.Versions[i], lr.Versions[j]) > 0
				})
				return &lr, nil
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

	return nil, fmt.Errorf("failed to resolve module versions for %q (initially %q)", module, original)
}

func (m *Module) setupTempModule(ctx context.Context, dir string) error {
	cmd := exec.CommandContext(ctx, m.goBinPath, "mod", "init", dummyModuleName)
	cmd.Dir = dir
	return cmd.Run()
}

func (m *Module) getModule(ctx context.Context, dir, moduleWithVersion string) error {
	cmd := exec.CommandContext(ctx, m.goBinPath, "get", moduleWithVersion)
	cmd.Dir = dir
	return cmd.Run()
}

func (m *Module) extractDependencies(ctx context.Context, dir, self string) ([]Dependency, error) {
	cmd := exec.CommandContext(ctx, m.goBinPath, "list", "-m", "all")
	cmd.Dir = dir

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go list -m all failed: %w", err)
	}

	seen := make(map[string]struct{}) // module name deduplication
	var deps []Dependency
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
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
		return 10 * time.Second
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
		if strings.HasPrefix(input, p) {
			input = strings.TrimPrefix(input, p)
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
