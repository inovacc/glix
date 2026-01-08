package database

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/inovacc/goinstall/internal/database/sqlc"
	"github.com/spf13/viper"
)

func TestNewDatabase(t *testing.T) {
	tmpDir := t.TempDir()

	viper.Set("installPath", filepath.Join(tmpDir, "modules.db"))

	db, err := NewDatabase(context.TODO())
	if err != nil {
		t.Fatal(err)
	}

	if db == nil {
		t.Fatal("db is nil")
	}

	// Verify Queries() returns valid interface
	if db.Queries() == nil {
		t.Fatal("Queries() returned nil")
	}

	// Cleanup
	if err := db.Close(); err != nil {
		t.Fatal("Failed to close database:", err)
	}
}

func TestWithTx_Commit(t *testing.T) {
	tmpDir := t.TempDir()
	viper.Set("installPath", filepath.Join(tmpDir, "test_tx.db"))

	db, err := NewDatabase(context.TODO())
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		_ = db.Close()
	}()

	// Test successful transaction
	err = db.WithTx(context.TODO(), func(q *sqlc.Queries) error {
		hash := "test-hash-123"
		now := time.Now()

		return q.UpsertModule(context.TODO(), sqlc.UpsertModuleParams{
			Name:         "github.com/test/module",
			Version:      "v1.0.0",
			Versions:     "[]",
			Dependencies: "[]",
			Hash:         &hash,
			Time:         &now,
		})
	})
	if err != nil {
		t.Fatal("Transaction failed:", err)
	}

	// Verify data was committed
	mod, err := db.Queries().GetModule(context.TODO(), sqlc.GetModuleParams{
		Name:    "github.com/test/module",
		Version: "v1.0.0",
	})
	if err != nil {
		t.Fatal("Failed to get module:", err)
	}

	if mod.Name != "github.com/test/module" {
		t.Errorf("Expected module name %q, got %q", "github.com/test/module", mod.Name)
	}
}

func TestWithTx_Rollback(t *testing.T) {
	tmpDir := t.TempDir()
	viper.Set("installPath", filepath.Join(tmpDir, "test_rollback.db"))

	db, err := NewDatabase(context.TODO())
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		_ = db.Close()
	}()

	// Test transaction rollback on error
	expectedErr := errors.New("forced error")

	err = db.WithTx(context.TODO(), func(q *sqlc.Queries) error {
		hash := "rollback-hash"
		now := time.Now()
		_ = q.UpsertModule(context.TODO(), sqlc.UpsertModuleParams{
			Name:         "github.com/test/rollback",
			Version:      "v1.0.0",
			Versions:     "[]",
			Dependencies: "[]",
			Hash:         &hash,
			Time:         &now,
		})

		return expectedErr // Force rollback
	})
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if !errors.Is(err, expectedErr) {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}

	// Verify data was NOT committed
	_, err = db.Queries().GetModule(context.TODO(), sqlc.GetModuleParams{
		Name:    "github.com/test/rollback",
		Version: "v1.0.0",
	})
	if err == nil {
		t.Fatal("Expected module to not exist after rollback")
	}
}
