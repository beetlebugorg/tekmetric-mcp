package mcp

import (
	"context"
	"strings"
	"testing"

	"github.com/beetlebugorg/tekmetric-mcp/internal/config"
	"github.com/beetlebugorg/tekmetric-mcp/pkg/tekmetric"
	"github.com/beetlebugorg/tekmetric-mcp/pkg/tekmetric/tekmetrictest"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

// expectedTools lists every tool the server registers.
var expectedTools = []string{
	"shops",
	"customers",
	"vehicles",
	"repair_orders",
	"jobs",
	"appointments",
	"employees",
	"inventory",
	"vehicle_service_analysis",
}

// newTestServer builds a server wired to the mock API.
func newTestServer(t *testing.T, api *tekmetrictest.API) *Server {
	t.Helper()

	cfg := &config.Config{
		Tekmetric: *api.Config(),
		Server:    config.ServerConfig{Name: "tekmetric-mcp", Version: "test"},
		Analysis:  config.AnalysisConfig{MaxPages: 50, MaxRecords: 5000, TimeoutSeconds: 120},
	}

	srv, err := NewServer(cfg, tekmetrictest.Logger())
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	return srv
}

// connect starts an in-process MCP session against the server. It exercises the
// real protocol, so it covers tool registration and the handler signature.
func connect(t *testing.T, srv *Server) *client.Client {
	t.Helper()

	mcpClient, err := client.NewInProcessClient(srv.server)
	if err != nil {
		t.Fatalf("NewInProcessClient() error = %v", err)
	}
	t.Cleanup(func() { _ = mcpClient.Close() })

	if err := mcpClient.Start(t.Context()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	var init mcp.InitializeRequest
	init.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	init.Params.ClientInfo = mcp.Implementation{Name: "test", Version: "1"}

	if _, err := mcpClient.Initialize(t.Context(), init); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	return mcpClient
}

// TestServerRegistersEveryTool drives tools/list over the protocol.
func TestServerRegistersEveryTool(t *testing.T) {
	api := tekmetrictest.New(t)
	mcpClient := connect(t, newTestServer(t, api))

	result, err := mcpClient.ListTools(t.Context(), mcp.ListToolsRequest{})
	if err != nil {
		t.Fatalf("ListTools() error = %v", err)
	}

	found := map[string]bool{}
	for _, tool := range result.Tools {
		found[tool.Name] = true
	}

	for _, name := range expectedTools {
		if !found[name] {
			t.Errorf("tool %q is not registered", name)
		}
	}
	if len(result.Tools) != len(expectedTools) {
		t.Errorf("registered %d tools, want %d", len(result.Tools), len(expectedTools))
	}
}

// TestToolsDeclareASchema confirms each tool carries a description and an input
// schema after the upgrade.
func TestToolsDeclareASchema(t *testing.T) {
	api := tekmetrictest.New(t)
	mcpClient := connect(t, newTestServer(t, api))

	result, err := mcpClient.ListTools(t.Context(), mcp.ListToolsRequest{})
	if err != nil {
		t.Fatalf("ListTools() error = %v", err)
	}

	for _, tool := range result.Tools {
		if tool.Description == "" {
			t.Errorf("tool %q has no description", tool.Name)
		}
		if tool.InputSchema.Type != "object" {
			t.Errorf("tool %q input schema type = %q, want object", tool.Name, tool.InputSchema.Type)
		}
	}
}

// TestCallToolOverTheProtocol runs a tool the way a client does, which covers
// the handler signature end to end.
func TestCallToolOverTheProtocol(t *testing.T) {
	api := tekmetrictest.New(t)
	api.Shops = []tekmetric.Shop{{ID: 1, Name: "Main Street Auto"}}

	mcpClient := connect(t, newTestServer(t, api))

	var call mcp.CallToolRequest
	call.Params.Name = "shops"
	call.Params.Arguments = map[string]any{}

	result, err := mcpClient.CallTool(t.Context(), call)
	if err != nil {
		t.Fatalf("CallTool() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("CallTool() reported an error: %s", resultText(result))
	}
	if !strings.Contains(resultText(result), "Main Street Auto") {
		t.Errorf("result does not name the shop:\n%s", resultText(result))
	}
}

// TestCallToolPassesArguments confirms arguments survive the request wrapper.
func TestCallToolPassesArguments(t *testing.T) {
	api := tekmetrictest.New(t)
	api.Customers = []tekmetric.Customer{{ID: 10, ShopID: 1, FirstName: "Ada", LastName: "Lovelace"}}

	mcpClient := connect(t, newTestServer(t, api))

	var call mcp.CallToolRequest
	call.Params.Name = "customers"
	call.Params.Arguments = map[string]any{"shop": float64(1), "limit": float64(5)}

	result, err := mcpClient.CallTool(t.Context(), call)
	if err != nil {
		t.Fatalf("CallTool() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("CallTool() reported an error: %s", resultText(result))
	}
	if !strings.Contains(resultText(result), "Lovelace") {
		t.Errorf("result does not name the customer:\n%s", resultText(result))
	}
	if got := api.LastRequest(t).Query; !strings.Contains(got, "shop=1") {
		t.Errorf("query = %s, want it to carry the shop", got)
	}
}

// TestCallToolReportsAToolError confirms a handler error reaches the client as a
// tool error rather than a protocol error.
func TestCallToolReportsAToolError(t *testing.T) {
	api := tekmetrictest.New(t)
	mcpClient := connect(t, newTestServer(t, api))

	var call mcp.CallToolRequest
	call.Params.Name = "repair_orders"
	call.Params.Arguments = map[string]any{"shop": float64(1), "start_date": "yesterday"}

	result, err := mcpClient.CallTool(t.Context(), call)
	if err != nil {
		t.Fatalf("CallTool() error = %v", err)
	}
	if !result.IsError {
		t.Fatalf("CallTool() accepted a malformed date:\n%s", resultText(result))
	}
}

// TestCallToolRecoversFromAPanic confirms the library contains a panic in a
// handler. Version 0.7.0 had no recovery, so a panic ended the process.
func TestCallToolRecoversFromAPanic(t *testing.T) {
	api := tekmetrictest.New(t)
	srv := newTestServer(t, api)

	// Register a tool that panics, next to the real ones.
	srv.server.AddTool(
		mcp.NewTool("panics", mcp.WithDescription("A tool that panics.")),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			panic("boom")
		},
	)

	mcpClient := connect(t, srv)

	var call mcp.CallToolRequest
	call.Params.Name = "panics"
	call.Params.Arguments = map[string]any{}

	// The call fails, but the process survives and the session keeps working.
	_, _ = mcpClient.CallTool(t.Context(), call)

	if _, err := mcpClient.ListTools(t.Context(), mcp.ListToolsRequest{}); err != nil {
		t.Fatalf("the session did not survive the panic: %v", err)
	}
}

// resultText joins the text content of a tool result.
func resultText(result *mcp.CallToolResult) string {
	var out strings.Builder
	for _, content := range result.Content {
		if text, ok := content.(mcp.TextContent); ok {
			out.WriteString(text.Text)
		}
	}
	return out.String()
}
