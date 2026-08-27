// Package mcp provides the Model Context Protocol server implementation for Tekmetric.
// It handles MCP server initialization, authentication, and tool registration.
package mcp

import (
	"context"
	"errors"
	"log/slog"
	"os"

	"github.com/beetlebugorg/tekmetric-mcp/internal/config"
	"github.com/beetlebugorg/tekmetric-mcp/internal/mcp/analysis"
	"github.com/beetlebugorg/tekmetric-mcp/internal/mcp/tools"
	"github.com/beetlebugorg/tekmetric-mcp/pkg/tekmetric"
	"github.com/mark3labs/mcp-go/server"
)

// Server represents the MCP server for Tekmetric.
// It wraps an MCP server instance and provides integration with the Tekmetric API.
type Server struct {
	server *server.MCPServer // The underlying MCP server
	client *tekmetric.Client // Authenticated Tekmetric API client
	config *config.Config    // Server configuration
	logger *slog.Logger      // Structured logger
}

// NewServer creates a new MCP server instance.
// It initializes the Tekmetric API client, creates the MCP server,
// and registers all available tools.
//
// The server is configured to communicate via stdio (standard input/output)
// which is the standard communication method for MCP servers.
//
// Parameters:
//   - cfg: Server configuration including Tekmetric API credentials
//   - logger: Structured logger for server operations
//
// Returns:
//   - *Server: Configured MCP server ready to start
//   - error: Any error during initialization
func NewServer(cfg *config.Config, logger *slog.Logger) (*Server, error) {
	// Create Tekmetric API client with OAuth2 authentication
	tekmetricClient := tekmetric.NewClient(&cfg.Tekmetric, logger)

	// Create MCP server instance
	// Tools are automatically enabled when registered via AddTool
	mcpServer := server.NewMCPServer(
		cfg.Server.Name,
		cfg.Server.Version,
		server.WithLogging(),
		// A panic in one tool returns an error for that call instead of ending
		// the process and dropping the client connection.
		server.WithRecovery(),
	)

	s := &Server{
		server: mcpServer,
		client: tekmetricClient,
		config: cfg,
		logger: logger,
	}

	// Register all Tekmetric tools (shops, customers, vehicles, etc.)
	toolRegistry := tools.NewRegistry(tekmetricClient, cfg, logger)
	toolRegistry.RegisterAll(mcpServer)

	// Register analysis tools
	analysisRegistry := analysis.NewRegistry(tekmetricClient, cfg, logger)
	analysisRegistry.Register(analysis.NewVehicleServiceAnalysis(tekmetricClient, cfg, logger))
	analysisRegistry.RegisterAll(mcpServer)

	return s, nil
}

// Start starts the MCP server and begins listening for requests.
//
// It serves MCP requests over stdio until the context is cancelled or the
// input closes. The context reaches the tool handlers, so cancelling it stops
// work that is already running.
//
// Parameters:
//   - ctx: Context for server lifecycle management
//
// Returns:
//   - error: Any error during server operation
func (s *Server) Start(ctx context.Context) error {
	// Authenticate before serving, so a bad credential fails at startup rather
	// than on the first tool call.
	if err := s.client.Authenticate(ctx); err != nil {
		return err
	}

	s.logger.Info("MCP server starting",
		"name", s.config.Server.Name,
		"version", s.config.Server.Version)

	// Listen takes the application context, so a shutdown signal reaches the
	// running handlers. ServeStdio installs its own signal handler and its own
	// context, which would ignore the one this server was given.
	err := server.NewStdioServer(s.server).Listen(ctx, os.Stdin, os.Stdout)
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}
