package monitor

import (
	"github.com/inovacc/glix/internal/database"
	"github.com/inovacc/glix/internal/module"
	"github.com/spf13/cobra"
)

func Monitor(cmd *cobra.Command, _ []string) error {
	db, err := database.NewStorage(module.GetDatabaseDirectory())
	if err != nil {
		return err
	}
	defer func(db *database.Storage) {
		cobra.CheckErr(db.Close())
	}(db)

	return moduleMonitor(db)
}

func moduleMonitor(db *database.Storage) error {
	_ = db // to be implemented
	return nil
}
