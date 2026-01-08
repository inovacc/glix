package database

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"

	"github.com/spf13/afero"
	"github.com/spf13/viper"
	_ "modernc.org/sqlite"
)

type Database struct {
	db *sql.DB
}

func NewDatabase(ctx context.Context, afs afero.Fs) (*Database, error) {
	dbPath := viper.GetString("installPath")
	if dbPath == "" {
		return nil, errors.New("installPath is required")
	}

	// Ensure directory exists
	if err := afs.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, err
	}

	// Ensure file exists before opening via SQL driver
	if exists, err := afero.Exists(afs, dbPath); err != nil {
		return nil, err
	} else if !exists {
		if f, err := afs.Create(dbPath); err != nil {
			return nil, err
		} else {
			_ = f.Close()
		}
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	if err := db.PingContext(ctx); err != nil {
		return nil, err
	}

	database := &Database{db: db}

	if err := database.setupSchema(); err != nil {
		return nil, err
	}

	return database, nil
}

func (d *Database) Close() error {
	return d.db.Close()
}

func (d *Database) Begin() (*sql.Tx, error) {
	return d.db.Begin()
}

func (d *Database) setupSchema() error {
	schema := []string{
		`CREATE TABLE IF NOT EXISTS modules (
			name TEXT NOT NULL,
			version TEXT NOT NULL,
			versions TEXT,
			dependencies TEXT,
			hash TEXT,
			time TIMESTAMP,
			PRIMARY KEY(name, version)
		);`,
		`CREATE TABLE IF NOT EXISTS dependencies (
			module_name TEXT NOT NULL,
			dep_name TEXT NOT NULL,
			dep_version TEXT,
			dep_hash TEXT,
			FOREIGN KEY(module_name) REFERENCES modules(name) ON DELETE CASCADE,
			PRIMARY KEY(module_name, dep_name)
		);`,
	}
	for _, stmt := range schema {
		if _, err := d.db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}
