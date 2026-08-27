package tekmetric_test

import (
	"context"
	"testing"

	"github.com/beetlebugorg/tekmetric-mcp/pkg/tekmetric"
	"github.com/beetlebugorg/tekmetric-mcp/pkg/tekmetric/tekmetrictest"
)

// The tests in this file record the exact query string each API method builds.
//
// The query strings are built by hand today. A later change replaces that code
// with a struct tag encoder. These tests are the guard on that change: a
// difference in the output is a change in behavior, not a formatting detail.
//
// Two details matter and are easy to lose:
//   - A repeated key carries a list. The API reads repairOrderStatusId and
//     partNumbers as repeated keys, not as one comma separated value.
//   - The methods that take a params struct sort their keys, because they use
//     url.Values.Encode. The methods that take positional arguments do not,
//     because they format the path directly.

// queryFor runs fn against the mock API and returns the query string it sent.
func queryFor(t *testing.T, fn func(context.Context, *tekmetric.Client)) string {
	t.Helper()

	api := tekmetrictest.New(t)
	client := api.AuthedClient(t)

	fn(t.Context(), client)
	return api.LastRequest(t).Query
}

func TestQueryStringsForParamsMethods(t *testing.T) {
	tests := []struct {
		name string
		call func(context.Context, *tekmetric.Client)
		want string
	}{
		{
			name: "customers",
			call: func(ctx context.Context, c *tekmetric.Client) {
				_, _ = c.GetCustomersWithParams(ctx, tekmetric.CustomerQueryParams{
					Shop: 1, Page: 2, Size: 50,
					Search:           "smith jones",
					CustomerTypeID:   2,
					UpdatedDateStart: "2025-01-01T00:00:00Z",
					UpdatedDateEnd:   "2025-02-01T00:00:00Z",
					Sort:             "lastName,firstName",
					SortDirection:    "asc",
				})
			},
			want: "customerTypeId=2&page=2&search=smith+jones&shop=1&size=50" +
				"&sort=lastName%2CfirstName&sortDirection=ASC" +
				"&updatedDateEnd=2025-02-01T00%3A00%3A00Z" +
				"&updatedDateStart=2025-01-01T00%3A00%3A00Z",
		},
		{
			name: "repair orders",
			call: func(ctx context.Context, c *tekmetric.Client) {
				_, _ = c.GetRepairOrdersWithParams(ctx, tekmetric.RepairOrderQueryParams{
					Shop: 1, Page: 2, Size: 25,
					Start:                "2025-01-01T00:00:00Z",
					End:                  "2025-02-01T00:00:00Z",
					RepairOrderNumber:    4242,
					RepairOrderStatusIds: []int{1, 2, 3},
					CustomerID:           7,
					VehicleID:            9,
					Search:               "ford f-150",
					Sort:                 "createdDate",
					SortDirection:        "desc",
				})
			},
			want: "customerId=7&end=2025-02-01T00%3A00%3A00Z&page=2&repairOrderNumber=4242" +
				"&repairOrderStatusId=1&repairOrderStatusId=2&repairOrderStatusId=3" +
				"&search=ford+f-150&shop=1&size=25&sort=createdDate&sortDirection=DESC" +
				"&start=2025-01-01T00%3A00%3A00Z&vehicleId=9",
		},
		{
			name: "vehicles",
			call: func(ctx context.Context, c *tekmetric.Client) {
				_, _ = c.GetVehiclesWithParams(ctx, tekmetric.VehicleQueryParams{
					Shop: 1, Page: 2, Size: 50,
					Search: "vin123", CustomerID: 7, SortDirection: "asc",
				})
			},
			want: "customerId=7&page=2&search=vin123&shop=1&size=50&sortDirection=ASC",
		},
		{
			name: "appointments",
			call: func(ctx context.Context, c *tekmetric.Client) {
				_, _ = c.GetAppointmentsWithParams(ctx, tekmetric.AppointmentQueryParams{
					Shop: 1, Page: 2, Size: 50,
					CustomerID: 7, VehicleID: 9,
					Start:         "2025-01-01T00:00:00Z",
					End:           "2025-02-01T00:00:00Z",
					SortDirection: "asc",
				})
			},
			// includeDeleted is added by the method. It is not a struct field.
			want: "customerId=7&end=2025-02-01T00%3A00%3A00Z&includeDeleted=false&page=2" +
				"&shop=1&size=50&sortDirection=ASC&start=2025-01-01T00%3A00%3A00Z&vehicleId=9",
		},
		{
			name: "jobs",
			call: func(ctx context.Context, c *tekmetric.Client) {
				_, _ = c.GetJobsWithParams(ctx, tekmetric.JobQueryParams{
					Shop: 1, Page: 2, Size: 50,
					VehicleID: 9, RepairOrderID: 3, CustomerID: 7,
					RepairOrderStatusIds: []int{1, 2},
					Sort:                 "authorizedDate",
					SortDirection:        "desc",
				})
			},
			want: "customerId=7&page=2&repairOrderId=3&repairOrderStatusId=1" +
				"&repairOrderStatusId=2&shop=1&size=50&sort=authorizedDate" +
				"&sortDirection=DESC&vehicleId=9",
		},
		{
			name: "employees",
			call: func(ctx context.Context, c *tekmetric.Client) {
				_, _ = c.GetEmployeesWithParams(ctx, tekmetric.EmployeeQueryParams{
					Shop: 1, Page: 2, Size: 50, SortDirection: "asc",
				})
			},
			want: "page=2&shop=1&size=50&sortDirection=ASC",
		},
		{
			name: "inventory",
			call: func(ctx context.Context, c *tekmetric.Client) {
				_, _ = c.GetInventoryWithParams(ctx, tekmetric.InventoryQueryParams{
					Shop: 1, PartTypeID: 2, Page: 2, Size: 50,
					PartNumbers:   []string{"AB-1", "CD-2"},
					TireSize:      "225/65R17",
					Sort:          "name,brand",
					SortDirection: "asc",
				})
			},
			want: "page=2&partNumbers=AB-1&partNumbers=CD-2&partTypeId=2&shop=1&size=50" +
				"&sort=name%2Cbrand&sortDirection=ASC&tireSize=225%2F65R17",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := queryFor(t, tt.call); got != tt.want {
				t.Errorf("query =\n  %s\nwant\n  %s", got, tt.want)
			}
		})
	}
}

// TestQueryStringsForPositionalMethods covers the methods that format the path
// directly. Their key order follows the code, not the alphabet.
func TestQueryStringsForPositionalMethods(t *testing.T) {
	tests := []struct {
		name string
		call func(context.Context, *tekmetric.Client)
		want string
	}{
		{
			name: "GetCustomers",
			call: func(ctx context.Context, c *tekmetric.Client) { _, _ = c.GetCustomers(ctx, 1, 0, 10) },
			want: "shop=1&page=0&size=10",
		},
		{
			name: "SearchCustomers escapes the query",
			call: func(ctx context.Context, c *tekmetric.Client) { _, _ = c.SearchCustomers(ctx, 1, "a b&c", 0, 10) },
			want: "shop=1&search=a+b%26c&page=0&size=10",
		},
		{
			name: "GetVehicles",
			call: func(ctx context.Context, c *tekmetric.Client) { _, _ = c.GetVehicles(ctx, 1, 0, 10) },
			want: "shop=1&page=0&size=10",
		},
		{
			name: "GetEmployees",
			call: func(ctx context.Context, c *tekmetric.Client) { _, _ = c.GetEmployees(ctx, 1, 0, 10) },
			want: "shop=1&page=0&size=10",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := queryFor(t, tt.call); got != tt.want {
				t.Errorf("query = %s, want %s", got, tt.want)
			}
		})
	}
}

// TestDefaultPageSize covers the fallback the params methods apply when Size is
// zero.
func TestDefaultPageSize(t *testing.T) {
	got := queryFor(t, func(ctx context.Context, c *tekmetric.Client) {
		_, _ = c.GetCustomersWithParams(ctx, tekmetric.CustomerQueryParams{Shop: 1})
	})

	if got != "page=0&shop=1&size=100" {
		t.Errorf("query = %s, want page=0&shop=1&size=100", got)
	}
}
