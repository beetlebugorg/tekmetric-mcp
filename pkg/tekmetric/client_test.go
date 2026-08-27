package tekmetric_test

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/beetlebugorg/tekmetric-mcp/pkg/tekmetric"
	"github.com/beetlebugorg/tekmetric-mcp/pkg/tekmetric/tekmetrictest"
)

const shopsPath = "/api/v1/shops"

// ---------------------------------------------------------------------------
// Authentication
// ---------------------------------------------------------------------------

func TestAuthenticate(t *testing.T) {
	api := tekmetrictest.New(t)
	api.Scope = []string{"1", "2", "3"}

	client := api.Client(t)
	if err := client.Authenticate(t.Context()); err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}

	if client.AccessToken() != tekmetrictest.AccessToken {
		t.Errorf("AccessToken() = %q, want %q", client.AccessToken(), tekmetrictest.AccessToken)
	}
	if got, want := strings.Join(client.ShopIDs(), ","), "1,2,3"; got != want {
		t.Errorf("ShopIDs() = %q, want %q", got, want)
	}
	if !client.TokenExpiry().After(time.Now()) {
		t.Errorf("TokenExpiry() = %v, want a time in the future", client.TokenExpiry())
	}
	if api.TokenCount() != 1 {
		t.Errorf("token requests = %d, want 1", api.TokenCount())
	}
}

func TestAuthenticateSendsClientCredentials(t *testing.T) {
	api := tekmetrictest.New(t)

	if err := api.Client(t).Authenticate(t.Context()); err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}

	req := api.Requests()[0]
	if req.Method != http.MethodPost {
		t.Errorf("method = %s, want POST", req.Method)
	}

	encoded, ok := strings.CutPrefix(req.Header.Get("Authorization"), "Basic ")
	if !ok {
		t.Fatalf("Authorization = %q, want a Basic credential", req.Header.Get("Authorization"))
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("Authorization is not valid base64: %v", err)
	}
	want := tekmetrictest.ClientID + ":" + tekmetrictest.ClientSecret
	if string(decoded) != want {
		t.Errorf("credential = %q, want %q", decoded, want)
	}

	if !strings.Contains(req.Body, "grant_type=client_credentials") {
		t.Errorf("body = %q, want it to set grant_type=client_credentials", req.Body)
	}
}

func TestAuthenticateRejectsBadCredentials(t *testing.T) {
	api := tekmetrictest.New(t)

	cfg := api.Config()
	cfg.ClientSecret = "wrong-secret"
	client := tekmetric.NewClient(cfg, tekmetrictest.Logger())

	if err := client.Authenticate(t.Context()); err == nil {
		t.Fatal("Authenticate() returned nil, want an error")
	}
	if client.AccessToken() != "" {
		t.Errorf("AccessToken() = %q, want it to stay empty", client.AccessToken())
	}
}

func TestAuthenticateFailure(t *testing.T) {
	tests := []struct {
		name   string
		status int
	}{
		{"unauthorized", http.StatusUnauthorized},
		{"forbidden", http.StatusForbidden},
		{"server error", http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := tekmetrictest.New(t)
			api.FailAlways(tekmetrictest.TokenPath, tt.status)

			client := api.Client(t)
			if err := client.Authenticate(t.Context()); err == nil {
				t.Fatal("Authenticate() returned nil, want an error")
			}
			if client.AccessToken() != "" {
				t.Errorf("AccessToken() = %q, want it to stay empty", client.AccessToken())
			}
		})
	}
}

// TestAuthenticateDoesNotLeakTheSecret confirms the error text omits the
// credential.
func TestAuthenticateDoesNotLeakTheSecret(t *testing.T) {
	api := tekmetrictest.New(t)
	api.FailAlways(tekmetrictest.TokenPath, http.StatusUnauthorized)

	err := api.Client(t).Authenticate(t.Context())
	if err == nil {
		t.Fatal("Authenticate() returned nil, want an error")
	}
	if strings.Contains(err.Error(), tekmetrictest.ClientSecret) {
		t.Errorf("error = %q, want it to omit the client secret", err)
	}
}

func TestAuthenticateMalformedTokenResponse(t *testing.T) {
	api := tekmetrictest.New(t)
	api.SetOverride(tekmetrictest.TokenPath, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `not json`)
	})

	if err := api.Client(t).Authenticate(t.Context()); err == nil {
		t.Fatal("Authenticate() returned nil, want an error")
	}
}

func TestAuthenticateDefaultsExpiryWhenAbsent(t *testing.T) {
	api := tekmetrictest.New(t)
	api.SetOverride(tekmetrictest.TokenPath, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"access_token":"t","scope":"1"}`)
	})

	client := api.Client(t)
	if err := client.Authenticate(t.Context()); err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}

	want := time.Now().Add(24 * time.Hour)
	if diff := client.TokenExpiry().Sub(want); diff < -time.Minute || diff > time.Minute {
		t.Errorf("TokenExpiry() = %v, want about %v", client.TokenExpiry(), want)
	}
}

func TestEnsureAuthenticatedReauthenticatesAfterExpiry(t *testing.T) {
	api := tekmetrictest.New(t)
	client := api.AuthedClient(t)

	client.SetTokenExpiry(time.Now().Add(-time.Minute))

	if _, err := client.GetShops(t.Context()); err != nil {
		t.Fatalf("GetShops() error = %v", err)
	}
	if api.TokenCount() != 2 {
		t.Errorf("token requests = %d, want 2", api.TokenCount())
	}
}

func TestEnsureAuthenticatedReusesAValidToken(t *testing.T) {
	api := tekmetrictest.New(t)
	client := api.Client(t)

	for i := 0; i < 3; i++ {
		if _, err := client.GetShops(t.Context()); err != nil {
			t.Fatalf("GetShops() error = %v", err)
		}
	}

	if api.TokenCount() != 1 {
		t.Errorf("token requests = %d, want 1", api.TokenCount())
	}
	if api.CallCount(shopsPath) != 3 {
		t.Errorf("shop requests = %d, want 3", api.CallCount(shopsPath))
	}
}

// ---------------------------------------------------------------------------
// Shop authorization
// ---------------------------------------------------------------------------

func TestIsAuthorizedShop(t *testing.T) {
	api := tekmetrictest.New(t)
	api.Scope = []string{"1", "2", "3"}
	client := api.AuthedClient(t)

	tests := []struct {
		name    string
		shopID  int
		wantErr bool
	}{
		{"zero means unspecified", 0, false},
		{"first shop in scope", 1, false},
		{"last shop in scope", 3, false},
		{"shop outside scope", 99, true},
		{"negative shop", -1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := client.IsAuthorizedShop(tt.shopID)
			if tt.wantErr && err == nil {
				t.Errorf("IsAuthorizedShop(%d) returned nil, want an error", tt.shopID)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("IsAuthorizedShop(%d) error = %v, want nil", tt.shopID, err)
			}
		})
	}
}

func TestUnauthorizedShopSkipsTheRequest(t *testing.T) {
	api := tekmetrictest.New(t)
	client := api.AuthedClient(t)

	_, err := client.GetCustomers(t.Context(), 99, 0, 10)
	if err == nil {
		t.Fatal("GetCustomers() returned nil, want an error")
	}
	if !strings.Contains(err.Error(), "not in token scope") {
		t.Errorf("error = %q, want it to mention the token scope", err)
	}
	if api.CallCount("/api/v1/customers") != 0 {
		t.Errorf("customer requests = %d, want 0", api.CallCount("/api/v1/customers"))
	}
}

// TestShopAuthorizationNeedsPriorAuthentication records a defect. The shop
// check reads the scope that Authenticate populates, but it runs before
// ensureAuthenticated. A client that has not authenticated rejects every shop.
//
// Update this test when the shop check runs after authentication.
func TestShopAuthorizationNeedsPriorAuthentication(t *testing.T) {
	api := tekmetrictest.New(t)
	client := api.Client(t)

	// Shop 1 is inside the token scope, but the client has not authenticated.
	_, err := client.GetCustomers(t.Context(), 1, 0, 10)
	if err == nil {
		t.Fatal("GetCustomers() returned nil; the ordering is fixed, so update this test")
	}
	if !strings.Contains(err.Error(), "not in token scope") {
		t.Errorf("error = %q, want it to mention the token scope", err)
	}
	if len(api.Requests()) != 0 {
		t.Errorf("requests = %d, want 0", len(api.Requests()))
	}
}

// ---------------------------------------------------------------------------
// Request behavior
// ---------------------------------------------------------------------------

func TestDoRequestSendsBearerTokenAndUserAgent(t *testing.T) {
	api := tekmetrictest.New(t)
	client := api.AuthedClient(t)

	if _, err := client.GetShops(t.Context()); err != nil {
		t.Fatalf("GetShops() error = %v", err)
	}

	req := api.LastRequest(t)
	if got, want := req.Header.Get("Authorization"), "Bearer "+tekmetrictest.AccessToken; got != want {
		t.Errorf("Authorization = %q, want %q", got, want)
	}
	if !strings.Contains(req.Header.Get("User-Agent"), "tekmetric-mcp") {
		t.Errorf("User-Agent = %q, want it to name tekmetric-mcp", req.Header.Get("User-Agent"))
	}
}

func TestDoRequestRetriesTemporaryStatuses(t *testing.T) {
	tests := []struct {
		name   string
		status int
	}{
		{"too many requests", http.StatusTooManyRequests},
		{"internal server error", http.StatusInternalServerError},
		{"bad gateway", http.StatusBadGateway},
		{"service unavailable", http.StatusServiceUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := tekmetrictest.New(t)
			// Fail twice, then serve the fixture on the third attempt.
			api.Failures = []*tekmetrictest.Failure{{Path: shopsPath, Status: tt.status, Times: 2}}

			client := api.AuthedClient(t)
			if _, err := client.GetShops(t.Context()); err != nil {
				t.Fatalf("GetShops() error = %v", err)
			}
			if api.CallCount(shopsPath) != 3 {
				t.Errorf("attempts = %d, want 3", api.CallCount(shopsPath))
			}
		})
	}
}

func TestDoRequestGivesUpAfterMaxRetries(t *testing.T) {
	api := tekmetrictest.New(t)
	api.FailAlways(shopsPath, http.StatusServiceUnavailable)

	client := api.AuthedClient(t)
	if _, err := client.GetShops(t.Context()); err == nil {
		t.Fatal("GetShops() returned nil, want an error")
	}

	// MaxRetries is 2, so the client makes the first attempt plus two retries.
	if api.CallCount(shopsPath) != 3 {
		t.Errorf("attempts = %d, want 3", api.CallCount(shopsPath))
	}
}

func TestDoRequestDoesNotRetryClientErrors(t *testing.T) {
	tests := []struct {
		name   string
		status int
	}{
		{"bad request", http.StatusBadRequest},
		{"unauthorized", http.StatusUnauthorized},
		{"forbidden", http.StatusForbidden},
		{"not found", http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := tekmetrictest.New(t)
			api.FailAlways(shopsPath, tt.status)

			client := api.AuthedClient(t)
			if _, err := client.GetShops(t.Context()); err == nil {
				t.Fatal("GetShops() returned nil, want an error")
			}
			if api.CallCount(shopsPath) != 1 {
				t.Errorf("attempts = %d, want 1", api.CallCount(shopsPath))
			}
		})
	}
}

func TestDoRequestDecodesTheResponse(t *testing.T) {
	api := tekmetrictest.New(t)
	api.Shops = []tekmetric.Shop{
		{ID: 1, Name: "Main Street Auto"},
		{ID: 2, Name: "Second Shop"},
	}

	shops, err := api.AuthedClient(t).GetShops(t.Context())
	if err != nil {
		t.Fatalf("GetShops() error = %v", err)
	}
	if len(shops) != 2 {
		t.Fatalf("len(shops) = %d, want 2", len(shops))
	}
	if shops[0].Name != "Main Street Auto" {
		t.Errorf("shops[0].Name = %q, want Main Street Auto", shops[0].Name)
	}
}

func TestDoRequestFetchesARecordByID(t *testing.T) {
	api := tekmetrictest.New(t)
	api.Shops = []tekmetric.Shop{{ID: 7, Name: "Seventh Shop"}}

	shop, err := api.AuthedClient(t).GetShop(t.Context(), 7)
	if err != nil {
		t.Fatalf("GetShop() error = %v", err)
	}
	if shop.Name != "Seventh Shop" {
		t.Errorf("Name = %q, want Seventh Shop", shop.Name)
	}
}

func TestDoRequestReportsAMissingRecord(t *testing.T) {
	api := tekmetrictest.New(t)

	if _, err := api.AuthedClient(t).GetShop(t.Context(), 404); err == nil {
		t.Fatal("GetShop() returned nil, want an error")
	}
}

func TestDoRequestRejectsMalformedJSON(t *testing.T) {
	api := tekmetrictest.New(t)
	api.SetOverride(shopsPath, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"id": `)
	})

	if _, err := api.AuthedClient(t).GetShops(t.Context()); err == nil {
		t.Fatal("GetShops() returned nil, want an error")
	}
}

// TestDoRequestLimitsBodySize covers the 10MB read cap. A larger body is cut,
// so the JSON no longer parses.
func TestDoRequestLimitsBodySize(t *testing.T) {
	api := tekmetrictest.New(t)
	api.SetOverride("/api/v1/shops/1", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"name":"`+strings.Repeat("x", 11*1024*1024)+`"}`)
	})

	if _, err := api.AuthedClient(t).GetShop(t.Context(), 1); err == nil {
		t.Fatal("GetShop() returned nil, want an error from the truncated body")
	}
}

func TestDoRequestHonorsContextCancellation(t *testing.T) {
	api := tekmetrictest.New(t)
	client := api.AuthedClient(t)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if _, err := client.GetShops(ctx); err == nil {
		t.Fatal("GetShops() returned nil, want an error from the cancelled context")
	}
}

// TestPaginationEnvelope confirms the mock splits fixtures into pages the same
// way the API does.
func TestPaginationEnvelope(t *testing.T) {
	api := tekmetrictest.New(t)
	for i := 1; i <= 25; i++ {
		api.Customers = append(api.Customers, tekmetric.Customer{ID: i, ShopID: 1})
	}

	client := api.AuthedClient(t)

	first, err := client.GetCustomers(t.Context(), 1, 0, 10)
	if err != nil {
		t.Fatalf("GetCustomers() error = %v", err)
	}
	if len(first.Content) != 10 {
		t.Errorf("page 0 length = %d, want 10", len(first.Content))
	}
	if first.TotalElements != 25 {
		t.Errorf("TotalElements = %d, want 25", first.TotalElements)
	}
	if first.Last {
		t.Error("Last = true on page 0, want false")
	}

	last, err := client.GetCustomers(t.Context(), 1, 2, 10)
	if err != nil {
		t.Fatalf("GetCustomers() error = %v", err)
	}
	if len(last.Content) != 5 {
		t.Errorf("page 2 length = %d, want 5", len(last.Content))
	}
	if !last.Last {
		t.Error("Last = false on the final page, want true")
	}
}
