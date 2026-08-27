package tools

import (
	"context"
	"fmt"

	"github.com/beetlebugorg/tekmetric-mcp/pkg/tekmetric"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterCustomerTools registers all customer-related tools
func (r *Registry) RegisterCustomerTools(s *server.MCPServer) {
	// Search/list customers - consolidated tool
	s.AddTool(
		mcp.NewTool("customers",
			mcp.WithDescription("Search for customers by name, email, phone, or get customer by ID. Supports advanced filtering by customer type, AR eligibility, and date ranges."),
			mcp.WithNumber("id",
				mcp.Description("Get specific customer by ID"),
			),
			mcp.WithString("search",
				mcp.Description("Search customers by name, email, or phone"),
			),
			mcp.WithNumber("shop",
				mcp.Description("Shop ID (defaults to configured shop)"),
			),
			mcp.WithNumber("customer_type",
				mcp.Description("Customer type: 1=Customer, 2=Business"),
			),
			mcp.WithBoolean("ar_eligible",
				mcp.Description("Filter by accounts receivable eligibility"),
			),
			mcp.WithBoolean("ok_for_marketing",
				mcp.Description("Filter by marketing permission"),
			),
			mcp.WithString("updated_date_start",
				mcp.Description("Filter by updated date start (YYYY-MM-DD)"),
			),
			mcp.WithString("updated_date_end",
				mcp.Description("Filter by updated date end (YYYY-MM-DD)"),
			),
			mcp.WithString("sort",
				mcp.Description("Sort field: lastName, firstName, email (comma-separated)"),
			),
			mcp.WithString("sort_direction",
				mcp.Description("Sort direction: ASC or DESC"),
			),
			mcp.WithNumber("limit",
				mcp.Description("Maximum results to return (max: 100, default: 10)"),
			),
			mcp.WithNumber("page",
				mcp.Description("Page number for pagination (default: 0)"),
			),
		),
		r.handleCustomers,
	)

	r.logger.Debug("registered customer tools")
}

// handleCustomers handles customer search and retrieval
func (r *Registry) handleCustomers(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	arguments := request.GetArguments()

	// If ID is provided, get specific customer
	if id, ok := parseFloatArg(arguments, "id"); ok {
		customer, err := r.client.GetCustomer(ctx, id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get customer: %v", err)), nil
		}
		return formatJSON(customer)
	}

	// Build query params
	params := tekmetric.CustomerQueryParams{
		Shop: r.config.Tekmetric.DefaultShopID,
		Page: 0,
		Size: 10,
	}

	// Parse optional parameters
	if shop, ok := parseFloatArg(arguments, "shop"); ok {
		params.Shop = shop
	}
	if search, ok := parseStringArg(arguments, "search"); ok {
		params.Search = search
	}
	if customerType, ok := parseFloatArg(arguments, "customer_type"); ok {
		params.CustomerTypeID = customerType
	}
	if arEligible, ok := parseBoolArg(arguments, "ar_eligible"); ok {
		params.EligibleForAccountsReceivable = &arEligible
	}
	if okMarketing, ok := parseBoolArg(arguments, "ok_for_marketing"); ok {
		params.OkForMarketing = &okMarketing
	}
	if updatedStart, ok := parseStringArg(arguments, "updated_date_start"); ok {
		params.UpdatedDateStart = updatedStart
	}
	if updatedEnd, ok := parseStringArg(arguments, "updated_date_end"); ok {
		params.UpdatedDateEnd = updatedEnd
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

	resp, err := r.client.GetCustomersWithParams(ctx, params)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to search customers: %v", err)), nil
	}

	return formatPaginatedResultWithWarning(
		resp.Content,
		resp.TotalElements,
		len(resp.Content),
		25,
		"CUSTOMERS",
	)
}
