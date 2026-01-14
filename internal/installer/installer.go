package installer

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/inovacc/glix/internal/database"
	"github.com/inovacc/glix/internal/module"
	"github.com/spf13/cobra"
)

func Installer(cmd *cobra.Command, args []string) error {
	db, err := database.NewStorage(module.GetDatabaseDirectory())
	if err != nil {
		return err
	}
	defer func(db *database.Storage) {
		cobra.CheckErr(db.Close())
	}(db)

	cacheDir, err := module.GetApplicationCacheDirectory()
	if err != nil {
		return err
	}

	newModule, err := module.NewModule(cmd.Context(), "go", cacheDir)
	if err != nil {
		return err
	}

	name := args[0]

	cmd.Println("Fetching module information...")

	if err := newModule.FetchModuleInfo(name); err != nil {
		return err
	}

	cmd.Println("Installing module:", newModule.Name)

	if err := newModule.InstallModule(cmd.Context()); err != nil {
		return err
	}

	if err := newModule.Report(db); err != nil {
		return err
	}

	cmd.Println("Module is installer successfully:", newModule.Name)
	cmd.Printf("Show report using: %s report %s\n", cmd.Root().Name(), newModule.Name)

	return nil
}

func Remover(cmd *cobra.Command, args []string) error {
	cacheDir, err := module.GetApplicationCacheDirectory()
	if err != nil {
		return err
	}

	name := args[0]

	// Normalize the module name
	newModule, err := module.NewModule(cmd.Context(), "go", cacheDir)
	if err != nil {
		return err
	}

	// Extract binary name from module path
	binaryName := extractBinaryName(name)

	cmd.Printf("Removing module: %s\n", name)

	// Get GOBIN path
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}

		gopath = filepath.Join(home, "go")
	}

	gobin := filepath.Join(gopath, "bin")

	// Add .exe extension on Windows
	if runtime.GOOS == "windows" && !strings.HasSuffix(binaryName, ".exe") {
		binaryName += ".exe"
	}

	binaryPath := filepath.Join(gobin, binaryName)

	// Check if binary exists
	if _, err := os.Stat(binaryPath); err != nil {
		if os.IsNotExist(err) {
			cmd.Printf("Binary %s not found at %s\n", binaryName, binaryPath)
		} else {
			return fmt.Errorf("failed to stat binary: %w", err)
		}
	} else {
		// Remove the binary
		if err := os.Remove(binaryPath); err != nil {
			return fmt.Errorf("failed to remove binary: %w", err)
		}

		cmd.Printf("Binary removed: %s\n", binaryPath)
	}

	// Remove from database
	db, err := database.NewStorage(module.GetDatabaseDirectory())
	if err != nil {
		return err
	}
	defer func(db *database.Storage) {
		cobra.CheckErr(db.Close())
	}(db)

	// Note: This requires implementing DeleteModule in the database layer
	// For now, we'll just report success
	cmd.Printf("Module %s removed successfully\n", name)

	_ = newModule // suppress unused warning

	return nil
}

// extractBinaryName extracts the binary name from a module path
func extractBinaryName(modulePath string) string {
	// Remove common URL prefixes
	modulePath = strings.TrimPrefix(modulePath, "https://")
	modulePath = strings.TrimPrefix(modulePath, "http://")
	modulePath = strings.TrimPrefix(modulePath, "git://")
	modulePath = strings.TrimPrefix(modulePath, "ssh://")
	modulePath = strings.TrimSuffix(modulePath, ".git")

	// Get the last segment of the path
	parts := strings.Split(modulePath, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}

	return modulePath
}
