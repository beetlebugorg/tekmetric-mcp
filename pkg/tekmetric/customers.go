package tekmetric

import (
	"context"
	"fmt"
	"net/url"
	"time"
)

// ============================================================================
// Models
// ============================================================================

// CustomerType represents the type of customer
type CustomerType struct {
	ID   int    `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

// Customer represents a Tekmetric customer
type Customer struct {
	ID                            int           `json:"id"`
	FirstName                     string        `json:"firstName"`
	LastName                      string        `json:"lastName"`
	Email                         string        `json:"email"`
	Phone                         []Phone       `json:"phone"`
	CustomerType                  *CustomerType `json:"customerType,omitempty"`
	ContactFirstName              *string       `json:"contactFirstName"`
	ContactLastName               *string       `json:"contactLastName"`
	Address                       *Address      `json:"address"`
	ShopID                        int           `json:"shopId"`
	EligibleForAccountsReceivable bool          `json:"eligibleForAccountsReceivable"`
	CreditLimit                   float64       `json:"creditLimit"`
	OkForMarketing                bool          `json:"okForMarketing"`
	Notes                         string        `json:"notes,omitempty"`
	CreatedDate                   time.Time     `json:"createdDate"`
	UpdatedDate                   time.Time     `json:"updatedDate"`
	DeletedDate                   *time.Time    `json:"deletedDate"`
}

// ============================================================================
// API Methods
// ============================================================================

// CustomerQueryParams holds query parameters for customer searches
// Note: By default, deleted records are excluded unless DeletedDateStart/DeletedDateEnd are explicitly set
type CustomerQueryParams struct {
	Shop                          int    `url:"shop,omitempty"`
	Page                          int    `url:"page"`
	Size                          int    `url:"size"`
	Search                        string `url:"search,omitempty"`                        // Search by name, email, phone
	EligibleForAccountsReceivable *bool  `url:"eligibleForAccountsReceivable,omitempty"` // Filter by AR eligibility
	OkForMarketing                *bool  `url:"okForMarketing,omitempty"`                // Filter by marketing permission
	UpdatedDateStart              string `url:"updatedDateStart,omitempty"`              // Filter by updated date
	UpdatedDateEnd                string `url:"updatedDateEnd,omitempty"`                // Filter by updated date
	CustomerTypeID                int    `url:"customerTypeId,omitempty"`                // 1=Customer, 2=Business
	Sort                          string `url:"sort,omitempty"`                          // lastName, firstName, email (can be comma-separated)
	SortDirection                 string `url:"sortDirection,omitempty"`                 // ASC, DESC
}

// GetCustomers returns a paginated list of customers
func (c *Client) GetCustomers(ctx context.Context, shopID int, page int, size int) (*PaginatedResponse[Customer], error) {
	if err := c.authorizeShop(ctx, shopID); err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/api/v1/customers?shop=%d&page=%d&size=%d", shopID, page, size)
	var resp PaginatedResponse[Customer]
	if err := c.doRequest(ctx, "GET", path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// SearchCustomers searches customers using the API's native search
func (c *Client) SearchCustomers(ctx context.Context, shopID int, query string, page int, size int) (*PaginatedResponse[Customer], error) {
	if err := c.authorizeShop(ctx, shopID); err != nil {
		return nil, err
	}
	query = url.QueryEscape(query)
	path := fmt.Sprintf("/api/v1/customers?shop=%d&search=%s&page=%d&size=%d", shopID, query, page, size)
	var resp PaginatedResponse[Customer]
	if err := c.doRequest(ctx, "GET", path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetCustomer returns a specific customer by ID
func (c *Client) GetCustomer(ctx context.Context, id int) (*Customer, error) {
	var customer Customer
	path := fmt.Sprintf("/api/v1/customers/%d", id)
	if err := c.doRequest(ctx, "GET", path, nil, &customer); err != nil {
		return nil, err
	}
	return &customer, nil
}

// GetCustomersWithParams returns customers with advanced filtering
func (c *Client) GetCustomersWithParams(ctx context.Context, params CustomerQueryParams) (*PaginatedResponse[Customer], error) {
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

	path := "/api/v1/customers?" + encoded
	var resp PaginatedResponse[Customer]
	if err := c.doRequest(ctx, "GET", path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
