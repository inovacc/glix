package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/inovacc/glix/internal/module"
	glixServer "github.com/inovacc/glix/internal/server"
	"github.com/spf13/cobra"
)

var serviceRunCmd = &cobra.Command{
	Use:    "run",
	Short:  "Run the gRPC server directly",
	Long:   `Run the glix gRPC server directly. This command is typically called by the service manager.`,
	Hidden: true, // Hidden because it's typically called by service managers
	RunE:   runServiceRun,
}

var (
	runNamespace    string
	runDatabasePath string
	runPort         int
	runBindAddress  string
	runIdleTimeout  time.Duration
)

func init() {
	serviceCmd.AddCommand(serviceRunCmd)

	serviceRunCmd.Flags().StringVar(&runNamespace, "namespace", "", "Namespace for the server (defaults to hostname)")
	serviceRunCmd.Flags().StringVar(&runDatabasePath, "database", "", "Path to the database file")
	serviceRunCmd.Flags().IntVar(&runPort, "port", glixServer.DefaultPort, "Port for the gRPC server")
	serviceRunCmd.Flags().StringVar(&runBindAddress, "bind", "localhost", "Address to bind the server to")
	serviceRunCmd.Flags().DurationVar(&runIdleTimeout, "idle-timeout", 0, "Shutdown after this duration of inactivity (0 = disabled)")
}

func runServiceRun(cmd *cobra.Command, args []string) error {
	// Use default database path if not specified
	dbPath := runDatabasePath
	if dbPath == "" {
		dbPath = module.GetDatabaseDirectory()
	}

	// Set up logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	cfg := glixServer.Config{
		Namespace:    runNamespace,
		DatabasePath: dbPath,
		Port:         runPort,
		BindAddress:  runBindAddress,
		IdleTimeout:  runIdleTimeout,
		Logger:       logger,
	}

	srv, err := glixServer.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	// Create context that cancels on interrupt
	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		logger.Info("received shutdown signal", "signal", sig.String())
		cancel()
	}()

	logger.Info("starting glix gRPC server",
		"address", srv.Address(),
		"namespace", cfg.Namespace,
		"database", cfg.DatabasePath,
	)

	if err := srv.Start(ctx); err != nil && ctx.Err() == nil {
		return fmt.Errorf("server error: %w", err)
	}

	logger.Info("server shutdown complete")

	return nil
}
