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

// PartType represents the type of a part
type PartType struct {
	ID   int    `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

// Part represents a vehicle part
type Part struct {
	ID               int       `json:"id"`
	Quantity         float64   `json:"quantity"`
	Brand            string    `json:"brand,omitempty"`
	Name             string    `json:"name,omitempty"`
	PartNumber       string    `json:"partNumber,omitempty"`
	Description      string    `json:"description,omitempty"`
	Cost             Currency  `json:"cost"`
	Retail           Currency  `json:"retail"`
	Model            *string   `json:"model,omitempty"`
	Width            *string   `json:"width,omitempty"`
	Ratio            *float64  `json:"ratio,omitempty"`
	Diameter         *float64  `json:"diameter,omitempty"`
	ConstructionType *string   `json:"constructionType,omitempty"`
	LoadIndex        *string   `json:"loadIndex,omitempty"`
	SpeedRating      *string   `json:"speedRating,omitempty"`
	PartType         *PartType `json:"partType,omitempty"`
	DOTNumbers       []string  `json:"dotNumbers,omitempty"`
}

// Labor represents labor on a job
type Labor struct {
	ID       int      `json:"id"`
	Name     string   `json:"name"`
	Rate     Currency `json:"rate"`
	Hours    float64  `json:"hours"`
	Complete bool     `json:"complete"`
}

// Fee represents a fee
type Fee struct {
	ID    int      `json:"id"`
	Name  string   `json:"name"`
	Total Currency `json:"total"`
}

// Discount represents a discount
type Discount struct {
	ID    int      `json:"id"`
	Name  string   `json:"name"`
	Total Currency `json:"total"`
}

// Job represents a job on a repair order
type Job struct {
	ID              int        `json:"id"`
	RepairOrderID   int        `json:"repairOrderId"`
	VehicleID       int        `json:"vehicleId"`
	CustomerID      int        `json:"customerId"`
	Name            string     `json:"name"`
	Authorized      bool       `json:"authorized"`
	AuthorizedDate  *string    `json:"authorizedDate,omitempty"`
	Selected        bool       `json:"selected"`
	TechnicianID    *int       `json:"technicianId"`
	Note            string     `json:"note,omitempty"`
	JobCategoryName string     `json:"jobCategoryName,omitempty"`
	PartsTotal      Currency   `json:"partsTotal"`
	LaborTotal      Currency   `json:"laborTotal"`
	DiscountTotal   Currency   `json:"discountTotal"`
	FeeTotal        Currency   `json:"feeTotal"`
	Subtotal        Currency   `json:"subtotal"`
	Archived        bool       `json:"archived"`
	CreatedDate     time.Time  `json:"createdDate"`
	UpdatedDate     time.Time  `json:"updatedDate"`
	CompletedDate   *time.Time `json:"completedDate,omitempty"`
	Labor           []Labor    `json:"labor,omitempty"`
	Parts           []Part     `json:"parts,omitempty"`
	Fees            []Fee      `json:"fees,omitempty"`
	Discounts       []Discount `json:"discounts,omitempty"`
	LaborHours      float64    `json:"laborHours"`
	LoggedHours     float64    `json:"loggedHours"`
	Sort            int        `json:"sort,omitempty"`
}

// Vendor represents a vendor/supplier
type Vendor struct {
	ID       int     `json:"id"`
	Name     string  `json:"name"`
	Nickname string  `json:"nickname,omitempty"`
	Website  *string `json:"website"`
	Phone    *string `json:"phone"`
}

// SubletItem represents an item in a sublet
type SubletItem struct {
	ID       int      `json:"id"`
	Name     string   `json:"name"`
	Cost     Currency `json:"cost"`
	Price    Currency `json:"price"`
	Complete bool     `json:"complete"`
}

// Sublet represents subcontracted work
type Sublet struct {
	ID             int          `json:"id"`
	Name           string       `json:"name"`
	Vendor         *Vendor      `json:"vendor,omitempty"`
	Authorized     *bool        `json:"authorized"`
	AuthorizedDate *string      `json:"authorizedDate"`
	Selected       bool         `json:"selected"`
	Note           *string      `json:"note"`
	Items          []SubletItem `json:"items,omitempty"`
	Price          Currency     `json:"price"`
	Cost           Currency     `json:"cost"`
}

// CustomerConcern represents a customer's concern
type CustomerConcern struct {
	ID          int     `json:"id"`
	Concern     string  `json:"concern"`
	TechComment *string `json:"techComment"`
}

// ============================================================================
// API Methods
// ============================================================================

// JobQueryParams holds query parameters for job searches
type JobQueryParams struct {
	Shop                 int    `url:"shop,omitempty"`
	Page                 int    `url:"page,omitempty"`
	Size                 int    `url:"size,omitempty"`
	VehicleID            int    `url:"vehicleId,omitempty"`           // Filter by vehicle ID
	RepairOrderID        int    `url:"repairOrderId,omitempty"`       // Filter by repair order
	CustomerID           int    `url:"customerId,omitempty"`          // Filter by customer ID
	Authorized           *bool  `url:"authorized,omitempty"`          // Filter by authorized jobs
	AuthorizedDateStart  string `url:"authorizedDateStart,omitempty"` // Filter by authorization date
	AuthorizedDateEnd    string `url:"authorizedDateEnd,omitempty"`   // Filter by authorization date
	UpdatedDateStart     string `url:"updatedDateStart,omitempty"`    // Filter by updated date
	UpdatedDateEnd       string `url:"updatedDateEnd,omitempty"`      // Filter by updated date
	RepairOrderStatusIds []int  `url:"repairOrderStatusId,omitempty"` // 1-6 (no Deleted status for jobs)
	Sort                 string `url:"sort,omitempty"`                // authorizedDate
	SortDirection        string `url:"sortDirection,omitempty"`       // ASC, DESC
}

// GetJobs returns a paginated list of jobs
func (c *Client) GetJobs(ctx context.Context, shopID int, page int, size int) (*PaginatedResponse[Job], error) {
	if err := c.isAuthorizedShop(shopID); err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/api/v1/jobs?shop=%d&page=%d&size=%d", shopID, page, size)
	var resp PaginatedResponse[Job]
	if err := c.doRequest(ctx, "GET", path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetJob returns a specific job by ID
func (c *Client) GetJob(ctx context.Context, id int) (*Job, error) {
	var job Job
	path := fmt.Sprintf("/api/v1/jobs/%d", id)
	if err := c.doRequest(ctx, "GET", path, nil, &job); err != nil {
		return nil, err
	}
	return &job, nil
}

// GetJobsWithParams returns jobs with advanced filtering
func (c *Client) GetJobsWithParams(ctx context.Context, params JobQueryParams) (*PaginatedResponse[Job], error) {
	if err := c.isAuthorizedShop(params.Shop); err != nil {
		return nil, err
	}
	if err := params.Validate(); err != nil {
		return nil, err
	}
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
	if params.VehicleID > 0 {
		query.Add("vehicleId", fmt.Sprintf("%d", params.VehicleID))
	}
	if params.RepairOrderID > 0 {
		query.Add("repairOrderId", fmt.Sprintf("%d", params.RepairOrderID))
	}
	if params.CustomerID > 0 {
		query.Add("customerId", fmt.Sprintf("%d", params.CustomerID))
	}
	if params.Authorized != nil {
		query.Add("authorized", fmt.Sprintf("%t", *params.Authorized))
	}
	if params.AuthorizedDateStart != "" {
		query.Add("authorizedDateStart", params.AuthorizedDateStart)
	}
	if params.AuthorizedDateEnd != "" {
		query.Add("authorizedDateEnd", params.AuthorizedDateEnd)
	}
	if params.UpdatedDateStart != "" {
		query.Add("updatedDateStart", params.UpdatedDateStart)
	}
	if params.UpdatedDateEnd != "" {
		query.Add("updatedDateEnd", params.UpdatedDateEnd)
	}
	for _, statusID := range params.RepairOrderStatusIds {
		query.Add("repairOrderStatusId", fmt.Sprintf("%d", statusID))
	}
	if params.Sort != "" {
		query.Add("sort", params.Sort)
	}
	if params.SortDirection != "" {
		query.Add("sortDirection", params.SortDirection)
	}

	path := "/api/v1/jobs?" + query.Encode()
	var resp PaginatedResponse[Job]
	if err := c.doRequest(ctx, "GET", path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
