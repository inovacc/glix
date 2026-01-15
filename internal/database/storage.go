package database

import (
	"crypto/sha256"
	"fmt"
	"time"

	pb "github.com/inovacc/glix/pkg/api/v1"
	bolt "go.etcd.io/bbolt"
	"google.golang.org/protobuf/proto"
)

// moduleKey generates a hash-based key from the module name (without version)
func moduleKey(name string) []byte {
	hash := sha256.Sum256([]byte(name))
	return hash[:]
}

// Bucket names
var (
	modulesBucket      = []byte("modules")
	dependenciesBucket = []byte("dependencies")
	timeIndexBucket    = []byte("indexes_by_time")
	nameIndexBucket    = []byte("indexes_by_name")
)

// Storage wraps BoltDB with module tracking functionality
type Storage struct {
	db *bolt.DB
}

// NewStorage initializes BoltDB connection and creates buckets
func NewStorage(dbPath string) (*Storage, error) {
	// Open BoltDB
	db, err := bolt.Open(dbPath, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	storage := &Storage{
		db: db,
	}

	// Initialize buckets
	if err := storage.initBuckets(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to initialize buckets: %w", err)
	}

	return storage, nil
}

// Close closes the database connection
func (s *Storage) Close() error {
	return s.db.Close()
}

// initBuckets creates all required buckets if they don't exist
func (s *Storage) initBuckets() error {
	return s.db.Update(func(tx *bolt.Tx) error {
		buckets := [][]byte{
			modulesBucket,
			dependenciesBucket,
			timeIndexBucket,
			nameIndexBucket,
		}

		for _, bucket := range buckets {
			if _, err := tx.CreateBucketIfNotExists(bucket); err != nil {
				return fmt.Errorf("failed to create bucket %s: %w", string(bucket), err)
			}
		}

		return nil
	})
}

// UpsertModules inserts or updates a module
func (s *Storage) UpsertModules(module []*pb.ModuleProto) error {
	for _, mod := range module {
		if err := s.UpsertModule(mod); err != nil {
			return err
		}
	}

	return nil
}

// UpsertModule inserts or updates a module
// Uses a hash of the module name (without version) as the primary key
// This ensures only one entry per module, with the latest version stored
func (s *Storage) UpsertModule(module *pb.ModuleProto) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		// Use hash of module name as primary key (ensures one entry per module)
		key := moduleKey(module.GetName())

		// Check if the module already exists and remove the old time index entry
		bucket := tx.Bucket(modulesBucket)
		existingData := bucket.Get(key)
		if existingData != nil {
			existingModule := &pb.ModuleProto{}
			if err := proto.Unmarshal(existingData, existingModule); err == nil {
				// Remove old time index entry
				if err := s.deleteFromTimeIndex(tx, existingModule.GetTimestampUnixNano()); err != nil {
					return fmt.Errorf("failed to delete old time index: %w", err)
				}
			}
		}

		// Serialize module to protobuf
		data, err := proto.Marshal(module)
		if err != nil {
			return fmt.Errorf("failed to marshal module: %w", err)
		}

		// Store in modules bucket (using hash key)
		if err := bucket.Put(key, data); err != nil {
			return fmt.Errorf("failed to put module: %w", err)
		}

		// Update time index (use module name as value for lookup)
		if err := s.updateTimeIndex(tx, module.GetTimestampUnixNano(), module.GetName()); err != nil {
			return fmt.Errorf("failed to update time index: %w", err)
		}

		return nil
	})
}

// GetModule retrieves a module by name (version is optional, ignored since we store one version per module)
func (s *Storage) GetModule(name, _ string) (*pb.ModuleProto, error) {
	var module *pb.ModuleProto

	err := s.db.View(func(tx *bolt.Tx) error {
		key := moduleKey(name)
		bucket := tx.Bucket(modulesBucket)

		data := bucket.Get(key)
		if data == nil {
			return fmt.Errorf("module not found: %s", name)
		}

		module = &pb.ModuleProto{}
		if err := proto.Unmarshal(data, module); err != nil {
			return fmt.Errorf("failed to unmarshal module: %w", err)
		}

		return nil
	})

	return module, err
}

// GetModuleByName retrieves a module by name (returns a slice for API compatibility)
func (s *Storage) GetModuleByName(name string) ([]*pb.ModuleProto, error) {
	module, err := s.GetModule(name, "")
	if err != nil {
		return nil, err
	}

	return []*pb.ModuleProto{module}, nil
}

// ListModules retrieves all modules ordered by time (most recent first)
func (s *Storage) ListModules() ([]*pb.ModuleProto, error) {
	var modules []*pb.ModuleProto

	err := s.db.View(func(tx *bolt.Tx) error {
		// Use time index for ordered retrieval
		timeIndex := tx.Bucket(timeIndexBucket)
		cursor := timeIndex.Cursor()

		// Iterate in reverse order (most recent first)
		for k, v := cursor.Last(); k != nil; k, v = cursor.Prev() {
			// v contains the module name, convert to hash key
			moduleName := string(v)
			hashKey := moduleKey(moduleName)

			modulesBkt := tx.Bucket(modulesBucket)

			data := modulesBkt.Get(hashKey)
			if data == nil {
				continue // Skip if module was deleted
			}

			module := &pb.ModuleProto{}
			if err := proto.Unmarshal(data, module); err != nil {
				return fmt.Errorf("failed to unmarshal module: %w", err)
			}

			modules = append(modules, module)
		}

		return nil
	})

	return modules, err
}

// DeleteModule removes a module and updates indexes (version is ignored since we store one version per module)
func (s *Storage) DeleteModule(name, _ string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		key := moduleKey(name)

		// Get module first to access timestamp
		bucket := tx.Bucket(modulesBucket)

		data := bucket.Get(key)
		if data == nil {
			return fmt.Errorf("module not found: %s", name)
		}

		module := &pb.ModuleProto{}
		if err := proto.Unmarshal(data, module); err != nil {
			return fmt.Errorf("failed to unmarshal module: %w", err)
		}

		// Delete from modules bucket
		if err := bucket.Delete(key); err != nil {
			return fmt.Errorf("failed to delete module: %w", err)
		}

		// Delete from time index
		if err := s.deleteFromTimeIndex(tx, module.GetTimestampUnixNano()); err != nil {
			return fmt.Errorf("failed to delete from time index: %w", err)
		}

		// Delete dependencies
		depKey := []byte(name)

		depBucket := tx.Bucket(dependenciesBucket)
		if err := depBucket.Delete(depKey); err != nil {
			return fmt.Errorf("failed to delete dependencies: %w", err)
		}

		return nil
	})
}

// CountModules returns the total number of modules
func (s *Storage) CountModules() (int64, error) {
	var count int64

	err := s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(modulesBucket)
		stats := bucket.Stats()
		count = int64(stats.KeyN)

		return nil
	})

	return count, err
}

// UpsertDependencies stores dependencies for a module
func (s *Storage) UpsertDependencies(moduleName string, deps *pb.DependenciesProto) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		data, err := proto.Marshal(deps)
		if err != nil {
			return fmt.Errorf("failed to marshal dependencies: %w", err)
		}

		bucket := tx.Bucket(dependenciesBucket)
		key := []byte(moduleName)

		if err := bucket.Put(key, data); err != nil {
			return fmt.Errorf("failed to put dependencies: %w", err)
		}

		return nil
	})
}

// GetDependenciesByModule retrieves dependencies for a module
func (s *Storage) GetDependenciesByModule(moduleName string) (*pb.DependenciesProto, error) {
	var deps *pb.DependenciesProto

	err := s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(dependenciesBucket)
		key := []byte(moduleName)

		data := bucket.Get(key)
		if data == nil {
			return fmt.Errorf("dependencies not found for module: %s", moduleName)
		}

		deps = &pb.DependenciesProto{}
		if err := proto.Unmarshal(data, deps); err != nil {
			return fmt.Errorf("failed to unmarshal dependencies: %w", err)
		}

		return nil
	})

	return deps, err
}

// CountDependencies returns the total number of dependency entries
func (s *Storage) CountDependencies() (int64, error) {
	var count int64

	err := s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(dependenciesBucket)
		stats := bucket.Stats()
		count = int64(stats.KeyN)

		return nil
	})

	return count, err
}

// updateTimeIndex adds/updates an entry in the time index
func (s *Storage) updateTimeIndex(tx *bolt.Tx, timestamp int64, moduleName string) error {
	bucket := tx.Bucket(timeIndexBucket)

	// Use timestamp as key for sorting
	key := fmt.Appendf(nil, "%020d", timestamp) // Zero-padded for lexicographic sorting
	value := []byte(moduleName)

	return bucket.Put(key, value)
}

// deleteFromTimeIndex removes an entry from the time index
func (s *Storage) deleteFromTimeIndex(tx *bolt.Tx, timestamp int64) error {
	bucket := tx.Bucket(timeIndexBucket)
	key := fmt.Appendf(nil, "%020d", timestamp)

	return bucket.Delete(key)
}
