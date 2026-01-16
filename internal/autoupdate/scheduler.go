package autoupdate

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/inovacc/glix/internal/module"
	pb "github.com/inovacc/glix/pkg/api/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

// DefaultServerAddress is the default gRPC server address
const DefaultServerAddress = "localhost:9742"

// UpdateResult represents the result of checking/updating a single module
type UpdateResult struct {
	Name            string
	PreviousVersion string
	NewVersion      string
	Updated         bool
	Error           error
}

// CheckResult represents the result of an auto-update check cycle
type CheckResult struct {
	CheckTime    time.Time
	ModulesCount int
	UpdatesFound int
	UpdatesDone  int
	Results      []UpdateResult
	Errors       []error
}

// Scheduler handles periodic update checks
type Scheduler struct {
	logger  *slog.Logger
	store   *configStore
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	mu      sync.Mutex
	running bool
	address string
}

// NewScheduler creates a new auto-update scheduler
func NewScheduler(logger *slog.Logger) *Scheduler {
	if logger == nil {
		logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))
	}

	return &Scheduler{
		logger:  logger,
		store:   GetStore(),
		address: DefaultServerAddress,
	}
}

// SetAddress sets the server address for the scheduler
func (s *Scheduler) SetAddress(address string) {
	s.address = address
}

// Start begins the auto-update scheduler
func (s *Scheduler) Start(ctx context.Context) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}

	ctx, s.cancel = context.WithCancel(ctx)
	s.running = true
	s.mu.Unlock()

	s.wg.Add(1)
	go s.run(ctx)

	s.logger.Info("auto-update scheduler started")
}

// Stop stops the auto-update scheduler
func (s *Scheduler) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}

	if s.cancel != nil {
		s.cancel()
	}
	s.running = false
	s.mu.Unlock()

	s.wg.Wait()
	s.logger.Info("auto-update scheduler stopped")
}

// IsRunning returns whether the scheduler is running
func (s *Scheduler) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// run is the main scheduler loop
func (s *Scheduler) run(ctx context.Context) {
	defer s.wg.Done()

	// Check immediately if we should
	if s.store.ShouldCheck() {
		s.performCheck(ctx)
	}

	// Then check periodically
	ticker := time.NewTicker(time.Minute) // Check every minute if it's time
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if s.store.ShouldCheck() {
				s.performCheck(ctx)
			}
		}
	}
}

// performCheck performs an update check cycle
func (s *Scheduler) performCheck(ctx context.Context) {
	s.logger.Info("starting auto-update check")

	result, err := s.CheckAndUpdate(ctx)
	if err != nil {
		s.logger.Error("auto-update check failed", "error", err)
		return
	}

	s.logger.Info("auto-update check completed",
		"modules", result.ModulesCount,
		"updates_found", result.UpdatesFound,
		"updates_done", result.UpdatesDone,
	)

	// Record the check
	if err := s.store.RecordCheck(result.UpdatesDone); err != nil {
		s.logger.Error("failed to record check", "error", err)
	}
}

// connectToServer creates a gRPC connection to the server
func (s *Scheduler) connectToServer(ctx context.Context) (pb.GlixServiceClient, *grpc.ClientConn, error) {
	dialCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(dialCtx, s.address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to server at %s: %w", s.address, err)
	}

	return pb.NewGlixServiceClient(conn), conn, nil
}

// CheckAndUpdate checks for updates and optionally applies them
func (s *Scheduler) CheckAndUpdate(ctx context.Context) (*CheckResult, error) {
	result := &CheckResult{
		CheckTime: time.Now(),
		Results:   make([]UpdateResult, 0),
		Errors:    make([]error, 0),
	}

	// Get config
	cfg := s.store.Get()

	// Connect to server
	client, conn, err := s.connectToServer(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = conn.Close()
	}()

	// List all installed modules
	resp, err := client.ListModules(ctx, &pb.ListModulesRequest{})
	if err != nil {
		return nil, fmt.Errorf("failed to list modules: %w", err)
	}

	modules := resp.GetModules()
	result.ModulesCount = len(modules)

	if len(modules) == 0 {
		return result, nil
	}

	// Check each module
	for _, mod := range modules {
		modResult := s.checkModule(ctx, mod.GetName(), mod.GetVersion(), cfg.NotifyOnly, client)
		result.Results = append(result.Results, modResult)

		if modResult.Error != nil {
			result.Errors = append(result.Errors, modResult.Error)
		} else if modResult.NewVersion != modResult.PreviousVersion {
			result.UpdatesFound++
			if modResult.Updated {
				result.UpdatesDone++
			}
		}
	}

	return result, nil
}

// checkModule checks a single module for updates
func (s *Scheduler) checkModule(ctx context.Context, name, installedVersion string, notifyOnly bool, client pb.GlixServiceClient) UpdateResult {
	result := UpdateResult{
		Name:            name,
		PreviousVersion: installedVersion,
	}

	// Create working directory
	cacheDir, err := module.GetApplicationCacheDirectory()
	if err != nil {
		result.Error = err
		return result
	}

	workDir := filepath.Join(cacheDir, fmt.Sprintf("autoupdate-%d", time.Now().UnixNano()))
	if err := os.MkdirAll(workDir, 0755); err != nil {
		result.Error = err
		return result
	}
	defer func() {
		_ = os.RemoveAll(workDir)
	}()

	// Fetch latest version info
	m, err := module.NewModule(ctx, "go", workDir)
	if err != nil {
		result.Error = err
		return result
	}

	if err := m.FetchModuleInfo(name); err != nil {
		result.Error = err
		return result
	}

	result.NewVersion = m.Version

	// Check if update is available
	if !isNewerVersion(m.Version, installedVersion) {
		return result // Already up to date
	}

	s.logger.Info("update available",
		"module", name,
		"current", installedVersion,
		"latest", m.Version,
	)

	// If notify only, don't install
	if notifyOnly {
		return result
	}

	// Install the update
	outputHandler := func(stream string, line string) {
		// Silent - could log at debug level if needed
	}

	if err := m.InstallModuleWithStreaming(ctx, outputHandler); err != nil {
		result.Error = fmt.Errorf("failed to install update: %w", err)
		return result
	}

	// Store updated module info
	if err := s.storeModule(ctx, client, m); err != nil {
		result.Error = fmt.Errorf("failed to store update: %w", err)
		return result
	}

	result.Updated = true
	s.logger.Info("module updated",
		"module", name,
		"from", installedVersion,
		"to", m.Version,
	)

	return result
}

// storeModule stores the module in the database via gRPC
func (s *Scheduler) storeModule(ctx context.Context, client pb.GlixServiceClient, m *module.Module) error {
	// Convert module to proto
	moduleProto := &pb.ModuleProto{
		Name:              m.Name,
		Version:           m.Version,
		Versions:          m.Versions,
		Hash:              m.Hash,
		TimestampUnixNano: m.Time.UnixNano(),
	}

	// Convert dependencies
	var deps []*pb.DependencyProto
	for _, d := range m.Dependencies {
		deps = append(deps, &pb.DependencyProto{
			Name:     d.Name,
			Version:  d.Version,
			Versions: d.Versions,
			Hash:     d.Hash,
		})
	}

	depsProto := &pb.DependenciesProto{
		Dependencies: deps,
	}

	resp, err := client.StoreModule(ctx, &pb.StoreModuleRequest{
		Module:       moduleProto,
		Dependencies: depsProto,
	})
	if err != nil {
		return fmt.Errorf("failed to store module: %w", err)
	}

	if !resp.GetSuccess() {
		return fmt.Errorf("failed to store module: %s", resp.GetErrorMessage())
	}

	return nil
}

// isNewerVersion compares two versions and returns true if newVer is newer than oldVer
func isNewerVersion(newVer, oldVer string) bool {
	// Ensure versions have 'v' prefix for semver comparison
	if newVer != "" && newVer[0] != 'v' {
		newVer = "v" + newVer
	}
	if oldVer != "" && oldVer[0] != 'v' {
		oldVer = "v" + oldVer
	}

	// If versions are identical, no update needed
	if newVer == oldVer {
		return false
	}

	// For pseudo-versions or non-standard versions, do string comparison
	return newVer > oldVer
}

// RunOnce performs a single update check immediately
func (s *Scheduler) RunOnce(ctx context.Context) (*CheckResult, error) {
	result, err := s.CheckAndUpdate(ctx)
	if err != nil {
		return nil, err
	}

	// Record the check
	if err := s.store.RecordCheck(result.UpdatesDone); err != nil {
		s.logger.Error("failed to record check", "error", err)
	}

	return result, nil
}

// Ping checks if the server is available
func (s *Scheduler) Ping(ctx context.Context) error {
	client, conn, err := s.connectToServer(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = conn.Close()
	}()

	_, err = client.Ping(ctx, &emptypb.Empty{})
	return err
}
