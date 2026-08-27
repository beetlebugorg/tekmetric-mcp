package tekmetric

import (
	"strings"
	"testing"
)

// TestSortDirectionIsValidatedEverywhere covers the direction check that every
// Validate method repeats.
func TestSortDirectionIsValidatedEverywhere(t *testing.T) {
	directions := []struct {
		value   string
		wantErr bool
	}{
		{"", false},
		{"ASC", false},
		{"DESC", false},
		{"asc", false},
		{"desc", false},
		{"Asc", false},
		{"ascending", true},
		{"UP", true},
		{"1", true},
	}

	validators := map[string]func(string) error{
		"RepairOrderQueryParams": func(d string) error {
			return (&RepairOrderQueryParams{SortDirection: d}).Validate()
		},
		"CustomerQueryParams": func(d string) error {
			return (&CustomerQueryParams{SortDirection: d}).Validate()
		},
		"VehicleQueryParams": func(d string) error {
			return (&VehicleQueryParams{SortDirection: d}).Validate()
		},
		"AppointmentQueryParams": func(d string) error {
			return (&AppointmentQueryParams{SortDirection: d}).Validate()
		},
		"JobQueryParams": func(d string) error {
			return (&JobQueryParams{SortDirection: d}).Validate()
		},
		"EmployeeQueryParams": func(d string) error {
			return (&EmployeeQueryParams{SortDirection: d}).Validate()
		},
		"InventoryQueryParams": func(d string) error {
			return (&InventoryQueryParams{Shop: 1, PartTypeID: 1, SortDirection: d}).Validate()
		},
	}

	for name, validate := range validators {
		for _, d := range directions {
			t.Run(name+"/"+d.value, func(t *testing.T) {
				err := validate(d.value)
				if d.wantErr && err == nil {
					t.Errorf("Validate() with %q returned nil, want an error", d.value)
				}
				if !d.wantErr && err != nil {
					t.Errorf("Validate() with %q error = %v, want nil", d.value, err)
				}
			})
		}
	}
}

// TestSortDirectionIsNormalized covers the upper case rewrite that every
// Validate method applies to a valid direction.
func TestSortDirectionIsNormalized(t *testing.T) {
	t.Run("RepairOrderQueryParams", func(t *testing.T) {
		p := &RepairOrderQueryParams{SortDirection: "asc"}
		if err := p.Validate(); err != nil {
			t.Fatalf("Validate() error = %v", err)
		}
		if p.SortDirection != "ASC" {
			t.Errorf("SortDirection = %q, want ASC", p.SortDirection)
		}
	})

	t.Run("CustomerQueryParams", func(t *testing.T) {
		p := &CustomerQueryParams{SortDirection: "desc"}
		if err := p.Validate(); err != nil {
			t.Fatalf("Validate() error = %v", err)
		}
		if p.SortDirection != "DESC" {
			t.Errorf("SortDirection = %q, want DESC", p.SortDirection)
		}
	})

	t.Run("InventoryQueryParams", func(t *testing.T) {
		p := &InventoryQueryParams{Shop: 1, PartTypeID: 1, SortDirection: "asc"}
		if err := p.Validate(); err != nil {
			t.Fatalf("Validate() error = %v", err)
		}
		if p.SortDirection != "ASC" {
			t.Errorf("SortDirection = %q, want ASC", p.SortDirection)
		}
	})
}

func TestRepairOrderQueryParamsSortField(t *testing.T) {
	tests := []struct {
		sort    string
		wantErr bool
	}{
		{"", false},
		{"createdDate", false},
		{"repairOrderNumber", false},
		{"customer.firstName", false},
		{"customer.lastName", false},
		{"createddate", true},
		{"updatedDate", true},
		{"createdDate,repairOrderNumber", true},
	}

	for _, tt := range tests {
		t.Run(tt.sort, func(t *testing.T) {
			err := (&RepairOrderQueryParams{Sort: tt.sort}).Validate()
			if tt.wantErr && err == nil {
				t.Errorf("Validate() with %q returned nil, want an error", tt.sort)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Validate() with %q error = %v, want nil", tt.sort, err)
			}
		})
	}
}

func TestRepairOrderQueryParamsStatusIDs(t *testing.T) {
	tests := []struct {
		name    string
		ids     []int
		wantErr bool
	}{
		{"empty", nil, false},
		{"all valid", []int{1, 2, 3, 4, 5, 6, 7}, false},
		{"single valid", []int{3}, false},
		{"zero is rejected", []int{0}, true},
		{"eight is rejected", []int{8}, true},
		{"negative is rejected", []int{-1}, true},
		{"one bad value in a valid set", []int{1, 2, 99}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := (&RepairOrderQueryParams{RepairOrderStatusIds: tt.ids}).Validate()
			if tt.wantErr && err == nil {
				t.Errorf("Validate() with %v returned nil, want an error", tt.ids)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Validate() with %v error = %v, want nil", tt.ids, err)
			}
		})
	}
}

// TestJobQueryParamsRejectsDeletedStatus covers the difference between jobs and
// repair orders. Jobs stop at status 6.
func TestJobQueryParamsRejectsDeletedStatus(t *testing.T) {
	if err := (&JobQueryParams{RepairOrderStatusIds: []int{6}}).Validate(); err != nil {
		t.Errorf("Validate() with status 6 error = %v, want nil", err)
	}

	err := (&JobQueryParams{RepairOrderStatusIds: []int{7}}).Validate()
	if err == nil {
		t.Fatal("Validate() with status 7 returned nil, want an error")
	}
	if !strings.Contains(err.Error(), "1-6") {
		t.Errorf("Validate() error = %q, want it to mention the range 1-6", err)
	}
}

func TestJobQueryParamsSortField(t *testing.T) {
	tests := []struct {
		sort    string
		wantErr bool
	}{
		{"", false},
		{"authorizedDate", false},
		{"createdDate", true},
		{"AUTHORIZEDDATE", true},
	}

	for _, tt := range tests {
		t.Run(tt.sort, func(t *testing.T) {
			err := (&JobQueryParams{Sort: tt.sort}).Validate()
			if tt.wantErr && err == nil {
				t.Errorf("Validate() with %q returned nil, want an error", tt.sort)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Validate() with %q error = %v, want nil", tt.sort, err)
			}
		})
	}
}

func TestCustomerQueryParamsCustomerType(t *testing.T) {
	tests := []struct {
		id      int
		wantErr bool
	}{
		{0, false},
		{1, false},
		{2, false},
		{3, true},
		{-1, true},
	}

	for _, tt := range tests {
		t.Run(string(rune('0'+tt.id)), func(t *testing.T) {
			err := (&CustomerQueryParams{CustomerTypeID: tt.id}).Validate()
			if tt.wantErr && err == nil {
				t.Errorf("Validate() with %d returned nil, want an error", tt.id)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Validate() with %d error = %v, want nil", tt.id, err)
			}
		})
	}
}

func TestCustomerQueryParamsSortList(t *testing.T) {
	tests := []struct {
		sort    string
		wantErr bool
	}{
		{"", false},
		{"lastName", false},
		{"firstName,lastName", false},
		{"lastName, firstName", false},
		{"email", false},
		{"phone", true},
		{"lastName,phone", true},
	}

	for _, tt := range tests {
		t.Run(tt.sort, func(t *testing.T) {
			err := (&CustomerQueryParams{Sort: tt.sort}).Validate()
			if tt.wantErr && err == nil {
				t.Errorf("Validate() with %q returned nil, want an error", tt.sort)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Validate() with %q error = %v, want nil", tt.sort, err)
			}
		})
	}
}

func TestInventoryQueryParamsRequiredFields(t *testing.T) {
	tests := []struct {
		name    string
		params  InventoryQueryParams
		wantMsg string
	}{
		{"missing shop", InventoryQueryParams{PartTypeID: 1}, "shop"},
		{"missing part type", InventoryQueryParams{Shop: 1}, "partTypeId"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.params.Validate()
			if err == nil {
				t.Fatal("Validate() returned nil, want an error")
			}
			if !strings.Contains(err.Error(), tt.wantMsg) {
				t.Errorf("Validate() error = %q, want it to mention %q", err, tt.wantMsg)
			}
		})
	}
}

func TestInventoryQueryParamsPartType(t *testing.T) {
	tests := []struct {
		id      int
		wantErr bool
	}{
		{1, false},
		{2, false},
		{5, false},
		{3, true},
		{4, true},
		{6, true},
	}

	for _, tt := range tests {
		t.Run(string(rune('0'+tt.id)), func(t *testing.T) {
			err := (&InventoryQueryParams{Shop: 1, PartTypeID: tt.id}).Validate()
			if tt.wantErr && err == nil {
				t.Errorf("Validate() with %d returned nil, want an error", tt.id)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Validate() with %d error = %v, want nil", tt.id, err)
			}
		})
	}
}

func TestInventoryQueryParamsSortList(t *testing.T) {
	tests := []struct {
		sort    string
		wantErr bool
	}{
		{"", false},
		{"id", false},
		{"name,brand", false},
		{"partNumber", false},
		{"price", true},
	}

	for _, tt := range tests {
		t.Run(tt.sort, func(t *testing.T) {
			err := (&InventoryQueryParams{Shop: 1, PartTypeID: 1, Sort: tt.sort}).Validate()
			if tt.wantErr && err == nil {
				t.Errorf("Validate() with %q returned nil, want an error", tt.sort)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Validate() with %q error = %v, want nil", tt.sort, err)
			}
		})
	}
}

// TestSortFieldIsNotValidated records the types that accept any sort field and
// leave the rejection to the API.
func TestSortFieldIsNotValidated(t *testing.T) {
	if err := (&VehicleQueryParams{Sort: "nonsense"}).Validate(); err != nil {
		t.Errorf("VehicleQueryParams.Validate() error = %v, want nil", err)
	}
	if err := (&AppointmentQueryParams{Sort: "nonsense"}).Validate(); err != nil {
		t.Errorf("AppointmentQueryParams.Validate() error = %v, want nil", err)
	}
	if err := (&EmployeeQueryParams{Sort: "nonsense"}).Validate(); err != nil {
		t.Errorf("EmployeeQueryParams.Validate() error = %v, want nil", err)
	}
}
