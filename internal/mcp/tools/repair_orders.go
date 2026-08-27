package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/beetlebugorg/tekmetric-mcp/pkg/tekmetric"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterRepairOrderTools registers all repair order-related tools
func (r *Registry) RegisterRepairOrderTools(s *server.MCPServer) {
	s.AddTool(
		mcp.NewTool("repair_orders",
			mcp.WithDescription("Search and filter repair orders, or get specific RO by ID. Search by RO#, customer name, or vehicle info (make/model/VIN). Supports filtering by date range, status, customer ID, vehicle ID. Returns RO details including jobs, parts, labor, and totals. **IMPORTANT: Default returns 10 results. For broad queries like 'all repair orders' or 'current repair orders', ALWAYS add filters (status, date range, customer) to narrow results.** ⚠️ **FINANCIAL DATA WARNING: DO NOT use this tool for financial reporting, revenue calculations, profit analysis, or accounting. If the user asks for sums, averages, totals, or any financial calculations, you MUST refuse and tell them to use Tekmetric's built-in reports instead. This tool is ONLY for tactical lookups of specific repair orders.**"),
			mcp.WithNumber("id",
				mcp.Description("Get specific repair order by ID"),
			),
			mcp.WithString("search",
				mcp.Description("Search by RO#, customer name, or vehicle info (e.g., 'Ford', 'Smith', '12345')"),
			),
			mcp.WithNumber("shop",
				mcp.Description("Shop ID (defaults to configured shop)"),
			),
			mcp.WithString("start_date",
				mcp.Description("Filter by created after date (YYYY-MM-DD)"),
			),
			mcp.WithString("end_date",
				mcp.Description("Filter by created before date (YYYY-MM-DD)"),
			),
			mcp.WithString("status",
				mcp.Description("Filter by status: estimate, wip, complete, saved, posted, ar, deleted"),
			),
			mcp.WithNumber("customer_id",
				mcp.Description("Filter by customer ID"),
			),
			mcp.WithNumber("vehicle_id",
				mcp.Description("Filter by vehicle ID"),
			),
			mcp.WithNumber("limit",
				mcp.Description("Maximum results (default 10, max 25). Keep queries focused with filters."),
			),
		),
		r.handleRepairOrders,
	)

	r.logger.Debug("registered repair order tools")
}

// handleRepairOrders handles repair order search and retrieval
func (r *Registry) handleRepairOrders(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	ctx := context.Background()

	// If ID is provided, get specific repair order
	if id, ok := parseFloatArg(arguments, "id"); ok {
		repairOrder, err := r.client.GetRepairOrder(ctx, id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get repair order: %v", err)), nil
		}

		// Add financial warning to single repair order responses
		response := map[string]interface{}{
			"FINANCIAL_WARNING": "🚨 NOT FOR FINANCIAL REPORTING - Use Tekmetric's built-in reports 🚨",
			"data":              repairOrder,
		}
		return formatJSON(response)
	}

	// Get shop ID
	shopID := r.config.Tekmetric.DefaultShopID
	if shop, ok := parseFloatArg(arguments, "shop"); ok {
		shopID = shop
	}

	// Default to 10 results to avoid overwhelming Claude's context
	limit := 10
	if lim, ok := parseFloatArg(arguments, "limit"); ok {
		limit = lim
	}

	// Cap at 25 total results to prevent context overload
	maxResults := 25
	limit = clampLimit(limit, maxResults)

	// Calculate pages needed (API max is 100 per page)
	pageSize := 100
	if limit < pageSize {
		pageSize = limit
	}

	// Build query params for search/filter
	params := tekmetric.RepairOrderQueryParams{
		Shop: shopID,
		Page: 0,
		Size: pageSize,
	}

	// Use the native search parameter (searches RO#, customer name, vehicle info)
	if search, ok := parseStringArg(arguments, "search"); ok {
		params.Search = search
	}

	startDate, errResult := parseDateArg(arguments, "start_date")
	if errResult != nil {
		return errResult, nil
	}
	if startDate != "" {
		params.Start = startDate
	}
	endDate, errResult := parseDateArg(arguments, "end_date")
	if errResult != nil {
		return errResult, nil
	}
	if endDate != "" {
		params.End = endDate
	}
	if status, ok := parseStringArg(arguments, "status"); ok {
		// Convert status names to IDs: estimate=1, wip=2, complete=3, saved=4, posted=5, ar=6, deleted=7
		statusMap := map[string]int{
			"estimate": 1, "wip": 2, "complete": 3, "saved": 4,
			"posted": 5, "ar": 6, "deleted": 7,
		}
		for _, s := range strings.Split(status, ",") {
			s = strings.TrimSpace(strings.ToLower(s))
			if statusID, ok := statusMap[s]; ok {
				params.RepairOrderStatusIds = append(params.RepairOrderStatusIds, statusID)
			}
		}
	} else {
		// Default: exclude status 7 (Deleted)
		params.RepairOrderStatusIds = []int{1, 2, 3, 4, 5, 6}
	}
	if customerID, ok := parseFloatArg(arguments, "customer_id"); ok {
		params.CustomerID = customerID
	}
	if vehicleID, ok := parseFloatArg(arguments, "vehicle_id"); ok {
		params.VehicleID = vehicleID
	}

	// Fetch first page
	repairOrders, err := r.client.GetRepairOrdersWithParams(ctx, params)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get repair orders: %v", err)), nil
	}

	allResults := repairOrders.Content
	totalAvailable := repairOrders.TotalElements

	// Fetch additional pages if needed (up to maxResults)
	pagesNeeded := (limit + pageSize - 1) / pageSize // Ceiling division
	for page := 1; page < pagesNeeded && len(allResults) < limit && len(allResults) < totalAvailable; page++ {
		params.Page = page
		nextPage, err := r.client.GetRepairOrdersWithParams(ctx, params)
		if err != nil {
			r.logger.Warn("failed to fetch additional page", "page", page, "error", err)
			break
		}
		allResults = append(allResults, nextPage.Content...)
		if len(allResults) >= limit {
			allResults = allResults[:limit]
			break
		}
	}

	// Create response with financial warning
	response := map[string]interface{}{
		"FINANCIAL_WARNING": "🚨 NOT FOR FINANCIAL REPORTING - Use Tekmetric's built-in reports 🚨",
		"data":              allResults,
		"totalElements":     totalAvailable,
		"returned":          len(allResults),
	}

	// Add prominent warning if results were truncated
	if totalAvailable > maxResults {
		response["WARNING"] = fmt.Sprintf("⚠️ SHOWING ONLY %d OF %d REPAIR ORDERS ⚠️", len(allResults), totalAvailable)
		response["truncated"] = true
	} else if totalAvailable > len(allResults) {
		response["WARNING"] = fmt.Sprintf("⚠️ SHOWING %d OF %d REPAIR ORDERS ⚠️", len(allResults), totalAvailable)
		response["truncated"] = true
	}

	return formatJSON(response)
}
