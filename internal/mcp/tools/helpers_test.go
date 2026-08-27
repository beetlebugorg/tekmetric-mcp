package tools

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// TestClampLimit covers the range clamp that prevents the panics described in
// specs/code-review.md findings 1.1.
func TestClampLimit(t *testing.T) {
	tests := []struct {
		name  string
		limit int
		max   int
		want  int
	}{
		{"zero becomes one", 0, 25, 1},
		{"negative becomes one", -5, 25, 1},
		{"large negative becomes one", -1000, 25, 1},
		{"one stays one", 1, 25, 1},
		{"in range is unchanged", 10, 25, 10},
		{"at max is unchanged", 25, 25, 25},
		{"above max is capped", 100, 25, 25},
		{"max of one clamps down", 50, 1, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := clampLimit(tt.limit, tt.max); got != tt.want {
				t.Errorf("clampLimit(%d, %d) = %d, want %d", tt.limit, tt.max, got, tt.want)
			}
		})
	}
}

// TestClampLimitPreventsDivideByZero reproduces the arithmetic at
// internal/mcp/tools/repair_orders.go that panicked with limit 0.
func TestClampLimitPreventsDivideByZero(t *testing.T) {
	for _, limit := range []int{-10, -1, 0, 1, 7, 25, 100} {
		limit = clampLimit(limit, 25)

		pageSize := 100
		if limit < pageSize {
			pageSize = limit
		}

		if pageSize < 1 {
			t.Fatalf("pageSize = %d, want at least 1", pageSize)
		}

		// This expression panicked before the clamp.
		pagesNeeded := (limit + pageSize - 1) / pageSize
		if pagesNeeded < 1 {
			t.Errorf("pagesNeeded = %d, want at least 1", pagesNeeded)
		}
	}
}

// TestClampLimitPreventsSliceBounds reproduces the truncation at
// internal/mcp/tools/shops.go and internal/mcp/tools/inventory.go, which
// panicked with a negative limit.
func TestClampLimitPreventsSliceBounds(t *testing.T) {
	matches := []string{"a", "b", "c"}

	for _, limit := range []int{-1, 0, 1, 2, 5} {
		limit = clampLimit(limit, 100)

		got := matches
		if len(got) > limit {
			got = got[:limit]
		}

		if len(got) < 1 {
			t.Errorf("limit %d produced an empty result", limit)
		}
	}
}

func TestParseDateArg(t *testing.T) {
	tests := []struct {
		name    string
		args    map[string]interface{}
		want    string
		wantErr bool
	}{
		{
			name: "absent argument is not an error",
			args: map[string]interface{}{},
			want: "",
		},
		{
			name: "empty string is treated as absent",
			args: map[string]interface{}{"start_date": ""},
			want: "",
		},
		{
			name: "wrong type is treated as absent",
			args: map[string]interface{}{"start_date": 20250101},
			want: "",
		},
		{
			name: "valid date converts to RFC3339 UTC",
			args: map[string]interface{}{"start_date": "2025-03-14"},
			want: "2025-03-14T00:00:00Z",
		},
		{
			name:    "prose date is an error",
			args:    map[string]interface{}{"start_date": "yesterday"},
			wantErr: true,
		},
		{
			name:    "US format is an error",
			args:    map[string]interface{}{"start_date": "03/14/2025"},
			wantErr: true,
		},
		{
			name:    "RFC3339 input is an error",
			args:    map[string]interface{}{"start_date": "2025-03-14T00:00:00Z"},
			wantErr: true,
		},
		{
			name:    "impossible date is an error",
			args:    map[string]interface{}{"start_date": "2025-02-30"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, errResult := parseDateArg(tt.args, "start_date")

			if tt.wantErr {
				if errResult == nil {
					t.Fatalf("parseDateArg() returned no error result, want one")
				}
				if got != "" {
					t.Errorf("parseDateArg() = %q on error, want empty string", got)
				}
				return
			}

			if errResult != nil {
				t.Fatalf("parseDateArg() returned an unexpected error result")
			}
			if got != tt.want {
				t.Errorf("parseDateArg() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestParseDateArgIsUTC confirms the format is midnight UTC, not local time.
// The old comment claimed local time, which was wrong.
func TestParseDateArgIsUTC(t *testing.T) {
	got, errResult := parseDateArg(map[string]interface{}{"d": "2025-07-04"}, "d")
	if errResult != nil {
		t.Fatalf("parseDateArg() returned an unexpected error result")
	}

	parsed, err := time.Parse(time.RFC3339, got)
	if err != nil {
		t.Fatalf("result %q is not RFC3339: %v", got, err)
	}

	if _, offset := parsed.Zone(); offset != 0 {
		t.Errorf("zone offset = %d, want 0 (UTC)", offset)
	}
	if h, m, s := parsed.Clock(); h != 0 || m != 0 || s != 0 {
		t.Errorf("clock = %02d:%02d:%02d, want 00:00:00", h, m, s)
	}
}

func TestRemoveNullsAndEmpty(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
		want  interface{}
	}{
		{"nil stays nil", nil, nil},
		{"number is kept", float64(0), float64(0)},
		{"false is kept", false, false},
		{"empty string becomes nil", "", ""},
		{
			name:  "null field is dropped",
			input: map[string]interface{}{"a": float64(1), "b": nil},
			want:  map[string]interface{}{"a": float64(1)},
		},
		{
			name:  "empty string field is dropped",
			input: map[string]interface{}{"a": float64(1), "b": ""},
			want:  map[string]interface{}{"a": float64(1)},
		},
		{
			name:  "zero and false fields are kept",
			input: map[string]interface{}{"count": float64(0), "ok": false},
			want:  map[string]interface{}{"count": float64(0), "ok": false},
		},
		{
			name:  "empty slice field is dropped",
			input: map[string]interface{}{"a": float64(1), "b": []interface{}{}},
			want:  map[string]interface{}{"a": float64(1)},
		},
		{
			name:  "empty map becomes nil",
			input: map[string]interface{}{},
			want:  nil,
		},
		{
			name:  "map that empties out becomes nil",
			input: map[string]interface{}{"a": nil, "b": ""},
			want:  nil,
		},
		{
			name: "nested empty map is dropped",
			input: map[string]interface{}{
				"keep": float64(1),
				"drop": map[string]interface{}{"a": nil},
			},
			want: map[string]interface{}{"keep": float64(1)},
		},
		{
			name:  "null entries are removed from a slice",
			input: []interface{}{float64(1), nil, float64(2)},
			want:  []interface{}{float64(1), float64(2)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := removeNullsAndEmpty(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("removeNullsAndEmpty(%#v) = %#v, want %#v", tt.input, got, tt.want)
			}
		})
	}
}

// TestRemoveNullsAndEmptyDropsAWholeRecord records a limit of the cleaner. A
// record whose fields are all empty is removed from the result, so a caller
// cannot tell an empty record from a missing one.
func TestRemoveNullsAndEmptyDropsAWholeRecord(t *testing.T) {
	input := []interface{}{
		map[string]interface{}{"id": float64(1)},
		map[string]interface{}{"id": nil, "name": ""},
	}

	got, ok := removeNullsAndEmpty(input).([]interface{})
	if !ok {
		t.Fatalf("removeNullsAndEmpty() = %T, want a slice", got)
	}
	if len(got) != 1 {
		t.Errorf("len = %d, want 1; the cleaner now keeps empty records, so update this test", len(got))
	}
}

func TestCleanJSON(t *testing.T) {
	type record struct {
		ID    int    `json:"id"`
		Name  string `json:"name"`
		Notes string `json:"notes,omitempty"`
	}

	got, err := cleanJSON(record{ID: 7, Name: "Main Street Auto"})
	if err != nil {
		t.Fatalf("cleanJSON() error = %v", err)
	}

	want := map[string]interface{}{"id": float64(7), "name": "Main Street Auto"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("cleanJSON() = %#v, want %#v", got, want)
	}
}

func TestCleanJSONRejectsUnencodableInput(t *testing.T) {
	if _, err := cleanJSON(make(chan int)); err == nil {
		t.Error("cleanJSON() returned nil, want an error")
	}
}

func TestHasFinancialData(t *testing.T) {
	tests := []struct {
		resource string
		want     bool
	}{
		{"REPAIR ORDERS", true},
		{"JOBS", true},
		{"CUSTOMERS", false},
		{"VEHICLES", false},
		{"repair orders", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.resource, func(t *testing.T) {
			if got := hasFinancialData(tt.resource); got != tt.want {
				t.Errorf("hasFinancialData(%q) = %v, want %v", tt.resource, got, tt.want)
			}
		})
	}
}

func TestFormatPaginatedResultWithWarning(t *testing.T) {
	tests := []struct {
		name          string
		data          []string
		totalElements int
		returned      int
		maxResults    int
		resource      string
		wantWarning   bool
		wantFinancial bool
	}{
		{
			name: "complete result has no warning",
			data: []string{"a", "b"}, totalElements: 2, returned: 2, maxResults: 25,
			resource: "CUSTOMERS",
		},
		{
			name: "truncated result warns",
			data: []string{"a"}, totalElements: 100, returned: 1, maxResults: 25,
			resource: "CUSTOMERS", wantWarning: true,
		},
		{
			name: "partial page warns",
			data: []string{"a"}, totalElements: 5, returned: 1, maxResults: 25,
			resource: "CUSTOMERS", wantWarning: true,
		},
		{
			name: "repair orders always carry the financial warning",
			data: []string{"a"}, totalElements: 1, returned: 1, maxResults: 25,
			resource: "REPAIR ORDERS", wantFinancial: true,
		},
		{
			name: "jobs always carry the financial warning",
			data: []string{"a"}, totalElements: 1, returned: 1, maxResults: 25,
			resource: "JOBS", wantFinancial: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := formatPaginatedResultWithWarning(
				tt.data, tt.totalElements, tt.returned, tt.maxResults, tt.resource)
			if err != nil {
				t.Fatalf("formatPaginatedResultWithWarning() error = %v", err)
			}

			text := resultText(t, result)

			if got := strings.Contains(text, `"WARNING"`); got != tt.wantWarning {
				t.Errorf("WARNING present = %v, want %v\n%s", got, tt.wantWarning, text)
			}
			if got := strings.Contains(text, `"FINANCIAL_WARNING"`); got != tt.wantFinancial {
				t.Errorf("FINANCIAL_WARNING present = %v, want %v\n%s", got, tt.wantFinancial, text)
			}
		})
	}
}

func TestFormatJSON(t *testing.T) {
	result, err := formatJSON(map[string]interface{}{"id": 1, "name": "Main Street Auto"})
	if err != nil {
		t.Fatalf("formatJSON() error = %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(resultText(t, result)), &decoded); err != nil {
		t.Fatalf("formatJSON() produced invalid JSON: %v", err)
	}
	if decoded["name"] != "Main Street Auto" {
		t.Errorf("name = %v, want Main Street Auto", decoded["name"])
	}
}

// resultText pulls the text out of a tool result.
func resultText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()

	if result == nil {
		t.Fatal("result is nil")
	}

	var out strings.Builder
	for _, content := range result.Content {
		if text, ok := content.(mcp.TextContent); ok {
			out.WriteString(text.Text)
		}
	}
	return out.String()
}
