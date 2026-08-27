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

// Vehicle represents a vehicle
type Vehicle struct {
	ID             int        `json:"id"`
	CustomerID     int        `json:"customerId"`
	ShopID         int        `json:"shopId"`
	Year           int        `json:"year"`
	Make           string     `json:"make"`
	Model          string     `json:"model"`
	SubModel       string     `json:"subModel,omitempty"`
	VIN            string     `json:"vin"`
	LicensePlate   string     `json:"licensePlate,omitempty"`
	Color          string     `json:"color,omitempty"`
	UnitNumber     string     `json:"unitNumber,omitempty"`
	ProductionDate *string    `json:"productionDate,omitempty"`
	Mileage        float64    `json:"mileage"`
	Engine         string     `json:"engine,omitempty"`
	Transmission   string     `json:"transmission,omitempty"`
	DriveType      string     `json:"driveType,omitempty"`
	Notes          string     `json:"notes,omitempty"`
	CreatedDate    time.Time  `json:"createdDate"`
	UpdatedDate    time.Time  `json:"updatedDate"`
	DeletedDate    *time.Time `json:"deletedDate"`
}

// ============================================================================
// API Methods
// ============================================================================

// VehicleQueryParams holds query parameters for vehicle searches
type VehicleQueryParams struct {
	Shop             int    `url:"shop,omitempty"`
	Page             int    `url:"page"`
	Size             int    `url:"size"`
	CustomerID       int    `url:"customerId,omitempty"`       // Filter by customer
	Search           string `url:"search,omitempty"`           // Search by year, make, model
	UpdatedDateStart string `url:"updatedDateStart,omitempty"` // Filter by updated date
	UpdatedDateEnd   string `url:"updatedDateEnd,omitempty"`   // Filter by updated date
	Sort             string `url:"sort,omitempty"`             // Sort field (API docs don't specify allowed values)
	SortDirection    string `url:"sortDirection,omitempty"`    // ASC, DESC
}

// GetVehicles returns a paginated list of vehicles
func (c *Client) GetVehicles(ctx context.Context, shopID int, page int, size int) (*PaginatedResponse[Vehicle], error) {
	if err := c.authorizeShop(ctx, shopID); err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/api/v1/vehicles?shop=%d&page=%d&size=%d", shopID, page, size)
	var resp PaginatedResponse[Vehicle]
	if err := c.doRequest(ctx, "GET", path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// SearchVehicles searches vehicles using the API's native search
func (c *Client) SearchVehicles(ctx context.Context, shopID int, query string, page int, size int) (*PaginatedResponse[Vehicle], error) {
	if err := c.authorizeShop(ctx, shopID); err != nil {
		return nil, err
	}
	query = url.QueryEscape(query)
	path := fmt.Sprintf("/api/v1/vehicles?shop=%d&search=%s&page=%d&size=%d", shopID, query, page, size)
	var resp PaginatedResponse[Vehicle]
	if err := c.doRequest(ctx, "GET", path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetVehicle returns a specific vehicle by ID
func (c *Client) GetVehicle(ctx context.Context, id int) (*Vehicle, error) {
	var vehicle Vehicle
	path := fmt.Sprintf("/api/v1/vehicles/%d", id)
	if err := c.doRequest(ctx, "GET", path, nil, &vehicle); err != nil {
		return nil, err
	}
	return &vehicle, nil
}

// GetVehiclesWithParams returns vehicles with advanced filtering
func (c *Client) GetVehiclesWithParams(ctx context.Context, params VehicleQueryParams) (*PaginatedResponse[Vehicle], error) {
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

	path := "/api/v1/vehicles?" + encoded
	var resp PaginatedResponse[Vehicle]
	if err := c.doRequest(ctx, "GET", path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
