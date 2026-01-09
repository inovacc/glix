# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

glix is a CLI wrapper around `go install` that adds BoltDB-based tracking for installed Go modules using Protocol Buffers, with features for installing, removing, and planned support for monitoring updates and auto-updating modules.

## Build and Development Commands

### Task Runner
This project uses [Task](https://taskfile.dev/) for all build operations. Run `task` or `task --list` to see available commands.

**Essential commands:**
- `task generate` - Generate version info and protobuf code (required before builds)
- `task proto:generate` - Generate Protocol Buffer code from proto definitions
- `task build:dev` - Build development snapshot with goreleaser
- `task test` - Run tests with race detection and coverage
- `task test:unit` - Run unit tests only (skip integration tests)
- `task lint` - Run golangci-lint
- `task lint:fix` - Auto-fix linting issues
- `task check` - Run all quality checks (fmt, vet, lint, test)
- `task deps` - Download, tidy, and verify dependencies
- `task deps:upgrade` - Upgrade all dependencies to latest versions
- `task clean` - Remove build artifacts, coverage files, and generated version files

**Release commands:**
- `task release:check` - Validate goreleaser configuration
- `task release:snapshot` - Create snapshot release without git tag
- `task release` - Create production release (requires git tag)

### Direct Go Commands
- `go install` - Install the CLI to `$GOPATH/bin`
- `go test -short ./...` - Run unit tests (skips slow integration tests)
- `go test ./internal/module` - Run tests for specific package

### Version Generation
Version information is auto-generated using [genversioninfo](https://github.com/inovacc/genversioninfo):
- Generates `.go-version` file (build metadata)
- Generates `cmd/version.go` (version command for Cobra CLI)
- Invoked via `go generate` or `task generate`
- Required before building releases

## Architecture

### Core Components

**Cobra CLI Structure** (`cmd/`)
- `root.go` - Main command with shorthand install support
- `install.go` - Install command for modules
- `remove.go` - Remove command for uninstalling modules
- `update.go` - Update command (stub, not yet implemented)
- `monitor.go` - Monitor installed modules for updates (in development)
- `report.go` - Generate reports on installed modules (in development)
- `version.go` - Auto-generated version command

**Module Management** (`internal/module/`)
- `module.go` - Core module operations: fetch version info, resolve dependencies, install, track in database
- Uses temporary Go modules (dummy module technique) to resolve dependencies without polluting user's workspace
- Normalizes various URL formats (https, git, ssh) to canonical Go module paths
- Version resolution: fetches all versions, sorts via semver, picks latest or user-specified version
- Dependency extraction: runs `go list -m all` in temp module to discover transitive dependencies

**Database Layer** (`internal/database/`)
- BoltDB embedded key-value database via `go.etcd.io/bbolt` (pure Go, no CGO)
- Protocol Buffer serialization via `google.golang.org/protobuf`
- Bucket structure:
  - `modules` - Stores ModuleProto with composite key (name@version)
  - `dependencies` - Stores DependenciesProto keyed by module name
  - `indexes_by_time` - Time-based secondary index for chronological queries
  - `indexes_by_name` - Name-based secondary index for version lookups
- Database location varies by OS (see Database Path section)
- Comprehensive test coverage in `storage_test.go` (18 tests)

**Installer** (`internal/installer/`)
- Orchestrates the installation flow: create DB connection → fetch module info → install → record to database
- Uses `afero.Fs` for filesystem abstraction (enables testing)

### Database Path Resolution

Database location is platform-specific (see `internal/config/config.go`):
- **Windows**: `%LOCALAPPDATA%\glix\modules.db`
- **macOS**: `~/Library/Application Support/glix/modules.db`
- **Linux/Unix**: `$XDG_DATA_HOME/glix/modules.db` (defaults to `~/.local/share/glix/modules.db`)
- **Override**: Set `GLIX_DB_PATH` environment variable

### Module Installation Flow

1. Parse and normalize module path (strip protocols, .git suffix)
2. Create temporary directory with dummy Go module
3. Fetch all available versions from Go proxy using `go list -m -versions -json`
4. Pick version (latest or user-specified via `@version` syntax)
5. Run `go get module@version` in temp module to resolve dependencies
6. Extract dependencies via `go list -m all`
7. Execute `go install module@version` to user's `$GOPATH/bin` (or use GoReleaser if `.goreleaser.yaml` exists)
8. Convert module and dependencies to Protocol Buffer format
9. Store in BoltDB with automatic index updates (time and name)
10. Clean up temporary directory

### Key Dependencies

- **Cobra** - CLI framework for commands and flags
- **BoltDB (bbolt)** - Embedded key-value database (`go.etcd.io/bbolt`)
- **Protocol Buffers** - Binary serialization (`google.golang.org/protobuf`)
- **afero** - Filesystem abstraction (enables in-memory testing)
- **golang.org/x/mod/semver** - Semantic version comparison

### Testing Strategy

- Uses `afero.MemMapFs` for filesystem isolation in tests
- Module tests use real `go` binary but operate in temp directories
- Database tests use temporary BoltDB files with automatic cleanup
- Storage layer has comprehensive test coverage (18 tests covering CRUD, indexes, concurrency)
- Run `go test -short` to skip slow integration tests
- Run `go test ./internal/database/` for database-specific tests

### Roadmap Status

Per README.md:
- [x] Install module - Implemented with smart CLI discovery and GoReleaser support
- [x] Remove command - Implemented
- [x] BoltDB migration - Complete with Protocol Buffers
- [ ] Update command - Stub exists, not implemented
- [ ] Report command - Stub exists, not implemented
- [ ] Monitoring - Stub exists, not implemented
- [ ] Auto update - Not started

## Configuration

### Build Configuration
- **goreleaser** - Currently builds Linux/amd64 only (Windows commented out in `.goreleaser.yaml`)
- **golangci-lint** - Extensive linter configuration with many rules enabled/disabled in `.golangci.yml`

### Environment Variables
- `GLIX_DB_PATH` - Override default database location
- `GITHUB_OWNER` - Used by Task/goreleaser (defaults to "dyammarcano")

## Common Gotchas

1. **Version info is empty**: Run `task generate` or `go generate ./...` before building
2. **Database not found**: Ensure parent directories are created (handled automatically by code)
3. **Module path normalization**: The tool accepts various URL formats but normalizes to Go module path format
4. **Dependency resolution**: Uses temporary "dummy" module to avoid polluting user's go.mod
