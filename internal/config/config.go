// Package config provides configuration management for the Tekmetric MCP server.
// It supports loading configuration from multiple sources: environment variables,
// JSON config files, and default values using the Viper library.
package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// Config holds all configuration for the Tekmetric MCP server.
// Configuration can be loaded from:
//   - Environment variables (prefixed with TEKMETRIC_)
//   - JSON config file (~/.config/tekmetric-mcp/config.json or ./config.json)
//   - Default values
type Config struct {
	Tekmetric TekmetricConfig `mapstructure:"tekmetric"` // Tekmetric API settings
	Server    ServerConfig    `mapstructure:"server"`    // MCP server settings
	Analysis  AnalysisConfig  `mapstructure:"analysis"`  // Analysis tool settings
}

// TekmetricConfig holds Tekmetric API configuration.
// Required fields are ClientID and ClientSecret which must be obtained
// from the Tekmetric dashboard at Settings → API Access.
type TekmetricConfig struct {
	BaseURL        string `mapstructure:"base_url"`        // API base URL (sandbox or production)
	ClientID       string `mapstructure:"client_id"`       // OAuth2 client ID (required)
	ClientSecret   string `mapstructure:"client_secret"`   // OAuth2 client secret (required)
	DefaultShopID  int    `mapstructure:"default_shop_id"` // Default shop ID for API calls
	TimeoutSeconds int    `mapstructure:"timeout_seconds"` // HTTP client timeout in seconds
	MaxRetries     int    `mapstructure:"max_retries"`     // Maximum retry attempts for failed requests
	MaxBackoffSec  int    `mapstructure:"max_backoff_sec"` // Maximum backoff time in seconds
}

// ServerConfig holds MCP server configuration.
type ServerConfig struct {
	Name    string `mapstructure:"name"`    // Server name
	Version string `mapstructure:"version"` // Server version
	Debug   bool   `mapstructure:"debug"`   // Enable debug logging
}

// AnalysisConfig holds configuration for analysis tools.
// These settings control safety limits and behavior for data analysis tools.
type AnalysisConfig struct {
	MaxPages       int `mapstructure:"max_pages"`       // Maximum pages to fetch per analysis (safety limit)
	MaxRecords     int `mapstructure:"max_records"`     // Maximum records to process (memory safety)
	TimeoutSeconds int `mapstructure:"timeout_seconds"` // Analysis timeout in seconds
}

// Load loads configuration from multiple sources in order of precedence:
// 1. Environment variables (highest priority)
// 2. JSON config file
// 3. Default values (lowest priority)
//
// Config file locations checked in order:
//   - ~/.config/tekmetric-mcp/config.json
//   - ./config.json
//
// Returns:
//   - *Config: Loaded and validated configuration
//   - error: Any error during loading or validation
func Load() (*Config, error) {
	v := viper.New()

	// Set default values for all configuration options
	v.SetDefault("tekmetric.base_url", "https://sandbox.tekmetric.com")
	v.SetDefault("tekmetric.timeout_seconds", 30)
	v.SetDefault("tekmetric.max_retries", 3)
	v.SetDefault("tekmetric.max_backoff_sec", 60)
	v.SetDefault("tekmetric.default_shop_id", 0)
	v.SetDefault("server.name", "tekmetric-mcp")
	v.SetDefault("server.version", "0.1.0")
	v.SetDefault("server.debug", false)
	v.SetDefault("analysis.max_pages", 50)
	v.SetDefault("analysis.max_records", 5000)
	v.SetDefault("analysis.timeout_seconds", 120)

	// Enable environment variable support
	// Environment variables should be prefixed with TEKMETRIC_
	v.SetEnvPrefix("TEKMETRIC")
	v.AutomaticEnv()

	// Bind specific environment variables to config keys
	// This allows both TEKMETRIC_CLIENT_ID and tekmetric.client_id formats
	v.BindEnv("tekmetric.client_id", "TEKMETRIC_CLIENT_ID")
	v.BindEnv("tekmetric.client_secret", "TEKMETRIC_CLIENT_SECRET")
	v.BindEnv("tekmetric.base_url", "TEKMETRIC_BASE_URL")
	v.BindEnv("tekmetric.default_shop_id", "TEKMETRIC_DEFAULT_SHOP_ID")
	v.BindEnv("server.debug", "TEKMETRIC_DEBUG")

	// Configure config file search
	v.SetConfigName("config")
	v.SetConfigType("json")

	// Add config file search paths
	homeDir, err := os.UserHomeDir()
	if err == nil {
		v.AddConfigPath(filepath.Join(homeDir, ".config", "tekmetric-mcp"))
	}
	v.AddConfigPath(".") // Current directory

	// Attempt to read config file (optional)
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
		// Config file not found is acceptable - we'll use env vars and defaults
	}

	// Unmarshal configuration into struct
	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("unable to decode config: %w", err)
	}

	// Validate required fields before returning
	if config.Tekmetric.ClientID == "" {
		return nil, fmt.Errorf("TEKMETRIC_CLIENT_ID is required")
	}
	if config.Tekmetric.ClientSecret == "" {
		return nil, fmt.Errorf("TEKMETRIC_CLIENT_SECRET is required")
	}

	return &config, nil
}

// Validate validates the configuration for consistency and required values.
// It checks that all required fields are present and values are within acceptable ranges.
//
// Returns:
//   - error: Description of validation failure, or nil if valid
func (c *Config) Validate() error {
	if c.Tekmetric.ClientID == "" {
		return fmt.Errorf("tekmetric.client_id is required")
	}
	if c.Tekmetric.ClientSecret == "" {
		return fmt.Errorf("tekmetric.client_secret is required")
	}
	if c.Tekmetric.BaseURL == "" {
		return fmt.Errorf("tekmetric.base_url is required")
	}

	// Validate base URL format and security
	u, err := url.Parse(c.Tekmetric.BaseURL)
	if err != nil {
		return fmt.Errorf("tekmetric.base_url must be a valid URL: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("tekmetric.base_url must include scheme (https://) and host")
	}

	// Enforce HTTPS except for sandbox/localhost
	if u.Scheme != "https" {
		isSandbox := strings.Contains(strings.ToLower(u.Host), "sandbox")
		isLocalhost := strings.Contains(strings.ToLower(u.Host), "localhost") || strings.HasPrefix(u.Host, "127.0.0.1")

		if !isSandbox && !isLocalhost {
			return fmt.Errorf("tekmetric.base_url must use HTTPS for production environments")
		}
	}

	if c.Tekmetric.TimeoutSeconds <= 0 {
		return fmt.Errorf("tekmetric.timeout_seconds must be positive")
	}
	if c.Tekmetric.MaxRetries < 0 {
		return fmt.Errorf("tekmetric.max_retries must be non-negative")
	}

	// The analysis limits bound how much data one tool call fetches.
	if c.Analysis.MaxPages < 1 {
		return fmt.Errorf("analysis.max_pages must be at least 1")
	}
	if c.Analysis.MaxRecords < 1 {
		return fmt.Errorf("analysis.max_records must be at least 1")
	}
	if c.Analysis.TimeoutSeconds < 1 {
		return fmt.Errorf("analysis.timeout_seconds must be at least 1")
	}
	return nil
}
