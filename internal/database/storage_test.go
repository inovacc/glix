package database

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	pb "github.com/inovacc/glix/pkg/api/v1"
	bolt "go.etcd.io/bbolt"
)

// setupTestStorage creates a temporary BoltDB for testing
func setupTestStorage(t *testing.T) (*Storage, func()) {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create directory
	err := os.MkdirAll(filepath.Dir(dbPath), 0755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Open BoltDB directly for testing
	db, err := bolt.Open(dbPath, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	storage := &Storage{
		db: db,
	}

	// Initialize buckets
	if err := storage.initBuckets(); err != nil {
		_ = db.Close()

		t.Fatalf("Failed to initialize buckets: %v", err)
	}

	cleanup := func() {
		_ = storage.Close()
		_ = os.RemoveAll(tmpDir)
	}

	return storage, cleanup
}

func TestNewStorage(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	if storage == nil {
		t.Fatal("Expected storage to be non-nil")
	}

	if storage.db == nil {
		t.Fatal("Expected db to be non-nil")
	}
}

func TestUpsertModule(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	module := &pb.ModuleProto{
		Name:              "github.com/test/module",
		Version:           "v1.0.0",
		Versions:          []string{"v1.0.0", "v0.9.0"},
		Hash:              "abc123",
		TimestampUnixNano: time.Now().UnixNano(),
	}

	err := storage.UpsertModule(module)
	if err != nil {
		t.Fatalf("UpsertModule failed: %v", err)
	}

	// Verify module was inserted
	retrieved, err := storage.GetModule(module.GetName(), module.GetVersion())
	if err != nil {
		t.Fatalf("GetModule failed: %v", err)
	}

	if retrieved.GetName() != module.GetName() {
		t.Errorf("Expected name %s, got %s", module.GetName(), retrieved.GetName())
	}

	if retrieved.GetVersion() != module.GetVersion() {
		t.Errorf("Expected version %s, got %s", module.GetVersion(), retrieved.GetVersion())
	}

	if retrieved.GetHash() != module.GetHash() {
		t.Errorf("Expected hash %s, got %s", module.GetHash(), retrieved.GetHash())
	}
}

func TestUpsertModule_Update(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	module := &pb.ModuleProto{
		Name:              "github.com/test/module",
		Version:           "v1.0.0",
		Hash:              "abc123",
		TimestampUnixNano: time.Now().UnixNano(),
	}

	// Insert
	err := storage.UpsertModule(module)
	if err != nil {
		t.Fatalf("First UpsertModule failed: %v", err)
	}

	// Update with new hash
	module.Hash = "def456"

	err = storage.UpsertModule(module)
	if err != nil {
		t.Fatalf("Second UpsertModule failed: %v", err)
	}

	// Verify update
	retrieved, err := storage.GetModule(module.GetName(), module.GetVersion())
	if err != nil {
		t.Fatalf("GetModule failed: %v", err)
	}

	if retrieved.GetHash() != "def456" {
		t.Errorf("Expected updated hash def456, got %s", retrieved.GetHash())
	}
}

func TestGetModule_NotFound(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	_, err := storage.GetModule("nonexistent", "v1.0.0")
	if err == nil {
		t.Fatal("Expected error for non-existent module")
	}

	expectedError := "module not found: nonexistent"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

func TestGetModuleByName(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	moduleName := "github.com/test/module"

	// Insert multiple versions - only the latest should be stored
	versions := []string{"v1.0.0", "v1.1.0", "v1.2.0"}
	for i, version := range versions {
		module := &pb.ModuleProto{
			Name:              moduleName,
			Version:           version,
			Hash:              "hash" + version,
			TimestampUnixNano: time.Now().UnixNano() + int64(i*1000000),
		}

		err := storage.UpsertModule(module)
		if err != nil {
			t.Fatalf("UpsertModule failed for %s: %v", version, err)
		}
	}

	// Retrieve module - should return only 1 (the latest version)
	modules, err := storage.GetModuleByName(moduleName)
	if err != nil {
		t.Fatalf("GetModuleByName failed: %v", err)
	}

	if len(modules) != 1 {
		t.Errorf("Expected 1 module, got %d", len(modules))
	}

	// Verify it's the latest version
	if modules[0].GetVersion() != "v1.2.0" {
		t.Errorf("Expected latest version v1.2.0, got %s", modules[0].GetVersion())
	}
}

func TestListModules(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	// Insert multiple modules with different timestamps
	modules := []*pb.ModuleProto{
		{
			Name:              "github.com/test/module1",
			Version:           "v1.0.0",
			Hash:              "hash1",
			TimestampUnixNano: time.Now().UnixNano(),
		},
		{
			Name:              "github.com/test/module2",
			Version:           "v1.0.0",
			Hash:              "hash2",
			TimestampUnixNano: time.Now().UnixNano() + 1000000,
		},
		{
			Name:              "github.com/test/module3",
			Version:           "v1.0.0",
			Hash:              "hash3",
			TimestampUnixNano: time.Now().UnixNano() + 2000000,
		},
	}

	if err := storage.UpsertModules(modules); err != nil {
		t.Fatalf("UpsertModules failed: %v", err)
	}

	// List all modules
	allModules, err := storage.ListModules()
	if err != nil {
		t.Fatalf("ListModules failed: %v", err)
	}

	if len(allModules) != 3 {
		t.Errorf("Expected 3 modules, got %d", len(allModules))
	}

	// Verify sorted by timestamp descending (most recent first)
	for i := 0; i < len(allModules)-1; i++ {
		if allModules[i].GetTimestampUnixNano() < allModules[i+1].GetTimestampUnixNano() {
			t.Error("Modules not sorted by timestamp descending")
		}
	}
}

func TestDeleteModule(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	module := &pb.ModuleProto{
		Name:              "github.com/test/module",
		Version:           "v1.0.0",
		Hash:              "abc123",
		TimestampUnixNano: time.Now().UnixNano(),
	}

	// Insert module
	err := storage.UpsertModule(module)
	if err != nil {
		t.Fatalf("UpsertModule failed: %v", err)
	}

	// Delete module
	err = storage.DeleteModule(module.GetName(), module.GetVersion())
	if err != nil {
		t.Fatalf("DeleteModule failed: %v", err)
	}

	// Verify deletion
	_, err = storage.GetModule(module.GetName(), module.GetVersion())
	if err == nil {
		t.Fatal("Expected error when getting deleted module")
	}
}

func TestDeleteModule_NotFound(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	err := storage.DeleteModule("nonexistent", "v1.0.0")
	if err == nil {
		t.Fatal("Expected error when deleting non-existent module")
	}
}

func TestCountModules(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	// Initially should be 0
	count, err := storage.CountModules()
	if err != nil {
		t.Fatalf("CountModules failed: %v", err)
	}

	if count != 0 {
		t.Errorf("Expected 0 modules, got %d", count)
	}

	// Insert 3 modules
	for i := 1; i <= 3; i++ {
		module := &pb.ModuleProto{
			Name:              "github.com/test/module" + string(rune('0'+i)),
			Version:           "v1.0.0",
			Hash:              "hash",
			TimestampUnixNano: time.Now().UnixNano() + int64(i*1000000),
		}

		err := storage.UpsertModule(module)
		if err != nil {
			t.Fatalf("UpsertModule failed: %v", err)
		}
	}

	// Should be 3
	count, err = storage.CountModules()
	if err != nil {
		t.Fatalf("CountModules failed: %v", err)
	}

	if count != 3 {
		t.Errorf("Expected 3 modules, got %d", count)
	}
}

func TestUpsertDependencies(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	moduleName := "github.com/test/module"

	deps := &pb.DependenciesProto{
		Dependencies: []*pb.DependencyProto{
			{
				Name:    "github.com/dep/one",
				Version: "v1.0.0",
				Hash:    "hash1",
			},
			{
				Name:    "github.com/dep/two",
				Version: "v2.0.0",
				Hash:    "hash2",
			},
		},
	}

	err := storage.UpsertDependencies(moduleName, deps)
	if err != nil {
		t.Fatalf("UpsertDependencies failed: %v", err)
	}

	// Verify dependencies were stored
	retrieved, err := storage.GetDependenciesByModule(moduleName)
	if err != nil {
		t.Fatalf("GetDependenciesByModule failed: %v", err)
	}

	if len(retrieved.GetDependencies()) != 2 {
		t.Errorf("Expected 2 dependencies, got %d", len(retrieved.GetDependencies()))
	}

	if retrieved.GetDependencies()[0].GetName() != "github.com/dep/one" {
		t.Errorf("Expected first dep name github.com/dep/one, got %s", retrieved.GetDependencies()[0].GetName())
	}
}

func TestGetDependenciesByModule_NotFound(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	_, err := storage.GetDependenciesByModule("nonexistent")
	if err == nil {
		t.Fatal("Expected error for non-existent dependencies")
	}
}

func TestCountDependencies(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	// Initially should be 0
	count, err := storage.CountDependencies()
	if err != nil {
		t.Fatalf("CountDependencies failed: %v", err)
	}

	if count != 0 {
		t.Errorf("Expected 0 dependency entries, got %d", count)
	}

	// Insert dependencies for 2 modules
	for i := 1; i <= 2; i++ {
		deps := &pb.DependenciesProto{
			Dependencies: []*pb.DependencyProto{
				{
					Name:    "github.com/dep/test",
					Version: "v1.0.0",
				},
			},
		}

		err := storage.UpsertDependencies("module"+string(rune('0'+i)), deps)
		if err != nil {
			t.Fatalf("UpsertDependencies failed: %v", err)
		}
	}

	// Should be 2 entries (one per module)
	count, err = storage.CountDependencies()
	if err != nil {
		t.Fatalf("CountDependencies failed: %v", err)
	}

	if count != 2 {
		t.Errorf("Expected 2 dependency entries, got %d", count)
	}
}

func TestModuleVersionUpdate(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	moduleName := "github.com/test/module"

	// Insert multiple versions - only the latest should be stored
	versions := []string{"v1.0.0", "v1.1.0", "v1.2.0"}
	for i, version := range versions {
		module := &pb.ModuleProto{
			Name:              moduleName,
			Version:           version,
			Hash:              "hash" + version,
			TimestampUnixNano: time.Now().UnixNano() + int64(i*1000000),
		}

		err := storage.UpsertModule(module)
		if err != nil {
			t.Fatalf("UpsertModule failed: %v", err)
		}
	}

	// Get by name should return only 1 module (latest version)
	modules, err := storage.GetModuleByName(moduleName)
	if err != nil {
		t.Fatalf("GetModuleByName failed: %v", err)
	}

	if len(modules) != 1 {
		t.Errorf("Expected 1 module, got %d", len(modules))
	}

	if modules[0].GetVersion() != "v1.2.0" {
		t.Errorf("Expected latest version v1.2.0, got %s", modules[0].GetVersion())
	}

	// Delete the module
	err = storage.DeleteModule(moduleName, "")
	if err != nil {
		t.Fatalf("DeleteModule failed: %v", err)
	}

	// Should now return error (not found)
	_, err = storage.GetModuleByName(moduleName)
	if err == nil {
		t.Error("Expected error for deleted module, got nil")
	}
}

func TestTimeIndex(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	// Insert modules with specific timestamps
	now := time.Now()
	modules := []*pb.ModuleProto{
		{
			Name:              "github.com/test/oldest",
			Version:           "v1.0.0",
			TimestampUnixNano: now.Add(-2 * time.Hour).UnixNano(),
		},
		{
			Name:              "github.com/test/middle",
			Version:           "v1.0.0",
			TimestampUnixNano: now.Add(-1 * time.Hour).UnixNano(),
		},
		{
			Name:              "github.com/test/newest",
			Version:           "v1.0.0",
			TimestampUnixNano: now.UnixNano(),
		},
	}

	if err := storage.UpsertModules(modules); err != nil {
		t.Fatalf("UpsertModules failed: %v", err)
	}

	// List should return in descending timestamp order
	allModules, err := storage.ListModules()
	if err != nil {
		t.Fatalf("ListModules failed: %v", err)
	}

	if len(allModules) != 3 {
		t.Errorf("Expected 3 modules, got %d", len(allModules))
	}

	// Verify order: newest first
	if allModules[0].GetName() != "github.com/test/newest" {
		t.Errorf("Expected newest first, got %s", allModules[0].GetName())
	}

	if allModules[1].GetName() != "github.com/test/middle" {
		t.Errorf("Expected middle second, got %s", allModules[1].GetName())
	}

	if allModules[2].GetName() != "github.com/test/oldest" {
		t.Errorf("Expected oldest last, got %s", allModules[2].GetName())
	}
}

func TestConcurrentReads(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	// Insert a module first
	module := &pb.ModuleProto{
		Name:              "github.com/test/module",
		Version:           "v1.0.0",
		Hash:              "abc123",
		TimestampUnixNano: time.Now().UnixNano(),
	}

	err := storage.UpsertModule(module)
	if err != nil {
		t.Fatalf("UpsertModule failed: %v", err)
	}

	// Test concurrent reads (BoltDB supports multiple readers)
	done := make(chan bool)

	for range 5 {
		go func() {
			_, _ = storage.ListModules()

			done <- true
		}()
	}

	for range 5 {
		<-done
	}

	// If we get here without deadlock, test passes
}

func TestModuleWithDependencies(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	// Create module with nested dependencies
	module := &pb.ModuleProto{
		Name:              "github.com/test/module",
		Version:           "v1.0.0",
		Hash:              "abc123",
		TimestampUnixNano: time.Now().UnixNano(),
		Dependencies: []*pb.DependencyProto{
			{
				Name:    "github.com/dep/one",
				Version: "v1.0.0",
				Hash:    "hash1",
				Dependencies: []*pb.DependencyProto{
					{
						Name:    "github.com/dep/nested",
						Version: "v2.0.0",
						Hash:    "hash2",
					},
				},
			},
		},
	}

	err := storage.UpsertModule(module)
	if err != nil {
		t.Fatalf("UpsertModule failed: %v", err)
	}

	// Retrieve and verify nested structure
	retrieved, err := storage.GetModule(module.GetName(), module.GetVersion())
	if err != nil {
		t.Fatalf("GetModule failed: %v", err)
	}

	if len(retrieved.GetDependencies()) != 1 {
		t.Errorf("Expected 1 dependency, got %d", len(retrieved.GetDependencies()))
	}

	if len(retrieved.GetDependencies()[0].GetDependencies()) != 1 {
		t.Errorf("Expected 1 nested dependency, got %d", len(retrieved.GetDependencies()[0].GetDependencies()))
	}

	nestedDep := retrieved.GetDependencies()[0].GetDependencies()[0]
	if nestedDep.GetName() != "github.com/dep/nested" {
		t.Errorf("Expected nested dep name github.com/dep/nested, got %s", nestedDep.GetName())
	}
}

func TestDeleteModule_WithDependencies(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	moduleName := "github.com/test/module"

	// Insert module
	module := &pb.ModuleProto{
		Name:              moduleName,
		Version:           "v1.0.0",
		Hash:              "abc123",
		TimestampUnixNano: time.Now().UnixNano(),
	}

	err := storage.UpsertModule(module)
	if err != nil {
		t.Fatalf("UpsertModule failed: %v", err)
	}

	// Insert dependencies
	deps := &pb.DependenciesProto{
		Dependencies: []*pb.DependencyProto{
			{
				Name:    "github.com/dep/one",
				Version: "v1.0.0",
			},
		},
	}

	err = storage.UpsertDependencies(moduleName, deps)
	if err != nil {
		t.Fatalf("UpsertDependencies failed: %v", err)
	}

	// Delete module (should also delete dependencies)
	err = storage.DeleteModule(moduleName, module.GetVersion())
	if err != nil {
		t.Fatalf("DeleteModule failed: %v", err)
	}

	// Verify dependencies are also deleted
	_, err = storage.GetDependenciesByModule(moduleName)
	if err == nil {
		t.Error("Expected error when getting dependencies for deleted module")
	}
}

func TestEmptyDatabase(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	// Test operations on empty database
	modules, err := storage.ListModules()
	if err != nil {
		t.Fatalf("ListModules failed on empty database: %v", err)
	}

	if len(modules) != 0 {
		t.Errorf("Expected 0 modules in empty database, got %d", len(modules))
	}

	// GetModuleByName should return error for nonexistent module
	_, err = storage.GetModuleByName("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent module, got nil")
	}
}
