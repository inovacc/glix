package config

import (
	"fmt"
	"hash/maphash"
	"os"
	"path/filepath"
	"sync"

	"github.com/spf13/cobra"
)

const (
	appName = "glix"
)

var (
	appDir   = ""
	cacheDir = ""
	once     sync.Once
)

func init() {
	if appDir = os.Getenv("GLIX_DB_PATH"); appDir == "" {
		dataDir, err := os.UserCacheDir()
		cobra.CheckErr(err)

		appDir = filepath.Join(dataDir, appName)
		cacheDir = filepath.Join(appDir, "cache")
	}
}

func GetApplicationDirectory() string {
	once.Do(makeDirIfNotExists(appDir))

	return appDir
}

func GetApplicationCacheDirectory() (string, error) {
	once.Do(makeDirIfNotExists(cacheDir))

	randomDir := filepath.Join(cacheDir, fmt.Sprint(new(maphash.Hash).Sum64()))
	once.Do(makeDirIfNotExists(randomDir))

	return randomDir, nil
}

func GetDatabaseDirectory() string {
	return filepath.Join(appDir, fmt.Sprintf("%s.bolt", appName))
}

func makeDirIfNotExists(dir string) func() {
	return func() {
		if err := os.MkdirAll(dir, 0755); err != nil {
			panic(err)
		}
	}
}
