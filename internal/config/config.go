package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

const (
	appName = "glix"
)

var appDir = ""

func init() {
	if appDir = os.Getenv("GLIX_DB_PATH"); appDir == "" {
		dataDir, err := os.UserCacheDir()
		cobra.CheckErr(err)

		appDir = filepath.Join(dataDir, appName)
	}
}

func GetApplicationDirectory() (string, error) {
	if err := os.MkdirAll(appDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create %s directory: %w", appName, err)
	}

	return appDir, nil
}

func GetDatabaseDirectory() string {
	return filepath.Join(appDir, fmt.Sprintf("%s.bolt", appName))
}
