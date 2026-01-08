package monitor

import (
	"github.com/inovacc/goinstall/internal/database"
	"github.com/spf13/cobra"
)

func Monitor(cmd *cobra.Command, args []string) error {
	db, err := database.NewDatabase(cmd.Context())
	if err != nil {
		return err
	}
	defer func(db *database.Database) {
		cobra.CheckErr(db.Close())
	}(db)

	return moduleMonitor(db)
}

func moduleMonitor(db *database.Database) error {
	return nil
}
