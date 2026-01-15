package module

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
		cacheDir = filepath.Join(appDir, "cache", fmt.Sprint(new(maphash.Hash).Sum64()))
	}

	if err := os.MkdirAll(appDir, 0755); err != nil {
		panic(err)
	}

	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		panic(err)
	}
}

func GetApplicationDirectory() string {
	return appDir
}

func GetApplicationCacheDirectory() (string, error) {
	return cacheDir, nil
}

func GetDatabaseDirectory() string {
	return filepath.Join(appDir, fmt.Sprintf("%s.bolt", appName))
}

// GetApplicationConfigDirectory returns the path to the config directory
func GetApplicationConfigDirectory() (string, error) {
	configDir := filepath.Join(appDir, "config")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return "", err
	}
	return configDir, nil
}
