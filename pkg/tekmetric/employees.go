package tekmetric

import (
	"context"
	"fmt"
	"time"
)

// ============================================================================
// Models
// ============================================================================

// EmployeeRole represents an employee's role
type EmployeeRole struct {
	ID   int    `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

// Employee represents an employee
type Employee struct {
	ID                  int           `json:"id"`
	FirstName           string        `json:"firstName"`
	LastName            string        `json:"lastName"`
	Email               string        `json:"email"`
	Phone               string        `json:"phone,omitempty"`
	EmployeeRole        *EmployeeRole `json:"employeeRole,omitempty"`
	CanPerformWork      bool          `json:"canPerformWork"`
	CertificationNumber string        `json:"certificationNumber,omitempty"`
	ShopID              int           `json:"shopId"`
	CreatedDate         time.Time     `json:"createdDate"`
	UpdatedDate         time.Time     `json:"updatedDate"`
	DeletedDate         *time.Time    `json:"deletedDate"`
}

// ============================================================================
// API Methods
// ============================================================================

// EmployeeQueryParams holds query parameters for employee searches
type EmployeeQueryParams struct {
	Shop             int    `url:"shop,omitempty"`
	Page             int    `url:"page"`
	Size             int    `url:"size"`
	Search           string `url:"search,omitempty"`           // Search by name
	UpdatedDateStart string `url:"updatedDateStart,omitempty"` // Filter by updated date
	UpdatedDateEnd   string `url:"updatedDateEnd,omitempty"`   // Filter by updated date
	Sort             string `url:"sort,omitempty"`             // Sort field (API docs don't specify allowed values)
	SortDirection    string `url:"sortDirection,omitempty"`    // ASC, DESC
}

// GetEmployees returns a paginated list of employees
func (c *Client) GetEmployees(ctx context.Context, shopID int, page int, size int) (*PaginatedResponse[Employee], error) {
	if err := c.authorizeShop(ctx, shopID); err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/api/v1/employees?shop=%d&page=%d&size=%d", shopID, page, size)
	var resp PaginatedResponse[Employee]
	if err := c.doRequest(ctx, "GET", path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetEmployee returns a specific employee by ID
func (c *Client) GetEmployee(ctx context.Context, id int) (*Employee, error) {
	var employee Employee
	path := fmt.Sprintf("/api/v1/employees/%d", id)
	if err := c.doRequest(ctx, "GET", path, nil, &employee); err != nil {
		return nil, err
	}
	return &employee, nil
}

// GetEmployeesWithParams returns employees with advanced filtering
func (c *Client) GetEmployeesWithParams(ctx context.Context, params EmployeeQueryParams) (*PaginatedResponse[Employee], error) {
	if err := c.authorizeShop(ctx, params.Shop); err != nil {
		return nil, err
	}
	if err := params.Validate(); err != nil {
		return nil, err
	}
	encoded, err := encodeQuery(params)
	if err != nil {
		return nil, err
	}

	path := "/api/v1/employees?" + encoded
	var resp PaginatedResponse[Employee]
	if err := c.doRequest(ctx, "GET", path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
