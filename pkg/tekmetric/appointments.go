package tekmetric

import (
	"context"
	"fmt"
	"time"
)

// ============================================================================
// Models
// ============================================================================

// Appointment represents an appointment
type Appointment struct {
	ID               int        `json:"id"`
	ShopID           int        `json:"shopId"`
	CustomerID       int        `json:"customerId"`
	VehicleID        int        `json:"vehicleId"`
	ServiceWriterID  *int       `json:"serviceWriterId"`
	TechnicianID     *int       `json:"technicianId"`
	StartTime        time.Time  `json:"startTime"`
	EndTime          time.Time  `json:"endTime"`
	Status           string     `json:"status"`
	CustomerConcerns string     `json:"customerConcerns,omitempty"`
	Notes            string     `json:"notes,omitempty"`
	CreatedDate      time.Time  `json:"createdDate"`
	UpdatedDate      time.Time  `json:"updatedDate"`
	DeletedDate      *time.Time `json:"deletedDate"`
}

// EnrichedAppointment represents an appointment with customer and vehicle details
type EnrichedAppointment struct {
	Appointment
	Customer *Customer `json:"customer,omitempty"`
	Vehicle  *Vehicle  `json:"vehicle,omitempty"`
}

// ============================================================================
// API Methods
// ============================================================================

// AppointmentQueryParams holds query parameters for appointment searches
type AppointmentQueryParams struct {
	Shop             int    `url:"shop,omitempty"`
	Page             int    `url:"page"`
	Size             int    `url:"size"`
	CustomerID       int    `url:"customerId,omitempty"`       // Filter by customer
	VehicleID        int    `url:"vehicleId,omitempty"`        // Filter by vehicle
	Start            string `url:"start,omitempty"`            // Start date filter
	End              string `url:"end,omitempty"`              // End date filter
	UpdatedDateStart string `url:"updatedDateStart,omitempty"` // Filter by updated date
	UpdatedDateEnd   string `url:"updatedDateEnd,omitempty"`   // Filter by updated date
	IncludeDeleted   *bool  `url:"includeDeleted"`             // Include deleted appointments (default: false)
	Sort             string `url:"sort,omitempty"`             // Sort field (API docs don't specify allowed values)
	SortDirection    string `url:"sortDirection,omitempty"`    // ASC, DESC
}

// GetAppointments returns a paginated list of appointments (excludes deleted by default)
func (c *Client) GetAppointments(ctx context.Context, shopID int, page int, size int) (*PaginatedResponse[Appointment], error) {
	if err := c.isAuthorizedShop(shopID); err != nil {
		return nil, err
	}
	includeDeleted := false
	path := fmt.Sprintf("/api/v1/appointments?shop=%d&page=%d&size=%d&includeDeleted=%t", shopID, page, size, includeDeleted)
	var resp PaginatedResponse[Appointment]
	if err := c.doRequest(ctx, "GET", path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetAppointment returns a specific appointment by ID
func (c *Client) GetAppointment(ctx context.Context, id int) (*Appointment, error) {
	var appointment Appointment
	path := fmt.Sprintf("/api/v1/appointments/%d", id)
	if err := c.doRequest(ctx, "GET", path, nil, &appointment); err != nil {
		return nil, err
	}
	return &appointment, nil
}

// GetAppointmentsWithParams returns appointments with advanced filtering
func (c *Client) GetAppointmentsWithParams(ctx context.Context, params AppointmentQueryParams) (*PaginatedResponse[Appointment], error) {
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

	path := "/api/v1/appointments?" + encoded
	var resp PaginatedResponse[Appointment]
	if err := c.doRequest(ctx, "GET", path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
