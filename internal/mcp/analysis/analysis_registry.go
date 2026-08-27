package analysis

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/beetlebugorg/tekmetric-mcp/internal/config"
	"github.com/beetlebugorg/tekmetric-mcp/pkg/tekmetric"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Registry manages all analysis tools and handles their registration
// with the MCP server.
type Registry struct {
	tools  []AnalysisTool
	client *tekmetric.Client
	config *config.Config
	logger *slog.Logger
}

// NewRegistry creates a new analysis tool registry
func NewRegistry(client *tekmetric.Client, cfg *config.Config, logger *slog.Logger) *Registry {
	return &Registry{
		tools:  make([]AnalysisTool, 0),
		client: client,
		config: cfg,
		logger: logger,
	}
}

// Register adds a analysis tool to the registry
func (r *Registry) Register(tool AnalysisTool) {
	r.tools = append(r.tools, tool)
	r.logger.Debug("registered analysis tool", "name", tool.Name())
}

// RegisterAll registers all analysis tools with the MCP server
func (r *Registry) RegisterAll(mcpServer *server.MCPServer) {
	for _, tool := range r.tools {
		r.registerOne(mcpServer, tool)
	}
	r.logger.Info("registered analysis tools", "count", len(r.tools))
}

// registerOne registers a single analysis tool as an MCP tool
func (r *Registry) registerOne(mcpServer *server.MCPServer, tool AnalysisTool) {
	// Build tool definition with parameters from schema
	schema := tool.Schema()
	toolOpts := []mcp.ToolOption{
		mcp.WithDescription(tool.Description()),
	}

	// Add schema properties as tool parameters
	if props, ok := schema["properties"].(map[string]interface{}); ok {
		for name, propDef := range props {
			if propMap, ok := propDef.(map[string]interface{}); ok {
				propType, _ := propMap["type"].(string)
				propDesc, _ := propMap["description"].(string)

				switch propType {
				case "number":
					toolOpts = append(toolOpts, mcp.WithNumber(name, mcp.Description(propDesc)))
				case "string":
					toolOpts = append(toolOpts, mcp.WithString(name, mcp.Description(propDesc)))
				case "boolean":
					toolOpts = append(toolOpts, mcp.WithBoolean(name, mcp.Description(propDesc)))
				}
			}
		}
	}

	mcpTool := mcp.NewTool(tool.Name(), toolOpts...)
	handler := r.createHandler(tool)
	mcpServer.AddTool(mcpTool, handler)

	r.logger.Debug("registered analysis tool",
		"name", tool.Name(),
		"description", tool.Description())
}

// createHandler creates an MCP tool handler for a analysis tool
func (r *Registry) createHandler(tool AnalysisTool) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		arguments := request.GetArguments()

		r.logger.Debug("executing analysis tool",
			"name", tool.Name(),
			"params", arguments)

		// Execute tool
		result, err := tool.Execute(ctx, arguments)
		if err != nil {
			r.logger.Error("analysis tool execution failed",
				"name", tool.Name(),
				"error", err)
			return mcp.NewToolResultError(fmt.Sprintf("Tool execution failed: %v", err)), nil
		}

		// Format result for MCP
		return r.formatResult(result)
	}
}

// formatResult formats a AnalysisResult into an MCP CallToolResult
func (r *Registry) formatResult(result *AnalysisResult) (*mcp.CallToolResult, error) {
	// Build the response text
	var responseText string

	// Add summary if present
	if result.Summary != "" {
		responseText += result.Summary + "\n\n"
	}

	// Add prompt if present (this is what Claude will see and respond to)
	if result.Prompt != "" {
		responseText += result.Prompt + "\n\n"
	}

	// Add metadata
	responseText += fmt.Sprintf("---\nMetadata: Fetched %d records (%d processed) across %d pages in %dms",
		result.Metadata.RecordsFetched,
		result.Metadata.RecordsProcessed,
		result.Metadata.PagesTraversed,
		result.Metadata.ExecutionTimeMs)

	// Include structured data if present
	contents := []mcp.Content{
		mcp.NewTextContent(responseText),
	}

	// If there's structured data, add it as JSON
	if result.Data != nil {
		dataJSON, err := json.MarshalIndent(result.Data, "", "  ")
		if err != nil {
			r.logger.Warn("failed to marshal data to JSON", "error", err)
		} else {
			contents = append(contents, mcp.NewTextContent(fmt.Sprintf("\n\nStructured Data:\n```json\n%s\n```", string(dataJSON))))
		}
	}

	return &mcp.CallToolResult{
		Content: contents,
	}, nil
}
