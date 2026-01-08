package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/inovacc/goinstall/internal/database/sqlc"
	"github.com/spf13/viper"
	_ "modernc.org/sqlite"
)

// Database wraps sqlc.Queries with additional functionality
type Database struct {
	db      *sql.DB
	queries *sqlc.Queries
}

// NewDatabase initializes database connection and runs schema setup
func NewDatabase(ctx context.Context) (*Database, error) {
	dbPath := viper.GetString("installPath")
	if dbPath == "" {
		return nil, errors.New("installPath is required")
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	// Ensure file exists before opening via SQL driver
	if _, err := os.Stat(dbPath); err != nil {
		if os.IsNotExist(err) {
			f, createErr := os.Create(dbPath)
			if createErr != nil {
				return nil, fmt.Errorf("failed to create database file: %w", createErr)
			}

			_ = f.Close()
		} else {
			return nil, fmt.Errorf("failed to stat database file: %w", err)
		}
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	database := &Database{
		db:      db,
		queries: sqlc.New(db),
	}

	// Setup schema using embedded SQL file
	if err := database.setupSchema(ctx); err != nil {
		return nil, fmt.Errorf("failed to setup schema: %w", err)
	}

	return database, nil
}

// Close closes the database connection
func (d *Database) Close() error {
	return d.db.Close()
}

// Queries returns the sqlc-generated queries interface
func (d *Database) Queries() *sqlc.Queries {
	return d.queries
}

// WithTx executes a function within a database transaction
func (d *Database) WithTx(ctx context.Context, fn func(*sqlc.Queries) error) error {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	qtx := d.queries.WithTx(tx)

	committed := false

	defer func() {
		if !committed {
			if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
				// Log rollback error but don't return it
				// Consider using slog here in the future
			}
		}
	}()

	if err := fn(qtx); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	committed = true

	return nil
}

// setupSchema initializes the database schema
func (d *Database) setupSchema(ctx context.Context) error {
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
		`CREATE INDEX IF NOT EXISTS idx_modules_name ON modules(name);`,
		`CREATE INDEX IF NOT EXISTS idx_modules_time ON modules(time DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_dependencies_module ON dependencies(module_name);`,
	}

	for _, stmt := range schema {
		if _, err := d.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("failed to execute schema statement: %w", err)
		}
	}

	return nil
}
