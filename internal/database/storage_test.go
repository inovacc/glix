package database

import (
	"context"
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
		db:   db,
		path: dbPath,
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

	if storage.path == "" {
		t.Fatal("Expected path to be set")
	}
}

func TestUpsertModule(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	ctx := context.Background()

	module := &pb.ModuleProto{
		Name:              "github.com/test/module",
		Version:           "v1.0.0",
		Versions:          []string{"v1.0.0", "v0.9.0"},
		Hash:              "abc123",
		TimestampUnixNano: time.Now().UnixNano(),
	}

	err := storage.UpsertModule(ctx, module)
	if err != nil {
		t.Fatalf("UpsertModule failed: %v", err)
	}

	// Verify module was inserted
	retrieved, err := storage.GetModule(ctx, module.GetName(), module.GetVersion())
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

	ctx := context.Background()

	module := &pb.ModuleProto{
		Name:              "github.com/test/module",
		Version:           "v1.0.0",
		Hash:              "abc123",
		TimestampUnixNano: time.Now().UnixNano(),
	}

	// Insert
	err := storage.UpsertModule(ctx, module)
	if err != nil {
		t.Fatalf("First UpsertModule failed: %v", err)
	}

	// Update with new hash
	module.Hash = "def456"
	err = storage.UpsertModule(ctx, module)
	if err != nil {
		t.Fatalf("Second UpsertModule failed: %v", err)
	}

	// Verify update
	retrieved, err := storage.GetModule(ctx, module.GetName(), module.GetVersion())
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

	ctx := context.Background()

	_, err := storage.GetModule(ctx, "nonexistent", "v1.0.0")
	if err == nil {
		t.Fatal("Expected error for non-existent module")
	}

	expectedError := "module not found: nonexistent@v1.0.0"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

func TestGetModuleByName(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	ctx := context.Background()

	moduleName := "github.com/test/module"

	// Insert multiple versions
	versions := []string{"v1.0.0", "v1.1.0", "v1.2.0"}
	for i, version := range versions {
		module := &pb.ModuleProto{
			Name:              moduleName,
			Version:           version,
			Hash:              "hash" + version,
			TimestampUnixNano: time.Now().UnixNano() + int64(i*1000000),
		}

		err := storage.UpsertModule(ctx, module)
		if err != nil {
			t.Fatalf("UpsertModule failed for %s: %v", version, err)
		}
	}

	// Retrieve all versions
	modules, err := storage.GetModuleByName(ctx, moduleName)
	if err != nil {
		t.Fatalf("GetModuleByName failed: %v", err)
	}

	if len(modules) != 3 {
		t.Errorf("Expected 3 modules, got %d", len(modules))
	}

	// Verify sorted by timestamp descending (most recent first)
	for i := 0; i < len(modules)-1; i++ {
		if modules[i].GetTimestampUnixNano() < modules[i+1].GetTimestampUnixNano() {
			t.Error("Modules not sorted by timestamp descending")
		}
	}
}

func TestListModules(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	ctx := context.Background()

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

	for _, module := range modules {
		err := storage.UpsertModule(ctx, module)
		if err != nil {
			t.Fatalf("UpsertModule failed: %v", err)
		}
	}

	// List all modules
	allModules, err := storage.ListModules(ctx)
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

	ctx := context.Background()

	module := &pb.ModuleProto{
		Name:              "github.com/test/module",
		Version:           "v1.0.0",
		Hash:              "abc123",
		TimestampUnixNano: time.Now().UnixNano(),
	}

	// Insert module
	err := storage.UpsertModule(ctx, module)
	if err != nil {
		t.Fatalf("UpsertModule failed: %v", err)
	}

	// Delete module
	err = storage.DeleteModule(ctx, module.GetName(), module.GetVersion())
	if err != nil {
		t.Fatalf("DeleteModule failed: %v", err)
	}

	// Verify deletion
	_, err = storage.GetModule(ctx, module.GetName(), module.GetVersion())
	if err == nil {
		t.Fatal("Expected error when getting deleted module")
	}
}

func TestDeleteModule_NotFound(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	ctx := context.Background()

	err := storage.DeleteModule(ctx, "nonexistent", "v1.0.0")
	if err == nil {
		t.Fatal("Expected error when deleting non-existent module")
	}
}

func TestCountModules(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	ctx := context.Background()

	// Initially should be 0
	count, err := storage.CountModules(ctx)
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

		err := storage.UpsertModule(ctx, module)
		if err != nil {
			t.Fatalf("UpsertModule failed: %v", err)
		}
	}

	// Should be 3
	count, err = storage.CountModules(ctx)
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

	ctx := context.Background()

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

	err := storage.UpsertDependencies(ctx, moduleName, deps)
	if err != nil {
		t.Fatalf("UpsertDependencies failed: %v", err)
	}

	// Verify dependencies were stored
	retrieved, err := storage.GetDependenciesByModule(ctx, moduleName)
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

	ctx := context.Background()

	_, err := storage.GetDependenciesByModule(ctx, "nonexistent")
	if err == nil {
		t.Fatal("Expected error for non-existent dependencies")
	}
}

func TestCountDependencies(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	ctx := context.Background()

	// Initially should be 0
	count, err := storage.CountDependencies(ctx)
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

		err := storage.UpsertDependencies(ctx, "module"+string(rune('0'+i)), deps)
		if err != nil {
			t.Fatalf("UpsertDependencies failed: %v", err)
		}
	}

	// Should be 2 entries (one per module)
	count, err = storage.CountDependencies(ctx)
	if err != nil {
		t.Fatalf("CountDependencies failed: %v", err)
	}

	if count != 2 {
		t.Errorf("Expected 2 dependency entries, got %d", count)
	}
}

func TestNameIndex(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	ctx := context.Background()

	moduleName := "github.com/test/module"

	// Insert multiple versions
	versions := []string{"v1.0.0", "v1.1.0", "v1.2.0"}
	for i, version := range versions {
		module := &pb.ModuleProto{
			Name:              moduleName,
			Version:           version,
			Hash:              "hash",
			TimestampUnixNano: time.Now().UnixNano() + int64(i*1000000),
		}

		err := storage.UpsertModule(ctx, module)
		if err != nil {
			t.Fatalf("UpsertModule failed: %v", err)
		}
	}

	// Get by name should return all versions
	modules, err := storage.GetModuleByName(ctx, moduleName)
	if err != nil {
		t.Fatalf("GetModuleByName failed: %v", err)
	}

	if len(modules) != 3 {
		t.Errorf("Expected 3 versions, got %d", len(modules))
	}

	// Delete one version
	err = storage.DeleteModule(ctx, moduleName, "v1.0.0")
	if err != nil {
		t.Fatalf("DeleteModule failed: %v", err)
	}

	// Should now return 2 versions
	modules, err = storage.GetModuleByName(ctx, moduleName)
	if err != nil {
		t.Fatalf("GetModuleByName failed after delete: %v", err)
	}

	if len(modules) != 2 {
		t.Errorf("Expected 2 versions after delete, got %d", len(modules))
	}
}

func TestTimeIndex(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	ctx := context.Background()

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

	for _, module := range modules {
		err := storage.UpsertModule(ctx, module)
		if err != nil {
			t.Fatalf("UpsertModule failed: %v", err)
		}
	}

	// List should return in descending timestamp order
	allModules, err := storage.ListModules(ctx)
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

	ctx := context.Background()

	// Insert a module first
	module := &pb.ModuleProto{
		Name:              "github.com/test/module",
		Version:           "v1.0.0",
		Hash:              "abc123",
		TimestampUnixNano: time.Now().UnixNano(),
	}

	err := storage.UpsertModule(ctx, module)
	if err != nil {
		t.Fatalf("UpsertModule failed: %v", err)
	}

	// Test concurrent reads (BoltDB supports multiple readers)
	done := make(chan bool)

	for i := 0; i < 5; i++ {
		go func() {
			_, _ = storage.ListModules(ctx)
			done <- true
		}()
	}

	for i := 0; i < 5; i++ {
		<-done
	}

	// If we get here without deadlock, test passes
}

func TestModuleWithDependencies(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	ctx := context.Background()

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

	err := storage.UpsertModule(ctx, module)
	if err != nil {
		t.Fatalf("UpsertModule failed: %v", err)
	}

	// Retrieve and verify nested structure
	retrieved, err := storage.GetModule(ctx, module.GetName(), module.GetVersion())
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

	ctx := context.Background()

	moduleName := "github.com/test/module"

	// Insert module
	module := &pb.ModuleProto{
		Name:              moduleName,
		Version:           "v1.0.0",
		Hash:              "abc123",
		TimestampUnixNano: time.Now().UnixNano(),
	}

	err := storage.UpsertModule(ctx, module)
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

	err = storage.UpsertDependencies(ctx, moduleName, deps)
	if err != nil {
		t.Fatalf("UpsertDependencies failed: %v", err)
	}

	// Delete module (should also delete dependencies)
	err = storage.DeleteModule(ctx, moduleName, module.GetVersion())
	if err != nil {
		t.Fatalf("DeleteModule failed: %v", err)
	}

	// Verify dependencies are also deleted
	_, err = storage.GetDependenciesByModule(ctx, moduleName)
	if err == nil {
		t.Error("Expected error when getting dependencies for deleted module")
	}
}

func TestEmptyDatabase(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	ctx := context.Background()

	// Test operations on empty database
	modules, err := storage.ListModules(ctx)
	if err != nil {
		t.Fatalf("ListModules failed on empty database: %v", err)
	}

	if len(modules) != 0 {
		t.Errorf("Expected 0 modules in empty database, got %d", len(modules))
	}

	modules, err = storage.GetModuleByName(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("GetModuleByName failed on empty database: %v", err)
	}

	if len(modules) != 0 {
		t.Errorf("Expected 0 modules for nonexistent name, got %d", len(modules))
	}
}
