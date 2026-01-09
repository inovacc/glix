package database

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"

	"github.com/inovacc/glix/internal/config"
	"github.com/inovacc/glix/internal/database/sqlc"
	_ "modernc.org/sqlite"
)

//go:embed schemas/schema/001_initial.sql
var schemaSQL string

// Database wraps sqlc.Queries with additional functionality
type Database struct {
	db      *sql.DB
	queries *sqlc.Queries
}

// NewDatabase initializes database connection and runs schema setup
func NewDatabase(ctx context.Context) (*Database, error) {
	db, err := sql.Open("sqlite", config.GetDatabaseDirectory())
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

// setupSchema executes the embedded schema SQL to create tables and indexes
func (d *Database) setupSchema(ctx context.Context) error {
	// Execute the schema SQL which contains CREATE TABLE and CREATE INDEX statements
	if _, err := d.db.ExecContext(ctx, schemaSQL); err != nil {
		return fmt.Errorf("failed to execute schema: %w", err)
	}

	return nil
}
