package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/beetlebugorg/tekmetric-mcp/internal/config"
	"github.com/beetlebugorg/tekmetric-mcp/pkg/tekmetric"
	"github.com/beetlebugorg/tekmetric-mcp/pkg/tekmetric/tekmetrictest"
)

// freePort reserves a port the operating system is not using.
func freePort(t *testing.T) int {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	defer listener.Close()

	return listener.Addr().(*net.TCPAddr).Port
}

// testToken is the bearer token the test servers require.
const testToken = "test-token"

// startHTTPServer runs the server on the http transport and returns its URL.
func startHTTPServer(t *testing.T, api *tekmetrictest.API) string {
	t.Helper()

	port := freePort(t)
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	cfg := &config.Config{
		Tekmetric: *api.Config(),
		Server: config.ServerConfig{
			Name:      "tekmetric-mcp",
			Version:   "test",
			Transport: config.TransportHTTP,
			Addr:      addr,
			Token:     testToken,
		},
		Analysis: config.AnalysisConfig{MaxPages: 50, MaxRecords: 5000, TimeoutSeconds: 120},
	}

	srv, err := NewServer(cfg, tekmetrictest.Logger())
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.Start(ctx) }()

	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("Start() error = %v", err)
			}
		case <-time.After(20 * time.Second):
			t.Error("the server did not stop")
		}
	})

	url := "http://" + addr + "/mcp"
	waitForServer(t, url)
	return url
}

// waitForServer blocks until the listener answers.
func waitForServer(t *testing.T, url string) {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Post(url, "application/json", strings.NewReader("{}"))
		if err == nil {
			resp.Body.Close()
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("the server at %s did not start", url)
}

// post sends one JSON-RPC message and returns the response and its headers.
func post(t *testing.T, url string, message any) (map[string]any, http.Header) {
	t.Helper()

	body, err := json.Marshal(message)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Authorization", "Bearer "+testToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}

	// A stateless server answers with JSON. An event stream would carry the
	// payload after a "data: " prefix.
	text := strings.TrimSpace(string(raw))
	if after, ok := strings.CutPrefix(text, "event:"); ok {
		_, text, _ = strings.Cut(after, "data:")
		text = strings.TrimSpace(text)
	}

	var decoded map[string]any
	if text != "" {
		if err := json.Unmarshal([]byte(text), &decoded); err != nil {
			t.Fatalf("the response is not JSON-RPC: %v\n%s", err, text)
		}
	}
	return decoded, resp.Header
}

func initializeMessage(id int) map[string]any {
	return map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2025-06-18",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "test", "version": "1"},
		},
	}
}

func callMessage(id int, name string, arguments map[string]any) map[string]any {
	return map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  "tools/call",
		"params":  map[string]any{"name": name, "arguments": arguments},
	}
}

// TestHTTPIssuesNoSessionID is the property that makes the transport stateless.
// A server that issues a session ID expects the client to return it, which ties
// that client to one replica.
func TestHTTPIssuesNoSessionID(t *testing.T) {
	api := tekmetrictest.New(t)
	url := startHTTPServer(t, api)

	_, header := post(t, url, initializeMessage(1))

	if got := header.Get("Mcp-Session-Id"); got != "" {
		t.Errorf("Mcp-Session-Id = %q, want no header", got)
	}
}

func TestHTTPInitialize(t *testing.T) {
	api := tekmetrictest.New(t)
	url := startHTTPServer(t, api)

	response, _ := post(t, url, initializeMessage(1))

	result, ok := response["result"].(map[string]any)
	if !ok {
		t.Fatalf("response has no result: %v", response)
	}
	info, ok := result["serverInfo"].(map[string]any)
	if !ok {
		t.Fatalf("result has no serverInfo: %v", result)
	}
	if info["name"] != "tekmetric-mcp" {
		t.Errorf("serverInfo.name = %v, want tekmetric-mcp", info["name"])
	}
}

// TestHTTPCallsWithoutASession confirms a request carries everything the server
// needs. Neither call sends a session header, and both succeed.
func TestHTTPCallsWithoutASession(t *testing.T) {
	api := tekmetrictest.New(t)
	api.Shops = []tekmetric.Shop{{ID: 1, Name: "Main Street Auto"}}
	url := startHTTPServer(t, api)

	for id := 1; id <= 2; id++ {
		response, header := post(t, url, callMessage(id, "shops", map[string]any{}))

		if got := header.Get("Mcp-Session-Id"); got != "" {
			t.Errorf("call %d: Mcp-Session-Id = %q, want no header", id, got)
		}
		if errObj, ok := response["error"]; ok {
			t.Fatalf("call %d returned an error: %v", id, errObj)
		}

		result, ok := response["result"].(map[string]any)
		if !ok {
			t.Fatalf("call %d has no result: %v", id, response)
		}
		if !strings.Contains(fmt.Sprint(result["content"]), "Main Street Auto") {
			t.Errorf("call %d does not name the shop: %v", id, result["content"])
		}
	}
}

// TestHTTPListsTools confirms the tools reach a client over HTTP.
func TestHTTPListsTools(t *testing.T) {
	api := tekmetrictest.New(t)
	url := startHTTPServer(t, api)

	response, _ := post(t, url, map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/list",
	})

	result, ok := response["result"].(map[string]any)
	if !ok {
		t.Fatalf("response has no result: %v", response)
	}
	tools, ok := result["tools"].([]any)
	if !ok {
		t.Fatalf("result has no tools: %v", result)
	}
	if len(tools) != len(expectedTools) {
		t.Errorf("listed %d tools, want %d", len(tools), len(expectedTools))
	}
}

// TestHTTPDoesNotAuthenticateAtStartup confirms a replica starts cold. A token
// fetched at startup would expire while the replica sits idle.
func TestHTTPDoesNotAuthenticateAtStartup(t *testing.T) {
	api := tekmetrictest.New(t)
	url := startHTTPServer(t, api)

	if api.TokenCount() != 0 {
		t.Errorf("token requests before the first call = %d, want 0", api.TokenCount())
	}

	post(t, url, callMessage(1, "shops", map[string]any{}))

	if api.TokenCount() != 1 {
		t.Errorf("token requests after the first call = %d, want 1", api.TokenCount())
	}
}

// TestTwoReplicasServeTheSameClient confirms two processes answer the same
// requests, which is what statelessness buys.
func TestTwoReplicasServeTheSameClient(t *testing.T) {
	api := tekmetrictest.New(t)
	api.Shops = []tekmetric.Shop{{ID: 1, Name: "Main Street Auto"}}

	first := startHTTPServer(t, api)
	second := startHTTPServer(t, api)

	if first == second {
		t.Fatal("the two replicas share an address")
	}

	// Initialize against one replica, then call the other. A stateless server
	// does not need the call to land where the session began.
	post(t, first, initializeMessage(1))

	response, _ := post(t, second, callMessage(2, "shops", map[string]any{}))
	if errObj, ok := response["error"]; ok {
		t.Fatalf("the second replica returned an error: %v", errObj)
	}

	result, ok := response["result"].(map[string]any)
	if !ok {
		t.Fatalf("the second replica returned no result: %v", response)
	}
	if !strings.Contains(fmt.Sprint(result["content"]), "Main Street Auto") {
		t.Errorf("the second replica does not name the shop: %v", result["content"])
	}
}

// TestHTTPReportsAToolError confirms a tool error crosses the transport.
func TestHTTPReportsAToolError(t *testing.T) {
	api := tekmetrictest.New(t)
	url := startHTTPServer(t, api)

	response, _ := post(t, url, callMessage(1, "repair_orders", map[string]any{
		"shop": float64(1), "start_date": "yesterday",
	}))

	result, ok := response["result"].(map[string]any)
	if !ok {
		t.Fatalf("response has no result: %v", response)
	}
	if result["isError"] != true {
		t.Errorf("isError = %v, want true", result["isError"])
	}
}

// postWithAuth sends one JSON-RPC message with an Authorization header and
// returns the response. An empty header value sends no header.
func postWithAuth(t *testing.T, url, authorization string, message any) *http.Response {
	t.Helper()

	body, err := json.Marshal(message)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

// TestHTTPRejectsABadToken confirms a caller without the token reaches no tool
// and causes no call to the Tekmetric API.
func TestHTTPRejectsABadToken(t *testing.T) {
	api := tekmetrictest.New(t)
	url := startHTTPServer(t, api)

	tests := []struct {
		name          string
		authorization string
	}{
		{"no header", ""},
		{"a wrong token", "Bearer wrong-token"},
		{"a prefix of the token", "Bearer test"},
		{"no scheme", testToken},
		{"another scheme", "Basic " + testToken},
		{"an empty token", "Bearer "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := postWithAuth(t, url, tt.authorization, callMessage(1, "shops", map[string]any{}))

			if resp.StatusCode != http.StatusUnauthorized {
				t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
			}
			if got := resp.Header.Get("WWW-Authenticate"); !strings.HasPrefix(got, "Bearer") {
				t.Errorf("WWW-Authenticate = %q, want a Bearer challenge", got)
			}
		})
	}

	if api.TokenCount() != 0 {
		t.Errorf("Tekmetric token requests = %d, want 0", api.TokenCount())
	}
}

// TestHTTPAcceptsTheToken confirms a caller with the token reaches the tools.
// The scheme name is case-insensitive.
func TestHTTPAcceptsTheToken(t *testing.T) {
	api := tekmetrictest.New(t)
	api.Shops = []tekmetric.Shop{{ID: 1, Name: "Main Street Auto"}}
	url := startHTTPServer(t, api)

	for _, scheme := range []string{"Bearer", "bearer"} {
		t.Run(scheme, func(t *testing.T) {
			resp := postWithAuth(t, url, scheme+" "+testToken, callMessage(1, "shops", map[string]any{}))

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
			}
			raw, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("ReadAll() error = %v", err)
			}
			if !strings.Contains(string(raw), "Main Street Auto") {
				t.Errorf("the response does not name the shop: %s", raw)
			}
		})
	}
}

// TestHTTPStartFailsWithoutAToken confirms Start returns an error when the
// token is empty and the caller skips Validate.
func TestHTTPStartFailsWithoutAToken(t *testing.T) {
	api := tekmetrictest.New(t)

	cfg := &config.Config{
		Tekmetric: *api.Config(),
		Server: config.ServerConfig{
			Name:      "tekmetric-mcp",
			Version:   "test",
			Transport: config.TransportHTTP,
			Addr:      fmt.Sprintf("127.0.0.1:%d", freePort(t)),
		},
		Analysis: config.AnalysisConfig{MaxPages: 50, MaxRecords: 5000, TimeoutSeconds: 120},
	}

	srv, err := NewServer(cfg, tekmetrictest.Logger())
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = srv.Start(ctx)
	if err == nil || !strings.Contains(err.Error(), "server.token") {
		t.Errorf("Start() error = %v, want it to mention server.token", err)
	}
}
