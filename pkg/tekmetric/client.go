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
	"sync"
	"time"

	"github.com/beetlebugorg/tekmetric-mcp/internal/config"
	"github.com/beetlebugorg/tekmetric-mcp/pkg/retry"
	"golang.org/x/time/rate"
)

// userAgent identifies this client to the API.
const userAgent = "tekmetric-mcp (https://github.com/beetlebugorg/tekmetric-mcp)"

// defaultRequestsPerSecond limits calls to the API when the config sets none.
const defaultRequestsPerSecond = 10

// maxResponseBytes caps a response body, so a large reply cannot exhaust memory.
const maxResponseBytes = 10 * 1024 * 1024

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
	retryer       *retry.Retryer // Retry logic with exponential backoff
	globalLimiter *rate.Limiter  // Global rate limiter (requests per second)
	logger        *slog.Logger   // Structured logger

	// fetchMu serializes the token fetch, so concurrent callers that find an
	// expired token produce one request rather than one request each.
	fetchMu sync.Mutex

	// mu guards the three fields below. Every request reads them, and a token
	// refresh writes them, so an HTTP server calling this client concurrently
	// would race without the lock.
	mu          sync.RWMutex
	accessToken string    // Current OAuth2 access token
	tokenExpiry time.Time // Token expiration time
	shopIDs     []string  // Shop IDs from token scope
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
	requestsPerSecond := cfg.RequestsPerSecond
	if requestsPerSecond < 1 {
		requestsPerSecond = defaultRequestsPerSecond
	}

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
		retryer: retry.New(cfg.MaxRetries, cfg.MaxBackoffSec),
		// The limit applies to this process. Running several replicas divides
		// the account budget, so lower it to match the replica count.
		globalLimiter: rate.NewLimiter(rate.Limit(requestsPerSecond), requestsPerSecond),
		logger:        logger,
	}
}

// Authenticate obtains an access token from the Tekmetric API.
// It always contacts the token endpoint. Call ensureAuthenticated instead to
// reuse a token that is still valid.
func (c *Client) Authenticate(ctx context.Context) error {
	c.fetchMu.Lock()
	defer c.fetchMu.Unlock()

	return c.fetchToken(ctx)
}

// ensureAuthenticated obtains a token when the cached one is missing or expired.
//
// Concurrent callers that find an expired token queue on fetchMu. The first one
// refreshes the token and the rest see the fresh token and return, so a burst of
// requests produces one token fetch.
func (c *Client) ensureAuthenticated(ctx context.Context) error {
	if c.tokenIsValid() {
		return nil
	}

	c.fetchMu.Lock()
	defer c.fetchMu.Unlock()

	// Another caller may have refreshed the token while this one waited.
	if c.tokenIsValid() {
		return nil
	}

	return c.fetchToken(ctx)
}

// tokenIsValid reports whether the cached token exists and has not expired.
func (c *Client) tokenIsValid() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.accessToken != "" && time.Now().Before(c.tokenExpiry)
}

// token returns the cached access token.
func (c *Client) token() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.accessToken
}

// fetchToken requests a token and caches it. The caller must hold fetchMu.
func (c *Client) fetchToken(ctx context.Context) error {
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
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send auth request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
		c.logger.Debug("authentication failed", "status", resp.StatusCode, "body", string(body))
		return fmt.Errorf("authentication failed with status %d", resp.StatusCode)
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return fmt.Errorf("failed to decode token response: %w", err)
	}

	expiry := time.Now().Add(24 * time.Hour) // Fallback when the API omits the lifetime
	if tokenResp.ExpiresIn > 0 {
		expiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	}

	shopIDs := strings.Fields(tokenResp.Scope) // Space-separated shop IDs

	c.mu.Lock()
	c.accessToken = tokenResp.AccessToken
	c.shopIDs = shopIDs
	c.tokenExpiry = expiry
	c.mu.Unlock()

	c.logger.Info("authentication successful",
		"shop_count", len(shopIDs),
		"expires_in", tokenResp.ExpiresIn)

	return nil
}

// authorizeShop confirms the token covers a shop.
//
// It authenticates first, because the shop list comes from the token scope. A
// client that has not authenticated has no scope, so checking first would reject
// every shop.
func (c *Client) authorizeShop(ctx context.Context, shopID int) error {
	if shopID == 0 {
		return nil
	}

	if err := c.ensureAuthenticated(ctx); err != nil {
		return err
	}
	return c.isAuthorizedShop(shopID)
}

// isAuthorizedShop checks if the client is authorized to access the specified shop.
// Authorization is determined by the shop IDs in the OAuth token scope.
func (c *Client) isAuthorizedShop(shopID int) error {
	// Skip validation if shopID is 0 (not specified)
	if shopID == 0 {
		return nil
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	shopIDStr := fmt.Sprintf("%d", shopID)
	for _, authorizedID := range c.shopIDs {
		if authorizedID == shopIDStr {
			return nil
		}
	}
	return fmt.Errorf("unauthorized access to shop %d: not in token scope", shopID)
}

// doRequest performs an HTTP request with authentication and rate limiting.
//
// It refreshes the token once when the API rejects it, because a token can
// expire between the check and the request.
func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	if err := c.ensureAuthenticated(ctx); err != nil {
		return err
	}

	// Wait for global rate limiter before making request
	if err := c.globalLimiter.Wait(ctx); err != nil {
		return fmt.Errorf("rate limiter wait failed: %w", err)
	}

	refreshed := false

	return c.retryer.Do(ctx, func() error {
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

		req.Header.Set("Authorization", "Bearer "+c.token())
		req.Header.Set("User-Agent", userAgent)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		fullURL := c.baseURL + path
		c.logger.Debug("API request", "method", method, "url", fullURL, "has_body", body != nil)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			// A cancelled context is final. Any other transport fault may clear.
			if ctxErr := ctx.Err(); ctxErr != nil {
				return fmt.Errorf("request failed: %w", ctxErr)
			}
			return newTransportError(err)
		}
		defer resp.Body.Close()

		limitedBody := io.LimitReader(resp.Body, maxResponseBytes)
		responseBody, err := io.ReadAll(limitedBody)
		if err != nil {
			return fmt.Errorf("failed to read response body: %w", err)
		}

		if int64(len(responseBody)) == int64(maxResponseBytes) {
			c.logger.Warn("response body may have been truncated", "path", path, "max_size", maxResponseBytes)
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			c.logger.Error("API request failed",
				"method", method,
				"url", fullURL,
				"status", resp.StatusCode,
				"response_body", string(responseBody))

			// The token may have expired between the check and the request.
			// Refresh once, then let the retryer make another attempt.
			if resp.StatusCode == http.StatusUnauthorized && !refreshed {
				refreshed = true
				if authErr := c.Authenticate(ctx); authErr != nil {
					return fmt.Errorf("re-authentication failed after status 401: %w", authErr)
				}
				return newTemporaryStatusError(resp.StatusCode)
			}

			// Rate limit (429) and server errors (5xx) may clear on a retry.
			if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
				return newTemporaryStatusError(resp.StatusCode)
			}
			return fmt.Errorf("API request failed with status %d", resp.StatusCode)
		}

		c.logger.Debug("API response", "status", resp.StatusCode, "content_length", len(responseBody))

		if result != nil {
			if err := json.Unmarshal(responseBody, result); err != nil {
				return fmt.Errorf("failed to decode response: %w", err)
			}
		}

		return nil
	})
}
