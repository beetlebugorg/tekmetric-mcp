package config

import (
	"strings"
	"testing"
)

// Tests use the reserved example.com domain. A test must never name a host
// that resolves to a real service.
//
// validConfig returns a Config that passes Validate. Each test changes one
// field so a failure names the field under test.
func validConfig() *Config {
	return &Config{
		Tekmetric: TekmetricConfig{
			BaseURL:           "https://api.example.com",
			ClientID:          "id",
			ClientSecret:      "secret",
			TimeoutSeconds:    30,
			MaxRetries:        3,
			MaxBackoffSec:     60,
			RequestsPerSecond: 10,
		},
		Server: ServerConfig{
			Name: "tekmetric-mcp", Version: "0.1.0",
			Transport: TransportStdio, Addr: ":8080",
		},
		Analysis: AnalysisConfig{MaxPages: 50, MaxRecords: 5000, TimeoutSeconds: 120},
	}
}

func TestValidateAcceptsAValidConfig(t *testing.T) {
	if err := validConfig().Validate(); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}
}

func TestValidateRequiredFields(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Config)
		wantMsg string
	}{
		{
			name:    "missing client id",
			mutate:  func(c *Config) { c.Tekmetric.ClientID = "" },
			wantMsg: "client_id",
		},
		{
			name:    "missing client secret",
			mutate:  func(c *Config) { c.Tekmetric.ClientSecret = "" },
			wantMsg: "client_secret",
		},
		{
			name:    "missing base url",
			mutate:  func(c *Config) { c.Tekmetric.BaseURL = "" },
			wantMsg: "base_url",
		},
		{
			name:    "zero timeout",
			mutate:  func(c *Config) { c.Tekmetric.TimeoutSeconds = 0 },
			wantMsg: "timeout_seconds",
		},
		{
			name:    "negative timeout",
			mutate:  func(c *Config) { c.Tekmetric.TimeoutSeconds = -1 },
			wantMsg: "timeout_seconds",
		},
		{
			name:    "negative max retries",
			mutate:  func(c *Config) { c.Tekmetric.MaxRetries = -1 },
			wantMsg: "max_retries",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			tt.mutate(cfg)

			err := cfg.Validate()
			if err == nil {
				t.Fatal("Validate() returned nil, want an error")
			}
			if !strings.Contains(err.Error(), tt.wantMsg) {
				t.Errorf("Validate() error = %q, want it to mention %q", err, tt.wantMsg)
			}
		})
	}
}

func TestValidateBaseURL(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		wantErr bool
	}{
		{"https production", "https://api.example.com", false},
		{"https sandbox", "https://sandbox.example.com", false},
		{"http sandbox is allowed", "http://sandbox.example.com", false},
		{"http localhost is allowed", "http://localhost:8080", false},
		{"http loopback address is allowed", "http://127.0.0.1:8080", false},
		{"http production is rejected", "http://api.example.com", true},
		{"no scheme is rejected", "api.example.com", true},
		{"scheme without host is rejected", "https://", true},
		{"malformed url is rejected", "://api.example.com", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.Tekmetric.BaseURL = tt.baseURL

			err := cfg.Validate()
			if tt.wantErr && err == nil {
				t.Errorf("Validate() with %q returned nil, want an error", tt.baseURL)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Validate() with %q error = %v, want nil", tt.baseURL, err)
			}
		})
	}
}

// TestValidateSandboxMatchIsASubstring records that the sandbox exemption
// matches anywhere in the host, so a hostile host name reaches the exemption.
func TestValidateSandboxMatchIsASubstring(t *testing.T) {
	cfg := validConfig()
	cfg.Tekmetric.BaseURL = "http://sandbox.not-the-vendor.example"

	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() error = %v; the host check is now stricter, so update this test", err)
	}
}

func TestValidateAllowsZeroMaxRetries(t *testing.T) {
	cfg := validConfig()
	cfg.Tekmetric.MaxRetries = 0

	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}
}

func TestValidateAnalysisLimits(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Config)
		wantMsg string
	}{
		{"zero max pages", func(c *Config) { c.Analysis.MaxPages = 0 }, "analysis.max_pages"},
		{"negative max pages", func(c *Config) { c.Analysis.MaxPages = -1 }, "analysis.max_pages"},
		{"zero max records", func(c *Config) { c.Analysis.MaxRecords = 0 }, "analysis.max_records"},
		{"zero timeout", func(c *Config) { c.Analysis.TimeoutSeconds = 0 }, "analysis.timeout_seconds"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			tt.mutate(cfg)

			err := cfg.Validate()
			if err == nil {
				t.Fatal("Validate() returned nil, want an error")
			}
			if !strings.Contains(err.Error(), tt.wantMsg) {
				t.Errorf("Validate() error = %q, want it to mention %q", err, tt.wantMsg)
			}
		})
	}
}

func TestValidateTransport(t *testing.T) {
	tests := []struct {
		name      string
		transport string
		addr      string
		wantErr   bool
	}{
		{"stdio", TransportStdio, "", false},
		{"stdio ignores the address", TransportStdio, ":8080", false},
		{"http with an address", TransportHTTP, ":8080", false},
		{"http with a host and port", TransportHTTP, "127.0.0.1:9000", false},
		{"http with no address", TransportHTTP, "", true},
		{"an unknown transport", "grpc", ":8080", true},
		{"an empty transport", "", ":8080", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.Server.Transport = tt.transport
			cfg.Server.Addr = tt.addr

			err := cfg.Validate()
			if tt.wantErr && err == nil {
				t.Errorf("Validate() returned nil, want an error")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Validate() error = %v, want nil", err)
			}
		})
	}
}

func TestValidateRequestsPerSecond(t *testing.T) {
	for _, rate := range []int{0, -1} {
		cfg := validConfig()
		cfg.Tekmetric.RequestsPerSecond = rate

		err := cfg.Validate()
		if err == nil {
			t.Fatalf("Validate() with %d returned nil, want an error", rate)
		}
		if !strings.Contains(err.Error(), "requests_per_second") {
			t.Errorf("Validate() error = %q, want it to mention requests_per_second", err)
		}
	}
}
