package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/beetlebugorg/tekmetric-mcp/pkg/tekmetric"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterShopTools registers all shop-related tools
func (r *Registry) RegisterShopTools(s *server.MCPServer) {
	s.AddTool(
		mcp.NewTool("shops",
			mcp.WithDescription("Search for shops by name or list all accessible Tekmetric shops. Returns shop information including shop ID, name, and settings."),
			mcp.WithString("query",
				mcp.Description("Search shops by name (optional - omit to list all)"),
			),
			mcp.WithNumber("limit",
				mcp.Description("Maximum results to return (default 10)"),
			),
		),
		r.handleShops,
	)

	r.logger.Debug("registered shop tools")
}

// handleShops searches or lists shops
func (r *Registry) handleShops(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	ctx := context.Background()

	shops, err := r.client.GetShops(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get shops: %v", err)), nil
	}

	// If query is provided, filter shops by name
	if query, ok := parseStringArg(arguments, "query"); ok {
		limit := 10
		if lim, ok := parseFloatArg(arguments, "limit"); ok {
			limit = lim
		}
		limit = clampLimit(limit, 100)

		matches := filterShops(shops, query)
		if len(matches) > limit {
			matches = matches[:limit]
		}

		return formatJSON(map[string]interface{}{
			"query":   query,
			"matches": len(matches),
			"results": matches,
		})
	}

	// No query - return all shops
	return formatJSON(shops)
}

// filterShops filters shops by name match
func filterShops(shops []tekmetric.Shop, query string) []tekmetric.Shop {
	query = strings.ToLower(query)
	var matches []tekmetric.Shop

	for _, shop := range shops {
		if strings.Contains(strings.ToLower(shop.Name), query) {
			matches = append(matches, shop)
		}
	}

	return matches
}
