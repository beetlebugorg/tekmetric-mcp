// Package tekmetric provides a client for the Tekmetric shop management API.
// It handles OAuth2 authentication, rate limiting, and provides methods for
// accessing shops, customers, vehicles, repair orders, and other resources.
package tekmetric

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/beetlebugorg/tekmetric-mcp/internal/config"
	"github.com/beetlebugorg/tekmetric-mcp/pkg/retry"
	"golang.org/x/time/rate"
)

// Client is a Tekmetric API client that handles authentication and API requests.
// It manages OAuth2 tokens, implements rate limiting, and provides a clean
// interface to the Tekmetric REST API.
//
// The client automatically:
//   - Obtains and refreshes OAuth2 access tokens
//   - Retries failed requests with exponential backoff
//   - Adds proper authentication headers
//   - Handles JSON encoding/decoding
type Client struct {
	baseURL       string         // API base URL (sandbox or production)
	clientID      string         // OAuth2 client ID
	clientSecret  string         // OAuth2 client secret
	httpClient    *http.Client   // HTTP client with timeout
	accessToken   string         // Current OAuth2 access token
	tokenExpiry   time.Time      // Token expiration time
	shopIDs       []string       // Shop IDs from token scope
	retryer       *retry.Retryer // Retry logic with exponential backoff
	globalLimiter *rate.Limiter  // Global rate limiter (requests per second)
	logger        *slog.Logger   // Structured logger
}

// NewClient creates a new Tekmetric API client.
// The client is ready to use but not yet authenticated.
// Call Authenticate() before making API requests.
//
// Parameters:
//   - cfg: Tekmetric API configuration (credentials, base URL, timeouts)
//   - logger: Structured logger for client operations
//
// Returns:
//   - *Client: Configured API client ready for authentication
func NewClient(cfg *config.TekmetricConfig, logger *slog.Logger) *Client {
	// Create HTTP transport with secure TLS configuration
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12, // Enforce TLS 1.2 minimum
			MaxVersion: 0,                // Allow highest available version
		},
	}

	return &Client{
		baseURL:      cfg.BaseURL,
		clientID:     cfg.ClientID,
		clientSecret: cfg.ClientSecret,
		httpClient: &http.Client{
			Timeout:   time.Duration(cfg.TimeoutSeconds) * time.Second,
			Transport: transport,
		},
		retryer:       retry.New(cfg.MaxRetries, cfg.MaxBackoffSec),
		globalLimiter: rate.NewLimiter(rate.Limit(10), 10), // 10 requests/sec with burst of 10
		logger:        logger,
	}
}

// Authenticate obtains an access token from the Tekmetric API
func (c *Client) Authenticate(ctx context.Context) error {
	c.logger.Info("authenticating with Tekmetric API")

	// Create Basic Auth header
	auth := base64.StdEncoding.EncodeToString([]byte(c.clientID + ":" + c.clientSecret))

	// Prepare request body
	data := url.Values{}
	data.Set("grant_type", "client_credentials")

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/v1/oauth/token", strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create auth request: %w", err)
	}

	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=UTF-8")
	req.Header.Set("User-Agent", "tekmetric-mcp (https://github.com/beetlebugorg/tekmetric-mcp)")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send auth request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		c.logger.Debug("authentication failed", "status", resp.StatusCode, "body", string(body))
		return fmt.Errorf("authentication failed with status %d", resp.StatusCode)
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return fmt.Errorf("failed to decode token response: %w", err)
	}

	c.accessToken = tokenResp.AccessToken
	c.shopIDs = strings.Fields(tokenResp.Scope) // Space-separated shop IDs

	// Use expires_in from response if provided, otherwise assume 24h
	if tokenResp.ExpiresIn > 0 {
		c.tokenExpiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
		c.logger.Info("authentication successful", "shop_count", len(c.shopIDs), "expires_in", tokenResp.ExpiresIn)
	} else {
		c.tokenExpiry = time.Now().Add(24 * time.Hour) // Fallback to 24h
		c.logger.Info("authentication successful", "shop_count", len(c.shopIDs), "expires_in", "24h (default)")
	}

	return nil
}

// ensureAuthenticated checks if we have a valid token and authenticates if needed
func (c *Client) ensureAuthenticated(ctx context.Context) error {
	if c.accessToken == "" || time.Now().After(c.tokenExpiry) {
		return c.Authenticate(ctx)
	}
	return nil
}

// isAuthorizedShop checks if the client is authorized to access the specified shop.
// Authorization is determined by the shop IDs in the OAuth token scope.
func (c *Client) isAuthorizedShop(shopID int) error {
	// Skip validation if shopID is 0 (not specified)
	if shopID == 0 {
		return nil
	}

	shopIDStr := fmt.Sprintf("%d", shopID)
	for _, authorizedID := range c.shopIDs {
		if authorizedID == shopIDStr {
			return nil
		}
	}
	return fmt.Errorf("unauthorized access to shop %d: not in token scope", shopID)
}

// doRequest performs an HTTP request with authentication and rate limiting
func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	if err := c.ensureAuthenticated(ctx); err != nil {
		return err
	}

	// Wait for global rate limiter before making request
	if err := c.globalLimiter.Wait(ctx); err != nil {
		return fmt.Errorf("rate limiter wait failed: %w", err)
	}

	return c.retryer.Do(func() error {
		var reqBody io.Reader
		if body != nil {
			jsonData, err := json.Marshal(body)
			if err != nil {
				return fmt.Errorf("failed to marshal request body: %w", err)
			}
			reqBody = bytes.NewReader(jsonData)
		}

		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Authorization", "Bearer "+c.accessToken)
		req.Header.Set("User-Agent", "tekmetric-mcp (https://github.com/beetlebugorg/tekmetric-mcp)")
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		// Log request details for debugging
		fullURL := c.baseURL + path
		c.logger.Debug("API request",
			"method", method,
			"url", fullURL,
			"has_body", body != nil)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("request failed: %w", err)
		}
		defer resp.Body.Close()

		// Limit response body to prevent memory exhaustion (10MB max)
		maxBodySize := int64(10 * 1024 * 1024)
		limitedBody := io.LimitReader(resp.Body, maxBodySize)
		responseBody, err := io.ReadAll(limitedBody)
		if err != nil {
			return fmt.Errorf("failed to read response body: %w", err)
		}

		// Check if we hit the size limit
		if int64(len(responseBody)) == maxBodySize {
			c.logger.Warn("response body may have been truncated", "path", path, "max_size", maxBodySize)
		}

		// Check for non-success status codes
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			// Log detailed error information
			c.logger.Error("API request failed",
				"method", method,
				"url", fullURL,
				"status", resp.StatusCode,
				"response_body", string(responseBody))

			// Rate limit (429) and server errors (5xx) are temporary - should retry
			if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
				return &temporaryError{
					statusCode: resp.StatusCode,
					message:    fmt.Sprintf("temporary error with status %d", resp.StatusCode),
				}
			}
			// Client errors (4xx except 429) are permanent - don't retry
			return fmt.Errorf("API request failed with status %d", resp.StatusCode)
		}

		// Log successful response at debug level
		c.logger.Debug("API response",
			"status", resp.StatusCode,
			"content_length", len(responseBody))

		if result != nil {
			if err := json.Unmarshal(responseBody, result); err != nil {
				return fmt.Errorf("failed to decode response: %w", err)
			}
		}

		return nil
	})
}
