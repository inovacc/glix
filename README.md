# Golang Installer [![Test](https://github.com/inovacc/goinstall/actions/workflows/test.yml/badge.svg?branch=main)](https://github.com/inovacc/goinstall/actions/workflows/test.yml)

A smart wrapper around `go install` with SQLite-based module tracking, automatic CLI discovery, and GoReleaser build support.

## Features

- **Smart CLI Discovery**: Automatically detects installable CLI tools in `cmd/` and `cli/` directories
- **GoReleaser Integration**: Builds modules with `.goreleaser.yaml` configurations locally
- **Module Tracking**: SQLite database tracks installed modules, versions, and dependencies
- **Flexible Input**: Accepts various URL formats (https, git, ssh) and normalizes them
- **Version Management**: Fetches all available versions and supports version pinning with `@version`
- **Dependency Tracking**: Extracts and stores transitive dependencies for each module

## Installation

```shell
go install github.com/inovacc/goinstall@latest
```

## Usage

### Basic Installation

Install a Go CLI tool by providing its module path:

```shell
# Direct CLI path
goinstall github.com/inovacc/ksuid/cmd/ksuid

# With version pinning
goinstall github.com/inovacc/ksuid/cmd/ksuid@v1.2.3

# Latest version (default)
goinstall github.com/inovacc/ksuid/cmd/ksuid@latest
```

### Flexible URL Formats

`goinstall` accepts various URL formats and normalizes them automatically:

```shell
# HTTPS URL
goinstall https://github.com/inovacc/ksuid/cmd/ksuid

# Git protocol
goinstall git://github.com/inovacc/ksuid/cmd/ksuid

# SSH format
goinstall ssh://github.com/inovacc/ksuid/cmd/ksuid

# With .git suffix
goinstall https://github.com/inovacc/ksuid/cmd/ksuid.git
```

All of these will be normalized to `github.com/inovacc/ksuid/cmd/ksuid`.

### Smart CLI Discovery

When you provide a library module path (without a specific CLI path), `goinstall` automatically discovers installable CLIs:

```shell
# Provide root module - automatically discovers CLIs in cmd/ or cli/ directories
goinstall github.com/inovacc/brdoc

# Output:
# Module "github.com/inovacc/brdoc" found but is not installable (no main package), searching for CLIs...
# Found installable CLI: github.com/inovacc/brdoc/cmd/brdoc
# Installing module: github.com/inovacc/brdoc/cmd/brdoc
```

**Discovery Methods**:
1. Searches `cmd/` directory for packages with `package main`
2. Searches `cli/` directory for packages with `package main`
3. Parses `.goreleaser.yaml` for build targets (if present)

If multiple CLIs are found, the first one is automatically selected.

### GoReleaser Build Support

For modules with `.goreleaser.yaml` or `.goreleaser.yml` configurations, `goinstall` automatically:

1. Detects the GoReleaser configuration
2. Installs `goreleaser` if not present
3. Builds the module using `goreleaser build --snapshot --clean`
4. Extracts the built binary for your platform
5. Installs it to `$GOPATH/bin`

```shell
# Module with .goreleaser.yaml - builds locally
goinstall github.com/inovacc/twig

# Output:
# Fetching module information...
# Found GoReleaser config: .goreleaser.yaml
# GoReleaser not found, installing...
# GoReleaser installed successfully
# Building with GoReleaser...
# Build completed successfully
# Found binary: twig_windows_amd64.exe
# Binary installed to: C:\Users\username\go\bin\twig.exe
```

This is particularly useful for:
- Modules that require local builds (CGO dependencies)
- Modules with custom build steps (code generation)
- Modules with platform-specific build requirements

### Version Support

`goinstall` supports both tagged releases and pseudo-versions:

```shell
# Tagged release
goinstall github.com/inovacc/brdoc@v1.0.0

# Pseudo-version (for modules without tags)
goinstall github.com/inovacc/twig@latest
# Automatically uses: v0.0.0-20250109123456-abcdef123456
```

## Database Tracking

All installed modules are tracked in a SQLite database with:

- Module name and version
- All available versions
- Installation timestamp
- Module hash
- Dependency tree

### Database Location

The database location varies by platform:

- **Windows**: `%LOCALAPPDATA%\goinstall\modules.db`
- **macOS**: `~/Library/Application Support/goinstall/modules.db`
- **Linux**: `$XDG_DATA_HOME/goinstall/modules.db` (defaults to `~/.local/share/goinstall/modules.db`)

Override with the `GOINSTALL_DB_PATH` environment variable:

```shell
export GOINSTALL_DB_PATH=/custom/path/modules.db
goinstall github.com/inovacc/ksuid/cmd/ksuid
```

## Example Output

```shell
$ goinstall github.com/inovacc/brdoc

Fetching module information...
Module "github.com/inovacc/brdoc" found but is not installable (no main package), searching for CLIs...
Found installable CLI: github.com/inovacc/brdoc/cmd/brdoc
Installing module: github.com/inovacc/brdoc/cmd/brdoc
Module is installer successfully: github.com/inovacc/brdoc/cmd/brdoc
Show report using: goinstall report github.com/inovacc/brdoc/cmd/brdoc
```

## Commands

### Install (default)

```shell
goinstall <module-path>[@version]
```

Installs a Go module and tracks it in the database.

### Report (planned)

```shell
goinstall report <module-name>
```

Generate reports on installed modules and their dependencies.

### Monitor (planned)

```shell
goinstall monitor
```

Monitor installed modules for available updates.

## Architecture

- **Cobra CLI**: Command structure and flag parsing
- **SQLite**: Pure Go SQLite driver (`modernc.org/sqlite`) for module tracking
- **Temporary Modules**: Uses "dummy" Go modules for dependency resolution without polluting workspace
- **Semantic Versioning**: Sorts and compares versions using `golang.org/x/mod/semver`

## Roadmap

- [x] Install module with smart CLI discovery
- [x] GoReleaser integration for local builds
- [x] Version resolution and dependency tracking
- [x] Support for pseudo-versions (untagged modules)
- [ ] Report command for installed modules
- [ ] Monitoring for module updates
- [ ] Auto-update functionality

## Contributing

Contributions are welcome! Please ensure:

- All tests pass: `go test ./...`
- Code is formatted: `go fmt ./...`
- Linting is clean: `golangci-lint run`

## License

See LICENSE file for details.
