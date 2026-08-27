package tekmetric

import (
	"encoding/json"
	"testing"
)

func TestCurrencyMarshalJSON(t *testing.T) {
	tests := []struct {
		name  string
		cents Currency
		want  string
	}{
		{"zero", 0, "0"},
		{"one cent", 1, "0.01"},
		{"ten cents", 10, "0.1"},
		{"one dollar", 100, "1"},
		{"dollars and cents", 1234, "12.34"},
		{"large amount", 123456789, "1234567.89"},
		{"negative", -500, "-5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.cents)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("Marshal(%d) = %s, want %s", int(tt.cents), got, tt.want)
			}
		})
	}
}

func TestCurrencyUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Currency
		wantErr bool
	}{
		{"zero", "0", 0, false},
		{"cents", "1234", 1234, false},
		{"negative", "-500", -500, false},
		{"null leaves the zero value", "null", 0, false},
		{"fractional input is an error", "12.5", 0, true},
		{"string input is an error", `"1234"`, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got Currency
			err := json.Unmarshal([]byte(tt.input), &got)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("Unmarshal(%s) returned no error, want one", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("Unmarshal(%s) error = %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("Unmarshal(%s) = %d, want %d", tt.input, int(got), int(tt.want))
			}
		})
	}
}

// TestCurrencyIsNotSymmetric records that Currency marshals to dollars but
// unmarshals from cents. A value that passes through both is not preserved.
// Change this test when the type gains a symmetric representation.
func TestCurrencyIsNotSymmetric(t *testing.T) {
	original := Currency(1234)

	encoded, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if string(encoded) != "12.34" {
		t.Fatalf("Marshal() = %s, want 12.34", encoded)
	}

	var decoded Currency
	if err := json.Unmarshal(encoded, &decoded); err == nil {
		t.Errorf("Unmarshal(%s) succeeded and gave %d; the encoding is now symmetric, so update this test",
			encoded, int(decoded))
	}
}

func TestCurrencyInStruct(t *testing.T) {
	type payload struct {
		Total Currency `json:"total"`
	}

	var got payload
	if err := json.Unmarshal([]byte(`{"total": 9999}`), &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if got.Total != 9999 {
		t.Errorf("Total = %d, want 9999", int(got.Total))
	}

	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if string(encoded) != `{"total":99.99}` {
		t.Errorf("Marshal() = %s, want {\"total\":99.99}", encoded)
	}
}
