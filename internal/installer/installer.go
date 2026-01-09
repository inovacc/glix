package installer

import (
	"fmt"

	"github.com/inovacc/goinstall/internal/database"
	"github.com/inovacc/goinstall/internal/module"
	"github.com/spf13/cobra"
)

func Installer(cmd *cobra.Command, args []string) error {
	db, err := database.NewDatabase(cmd.Context())
	if err != nil {
		return err
	}
	defer func(db *database.Database) {
		cobra.CheckErr(db.Close())
	}(db)

	newModule, err := module.NewModule(cmd.Context(), "go")
	if err != nil {
		return err
	}

	name := args[0]

	cmd.Println("Fetching module information...")

	if err := newModule.FetchModuleInfo(name); err != nil {
		return err
	}

	// Check if multiple paths were discovered
	if len(newModule.GetDiscoveredPaths()) > 1 {
		selected, err := module.PromptCLISelection(newModule.GetDiscoveredPaths())
		if err != nil {
			return fmt.Errorf("CLI selection failed: %w", err)
		}

		// Install each selected CLI
		for _, path := range selected {
			if err := installSingleCLI(cmd, db, path, newModule.Version); err != nil {
				cmd.PrintErrf("Failed to install %s: %v\n", path, err)
				continue
			}

			cmd.Printf("Successfully installed: %s\n", path)
		}

		return nil
	}

	// Original single-CLI installation
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

// installSingleCLI installs a single CLI with a specific path
func installSingleCLI(cmd *cobra.Command, db *database.Database, path, version string) error {
	// Create temporary module for this path
	tempModule, err := module.NewModule(cmd.Context(), "go")
	if err != nil {
		return err
	}

	// Fetch module info for the specific path
	if err := tempModule.FetchModuleInfo(path); err != nil {
		return err
	}

	// Install
	cmd.Println("Installing module:", tempModule.Name)

	if err := tempModule.InstallModule(cmd.Context()); err != nil {
		return err
	}

	// Report to database
	if err := tempModule.Report(db); err != nil {
		return err
	}

	cmd.Printf("Show report using: %s report %s\n", cmd.Root().Name(), tempModule.Name)

	return nil
}
