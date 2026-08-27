package analysis

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/beetlebugorg/tekmetric-mcp/internal/config"
	"github.com/beetlebugorg/tekmetric-mcp/pkg/tekmetric"
	"github.com/beetlebugorg/tekmetric-mcp/pkg/tekmetric/tekmetrictest"
)

// pageFetcher returns a fetcher that serves total items in pages of size.
func pageFetcher(total, size int) func(int) (*tekmetric.PaginatedResponse[int], error) {
	return func(page int) (*tekmetric.PaginatedResponse[int], error) {
		start := page * size
		if start > total {
			start = total
		}
		end := start + size
		if end > total {
			end = total
		}

		content := make([]int, 0, end-start)
		for i := start; i < end; i++ {
			content = append(content, i)
		}

		return &tekmetric.PaginatedResponse[int]{
			Content:       content,
			TotalElements: total,
			Last:          end >= total,
		}, nil
	}
}

func TestFetchAllPagesReadsEveryPage(t *testing.T) {
	items, metadata, err := FetchAllPages(t.Context(), tekmetrictest.Logger(),
		pageFetcher(25, 10), 50)
	if err != nil {
		t.Fatalf("FetchAllPages() error = %v", err)
	}

	if len(items) != 25 {
		t.Errorf("len(items) = %d, want 25", len(items))
	}
	if metadata.PagesTraversed != 3 {
		t.Errorf("PagesTraversed = %d, want 3", metadata.PagesTraversed)
	}
	if metadata.RecordsFetched != 25 {
		t.Errorf("RecordsFetched = %d, want 25", metadata.RecordsFetched)
	}
	if metadata.RecordsProcessed != 25 {
		t.Errorf("RecordsProcessed = %d, want 25", metadata.RecordsProcessed)
	}
}

func TestFetchAllPagesStopsAtMaxPages(t *testing.T) {
	items, metadata, err := FetchAllPages(t.Context(), tekmetrictest.Logger(),
		pageFetcher(1000, 10), 3)
	if err != nil {
		t.Fatalf("FetchAllPages() error = %v", err)
	}

	if len(items) != 30 {
		t.Errorf("len(items) = %d, want 30", len(items))
	}
	if metadata.PagesTraversed != 3 {
		t.Errorf("PagesTraversed = %d, want 3", metadata.PagesTraversed)
	}
}

func TestFetchAllPagesStopsOnAnEmptyPage(t *testing.T) {
	calls := 0
	fetcher := func(page int) (*tekmetric.PaginatedResponse[int], error) {
		calls++
		return &tekmetric.PaginatedResponse[int]{Content: nil, Last: false}, nil
	}

	items, _, err := FetchAllPages(t.Context(), tekmetrictest.Logger(), fetcher, 10)
	if err != nil {
		t.Fatalf("FetchAllPages() error = %v", err)
	}
	if len(items) != 0 {
		t.Errorf("len(items) = %d, want 0", len(items))
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}
}

func TestFetchAllPagesReportsAFetchError(t *testing.T) {
	want := errors.New("upstream is down")
	fetcher := func(page int) (*tekmetric.PaginatedResponse[int], error) {
		if page == 1 {
			return nil, want
		}
		return &tekmetric.PaginatedResponse[int]{Content: []int{1}, Last: false}, nil
	}

	_, _, err := FetchAllPages(t.Context(), tekmetrictest.Logger(), fetcher, 10)
	if err == nil {
		t.Fatal("FetchAllPages() returned nil, want an error")
	}
	if !errors.Is(err, want) {
		t.Errorf("error = %v, want it to wrap %v", err, want)
	}
	if !strings.Contains(err.Error(), "page 1") {
		t.Errorf("error = %q, want it to name the page", err)
	}
}

// TestFetchAllPagesIgnoresCancellation records that the loop does not check the
// context between pages. Delete this test when the loop honors cancellation.
func TestFetchAllPagesIgnoresCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	items, _, err := FetchAllPages(ctx, tekmetrictest.Logger(), pageFetcher(30, 10), 10)
	if err != nil {
		t.Fatalf("FetchAllPages() error = %v; the loop now checks the context, so update this test", err)
	}
	if len(items) != 30 {
		t.Errorf("len(items) = %d, want 30", len(items))
	}
}

func TestAggregationError(t *testing.T) {
	underlying := errors.New("connection reset")
	err := &AggregationError{
		Stage:      "fetch",
		Underlying: underlying,
		Metadata:   AggregationMetadata{RecordsFetched: 12, PagesTraversed: 2},
	}

	if !errors.Is(err, underlying) {
		t.Error("errors.Is() did not match the underlying error")
	}

	message := err.Error()
	for _, want := range []string{"fetch", "connection reset", "12", "2"} {
		if !strings.Contains(message, want) {
			t.Errorf("error = %q, want it to contain %q", message, want)
		}
	}
}

// newVehicleTool wires the vehicle analysis tool to the mock API with the
// default limits.
func newVehicleTool(t *testing.T, api *tekmetrictest.API) *VehicleServiceAnalysis {
	t.Helper()
	return newVehicleToolWithLimits(t, api,
		config.AnalysisConfig{MaxPages: 50, MaxRecords: 5000, TimeoutSeconds: 120})
}

// newVehicleToolWithLimits wires the tool with the limits a test chooses.
func newVehicleToolWithLimits(t *testing.T, api *tekmetrictest.API, limits config.AnalysisConfig) *VehicleServiceAnalysis {
	t.Helper()

	cfg := &config.Config{Tekmetric: *api.Config(), Analysis: limits}
	return NewVehicleServiceAnalysis(api.AuthedClient(t), cfg, tekmetrictest.Logger())
}

// seedRepairOrders fills the mock with a vehicle and count repair orders.
func seedRepairOrders(api *tekmetrictest.API, count int) {
	api.Vehicles = []tekmetric.Vehicle{{ID: 20, Year: 2019, Make: "Ford", Model: "F-150"}}
	for i := 1; i <= count; i++ {
		api.RepairOrders = append(api.RepairOrders, tekmetric.RepairOrder{
			ID: i, ShopID: 1, VehicleID: 20, RepairOrderNumber: 1000 + i,
		})
	}
}

func TestBaseAnalysisToolReadsTheConfigLimits(t *testing.T) {
	api := tekmetrictest.New(t)
	tool := newVehicleToolWithLimits(t, api,
		config.AnalysisConfig{MaxPages: 7, MaxRecords: 300, TimeoutSeconds: 45})

	if got := tool.MaxPages(); got != 7 {
		t.Errorf("MaxPages() = %d, want 7", got)
	}
	if got := tool.MaxRecords(); got != 300 {
		t.Errorf("MaxRecords() = %d, want 300", got)
	}
	if got := tool.AnalysisTimeout(); got != 45*time.Second {
		t.Errorf("AnalysisTimeout() = %v, want 45s", got)
	}
}

// TestConfiguredPageCeilingIsEnforced confirms analysis.max_pages bounds the
// fetch. The tool must not read the whole history when the config says stop.
func TestConfiguredPageCeilingIsEnforced(t *testing.T) {
	api := tekmetrictest.New(t)
	seedRepairOrders(api, 500)

	tool := newVehicleToolWithLimits(t, api,
		config.AnalysisConfig{MaxPages: 2, MaxRecords: 5000, TimeoutSeconds: 120})

	result, err := tool.Execute(t.Context(), map[string]interface{}{"vehicle_id": float64(20)})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// The tool requests 100 records per page, so two pages hold 200 of the 500.
	if result.Metadata.PagesTraversed != 2 {
		t.Errorf("PagesTraversed = %d, want 2", result.Metadata.PagesTraversed)
	}
	if result.Metadata.RecordsFetched != 200 {
		t.Errorf("RecordsFetched = %d, want 200", result.Metadata.RecordsFetched)
	}
}

// TestCallerMayAskForFewerPages confirms a smaller max_pages argument wins.
func TestCallerMayAskForFewerPages(t *testing.T) {
	api := tekmetrictest.New(t)
	seedRepairOrders(api, 500)

	tool := newVehicleToolWithLimits(t, api,
		config.AnalysisConfig{MaxPages: 20, MaxRecords: 5000, TimeoutSeconds: 120})

	result, err := tool.Execute(t.Context(), map[string]interface{}{
		"vehicle_id": float64(20),
		"max_pages":  float64(3),
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Metadata.PagesTraversed != 3 {
		t.Errorf("PagesTraversed = %d, want 3", result.Metadata.PagesTraversed)
	}
}

// TestCallerCannotRaiseThePageCeiling confirms the config is the ceiling, not a
// default the caller can exceed.
func TestCallerCannotRaiseThePageCeiling(t *testing.T) {
	api := tekmetrictest.New(t)
	seedRepairOrders(api, 500)

	tool := newVehicleToolWithLimits(t, api,
		config.AnalysisConfig{MaxPages: 2, MaxRecords: 5000, TimeoutSeconds: 120})

	result, err := tool.Execute(t.Context(), map[string]interface{}{
		"vehicle_id": float64(20),
		"max_pages":  float64(1000),
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Metadata.PagesTraversed != 2 {
		t.Errorf("PagesTraversed = %d, want 2", result.Metadata.PagesTraversed)
	}
}

func TestPageCeilingIsAtLeastOne(t *testing.T) {
	api := tekmetrictest.New(t)
	seedRepairOrders(api, 500)

	tool := newVehicleToolWithLimits(t, api,
		config.AnalysisConfig{MaxPages: 5, MaxRecords: 5000, TimeoutSeconds: 120})

	result, err := tool.Execute(t.Context(), map[string]interface{}{
		"vehicle_id": float64(20),
		"max_pages":  float64(0),
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Metadata.PagesTraversed != 1 {
		t.Errorf("PagesTraversed = %d, want 1", result.Metadata.PagesTraversed)
	}
}

// TestConfiguredRecordLimitTruncates confirms analysis.max_records bounds the
// result the tool processes.
func TestConfiguredRecordLimitTruncates(t *testing.T) {
	api := tekmetrictest.New(t)
	seedRepairOrders(api, 250)

	tool := newVehicleToolWithLimits(t, api,
		config.AnalysisConfig{MaxPages: 50, MaxRecords: 40, TimeoutSeconds: 120})

	result, err := tool.Execute(t.Context(), map[string]interface{}{"vehicle_id": float64(20)})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Metadata.RecordsProcessed != 40 {
		t.Errorf("RecordsProcessed = %d, want 40", result.Metadata.RecordsProcessed)
	}
	if result.Metadata.RecordsFetched != 250 {
		t.Errorf("RecordsFetched = %d, want 250", result.Metadata.RecordsFetched)
	}
}

func TestVehicleServiceAnalysisDescribesItself(t *testing.T) {
	api := tekmetrictest.New(t)
	tool := newVehicleTool(t, api)

	if tool.Name() != "vehicle_service_analysis" {
		t.Errorf("Name() = %q, want vehicle_service_analysis", tool.Name())
	}
	if tool.Description() == "" {
		t.Error("Description() is empty")
	}

	schema := tool.Schema()
	props, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Schema() has no properties map")
	}
	if _, ok := props["vehicle_id"]; !ok {
		t.Error("Schema() does not declare vehicle_id")
	}

	required, ok := schema["required"].([]string)
	if !ok || len(required) == 0 || required[0] != "vehicle_id" {
		t.Errorf("Schema() required = %v, want vehicle_id", schema["required"])
	}
}

func TestVehicleServiceAnalysisNeedsAVehicleID(t *testing.T) {
	api := tekmetrictest.New(t)

	_, err := newVehicleTool(t, api).Execute(t.Context(), map[string]interface{}{})
	if err == nil {
		t.Fatal("Execute() returned nil, want an error")
	}
	if !strings.Contains(err.Error(), "vehicle_id") {
		t.Errorf("error = %q, want it to name vehicle_id", err)
	}
}

func TestVehicleServiceAnalysisReportsAMissingVehicle(t *testing.T) {
	api := tekmetrictest.New(t)

	_, err := newVehicleTool(t, api).Execute(t.Context(),
		map[string]interface{}{"vehicle_id": float64(404)})
	if err == nil {
		t.Fatal("Execute() returned nil, want an error")
	}

	var aggErr *AggregationError
	if !errors.As(err, &aggErr) {
		t.Fatalf("error = %T, want an AggregationError", err)
	}
	if aggErr.Stage != "fetch" {
		t.Errorf("Stage = %q, want fetch", aggErr.Stage)
	}
}

func TestVehicleServiceAnalysisBuildsATimeline(t *testing.T) {
	api := tekmetrictest.New(t)
	api.Vehicles = []tekmetric.Vehicle{{
		ID: 20, CustomerID: 10, Year: 2019, Make: "Ford", Model: "F-150",
	}}
	for i := 1; i <= 3; i++ {
		api.RepairOrders = append(api.RepairOrders, tekmetric.RepairOrder{
			ID: i, ShopID: 1, VehicleID: 20, RepairOrderNumber: 1000 + i,
		})
	}

	result, err := newVehicleTool(t, api).Execute(t.Context(),
		map[string]interface{}{"vehicle_id": float64(20)})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Metadata.RecordsFetched != 3 {
		t.Errorf("RecordsFetched = %d, want 3", result.Metadata.RecordsFetched)
	}
	if result.Summary == "" {
		t.Error("Summary is empty")
	}
	if result.Prompt == "" {
		t.Error("Prompt is empty")
	}
	if result.Data == nil {
		t.Error("Data is nil")
	}
}

func TestVehicleServiceAnalysisHandlesNoHistory(t *testing.T) {
	api := tekmetrictest.New(t)
	api.Vehicles = []tekmetric.Vehicle{{ID: 20, Year: 2019, Make: "Ford", Model: "F-150"}}

	result, err := newVehicleTool(t, api).Execute(t.Context(),
		map[string]interface{}{"vehicle_id": float64(20)})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Metadata.RecordsFetched != 0 {
		t.Errorf("RecordsFetched = %d, want 0", result.Metadata.RecordsFetched)
	}
}

func TestRegistryRegistersTools(t *testing.T) {
	api := tekmetrictest.New(t)
	cfg := &config.Config{Tekmetric: *api.Config()}

	registry := NewRegistry(api.AuthedClient(t), cfg, tekmetrictest.Logger())
	if len(registry.tools) != 0 {
		t.Fatalf("a new registry holds %d tools, want 0", len(registry.tools))
	}

	registry.Register(NewVehicleServiceAnalysis(api.AuthedClient(t), cfg, tekmetrictest.Logger()))
	if len(registry.tools) != 1 {
		t.Errorf("registry holds %d tools, want 1", len(registry.tools))
	}
}

func TestFormatResult(t *testing.T) {
	api := tekmetrictest.New(t)
	cfg := &config.Config{Tekmetric: *api.Config()}
	registry := NewRegistry(api.AuthedClient(t), cfg, tekmetrictest.Logger())

	result, err := registry.formatResult(&AnalysisResult{
		Summary: "a summary",
		Prompt:  "a prompt",
		Data:    map[string]any{"key": "value"},
		Metadata: AggregationMetadata{
			RecordsFetched: 5, RecordsProcessed: 5, PagesTraversed: 1, ExecutionTimeMs: 12,
		},
	})
	if err != nil {
		t.Fatalf("formatResult() error = %v", err)
	}

	var combined strings.Builder
	for _, content := range result.Content {
		combined.WriteString(fmt.Sprintf("%v", content))
	}
	text := combined.String()

	for _, want := range []string{"a summary", "a prompt", "Fetched 5 records", "key"} {
		if !strings.Contains(text, want) {
			t.Errorf("result omits %q:\n%s", want, text)
		}
	}
}
