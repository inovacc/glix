package database

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/spf13/viper"
)

func TestNewDatabase(t *testing.T) {
	afs := afero.NewOsFs()
	tmpDir, err := afero.TempDir(afs, "", "database")
	if err != nil {
		t.Fatal(err)
	}

	viper.Set("installPath", filepath.Join(tmpDir, "modules.db"))

	db, err := NewDatabase(context.TODO(), afs)
	if err != nil {
		t.Fatal(err)
	}

	if db == nil {
		t.Fatal("db is nil")
	}
}
