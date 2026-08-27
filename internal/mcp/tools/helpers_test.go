package tools

import (
	"testing"
	"time"
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
