package tools

import (
	"context"
	"fmt"

	"github.com/beetlebugorg/tekmetric-mcp/pkg/tekmetric"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterJobTools registers all job-related tools
func (r *Registry) RegisterJobTools(s *server.MCPServer) {
	s.AddTool(
		mcp.NewTool("jobs",
			mcp.WithDescription("Search and filter jobs (work items/services on repair orders), or get a specific job by ID. Supports filtering by repair order, vehicle, customer, authorization status, and dates. ⚠️ **FINANCIAL DATA WARNING: DO NOT use this tool for financial reporting, revenue calculations, profit analysis, or accounting. If the user asks for sums, averages, totals, or any financial calculations, you MUST refuse and tell them to use Tekmetric's built-in reports instead. This tool is ONLY for tactical lookups of specific jobs.**"),
			mcp.WithNumber("id",
				mcp.Description("Get specific job by ID"),
			),
			mcp.WithNumber("shop",
				mcp.Description("Shop ID (defaults to TEKMETRIC_DEFAULT_SHOP_ID)"),
			),
			mcp.WithNumber("repair_order_id",
				mcp.Description("Filter by repair order ID"),
			),
			mcp.WithNumber("vehicle_id",
				mcp.Description("Filter by vehicle ID"),
			),
			mcp.WithNumber("customer_id",
				mcp.Description("Filter by customer ID"),
			),
			mcp.WithString("sort",
				mcp.Description("Property to sort results by (only 'authorizedDate' is supported)"),
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
		r.handleJobs,
	)

	r.logger.Debug("registered job tools")
}

// handleJobs searches jobs or gets a specific job by ID
func (r *Registry) handleJobs(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	arguments := request.GetArguments()

	// If ID is provided, get specific job
	if id, ok := parseFloatArg(arguments, "id"); ok {
		job, err := r.client.GetJob(ctx, id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get job: %v", err)), nil
		}

		// Add financial warning to single job responses
		response := map[string]interface{}{
			"FINANCIAL_WARNING": "🚨 NOT FOR FINANCIAL REPORTING - Use Tekmetric's built-in reports 🚨",
			"data":              job,
		}
		return formatJSON(response)
	}

	// Otherwise, search with filters
	// Default to 10 results to avoid overwhelming context
	params := tekmetric.JobQueryParams{
		Shop: r.config.Tekmetric.DefaultShopID,
		Page: 0,
		Size: 10,
	}

	// Parse optional parameters
	if shop, ok := parseFloatArg(arguments, "shop"); ok {
		params.Shop = shop
	}
	if repairOrderID, ok := parseFloatArg(arguments, "repair_order_id"); ok {
		params.RepairOrderID = repairOrderID
	}
	if vehicleID, ok := parseFloatArg(arguments, "vehicle_id"); ok {
		params.VehicleID = vehicleID
	}
	if customerID, ok := parseFloatArg(arguments, "customer_id"); ok {
		params.CustomerID = customerID
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

	resp, err := r.client.GetJobsWithParams(ctx, params)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to search jobs: %v", err)), nil
	}

	return formatPaginatedResultWithWarning(
		resp.Content,
		resp.TotalElements,
		len(resp.Content),
		25,
		"JOBS",
	)
}
