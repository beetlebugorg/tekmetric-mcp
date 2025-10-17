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

// RepairOrderStatus represents the status of a repair order
type RepairOrderStatus struct {
	ID   int    `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

// RepairOrderLabel represents a label for a repair order
type RepairOrderLabel struct {
	ID     int                `json:"id"`
	Code   string             `json:"code"`
	Name   string             `json:"name"`
	Status *RepairOrderStatus `json:"status,omitempty"`
}

// RepairOrderCustomLabel represents a custom label
type RepairOrderCustomLabel struct {
	Name string `json:"name"`
}

// RepairOrder represents a repair order
type RepairOrder struct {
	ID                     int                     `json:"id"`
	RepairOrderNumber      int                     `json:"repairOrderNumber"`
	ShopID                 int                     `json:"shopId"`
	RepairOrderStatus      RepairOrderStatus       `json:"repairOrderStatus"`
	RepairOrderLabel       *RepairOrderLabel       `json:"repairOrderLabel,omitempty"`
	RepairOrderCustomLabel *RepairOrderCustomLabel `json:"repairOrderCustomLabel,omitempty"`
	Color                  string                  `json:"color,omitempty"`
	AppointmentStartTime   *time.Time              `json:"appointmentStartTime,omitempty"`
	CustomerID             int                     `json:"customerId"`
	TechnicianID           *int                    `json:"technicianId"`
	ServiceWriterID        *int                    `json:"serviceWriterId"`
	VehicleID              int                     `json:"vehicleId"`
	MilesIn                *float64                `json:"milesIn"`
	MilesOut               *float64                `json:"milesOut"`
	Keytag                 *string                 `json:"keytag"`
	CompletedDate          *time.Time              `json:"completedDate"`
	PostedDate             *time.Time              `json:"postedDate"`
	LaborSales             Currency                `json:"laborSales"`
	PartsSales             Currency                `json:"partsSales"`
	SubletSales            Currency                `json:"subletSales"`
	DiscountTotal          Currency                `json:"discountTotal"`
	FeeTotal               Currency                `json:"feeTotal"`
	Taxes                  Currency                `json:"taxes"`
	AmountPaid             Currency                `json:"amountPaid"`
	TotalSales             Currency                `json:"totalSales"`
	Jobs                   []Job                   `json:"jobs,omitempty"`
	Sublets                []Sublet                `json:"sublets,omitempty"`
	Fees                   []Fee                   `json:"fees,omitempty"`
	Discounts              []Discount              `json:"discounts,omitempty"`
	CustomerConcerns       []CustomerConcern       `json:"customerConcerns,omitempty"`
	CreatedDate            time.Time               `json:"createdDate"`
	UpdatedDate            time.Time               `json:"updatedDate"`
	DeletedDate            *time.Time              `json:"deletedDate"`
}

// ============================================================================
// API Methods
// ============================================================================

// RepairOrderQueryParams holds query parameters for repair order searches
type RepairOrderQueryParams struct {
	Shop                 int    `url:"shop,omitempty"`
	Page                 int    `url:"page,omitempty"`
	Size                 int    `url:"size,omitempty"`
	Start                string `url:"start,omitempty"`            // Date format: YYYY-MM-DD
	End                  string `url:"end,omitempty"`              // Date format: YYYY-MM-DD
	PostedDateStart      string `url:"postedDateStart,omitempty"`  // Date format: YYYY-MM-DD
	PostedDateEnd        string `url:"postedDateEnd,omitempty"`    // Date format: YYYY-MM-DD
	UpdatedDateStart     string `url:"updatedDateStart,omitempty"` // Date format: YYYY-MM-DD
	UpdatedDateEnd       string `url:"updatedDateEnd,omitempty"`   // Date format: YYYY-MM-DD
	RepairOrderNumber    int    `url:"repairOrderNumber,omitempty"`
	RepairOrderStatusIds []int  `url:"repairOrderStatusId,omitempty"` // 1-Estimate, 2-WIP, 3-Complete, 4-Saved, 5-Posted, 6-AR, 7-Deleted
	CustomerID           int    `url:"customerId,omitempty"`
	VehicleID            int    `url:"vehicleId,omitempty"`
	Search               string `url:"search,omitempty"`        // Search by RO#, customer name, vehicle info
	Sort                 string `url:"sort,omitempty"`          // createdDate, repairOrderNumber, customer.firstName, customer.lastName
	SortDirection        string `url:"sortDirection,omitempty"` // ASC, DESC
}

// GetRepairOrders returns a paginated list of repair orders (excludes deleted status 7 by default)
func (c *Client) GetRepairOrders(ctx context.Context, shopID int, page int, size int) (*PaginatedResponse[RepairOrder], error) {
	params := RepairOrderQueryParams{
		Shop:                 shopID,
		Page:                 page,
		Size:                 size,
		RepairOrderStatusIds: []int{1, 2, 3, 4, 5, 6}, // Exclude status 7 (Deleted)
	}
	return c.GetRepairOrdersWithParams(ctx, params)
}

// GetRepairOrdersWithParams returns repair orders with advanced filtering
func (c *Client) GetRepairOrdersWithParams(ctx context.Context, params RepairOrderQueryParams) (*PaginatedResponse[RepairOrder], error) {
	if err := c.isAuthorizedShop(params.Shop); err != nil {
		return nil, err
	}
	if err := params.Validate(); err != nil {
		return nil, err
	}
	// Build query string
	query := url.Values{}
	if params.Shop > 0 {
		query.Add("shop", fmt.Sprintf("%d", params.Shop))
	}
	query.Add("page", fmt.Sprintf("%d", params.Page))
	if params.Size > 0 {
		query.Add("size", fmt.Sprintf("%d", params.Size))
	} else {
		query.Add("size", "100")
	}
	if params.Start != "" {
		query.Add("start", params.Start)
	}
	if params.End != "" {
		query.Add("end", params.End)
	}
	if params.PostedDateStart != "" {
		query.Add("postedDateStart", params.PostedDateStart)
	}
	if params.PostedDateEnd != "" {
		query.Add("postedDateEnd", params.PostedDateEnd)
	}
	if params.UpdatedDateStart != "" {
		query.Add("updatedDateStart", params.UpdatedDateStart)
	}
	if params.UpdatedDateEnd != "" {
		query.Add("updatedDateEnd", params.UpdatedDateEnd)
	}
	if params.RepairOrderNumber > 0 {
		query.Add("repairOrderNumber", fmt.Sprintf("%d", params.RepairOrderNumber))
	}
	for _, statusID := range params.RepairOrderStatusIds {
		query.Add("repairOrderStatusId", fmt.Sprintf("%d", statusID))
	}
	if params.CustomerID > 0 {
		query.Add("customerId", fmt.Sprintf("%d", params.CustomerID))
	}
	if params.VehicleID > 0 {
		query.Add("vehicleId", fmt.Sprintf("%d", params.VehicleID))
	}
	if params.Search != "" {
		query.Add("search", params.Search)
	}
	if params.Sort != "" {
		query.Add("sort", params.Sort)
	}
	if params.SortDirection != "" {
		query.Add("sortDirection", params.SortDirection)
	}

	path := "/api/v1/repair-orders?" + query.Encode()
	var resp PaginatedResponse[RepairOrder]
	if err := c.doRequest(ctx, "GET", path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetRepairOrder returns a specific repair order by ID
func (c *Client) GetRepairOrder(ctx context.Context, id int) (*RepairOrder, error) {
	var ro RepairOrder
	path := fmt.Sprintf("/api/v1/repair-orders/%d", id)
	if err := c.doRequest(ctx, "GET", path, nil, &ro); err != nil {
		return nil, err
	}
	return &ro, nil
}
