// Package tekmetrictest provides a mock Tekmetric API for tests.
//
// The mock serves the routes the client calls, backed by in-memory fixtures.
// It never reaches the network. Use it in place of the real API:
//
//	api := tekmetrictest.New(t)
//	client := api.Client(t)
//	shops, err := client.GetShops(context.Background())
//
// A test changes the fixtures before it makes a call, and reads api.Requests()
// after. Set api.Fail to make a route return an error status.
package tekmetrictest

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/beetlebugorg/tekmetric-mcp/internal/config"
	"github.com/beetlebugorg/tekmetric-mcp/pkg/tekmetric"
)

// ClientID and ClientSecret are the credentials the mock accepts.
const (
	ClientID     = "test-client-id"
	ClientSecret = "test-client-secret"
	AccessToken  = "test-access-token"
)

// Request is one call the mock received.
type Request struct {
	Method string
	Path   string
	Query  string
	Header http.Header
	Body   string
}

// Failure makes a route return a status instead of its fixture.
type Failure struct {
	// Path is the exact request path, such as "/api/v1/shops".
	Path string
	// Status is the status code to return.
	Status int
	// Times limits how many calls fail. Zero means every call fails.
	Times int

	sent int
}

// API is a mock Tekmetric API backed by an httptest server.
type API struct {
	// Shops is the fixture for GET /api/v1/shops.
	Shops []tekmetric.Shop
	// Customers is the fixture for the customer routes.
	Customers []tekmetric.Customer
	// Vehicles is the fixture for the vehicle routes.
	Vehicles []tekmetric.Vehicle
	// RepairOrders is the fixture for the repair order routes.
	RepairOrders []tekmetric.RepairOrder
	// Appointments is the fixture for the appointment routes.
	Appointments []tekmetric.Appointment
	// Employees is the fixture for the employee routes.
	Employees []tekmetric.Employee
	// Inventory is the fixture for the inventory routes.
	Inventory []tekmetric.InventoryPart
	// Jobs is the fixture for the job routes.
	Jobs []tekmetric.Job
	// CannedJobs is the fixture for GET /api/v1/canned-jobs.
	CannedJobs []tekmetric.CannedJob

	// Override replaces the fixture response for a path. Use it when a test
	// needs a body the fixtures cannot express, such as malformed JSON.
	Override map[string]http.HandlerFunc

	// Scope is the shop list the token endpoint reports.
	Scope []string
	// ExpiresIn is the token lifetime in seconds.
	ExpiresIn int

	// Failures make routes return an error status.
	Failures []*Failure

	// PageSize splits list fixtures into pages of this size.
	PageSize int

	server   *httptest.Server
	mu       sync.Mutex
	requests []Request
	tokens   int
}

// New starts a mock API with one shop in scope and closes it when the test ends.
func New(t *testing.T) *API {
	t.Helper()

	api := &API{
		Shops:     []tekmetric.Shop{{ID: 1, Name: "Test Shop", Nickname: "Test"}},
		Scope:     []string{"1"},
		ExpiresIn: 3600,
		PageSize:  100,
	}

	api.server = httptest.NewServer(http.HandlerFunc(api.serve))
	t.Cleanup(api.server.Close)

	return api
}

// URL returns the base URL of the mock.
func (a *API) URL() string { return a.server.URL }

// Config returns a TekmetricConfig aimed at the mock.
func (a *API) Config() *config.TekmetricConfig {
	return &config.TekmetricConfig{
		BaseURL:        a.server.URL,
		ClientID:       ClientID,
		ClientSecret:   ClientSecret,
		DefaultShopID:  1,
		TimeoutSeconds: 5,
		MaxRetries:     2,
		MaxBackoffSec:  0,
	}
}

// Client returns a client aimed at the mock. The client is not authenticated.
func (a *API) Client(t *testing.T) *tekmetric.Client {
	t.Helper()
	return tekmetric.NewClient(a.Config(), Logger())
}

// AuthedClient returns a client that has already authenticated.
func (a *API) AuthedClient(t *testing.T) *tekmetric.Client {
	t.Helper()

	client := a.Client(t)
	if err := client.Authenticate(t.Context()); err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	return client
}

// Logger returns a logger that discards output.
func Logger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
}

// Requests returns every call the mock received, oldest first.
func (a *API) Requests() []Request {
	a.mu.Lock()
	defer a.mu.Unlock()

	out := make([]Request, len(a.requests))
	copy(out, a.requests)
	return out
}

// LastRequest returns the most recent call that was not a token call.
func (a *API) LastRequest(t *testing.T) Request {
	t.Helper()

	all := a.Requests()
	for i := len(all) - 1; i >= 0; i-- {
		if all[i].Path != tokenPath {
			return all[i]
		}
	}
	t.Fatal("the mock received no API request")
	return Request{}
}

// CallCount returns how many calls a path received.
func (a *API) CallCount(path string) int {
	a.mu.Lock()
	defer a.mu.Unlock()

	count := 0
	for _, req := range a.requests {
		if req.Path == path {
			count++
		}
	}
	return count
}

// TokenCount returns how many times the client authenticated.
func (a *API) TokenCount() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.tokens
}

// Reset clears the recorded calls and the failure counters.
func (a *API) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.requests = nil
	a.tokens = 0
	for _, f := range a.Failures {
		f.sent = 0
	}
}

// SetOverride replaces the response for a path.
func (a *API) SetOverride(path string, handler http.HandlerFunc) {
	if a.Override == nil {
		a.Override = map[string]http.HandlerFunc{}
	}
	a.Override[path] = handler
}

// FailOnce makes the next call to a path return a status.
func (a *API) FailOnce(path string, status int) {
	a.Failures = append(a.Failures, &Failure{Path: path, Status: status, Times: 1})
}

// FailAlways makes every call to a path return a status.
func (a *API) FailAlways(path string, status int) {
	a.Failures = append(a.Failures, &Failure{Path: path, Status: status})
}

// TokenPath is the OAuth token route. Pass it to SetOverride or FailAlways to
// control authentication in a test.
const TokenPath = "/api/v1/oauth/token"

const tokenPath = TokenPath

func (a *API) serve(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)

	a.mu.Lock()
	a.requests = append(a.requests, Request{
		Method: r.Method,
		Path:   r.URL.Path,
		Query:  r.URL.RawQuery,
		Header: r.Header.Clone(),
		Body:   string(body),
	})
	if r.URL.Path == tokenPath {
		a.tokens++
	}
	a.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")

	if status, ok := a.failureFor(r.URL.Path); ok {
		w.WriteHeader(status)
		_, _ = io.WriteString(w, `{"error":"injected failure"}`)
		return
	}

	if handler, ok := a.Override[r.URL.Path]; ok {
		handler(w, r)
		return
	}

	if r.URL.Path == tokenPath {
		a.serveToken(w, r)
		return
	}

	a.serveAPI(w, r)
}

func (a *API) failureFor(path string) (int, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()

	for _, f := range a.Failures {
		if f.Path != path {
			continue
		}
		if f.Times > 0 && f.sent >= f.Times {
			continue
		}
		f.sent++
		return f.Status, true
	}
	return 0, false
}

func (a *API) serveToken(w http.ResponseWriter, r *http.Request) {
	user, pass, ok := r.BasicAuth()
	if !ok || user != ClientID || pass != ClientSecret {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":"invalid_client"}`)
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]any{
		"access_token": AccessToken,
		"token_type":   "bearer",
		"expires_in":   a.ExpiresIn,
		"scope":        strings.Join(a.Scope, " "),
	})
}

func (a *API) serveAPI(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") != "Bearer "+AccessToken {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":"invalid_token"}`)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/v1/")
	parts := strings.Split(path, "/")

	// A second element is a record ID, as in customers/42.
	if len(parts) == 2 {
		a.serveByID(w, parts[0], parts[1])
		return
	}

	switch parts[0] {
	case "shops":
		writeJSON(w, a.Shops)
	case "customers":
		writePage(w, a.Customers, r, a.PageSize)
	case "vehicles":
		writePage(w, a.Vehicles, r, a.PageSize)
	case "repair-orders":
		writePage(w, a.RepairOrders, r, a.PageSize)
	case "appointments":
		writePage(w, a.Appointments, r, a.PageSize)
	case "employees":
		writePage(w, a.Employees, r, a.PageSize)
	case "inventory":
		writePage(w, a.Inventory, r, a.PageSize)
	case "jobs":
		writePage(w, a.Jobs, r, a.PageSize)
	case "canned-jobs":
		writePage(w, a.CannedJobs, r, a.PageSize)
	default:
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprintf(w, `{"error":"no route for %s"}`, r.URL.Path)
	}
}

func (a *API) serveByID(w http.ResponseWriter, resource, rawID string) {
	id, err := strconv.Atoi(rawID)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"error":"id must be a number"}`)
		return
	}

	switch resource {
	case "shops":
		findByID(w, a.Shops, id, func(s tekmetric.Shop) int { return s.ID })
	case "customers":
		findByID(w, a.Customers, id, func(c tekmetric.Customer) int { return c.ID })
	case "vehicles":
		findByID(w, a.Vehicles, id, func(v tekmetric.Vehicle) int { return v.ID })
	case "repair-orders":
		findByID(w, a.RepairOrders, id, func(ro tekmetric.RepairOrder) int { return ro.ID })
	case "appointments":
		findByID(w, a.Appointments, id, func(ap tekmetric.Appointment) int { return ap.ID })
	case "employees":
		findByID(w, a.Employees, id, func(e tekmetric.Employee) int { return e.ID })
	case "jobs":
		findByID(w, a.Jobs, id, func(j tekmetric.Job) int { return j.ID })
	default:
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprintf(w, `{"error":"no route for %s/%d"}`, resource, id)
	}
}

func findByID[T any](w http.ResponseWriter, items []T, id int, idOf func(T) int) {
	for _, item := range items {
		if idOf(item) == id {
			writeJSON(w, item)
			return
		}
	}
	w.WriteHeader(http.StatusNotFound)
	_, _ = fmt.Fprintf(w, `{"error":"no record with id %d"}`, id)
}

func writeJSON(w http.ResponseWriter, v any) {
	_ = json.NewEncoder(w).Encode(v)
}

// writePage returns the slice of items the page and size arguments select,
// wrapped in the paginated envelope the API uses.
func writePage[T any](w http.ResponseWriter, items []T, r *http.Request, defaultSize int) {
	page := intParam(r, "page", 0)
	size := intParam(r, "size", defaultSize)
	if size < 1 {
		size = defaultSize
	}

	start := page * size
	if start > len(items) {
		start = len(items)
	}
	end := start + size
	if end > len(items) {
		end = len(items)
	}

	content := items[start:end]
	totalPages := (len(items) + size - 1) / size

	writeJSON(w, map[string]any{
		"content":          content,
		"totalPages":       totalPages,
		"totalElements":    len(items),
		"last":             end >= len(items),
		"first":            page == 0,
		"size":             size,
		"number":           page,
		"numberOfElements": len(content),
		"empty":            len(content) == 0,
	})
}

func intParam(r *http.Request, key string, fallback int) int {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}
