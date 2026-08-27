package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/beetlebugorg/tekmetric-mcp/pkg/tekmetric/tekmetrictest"
	"github.com/mark3labs/mcp-go/mcp"
)

// manifest is the part of manifest.json these tests read.
type manifest struct {
	Server struct {
		MCPConfig struct {
			Args []string          `json:"args"`
			Env  map[string]string `json:"env"`
		} `json:"mcp_config"`
	} `json:"server"`
	UserConfig map[string]struct {
		Default  any  `json:"default"`
		Required bool `json:"required"`
	} `json:"user_config"`
	Tools []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	} `json:"tools"`
}

// readManifest loads manifest.json from the repository root.
func readManifest(t *testing.T) manifest {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join("..", "..", "manifest.json"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	var m manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("manifest.json is not valid JSON: %v", err)
	}
	return m
}

// TestManifestListsEveryTool keeps the Desktop extension in step with the code.
// Claude Desktop shows this list before it starts the server, so a missing tool
// is a tool the user does not know exists.
func TestManifestListsEveryTool(t *testing.T) {
	api := tekmetrictest.New(t)
	mcpClient := connect(t, newTestServer(t, api))

	result, err := mcpClient.ListTools(t.Context(), mcp.ListToolsRequest{})
	if err != nil {
		t.Fatalf("ListTools() error = %v", err)
	}

	registered := make([]string, 0, len(result.Tools))
	for _, tool := range result.Tools {
		registered = append(registered, tool.Name)
	}

	m := readManifest(t)
	listed := make([]string, 0, len(m.Tools))
	for _, tool := range m.Tools {
		listed = append(listed, tool.Name)
		if tool.Description == "" {
			t.Errorf("manifest.json gives tool %q no description", tool.Name)
		}
	}

	sort.Strings(registered)
	sort.Strings(listed)

	if strings.Join(registered, ",") != strings.Join(listed, ",") {
		t.Errorf("manifest.json and the server disagree\n  manifest:   %v\n  registered: %v",
			listed, registered)
	}
}

// TestManifestUsesTheAPIEndpoint guards the default the extension prefills.
// The host api.tekmetric.com serves the access request page, not the API, so a
// user who accepts the default reaches the wrong host.
func TestManifestUsesTheAPIEndpoint(t *testing.T) {
	m := readManifest(t)

	baseURL, ok := m.UserConfig["base_url"]
	if !ok {
		t.Fatal("manifest.json declares no base_url setting")
	}

	got, _ := baseURL.Default.(string)
	if got != "https://shop.tekmetric.com" {
		t.Errorf("base_url default = %q, want https://shop.tekmetric.com", got)
	}
}

// TestManifestStartsTheStdioTransport confirms the extension launches the
// server the way Claude Desktop needs. Desktop speaks stdio, which is the
// default, so the arguments must not name another transport.
func TestManifestStartsTheStdioTransport(t *testing.T) {
	m := readManifest(t)

	args := m.Server.MCPConfig.Args
	if len(args) == 0 || args[0] != "serve" {
		t.Fatalf("args = %v, want serve first", args)
	}
	for _, arg := range args {
		if strings.Contains(arg, "transport") {
			t.Errorf("args name a transport (%q); Desktop needs the stdio default", arg)
		}
	}
}

// TestManifestPassesTheCredentials confirms the extension forwards every
// setting the server needs.
func TestManifestPassesTheCredentials(t *testing.T) {
	m := readManifest(t)

	for _, name := range []string{
		"TEKMETRIC_CLIENT_ID",
		"TEKMETRIC_CLIENT_SECRET",
		"TEKMETRIC_BASE_URL",
		"TEKMETRIC_DEFAULT_SHOP_ID",
	} {
		if _, ok := m.Server.MCPConfig.Env[name]; !ok {
			t.Errorf("manifest.json does not pass %s", name)
		}
	}

	for _, name := range []string{"client_id", "client_secret", "default_shop_id"} {
		setting, ok := m.UserConfig[name]
		if !ok {
			t.Errorf("manifest.json declares no %s setting", name)
			continue
		}
		if !setting.Required {
			t.Errorf("manifest.json marks %s optional, but the server needs it", name)
		}
	}
}
