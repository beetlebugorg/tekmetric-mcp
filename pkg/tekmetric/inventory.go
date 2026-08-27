package tekmetric

import (
	"context"
	"fmt"
	"time"
)

// ============================================================================
// Models
// ============================================================================

// InventoryPart represents an inventory part
type InventoryPart struct {
	ID          int        `json:"id"`
	ShopID      int        `json:"shopId"`
	PartNumber  string     `json:"partNumber"`
	Description string     `json:"description"`
	Brand       string     `json:"brand,omitempty"`
	Cost        Currency   `json:"cost"`
	Retail      Currency   `json:"retail"`
	Quantity    float64    `json:"quantity"`
	Location    string     `json:"location,omitempty"`
	CreatedDate time.Time  `json:"createdDate"`
	UpdatedDate time.Time  `json:"updatedDate"`
	DeletedDate *time.Time `json:"deletedDate"`
}

// CannedJob represents a predefined job template
type CannedJob struct {
	ID           int       `json:"id"`
	ShopID       int       `json:"shopId"`
	Name         string    `json:"name"`
	Description  string    `json:"description,omitempty"`
	CategoryName string    `json:"categoryName,omitempty"`
	LaborRate    int       `json:"laborRate"`
	LaborHours   float64   `json:"laborHours"`
	CreatedDate  time.Time `json:"createdDate"`
	UpdatedDate  time.Time `json:"updatedDate"`
}

// ============================================================================
// API Methods
// ============================================================================

// InventoryQueryParams holds query parameters for inventory searches
type InventoryQueryParams struct {
	Shop          int      `url:"shop"`       // Required: Shop ID
	PartTypeID    int      `url:"partTypeId"` // Required: 1=Part, 2=Tire, 5=Battery
	Page          int      `url:"page"`
	Size          int      `url:"size"`
	PartNumbers   []string `url:"partNumbers,omitempty"`   // Exact match on part numbers
	Width         string   `url:"width,omitempty"`         // Tire width (tires only)
	Ratio         float64  `url:"ratio,omitempty"`         // Tire ratio (tires only)
	Diameter      float64  `url:"diameter,omitempty"`      // Tire diameter (tires only)
	TireSize      string   `url:"tireSize,omitempty"`      // Tire size: width+ratio+diameter (tires only)
	Sort          string   `url:"sort,omitempty"`          // id, name, brand, partNumber (comma-separated)
	SortDirection string   `url:"sortDirection,omitempty"` // ASC, DESC
}

// GetInventory returns a paginated list of inventory parts
// Note: partTypeId is REQUIRED by the Tekmetric API (1=Part, 2=Tire, 5=Battery)
func (c *Client) GetInventory(ctx context.Context, shopID int, partTypeID int, page int, size int) (*PaginatedResponse[InventoryPart], error) {
	params := InventoryQueryParams{
		Shop:       shopID,
		PartTypeID: partTypeID,
		Page:       page,
		Size:       size,
	}
	return c.GetInventoryWithParams(ctx, params)
}

// GetInventoryWithParams returns inventory parts with advanced filtering
func (c *Client) GetInventoryWithParams(ctx context.Context, params InventoryQueryParams) (*PaginatedResponse[InventoryPart], error) {
	if err := c.isAuthorizedShop(params.Shop); err != nil {
		return nil, err
	}
	if err := params.Validate(); err != nil {
		return nil, err
	}

	encoded, err := encodeQuery(params)
	if err != nil {
		return nil, err
	}

	path := "/api/v1/inventory?" + encoded
	var resp PaginatedResponse[InventoryPart]
	if err := c.doRequest(ctx, "GET", path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetCannedJobs returns a paginated list of canned jobs
func (c *Client) GetCannedJobs(ctx context.Context, shopID int, page int, size int) (*PaginatedResponse[CannedJob], error) {
	if err := c.isAuthorizedShop(shopID); err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/api/v1/canned-jobs?shop=%d&page=%d&size=%d", shopID, page, size)
	var resp PaginatedResponse[CannedJob]
	if err := c.doRequest(ctx, "GET", path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
