package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/alecthomas/kong"
	"github.com/beetlebugorg/tekmetric-mcp/internal/config"
	"github.com/beetlebugorg/tekmetric-mcp/internal/mcp"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

type CLI struct {
	// Global flags
	Debug   bool       `help:"Enable debug logging" short:"d" env:"TEKMETRIC_DEBUG"`
	Version VersionCmd `cmd:"" help:"Show version information"`

	// Commands
	Serve ServeCmd `cmd:"" help:"Start the MCP server" default:"withargs"`
}

type ServeCmd struct {
	ClientID     string `help:"Tekmetric client ID" env:"TEKMETRIC_CLIENT_ID"`
	ClientSecret string `help:"Tekmetric client secret" env:"TEKMETRIC_CLIENT_SECRET"`
	BaseURL      string `help:"Tekmetric API base URL" env:"TEKMETRIC_BASE_URL" default:"https://sandbox.tekmetric.com"`
	ShopID       int    `help:"Default shop ID" env:"TEKMETRIC_DEFAULT_SHOP_ID" default:"0"`
	Transport    string `help:"Transport to serve: stdio or http" env:"TEKMETRIC_TRANSPORT" enum:"stdio,http," default:""`
	Addr         string `help:"Listen address for the http transport" env:"TEKMETRIC_ADDR" default:""`
}

type VersionCmd struct{}

func (c *ServeCmd) Run(ctx *kong.Context, globalCLI *CLI) error {
	// Set up logger
	logLevel := slog.LevelInfo
	if globalCLI.Debug {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: logLevel,
	}))

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load configuration", "error", err)
		return err
	}

	// Override with CLI flags if provided
	if c.ClientID != "" {
		cfg.Tekmetric.ClientID = c.ClientID
	}
	if c.ClientSecret != "" {
		cfg.Tekmetric.ClientSecret = c.ClientSecret
	}
	if c.BaseURL != "" {
		cfg.Tekmetric.BaseURL = c.BaseURL
	}
	if c.ShopID != 0 {
		cfg.Tekmetric.DefaultShopID = c.ShopID
	}
	if c.Transport != "" {
		cfg.Server.Transport = c.Transport
	}
	if c.Addr != "" {
		cfg.Server.Addr = c.Addr
	}

	cfg.Server.Debug = globalCLI.Debug

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		logger.Error("invalid configuration", "error", err)
		return err
	}

	// Create context with cancellation for graceful shutdown
	appCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals for graceful shutdown. Cancelling appCtx stops the
	// transport and every handler running under it.
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	go func() {
		sig := <-sigChan
		logger.Info("received shutdown signal", "signal", sig)
		cancel()
	}()

	// Create and start MCP server
	server, err := mcp.NewServer(cfg, logger)
	if err != nil {
		logger.Error("failed to create MCP server", "error", err)
		return err
	}

	logger.Info("starting Tekmetric MCP server",
		"version", version,
		"commit", commit,
		"base_url", cfg.Tekmetric.BaseURL,
		"default_shop_id", cfg.Tekmetric.DefaultShopID,
		"transport", cfg.Server.Transport,
	)

	// Start server (blocking)
	if err := server.Start(appCtx); err != nil {
		logger.Error("server error", "error", err)
		return err
	}

	logger.Info("server stopped gracefully")
	return nil
}

func (c *VersionCmd) Run() error {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	logger.Info("tekmetric-mcp",
		"version", version,
		"commit", commit,
		"built", date,
	)
	return nil
}

func main() {
	cli := &CLI{}
	ctx := kong.Parse(cli,
		kong.Name("tekmetric-mcp"),
		kong.Description("MCP server for Tekmetric API integration"),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{
			Compact: true,
			Summary: true,
		}),
		kong.Vars{
			"version": version,
		},
	)

	err := ctx.Run(cli)
	ctx.FatalIfErrorf(err)
}
