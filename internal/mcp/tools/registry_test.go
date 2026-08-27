package tools

import (
	"strings"
	"testing"

	"github.com/beetlebugorg/tekmetric-mcp/internal/config"
	"github.com/beetlebugorg/tekmetric-mcp/pkg/tekmetric"
	"github.com/beetlebugorg/tekmetric-mcp/pkg/tekmetric/tekmetrictest"
	"github.com/mark3labs/mcp-go/mcp"
)

// newTestRegistry returns a registry wired to the mock API.
func newTestRegistry(t *testing.T, api *tekmetrictest.API) *Registry {
	t.Helper()

	cfg := &config.Config{
		Tekmetric: *api.Config(),
		Server:    config.ServerConfig{Name: "tekmetric-mcp", Version: "test"},
		Analysis:  config.AnalysisConfig{MaxPages: 50, MaxRecords: 5000, TimeoutSeconds: 120},
	}

	return NewRegistry(api.AuthedClient(t), cfg, tekmetrictest.Logger())
}

// args builds a tool argument map. Numbers arrive as float64 over JSON, so the
// helper converts them the same way.
func args(pairs map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range pairs {
		switch v := value.(type) {
		case int:
			out[key] = float64(v)
		default:
			out[key] = v
		}
	}
	return out
}

// text returns the text of a tool result.
func text(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()

	if result == nil {
		t.Fatal("result is nil")
	}

	var out strings.Builder
	for _, content := range result.Content {
		if tc, ok := content.(mcp.TextContent); ok {
			out.WriteString(tc.Text)
		}
	}
	return out.String()
}

// seedShop fills the mock with one record of each resource for shop 1.
func seedShop(api *tekmetrictest.API) {
	api.Shops = []tekmetric.Shop{{ID: 1, Name: "Main Street Auto"}}
	api.Customers = []tekmetric.Customer{{ID: 10, ShopID: 1, FirstName: "Ada", LastName: "Lovelace"}}
	api.Vehicles = []tekmetric.Vehicle{{ID: 20, CustomerID: 10, Year: 2019, Make: "Ford", Model: "F-150"}}
	api.RepairOrders = []tekmetric.RepairOrder{{ID: 30, ShopID: 1, RepairOrderNumber: 1001}}
	api.Appointments = []tekmetric.Appointment{{ID: 40, ShopID: 1}}
	api.Employees = []tekmetric.Employee{{ID: 50, ShopID: 1, FirstName: "Grace", LastName: "Hopper"}}
	api.Inventory = []tekmetric.InventoryPart{{ID: 60, ShopID: 1, PartNumber: "OF-100", Description: "Oil Filter"}}
	api.Jobs = []tekmetric.Job{{ID: 70, Name: "Oil Change"}}
}

// TestHandlersReturnData runs each tool against the mock and confirms it
// returns its fixture without an error.
func TestHandlersReturnData(t *testing.T) {
	tests := []struct {
		name    string
		handler func(*Registry) func(map[string]any) (*mcp.CallToolResult, error)
		args    map[string]any
		want    string
	}{
		{
			name:    "shops",
			handler: func(r *Registry) func(map[string]any) (*mcp.CallToolResult, error) { return r.handleShops },
			args:    args(map[string]any{}),
			want:    "Main Street Auto",
		},
		{
			name:    "customers",
			handler: func(r *Registry) func(map[string]any) (*mcp.CallToolResult, error) { return r.handleCustomers },
			args:    args(map[string]any{"shop": 1}),
			want:    "Lovelace",
		},
		{
			name:    "vehicles",
			handler: func(r *Registry) func(map[string]any) (*mcp.CallToolResult, error) { return r.handleVehicles },
			args:    args(map[string]any{"shop": 1}),
			want:    "F-150",
		},
		{
			name:    "repair orders",
			handler: func(r *Registry) func(map[string]any) (*mcp.CallToolResult, error) { return r.handleRepairOrders },
			args:    args(map[string]any{"shop": 1}),
			want:    "1001",
		},
		{
			name:    "appointments",
			handler: func(r *Registry) func(map[string]any) (*mcp.CallToolResult, error) { return r.handleAppointments },
			args:    args(map[string]any{"shop": 1}),
			want:    "40",
		},
		{
			name:    "employees",
			handler: func(r *Registry) func(map[string]any) (*mcp.CallToolResult, error) { return r.handleEmployees },
			args:    args(map[string]any{"shop": 1}),
			want:    "Hopper",
		},
		{
			name:    "inventory",
			handler: func(r *Registry) func(map[string]any) (*mcp.CallToolResult, error) { return r.handleInventory },
			args:    args(map[string]any{"shop": 1, "part_type_id": 1}),
			want:    "Oil Filter",
		},
		{
			name:    "jobs",
			handler: func(r *Registry) func(map[string]any) (*mcp.CallToolResult, error) { return r.handleJobs },
			args:    args(map[string]any{"shop": 1}),
			want:    "Oil Change",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := tekmetrictest.New(t)
			seedShop(api)

			result, err := tt.handler(newTestRegistry(t, api))(tt.args)
			if err != nil {
				t.Fatalf("handler error = %v", err)
			}
			if result.IsError {
				t.Fatalf("handler reported an error: %s", text(t, result))
			}
			if got := text(t, result); !strings.Contains(got, tt.want) {
				t.Errorf("result does not contain %q:\n%s", tt.want, got)
			}
		})
	}
}

// TestLimitsDoNotPanic covers the argument values that used to end the process.
// A limit below 1 reached a division and a slice bound.
func TestLimitsDoNotPanic(t *testing.T) {
	limits := []int{-1000, -1, 0, 1}

	for _, limit := range limits {
		t.Run("repair_orders", func(t *testing.T) {
			api := tekmetrictest.New(t)
			seedShop(api)

			result, err := newTestRegistry(t, api).handleRepairOrders(
				args(map[string]any{"shop": 1, "limit": limit}))
			if err != nil {
				t.Fatalf("limit %d: error = %v", limit, err)
			}
			if result.IsError {
				t.Fatalf("limit %d: %s", limit, text(t, result))
			}
		})

		t.Run("shops", func(t *testing.T) {
			api := tekmetrictest.New(t)
			seedShop(api)

			result, err := newTestRegistry(t, api).handleShops(
				args(map[string]any{"query": "Main", "limit": limit}))
			if err != nil {
				t.Fatalf("limit %d: error = %v", limit, err)
			}
			if result.IsError {
				t.Fatalf("limit %d: %s", limit, text(t, result))
			}
		})

		t.Run("inventory", func(t *testing.T) {
			api := tekmetrictest.New(t)
			seedShop(api)

			result, err := newTestRegistry(t, api).handleInventory(
				args(map[string]any{"shop": 1, "part_type_id": 1, "limit": limit, "query": "Oil"}))
			if err != nil {
				t.Fatalf("limit %d: error = %v", limit, err)
			}
			if result.IsError {
				t.Fatalf("limit %d: %s", limit, text(t, result))
			}
		})
	}
}

// TestMalformedDateIsReported covers the date arguments. A date the tool cannot
// parse must produce an error, not a silent unfiltered result.
func TestMalformedDateIsReported(t *testing.T) {
	tests := []struct {
		name    string
		handler func(*Registry) func(map[string]any) (*mcp.CallToolResult, error)
		args    map[string]any
	}{
		{
			name:    "repair orders start_date",
			handler: func(r *Registry) func(map[string]any) (*mcp.CallToolResult, error) { return r.handleRepairOrders },
			args:    map[string]any{"shop": float64(1), "start_date": "yesterday"},
		},
		{
			name:    "repair orders end_date",
			handler: func(r *Registry) func(map[string]any) (*mcp.CallToolResult, error) { return r.handleRepairOrders },
			args:    map[string]any{"shop": float64(1), "end_date": "03/14/2025"},
		},
		{
			name:    "appointments start_date",
			handler: func(r *Registry) func(map[string]any) (*mcp.CallToolResult, error) { return r.handleAppointments },
			args:    map[string]any{"shop": float64(1), "start_date": "not a date"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := tekmetrictest.New(t)
			seedShop(api)

			result, err := tt.handler(newTestRegistry(t, api))(tt.args)
			if err != nil {
				t.Fatalf("handler error = %v", err)
			}
			if !result.IsError {
				t.Fatalf("handler accepted a malformed date:\n%s", text(t, result))
			}
			if got := text(t, result); !strings.Contains(got, "YYYY-MM-DD") {
				t.Errorf("error does not name the expected format:\n%s", got)
			}
		})
	}
}

func TestValidDateIsAccepted(t *testing.T) {
	api := tekmetrictest.New(t)
	seedShop(api)

	result, err := newTestRegistry(t, api).handleRepairOrders(
		map[string]any{"shop": float64(1), "start_date": "2025-01-01"})
	if err != nil {
		t.Fatalf("handler error = %v", err)
	}
	if result.IsError {
		t.Fatalf("handler rejected a valid date: %s", text(t, result))
	}

	if got := api.LastRequest(t).Query; !strings.Contains(got, "start=2025-01-01T00") {
		t.Errorf("query = %s, want it to carry the start date", got)
	}
}

func TestUnauthorizedShopIsReported(t *testing.T) {
	api := tekmetrictest.New(t)
	seedShop(api)

	result, err := newTestRegistry(t, api).handleCustomers(args(map[string]any{"shop": 99}))
	if err != nil {
		t.Fatalf("handler error = %v", err)
	}
	if !result.IsError {
		t.Fatal("handler accepted a shop outside the token scope")
	}
}

func TestInventoryRequiresPartType(t *testing.T) {
	api := tekmetrictest.New(t)
	seedShop(api)

	result, err := newTestRegistry(t, api).handleInventory(args(map[string]any{"shop": 1}))
	if err != nil {
		t.Fatalf("handler error = %v", err)
	}
	if !result.IsError {
		t.Fatal("handler accepted a call with no part_type_id")
	}
}

// TestRepairOrdersCarryTheFinancialWarning covers the guard that steers callers
// away from using this data for reporting.
func TestRepairOrdersCarryTheFinancialWarning(t *testing.T) {
	api := tekmetrictest.New(t)
	seedShop(api)

	result, err := newTestRegistry(t, api).handleRepairOrders(args(map[string]any{"shop": 1}))
	if err != nil {
		t.Fatalf("handler error = %v", err)
	}
	if got := text(t, result); !strings.Contains(got, "FINANCIAL_WARNING") {
		t.Errorf("result omits the financial warning:\n%s", got)
	}
}

func TestAPIFailureBecomesAToolError(t *testing.T) {
	api := tekmetrictest.New(t)
	seedShop(api)
	api.FailAlways("/api/v1/customers", 500)

	result, err := newTestRegistry(t, api).handleCustomers(args(map[string]any{"shop": 1}))
	if err != nil {
		t.Fatalf("handler error = %v", err)
	}
	if !result.IsError {
		t.Fatal("handler did not report the API failure")
	}
}
