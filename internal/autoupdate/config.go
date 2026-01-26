package autoupdate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/inovacc/glix/internal/module"
)

// DefaultInterval is the default interval between update checks
const DefaultInterval = 24 * time.Hour

// Config holds auto-update configuration
type Config struct {
	Enabled       bool          `json:"enabled"`
	Interval      time.Duration `json:"interval"`
	LastCheck     time.Time     `json:"last_check"`
	LastUpdate    time.Time     `json:"last_update"`
	UpdatedCount  int           `json:"updated_count"`
	CheckedCount  int           `json:"checked_count"`
	NotifyOnly    bool          `json:"notify_only"` // If true, only notify about updates, don't auto-install
	IncludePrerel bool          `json:"include_prerelease"`
}

// configStore handles persistent storage of auto-update configuration
type configStore struct {
	mu       sync.RWMutex
	config   Config
	filePath string
}

var (
	store     *configStore
	storeOnce sync.Once
)

// getConfigPath returns the path to the auto-update config file
func getConfigPath() string {
	configDir, err := module.GetApplicationConfigDirectory()
	if err != nil {
		// Fallback to cache directory
		configDir, _ = module.GetApplicationCacheDirectory()
	}

	return filepath.Join(configDir, "autoupdate.json")
}

// GetStore returns the singleton config store
func GetStore() *configStore {
	storeOnce.Do(func() {
		store = &configStore{
			filePath: getConfigPath(),
			config: Config{
				Enabled:  false,
				Interval: DefaultInterval,
			},
		}
		// Load existing config if available
		_ = store.load()
	})

	return store
}

// load reads the configuration from disk
func (s *configStore) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Use defaults
		}

		return fmt.Errorf("failed to read config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	s.config = cfg

	return nil
}

// save writes the configuration to disk
func (s *configStore) save() error {
	// Ensure directory exists
	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(s.config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(s.filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// Get returns a copy of the current configuration
func (s *configStore) Get() Config {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.config
}

// SetEnabled enables or disables auto-update
func (s *configStore) SetEnabled(enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.config.Enabled = enabled

	return s.save()
}

// SetInterval sets the update check interval
func (s *configStore) SetInterval(interval time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if interval < time.Hour {
		return fmt.Errorf("interval must be at least 1 hour")
	}

	s.config.Interval = interval

	return s.save()
}

// SetNotifyOnly sets whether to only notify about updates without installing
func (s *configStore) SetNotifyOnly(notifyOnly bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.config.NotifyOnly = notifyOnly

	return s.save()
}

// RecordCheck records that an update check was performed
func (s *configStore) RecordCheck(updatedCount int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.config.LastCheck = time.Now()

	s.config.CheckedCount++
	if updatedCount > 0 {
		s.config.LastUpdate = time.Now()
		s.config.UpdatedCount += updatedCount
	}

	return s.save()
}

// ShouldCheck returns true if enough time has passed since the last check
func (s *configStore) ShouldCheck() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.config.Enabled {
		return false
	}

	if s.config.LastCheck.IsZero() {
		return true
	}

	return time.Since(s.config.LastCheck) >= s.config.Interval
}
