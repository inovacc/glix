package server

import (
	"context"
	"fmt"
	"time"

	"github.com/inovacc/glix/internal/module"
	pb "github.com/inovacc/glix/pkg/api/v1"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Install installs a Go module (non-streaming)
func (s *Server) Install(ctx context.Context, req *pb.InstallRequest) (*pb.InstallResponse, error) {
	s.logger.Info("install request",
		"module", req.GetModulePath(),
		"version", req.GetVersion(),
		"force", req.GetForce(),
	)

	cacheDir, err := module.GetApplicationCacheDirectory()
	if err != nil {
		return &pb.InstallResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to get cache directory: %v", err),
		}, nil
	}

	m, err := module.NewModule(ctx, "go", cacheDir)
	if err != nil {
		return &pb.InstallResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to create module: %v", err),
		}, nil
	}

	modulePath := req.GetModulePath()
	if req.GetVersion() != "" && req.GetVersion() != "latest" {
		modulePath = fmt.Sprintf("%s@%s", req.GetModulePath(), req.GetVersion())
	}

	if err := m.FetchModuleInfo(modulePath); err != nil {
		return &pb.InstallResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to fetch module info: %v", err),
		}, nil
	}

	if err := m.InstallModuleWithStreaming(ctx, nil); err != nil {
		return &pb.InstallResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to install module: %v", err),
		}, nil
	}

	if err := m.Report(s.db); err != nil {
		s.logger.Warn("failed to store module in database", "error", err)
	}

	// Convert to protobuf
	modProto := moduleToProto(m)

	return &pb.InstallResponse{
		Module:  modProto,
		Success: true,
	}, nil
}

// InstallStream installs a Go module with streaming output
func (s *Server) InstallStream(req *pb.InstallRequest, stream grpc.ServerStreamingServer[pb.InstallProgress]) error {
	ctx := stream.Context()

	s.logger.Info("install stream request",
		"module", req.GetModulePath(),
		"version", req.GetVersion(),
		"force", req.GetForce(),
	)

	// Send initial progress
	if err := stream.Send(&pb.InstallProgress{
		Update: &pb.InstallProgress_Progress{
			Progress: &pb.ProgressUpdate{
				Phase:           "initializing",
				Message:         "Preparing installation",
				PercentComplete: 0,
			},
		},
	}); err != nil {
		return fmt.Errorf("failed to send progress: %w", err)
	}

	cacheDir, err := module.GetApplicationCacheDirectory()
	if err != nil {
		return sendInstallError(stream, fmt.Sprintf("failed to get cache directory: %v", err))
	}

	m, err := module.NewModule(ctx, "go", cacheDir)
	if err != nil {
		return sendInstallError(stream, fmt.Sprintf("failed to create module: %v", err))
	}

	modulePath := req.GetModulePath()
	if req.GetVersion() != "" && req.GetVersion() != "latest" {
		modulePath = fmt.Sprintf("%s@%s", req.GetModulePath(), req.GetVersion())
	}

	// Send fetching progress
	if err := stream.Send(&pb.InstallProgress{
		Update: &pb.InstallProgress_Progress{
			Progress: &pb.ProgressUpdate{
				Phase:           "fetching",
				Message:         "Fetching module information",
				PercentComplete: 10,
			},
		},
	}); err != nil {
		return fmt.Errorf("failed to send progress: %w", err)
	}

	if err := m.FetchModuleInfo(modulePath); err != nil {
		return sendInstallError(stream, fmt.Sprintf("failed to fetch module info: %v", err))
	}

	// Send installing progress
	if err := stream.Send(&pb.InstallProgress{
		Update: &pb.InstallProgress_Progress{
			Progress: &pb.ProgressUpdate{
				Phase:           "installing",
				Message:         fmt.Sprintf("Installing %s@%s", m.Name, m.Version),
				PercentComplete: 30,
			},
		},
	}); err != nil {
		return fmt.Errorf("failed to send progress: %w", err)
	}

	// Create output handler that streams to client
	outputHandler := func(streamType string, line string) {
		outputLine := &pb.OutputLine{
			Line:              line,
			TimestampUnixNano: time.Now().UnixNano(),
		}
		if streamType == "stderr" {
			outputLine.Stream = pb.OutputLine_STDERR
		} else {
			outputLine.Stream = pb.OutputLine_STDOUT
		}

		if err := stream.Send(&pb.InstallProgress{
			Update: &pb.InstallProgress_Output{
				Output: outputLine,
			},
		}); err != nil {
			s.logger.Warn("failed to send output", "error", err)
		}
	}

	if err := m.InstallModuleWithStreaming(ctx, outputHandler); err != nil {
		return sendInstallError(stream, fmt.Sprintf("failed to install module: %v", err))
	}

	// Store in database
	if err := m.Report(s.db); err != nil {
		s.logger.Warn("failed to store module in database", "error", err)
	}

	// Send final result
	modProto := moduleToProto(m)
	if err := stream.Send(&pb.InstallProgress{
		Update: &pb.InstallProgress_Result{
			Result: &pb.InstallResponse{
				Module:  modProto,
				Success: true,
			},
		},
	}); err != nil {
		return fmt.Errorf("failed to send result: %w", err)
	}

	return nil
}

// Remove removes an installed module
func (s *Server) Remove(ctx context.Context, req *pb.RemoveRequest) (*pb.RemoveResponse, error) {
	s.logger.Info("remove request",
		"module", req.GetModulePath(),
		"version", req.GetVersion(),
	)

	version := req.GetVersion()

	// If no version specified, look up the module by name to find installed versions
	if version == "" {
		mods, err := s.db.GetModuleByName(req.GetModulePath())
		if err != nil || len(mods) == 0 {
			return &pb.RemoveResponse{
				Success:      false,
				ErrorMessage: fmt.Sprintf("module not found: %s", req.GetModulePath()),
			}, nil
		}

		// Remove all versions found
		var lastErr error
		removed := 0
		for _, mod := range mods {
			if err := s.db.DeleteModule(mod.GetName(), mod.GetVersion()); err != nil {
				lastErr = err
			} else {
				removed++
			}
		}

		if removed == 0 && lastErr != nil {
			return &pb.RemoveResponse{
				Success:      false,
				ErrorMessage: fmt.Sprintf("failed to delete module: %v", lastErr),
			}, nil
		}

		return &pb.RemoveResponse{
			Success: true,
		}, nil
	}

	if err := s.db.DeleteModule(req.GetModulePath(), version); err != nil {
		return &pb.RemoveResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to delete module: %v", err),
		}, nil
	}

	return &pb.RemoveResponse{
		Success: true,
	}, nil
}

// Update updates a module (stub for now)
func (s *Server) Update(ctx context.Context, req *pb.UpdateRequest) (*pb.UpdateResponse, error) {
	s.logger.Info("update request", "module", req.GetModulePath())

	return &pb.UpdateResponse{
		Success:      false,
		ErrorMessage: "update not yet implemented",
	}, nil
}

// UpdateStream updates a module with streaming (stub for now)
func (s *Server) UpdateStream(req *pb.UpdateRequest, stream grpc.ServerStreamingServer[pb.InstallProgress]) error {
	s.logger.Info("update stream request", "module", req.GetModulePath())

	return sendInstallError(stream, "update not yet implemented")
}

// ListModules returns all installed modules
func (s *Server) ListModules(ctx context.Context, req *pb.ListModulesRequest) (*pb.ListModulesResponse, error) {
	s.logger.Debug("list modules request",
		"limit", req.GetLimit(),
		"offset", req.GetOffset(),
		"filter", req.GetNameFilter(),
	)

	modules, err := s.db.ListModules()
	if err != nil {
		return nil, fmt.Errorf("failed to list modules: %w", err)
	}

	// Apply filtering if provided
	var filteredModules []*pb.ModuleProto

	for _, m := range modules {
		if req.GetNameFilter() != "" {
			// Simple contains filter
			if !containsIgnoreCase(m.GetName(), req.GetNameFilter()) {
				continue
			}
		}

		filteredModules = append(filteredModules, m)
	}

	totalCount := int64(len(filteredModules))

	// Apply pagination
	offset := int(req.GetOffset())
	limit := int(req.GetLimit())

	if offset > len(filteredModules) {
		filteredModules = nil
	} else {
		filteredModules = filteredModules[offset:]
		if limit > 0 && limit < len(filteredModules) {
			filteredModules = filteredModules[:limit]
		}
	}

	return &pb.ListModulesResponse{
		Modules:    filteredModules,
		TotalCount: totalCount,
	}, nil
}

// GetModule retrieves a specific module
func (s *Server) GetModule(ctx context.Context, req *pb.GetModuleRequest) (*pb.GetModuleResponse, error) {
	s.logger.Debug("get module request",
		"name", req.GetName(),
		"version", req.GetVersion(),
	)

	var (
		mod *pb.ModuleProto
		err error
	)

	if req.GetVersion() != "" {
		mod, err = s.db.GetModule(req.GetName(), req.GetVersion())
	} else {
		// Get by name (returns all versions, pick latest)
		mods, getErr := s.db.GetModuleByName(req.GetName())
		if getErr != nil || len(mods) == 0 {
			return &pb.GetModuleResponse{
				Found: false,
			}, nil
		}

		mod = mods[0] // Return the first (most recent) one
		err = nil
	}

	if err != nil {
		return &pb.GetModuleResponse{
			Found: false,
		}, nil
	}

	return &pb.GetModuleResponse{
		Module: mod,
		Found:  true,
	}, nil
}

// GetDependencies retrieves dependencies for a module
func (s *Server) GetDependencies(ctx context.Context, req *pb.GetModuleRequest) (*pb.GetDependenciesResponse, error) {
	s.logger.Debug("get dependencies request",
		"name", req.GetName(),
		"version", req.GetVersion(),
	)

	key := req.GetName()
	if req.GetVersion() != "" {
		key = fmt.Sprintf("%s@%s", req.GetName(), req.GetVersion())
	}

	deps, err := s.db.GetDependenciesByModule(key)
	if err != nil {
		return &pb.GetDependenciesResponse{
			Found: false,
		}, nil
	}

	return &pb.GetDependenciesResponse{
		Dependencies: deps,
		Found:        true,
	}, nil
}

// GetStatus returns the server status
func (s *Server) GetStatus(ctx context.Context, _ *emptypb.Empty) (*pb.ServerStatus, error) {
	moduleCount, err := s.db.CountModules()
	if err != nil {
		moduleCount = 0
	}

	return &pb.ServerStatus{
		Running:       s.IsRunning(),
		Namespace:     s.config.Namespace,
		DatabasePath:  s.config.DatabasePath,
		Address:       s.Address(),
		UptimeSeconds: s.Uptime(),
		ModuleCount:   moduleCount,
	}, nil
}

// Ping is a health check endpoint
func (s *Server) Ping(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

// Helper functions

func sendInstallError(stream grpc.ServerStreamingServer[pb.InstallProgress], errMsg string) error {
	if err := stream.Send(&pb.InstallProgress{
		Update: &pb.InstallProgress_Result{
			Result: &pb.InstallResponse{
				Success:      false,
				ErrorMessage: errMsg,
			},
		},
	}); err != nil {
		return fmt.Errorf("failed to send error: %w", err)
	}

	return nil
}

func moduleToProto(m *module.Module) *pb.ModuleProto {
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

	return &pb.ModuleProto{
		Name:              m.Name,
		Version:           m.Version,
		Versions:          m.Versions,
		Dependencies:      deps,
		Hash:              m.Hash,
		TimestampUnixNano: time.Now().UnixNano(),
	}
}

func containsIgnoreCase(s, substr string) bool {
	// Simple case-insensitive contains
	for i := 0; i <= len(s)-len(substr); i++ {
		if equalIgnoreCase(s[i:i+len(substr)], substr) {
			return true
		}
	}

	return false
}

func equalIgnoreCase(a, b string) bool {
	if len(a) != len(b) {
		return false
	}

	for i := 0; i < len(a); i++ {
		ca := a[i]
		cb := b[i]

		if ca >= 'A' && ca <= 'Z' {
			ca += 'a' - 'A'
		}

		if cb >= 'A' && cb <= 'Z' {
			cb += 'a' - 'A'
		}

		if ca != cb {
			return false
		}
	}

	return true
}
