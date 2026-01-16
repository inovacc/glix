package client

import (
	"context"
	"fmt"
	"time"

	"github.com/inovacc/glix/internal/module"
	pb "github.com/inovacc/glix/pkg/api/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Client wraps the gRPC client for the Glix service
type Client struct {
	conn   *grpc.ClientConn
	client pb.GlixServiceClient
}

// Config holds client configuration
type Config struct {
	Address     string
	DialTimeout time.Duration
}

// DefaultConfig returns the default client configuration
func DefaultConfig() Config {
	return Config{
		Address:     "localhost:9742",
		DialTimeout: 5 * time.Second,
	}
}

// New creates a new gRPC client
func New(cfg Config) (*Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.DialTimeout)
	defer cancel()

	conn, err := grpc.DialContext(ctx, cfg.Address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server at %s: %w", cfg.Address, err)
	}

	return &Client{
		conn:   conn,
		client: pb.NewGlixServiceClient(conn),
	}, nil
}

// Close closes the client connection
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}

	return nil
}

// Ping checks if the server is responsive
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.client.Ping(ctx, &emptypb.Empty{})
	return err
}

// GetStatus returns the server status
func (c *Client) GetStatus(ctx context.Context) (*pb.ServerStatus, error) {
	return c.client.GetStatus(ctx, &emptypb.Empty{})
}

// StoreModule stores module info in the database after local installation
func (c *Client) StoreModule(ctx context.Context, m *module.Module) error {
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

	resp, err := c.client.StoreModule(ctx, &pb.StoreModuleRequest{
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

// Remove removes an installed module
func (c *Client) Remove(ctx context.Context, modulePath, version string) (*pb.RemoveResponse, error) {
	return c.client.Remove(ctx, &pb.RemoveRequest{
		ModulePath: modulePath,
		Version:    version,
	})
}

// ListModules returns all installed modules
func (c *Client) ListModules(ctx context.Context, limit, offset int32, nameFilter string) (*pb.ListModulesResponse, error) {
	return c.client.ListModules(ctx, &pb.ListModulesRequest{
		Limit:      limit,
		Offset:     offset,
		NameFilter: nameFilter,
	})
}

// GetModule retrieves a specific module
func (c *Client) GetModule(ctx context.Context, name, version string) (*pb.GetModuleResponse, error) {
	return c.client.GetModule(ctx, &pb.GetModuleRequest{
		Name:    name,
		Version: version,
	})
}

// GetDependencies retrieves dependencies for a module
func (c *Client) GetDependencies(ctx context.Context, name, version string) (*pb.GetDependenciesResponse, error) {
	return c.client.GetDependencies(ctx, &pb.GetModuleRequest{
		Name:    name,
		Version: version,
	})
}
