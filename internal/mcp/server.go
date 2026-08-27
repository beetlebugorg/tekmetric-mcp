// Package mcp provides the Model Context Protocol server implementation for Tekmetric.
// It handles MCP server initialization, authentication, and tool registration.
package mcp

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/beetlebugorg/tekmetric-mcp/internal/config"
	"github.com/beetlebugorg/tekmetric-mcp/internal/mcp/analysis"
	"github.com/beetlebugorg/tekmetric-mcp/internal/mcp/tools"
	"github.com/beetlebugorg/tekmetric-mcp/pkg/tekmetric"
	"github.com/mark3labs/mcp-go/server"
)

// shutdownTimeout bounds how long the HTTP listener waits for open requests.
const shutdownTimeout = 15 * time.Second

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

// Start serves MCP requests until the context is cancelled.
//
// The stdio transport serves one client over standard input and output. It
// authenticates first, so a bad credential fails at startup rather than on the
// first tool call.
//
// The http transport serves many clients over streamable HTTP with no session
// state. Each request carries everything the server needs, so any replica can
// answer any request. It does not authenticate at startup, because a replica
// may sit idle and a token would expire before the first call.
//
// Parameters:
//   - ctx: Context for server lifecycle management
//
// Returns:
//   - error: Any error during server operation
func (s *Server) Start(ctx context.Context) error {
	switch s.config.Server.Transport {
	case config.TransportHTTP:
		return s.serveHTTP(ctx)
	default:
		return s.serveStdio(ctx)
	}
}

// serveStdio serves one client over standard input and output.
func (s *Server) serveStdio(ctx context.Context) error {
	// Authenticate before serving, so a bad credential fails at startup rather
	// than on the first tool call.
	if err := s.client.Authenticate(ctx); err != nil {
		return err
	}

	s.logger.Info("MCP server starting",
		"name", s.config.Server.Name,
		"version", s.config.Server.Version,
		"transport", config.TransportStdio)

	// Listen takes the application context, so a shutdown signal reaches the
	// running handlers. ServeStdio builds its own context and its own signal
	// handler, which would ignore the one this server was given.
	err := server.NewStdioServer(s.server).Listen(ctx, os.Stdin, os.Stdout)
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

// serveHTTP serves many clients over streamable HTTP without session state.
//
// The client does not authenticate here. The first tool call obtains a token,
// and the client caches it for later calls.
func (s *Server) serveHTTP(ctx context.Context) error {
	httpServer := server.NewStreamableHTTPServer(s.server,
		// Stateless mode issues no session ID and keeps no per-client state,
		// so a load balancer may send each request to any replica.
		server.WithStateLess(true),
	)

	s.logger.Info("MCP server starting",
		"name", s.config.Server.Name,
		"version", s.config.Server.Version,
		"transport", config.TransportHTTP,
		"addr", s.config.Server.Addr)

	// Stop the listener when the application context ends.
	errs := make(chan error, 1)
	go func() {
		errs <- httpServer.Start(s.config.Server.Addr)
	}()

	select {
	case err := <-errs:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		s.logger.Info("shutting down the HTTP listener")

		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
		defer cancel()

		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			return err
		}
		return nil
	}
}
