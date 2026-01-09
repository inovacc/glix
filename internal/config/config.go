package config

import (
	"fmt"
	"hash/maphash"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

const (
	appName = "glix"
)

var (
	appDir   = ""
	cacheDir = ""
)

func init() {
	if appDir = os.Getenv("GLIX_DB_PATH"); appDir == "" {
		dataDir, err := os.UserCacheDir()
		cobra.CheckErr(err)

		appDir = filepath.Join(dataDir, appName)
		cobra.CheckErr(os.MkdirAll(appDir, 0755))

		cacheDir = filepath.Join(appDir, "cache")
		cobra.CheckErr(os.MkdirAll(cacheDir, 0755))
	}
}

func GetApplicationDirectory() string {
	return appDir
}

func GetApplicationCacheDirectory() (string, error) {
	randomDir := filepath.Join(cacheDir, fmt.Sprint(new(maphash.Hash).Sum64()))

	if err := os.MkdirAll(randomDir, 0755); err != nil {
		return "", err
	}

	return randomDir, nil
}

func GetDatabaseDirectory() string {
	return filepath.Join(appDir, fmt.Sprintf("%s.bolt", appName))
}
