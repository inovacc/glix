package server

import (
	"context"
	"fmt"

	pb "github.com/inovacc/glix/pkg/api/v1"
	"google.golang.org/protobuf/types/known/emptypb"
)

// StoreModule stores module info in the database (called by CLI after local installation)
func (s *Server) StoreModule(ctx context.Context, req *pb.StoreModuleRequest) (*pb.StoreModuleResponse, error) {
	s.logger.Info("store module request",
		"name", req.GetModule().GetName(),
		"version", req.GetModule().GetVersion(),
	)

	// Store module
	if err := s.db.UpsertModule(req.GetModule()); err != nil {
		return &pb.StoreModuleResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to store module: %v", err),
		}, nil
	}

	// Store dependencies if provided
	if req.GetDependencies() != nil && len(req.GetDependencies().GetDependencies()) > 0 {
		if err := s.db.UpsertDependencies(req.GetModule().GetName(), req.GetDependencies()); err != nil {
			s.logger.Warn("failed to store dependencies", "error", err)
		}
	}

	return &pb.StoreModuleResponse{
		Success: true,
	}, nil
}

// Remove removes an installed module from the database
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
