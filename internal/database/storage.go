package database

import (
	"fmt"
	"sort"
	"strings"
	"time"

	pb "github.com/inovacc/glix/pkg/api/v1"
	bolt "go.etcd.io/bbolt"
	"google.golang.org/protobuf/proto"
)

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
func (s *Storage) UpsertModule(module *pb.ModuleProto) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		// Serialize module to protobuf
		data, err := proto.Marshal(module)
		if err != nil {
			return fmt.Errorf("failed to marshal module: %w", err)
		}

		// Create composite key: name@version
		key := []byte(fmt.Sprintf("%s@%s", module.GetName(), module.GetVersion()))

		// Store in modules bucket
		bucket := tx.Bucket(modulesBucket)
		if err := bucket.Put(key, data); err != nil {
			return fmt.Errorf("failed to put module: %w", err)
		}

		// Update time index
		if err := s.updateTimeIndex(tx, module.GetTimestampUnixNano(), string(key)); err != nil {
			return fmt.Errorf("failed to update time index: %w", err)
		}

		// Update name index
		if err := s.updateNameIndex(tx, module.GetName(), module.GetVersion()); err != nil {
			return fmt.Errorf("failed to update name index: %w", err)
		}

		return nil
	})
}

// GetModule retrieves a module by name and version
func (s *Storage) GetModule(name, version string) (*pb.ModuleProto, error) {
	var module *pb.ModuleProto

	err := s.db.View(func(tx *bolt.Tx) error {
		key := []byte(fmt.Sprintf("%s@%s", name, version))
		bucket := tx.Bucket(modulesBucket)

		data := bucket.Get(key)
		if data == nil {
			return fmt.Errorf("module not found: %s@%s", name, version)
		}

		module = &pb.ModuleProto{}
		if err := proto.Unmarshal(data, module); err != nil {
			return fmt.Errorf("failed to unmarshal module: %w", err)
		}

		return nil
	})

	return module, err
}

// GetModuleByName retrieves all versions of a module by name
func (s *Storage) GetModuleByName(name string) ([]*pb.ModuleProto, error) {
	var modules []*pb.ModuleProto

	err := s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(modulesBucket)
		cursor := bucket.Cursor()

		// Scan for all keys with prefix "name@"
		prefix := []byte(name + "@")
		for k, v := cursor.Seek(prefix); k != nil && strings.HasPrefix(string(k), string(prefix)); k, v = cursor.Next() {
			module := &pb.ModuleProto{}
			if err := proto.Unmarshal(v, module); err != nil {
				return fmt.Errorf("failed to unmarshal module: %w", err)
			}

			modules = append(modules, module)
		}

		return nil
	})

	// Sort by timestamp descending (most recent first)
	sort.Slice(modules, func(i, j int) bool {
		return modules[i].GetTimestampUnixNano() > modules[j].GetTimestampUnixNano()
	})

	return modules, err
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
			// v contains the module key (name@version)
			modulesBkt := tx.Bucket(modulesBucket)

			data := modulesBkt.Get(v)
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

// DeleteModule removes a module and updates indexes
func (s *Storage) DeleteModule(name, version string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		key := []byte(fmt.Sprintf("%s@%s", name, version))

		// Get module first to access timestamp
		bucket := tx.Bucket(modulesBucket)

		data := bucket.Get(key)
		if data == nil {
			return fmt.Errorf("module not found: %s@%s", name, version)
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

		// Update name index (remove this version)
		if err := s.deleteFromNameIndex(tx, name, version); err != nil {
			return fmt.Errorf("failed to update name index: %w", err)
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
func (s *Storage) updateTimeIndex(tx *bolt.Tx, timestamp int64, moduleKey string) error {
	bucket := tx.Bucket(timeIndexBucket)

	// Use timestamp as key for sorting
	key := []byte(fmt.Sprintf("%020d", timestamp)) // Zero-padded for lexicographic sorting
	value := []byte(moduleKey)

	return bucket.Put(key, value)
}

// deleteFromTimeIndex removes an entry from the time index
func (s *Storage) deleteFromTimeIndex(tx *bolt.Tx, timestamp int64) error {
	bucket := tx.Bucket(timeIndexBucket)
	key := []byte(fmt.Sprintf("%020d", timestamp))

	return bucket.Delete(key)
}

// updateNameIndex adds a version to the name index
func (s *Storage) updateNameIndex(tx *bolt.Tx, name, version string) error {
	bucket := tx.Bucket(nameIndexBucket)
	key := []byte(name)

	// Get existing versions
	data := bucket.Get(key)
	versionList := &pb.VersionListProto{}

	if data != nil {
		if err := proto.Unmarshal(data, versionList); err != nil {
			return fmt.Errorf("failed to unmarshal version list: %w", err)
		}
	}

	// Add version if not already present
	found := false

	for _, v := range versionList.GetVersions() {
		if v == version {
			found = true
			break
		}
	}

	if !found {
		versionList.Versions = append(versionList.Versions, version)
	}

	// Marshal and store
	data, err := proto.Marshal(versionList)
	if err != nil {
		return fmt.Errorf("failed to marshal version list: %w", err)
	}

	return bucket.Put(key, data)
}

// deleteFromNameIndex removes a version from the name index
func (s *Storage) deleteFromNameIndex(tx *bolt.Tx, name, version string) error {
	bucket := tx.Bucket(nameIndexBucket)
	key := []byte(name)

	// Get existing versions
	data := bucket.Get(key)
	if data == nil {
		return nil // Nothing to delete
	}

	versionList := &pb.VersionListProto{}
	if err := proto.Unmarshal(data, versionList); err != nil {
		return fmt.Errorf("failed to unmarshal version list: %w", err)
	}

	// Remove the version
	newVersions := make([]string, 0, len(versionList.GetVersions()))
	for _, v := range versionList.GetVersions() {
		if v != version {
			newVersions = append(newVersions, v)
		}
	}

	// If no versions left, delete the key
	if len(newVersions) == 0 {
		return bucket.Delete(key)
	}

	// Update the list
	versionList.Versions = newVersions

	data, err := proto.Marshal(versionList)
	if err != nil {
		return fmt.Errorf("failed to marshal version list: %w", err)
	}

	return bucket.Put(key, data)
}
