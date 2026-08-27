package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/beetlebugorg/tekmetric-mcp/pkg/tekmetric"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterInventoryTools registers all inventory-related tools
func (r *Registry) RegisterInventoryTools(s *server.MCPServer) {
	s.AddTool(
		mcp.NewTool("inventory",
			mcp.WithDescription("Search inventory parts by type, name, or number. Returns part details including SKU, price, and quantity (Beta feature). **REQUIRED: You must specify part_type_id.**"),
			mcp.WithNumber("part_type_id",
				mcp.Description("REQUIRED: Part type - 1 (Part), 2 (Tire), or 5 (Battery)"),
			),
			mcp.WithString("query",
				mcp.Description("Search parts by name or part number (optional - omit to list all)"),
			),
			mcp.WithNumber("shop",
				mcp.Description("Shop ID (defaults to configured shop)"),
			),
			mcp.WithNumber("limit",
				mcp.Description("Maximum results to return (default 20, max 100)"),
			),
		),
		r.handleInventory,
	)

	r.logger.Debug("registered inventory tools")
}

// handleInventory searches or lists inventory parts
func (r *Registry) handleInventory(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	arguments := request.GetArguments()

	// Get required part type ID
	partTypeID, errResult := requireFloatArg(arguments, "part_type_id")
	if errResult != nil {
		return errResult, nil
	}

	// Get shop ID
	shopID := r.config.Tekmetric.DefaultShopID
	if shop, ok := parseFloatArg(arguments, "shop"); ok {
		shopID = shop
	}

	// Get limit
	limit := 20
	if lim, ok := parseFloatArg(arguments, "limit"); ok {
		limit = clampLimit(lim, 100)
	}

	// Fetch inventory (always fetch first page for now)
	inventory, err := r.client.GetInventory(ctx, shopID, partTypeID, 0, 100)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get inventory: %v", err)), nil
	}

	// If query is provided, filter parts
	if query, ok := parseStringArg(arguments, "query"); ok {
		matches := filterInventory(inventory.Content, query)
		if len(matches) > limit {
			matches = matches[:limit]
		}

		return formatJSON(map[string]interface{}{
			"query":   query,
			"matches": len(matches),
			"results": matches,
		})
	}

	// No query - return limited results
	results := inventory.Content
	if len(results) > limit {
		results = results[:limit]
	}

	return formatJSON(map[string]interface{}{
		"total":   inventory.TotalElements,
		"showing": len(results),
		"results": results,
	})
}

// filterInventory filters inventory parts by description or part number
func filterInventory(parts []tekmetric.InventoryPart, query string) []tekmetric.InventoryPart {
	query = strings.ToLower(query)
	var matches []tekmetric.InventoryPart

	for _, part := range parts {
		if strings.Contains(strings.ToLower(part.Description), query) ||
			strings.Contains(strings.ToLower(part.PartNumber), query) ||
			strings.Contains(strings.ToLower(part.Brand), query) {
			matches = append(matches, part)
		}
	}

	return matches
}
