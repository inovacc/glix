package installer

import (
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
