package monitor

import (
	"github.com/inovacc/glix/internal/database"
	"github.com/spf13/cobra"
)

func Monitor(cmd *cobra.Command, args []string) error {
	db, err := database.NewStorage(cmd.Context())
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
