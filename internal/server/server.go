package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"sync"
	"time"

	"github.com/inovacc/glix/internal/autoupdate"
	"github.com/inovacc/glix/internal/database"
	"github.com/inovacc/glix/internal/module"
	pb "github.com/inovacc/glix/pkg/api/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// DefaultPort is the default gRPC server port
const DefaultPort = 9742

// Config holds the server configuration
type Config struct {
	Namespace    string
	DatabasePath string
	Port         int
	BindAddress  string
	IdleTimeout  time.Duration // If > 0, server shuts down after this duration of inactivity
	Logger       *slog.Logger
}

// Server represents the gRPC server for glix
type Server struct {
	pb.UnimplementedGlixServiceServer

	config       Config
	db           *database.Storage
	grpcSrv      *grpc.Server
	listener     net.Listener
	startTime    time.Time
	lastActivity time.Time
	logger       *slog.Logger
	cancelIdle   context.CancelFunc
	autoUpdater  *autoupdate.Scheduler

	mu      sync.RWMutex
	running bool
}

// New creates a new gRPC server instance
func New(cfg Config) (*Server, error) {
	// Set defaults
	if cfg.Port == 0 {
		cfg.Port = DefaultPort
	}

	if cfg.BindAddress == "" {
		cfg.BindAddress = "localhost"
	}

	if cfg.Namespace == "" {
		hostname, err := os.Hostname()
		if err != nil {
			cfg.Namespace = "default"
		} else {
			cfg.Namespace = hostname
		}
	}

	if cfg.DatabasePath == "" {
		cfg.DatabasePath = module.GetDatabaseDirectory()
	}

	if cfg.Logger == nil {
		cfg.Logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))
	}

	// Open database
	db, err := database.NewStorage(cfg.DatabasePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	return &Server{
		config:      cfg,
		db:          db,
		logger:      cfg.Logger,
		autoUpdater: autoupdate.NewScheduler(cfg.Logger),
	}, nil
}

// Start starts the gRPC server
func (s *Server) Start(ctx context.Context) error {
	s.mu.Lock()

	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("server is already running")
	}

	addr := fmt.Sprintf("%s:%d", s.config.BindAddress, s.config.Port)

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		s.mu.Unlock()
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	s.listener = listener
	s.grpcSrv = grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			s.activityInterceptor,
			s.loggingInterceptor,
			s.recoveryInterceptor,
		),
		grpc.ChainStreamInterceptor(
			s.streamActivityInterceptor,
			s.streamLoggingInterceptor,
			s.streamRecoveryInterceptor,
		),
	)

	// Register the service
	pb.RegisterGlixServiceServer(s.grpcSrv, s)

	// Enable reflection for debugging
	reflection.Register(s.grpcSrv)

	s.startTime = time.Now()
	s.lastActivity = time.Now()
	s.running = true
	s.mu.Unlock()

	s.logger.Info("gRPC server started",
		"address", addr,
		"namespace", s.config.Namespace,
		"database", s.config.DatabasePath,
		"idle_timeout", s.config.IdleTimeout,
	)

	// Handle context cancellation
	go func() {
		<-ctx.Done()
		s.Stop()
	}()

	// Start idle monitor if timeout is configured
	if s.config.IdleTimeout > 0 {
		idleCtx, cancel := context.WithCancel(ctx)

		s.cancelIdle = cancel
		go s.monitorIdle(idleCtx)
	}

	// Start auto-update scheduler
	if s.autoUpdater != nil {
		s.autoUpdater.Start(ctx)
	}

	// Serve requests
	if err := s.grpcSrv.Serve(listener); err != nil {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}

// touchActivity updates the last activity timestamp
func (s *Server) touchActivity() {
	s.mu.Lock()
	s.lastActivity = time.Now()
	s.mu.Unlock()
}

// monitorIdle monitors for idle timeout and shuts down the server
func (s *Server) monitorIdle(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.mu.RLock()
			idle := time.Since(s.lastActivity)
			s.mu.RUnlock()

			if idle >= s.config.IdleTimeout {
				s.logger.Info("idle timeout reached, shutting down",
					"idle_duration", idle,
					"timeout", s.config.IdleTimeout,
				)
				s.Stop()

				return
			}
		}
	}
}

// Stop gracefully stops the gRPC server
func (s *Server) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	s.logger.Info("stopping gRPC server")

	// Stop auto-update scheduler
	if s.autoUpdater != nil {
		s.autoUpdater.Stop()
	}

	if s.grpcSrv != nil {
		s.grpcSrv.GracefulStop()
	}

	if s.db != nil {
		if err := s.db.Close(); err != nil {
			s.logger.Error("error closing database", "error", err)
		}
	}

	s.running = false
	s.logger.Info("gRPC server stopped")
}

// IsRunning returns whether the server is currently running
func (s *Server) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.running
}

// Address returns the server address
func (s *Server) Address() string {
	return fmt.Sprintf("%s:%d", s.config.BindAddress, s.config.Port)
}

// Uptime returns the server uptime in seconds
func (s *Server) Uptime() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.running {
		return 0
	}

	return int64(time.Since(s.startTime).Seconds())
}
