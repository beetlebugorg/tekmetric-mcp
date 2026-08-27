package tools

import (
	"context"
	"fmt"

	"github.com/beetlebugorg/tekmetric-mcp/pkg/tekmetric"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterAppointmentTools registers all appointment-related tools
func (r *Registry) RegisterAppointmentTools(s *server.MCPServer) {
	s.AddTool(
		mcp.NewTool("appointments",
			mcp.WithDescription("Search and filter appointments, or get a specific appointment by ID. Supports filtering by date range, customer, vehicle, and more."),
			mcp.WithNumber("id",
				mcp.Description("Get specific appointment by ID"),
			),
			mcp.WithString("search",
				mcp.Description("Search appointments by customer name or vehicle info"),
			),
			mcp.WithNumber("shop",
				mcp.Description("Shop ID (defaults to TEKMETRIC_DEFAULT_SHOP_ID)"),
			),
			mcp.WithNumber("customer_id",
				mcp.Description("Filter by customer ID"),
			),
			mcp.WithNumber("vehicle_id",
				mcp.Description("Filter by vehicle ID"),
			),
			mcp.WithString("start_date",
				mcp.Description("Filter appointments starting from this date (YYYY-MM-DD format)"),
			),
			mcp.WithString("end_date",
				mcp.Description("Filter appointments up to this date (YYYY-MM-DD format)"),
			),
			mcp.WithString("updated_start",
				mcp.Description("Filter by appointments updated after this date (YYYY-MM-DD format)"),
			),
			mcp.WithString("updated_end",
				mcp.Description("Filter by appointments updated before this date (YYYY-MM-DD format)"),
			),
			mcp.WithString("status",
				mcp.Description("Filter by appointment status"),
			),
			mcp.WithString("sort",
				mcp.Description("Property to sort results by (e.g., scheduledTime, createdDate, customer.lastName)"),
			),
			mcp.WithString("sort_direction",
				mcp.Description("Sort direction (ASC or DESC)"),
			),
			mcp.WithNumber("limit",
				mcp.Description("Maximum number of results to return (max: 100, default: 20)"),
			),
			mcp.WithNumber("page",
				mcp.Description("Page number for pagination (default: 0)"),
			),
		),
		r.handleAppointments,
	)

	r.logger.Debug("registered appointment tools")
}

// handleAppointments searches appointments or gets a specific appointment by ID
func (r *Registry) handleAppointments(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	ctx := context.Background()

	// If ID is provided, get specific appointment
	if id, ok := parseFloatArg(arguments, "id"); ok {
		appointment, err := r.client.GetAppointment(ctx, id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get appointment: %v", err)), nil
		}
		enriched := r.enrichAppointment(ctx, appointment)
		return formatJSON(enriched)
	}

	// Otherwise, search with filters
	// Default to 10 results to avoid overwhelming context
	params := tekmetric.AppointmentQueryParams{
		Shop: r.config.Tekmetric.DefaultShopID,
		Page: 0,
		Size: 10,
	}

	// Parse optional parameters
	if shop, ok := parseFloatArg(arguments, "shop"); ok {
		params.Shop = shop
	}
	if customerID, ok := parseFloatArg(arguments, "customer_id"); ok {
		params.CustomerID = customerID
	}
	if vehicleID, ok := parseFloatArg(arguments, "vehicle_id"); ok {
		params.VehicleID = vehicleID
	}
	start, errResult := parseDateArg(arguments, "start_date")
	if errResult != nil {
		return errResult, nil
	}
	if start != "" {
		params.Start = start
	}
	end, errResult := parseDateArg(arguments, "end_date")
	if errResult != nil {
		return errResult, nil
	}
	if end != "" {
		params.End = end
	}
	if sort, ok := parseStringArg(arguments, "sort"); ok {
		params.Sort = sort
	}
	if sortDirection, ok := parseStringArg(arguments, "sort_direction"); ok {
		params.SortDirection = sortDirection
	}
	if limit, ok := parseFloatArg(arguments, "limit"); ok {
		params.Size = limit
		if params.Size > 100 {
			params.Size = 100
		}
	}
	if page, ok := parseFloatArg(arguments, "page"); ok {
		params.Page = page
	}

	resp, err := r.client.GetAppointmentsWithParams(ctx, params)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to search appointments: %v", err)), nil
	}

	// Enrich appointments with customer and vehicle data
	enrichedResp := r.enrichAppointments(ctx, resp)

	// Return with warning if needed
	return formatPaginatedResultWithWarning(
		enrichedResp.Content,
		enrichedResp.TotalElements,
		len(enrichedResp.Content),
		25,
		"APPOINTMENTS",
	)
}

// enrichAppointment adds customer and vehicle details to an appointment
func (r *Registry) enrichAppointment(ctx context.Context, appt *tekmetric.Appointment) *tekmetric.EnrichedAppointment {
	enriched := &tekmetric.EnrichedAppointment{
		Appointment: *appt,
	}

	// Fetch customer details
	if customer, err := r.client.GetCustomer(ctx, appt.CustomerID); err == nil {
		enriched.Customer = customer
	} else {
		r.logger.Warn("failed to fetch customer", "customerId", appt.CustomerID, "error", err)
	}

	// Fetch vehicle details
	if vehicle, err := r.client.GetVehicle(ctx, appt.VehicleID); err == nil {
		enriched.Vehicle = vehicle
	} else {
		r.logger.Warn("failed to fetch vehicle", "vehicleId", appt.VehicleID, "error", err)
	}

	return enriched
}

// enrichAppointments adds customer and vehicle details to a paginated response of appointments
func (r *Registry) enrichAppointments(ctx context.Context, resp *tekmetric.PaginatedResponse[tekmetric.Appointment]) *tekmetric.PaginatedResponse[tekmetric.EnrichedAppointment] {
	enrichedContent := make([]tekmetric.EnrichedAppointment, len(resp.Content))

	for i, appt := range resp.Content {
		enriched := r.enrichAppointment(ctx, &appt)
		enrichedContent[i] = *enriched
	}

	return &tekmetric.PaginatedResponse[tekmetric.EnrichedAppointment]{
		Content:          enrichedContent,
		TotalPages:       resp.TotalPages,
		TotalElements:    resp.TotalElements,
		Last:             resp.Last,
		First:            resp.First,
		Size:             resp.Size,
		Number:           resp.Number,
		NumberOfElements: resp.NumberOfElements,
		Empty:            resp.Empty,
	}
}
