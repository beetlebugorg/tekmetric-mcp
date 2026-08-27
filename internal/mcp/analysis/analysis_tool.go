// Package analysis provides analysis tools for MCP.
// These tools fetch and aggregate data server-side, then return structured
// results with guidance for Claude to format and analyze intelligently.
package analysis

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/beetlebugorg/tekmetric-mcp/internal/config"
	"github.com/beetlebugorg/tekmetric-mcp/pkg/tekmetric"
)

// AnalysisTool defines the interface for all analysis tools.
// Tools fetch paginated data, process it server-side, then return
// structured results with guidance for Claude to format and interpret.
type AnalysisTool interface {
	// Name returns the tool name (e.g., "vehicle_service_analysis")
	Name() string

	// Description returns the tool description for Claude
	Description() string

	// Schema returns the MCP tool input schema
	Schema() map[string]interface{}

	// Execute runs the analysis and returns results
	Execute(ctx context.Context, params map[string]interface{}) (*AnalysisResult, error)
}

// AnalysisResult is the standardized output format for analysis tools.
// Tools fetch and aggregate data, then return structured results with
// instructions for Claude on how to format and present the information.
//
// This creates a collaborative pattern:
// - Tool: Fetches data, does aggregation/processing (what computers are good at)
// - Claude: Formats, categorizes, interprets (what LLMs are good at)
type AnalysisResult struct {
	// Summary is a human-readable text summary (optional)
	Summary string `json:"summary,omitempty"`

	// Prompt is instructions/questions for Claude to process
	// This is the PRIMARY output - asking Claude to use its intelligence
	// Examples:
	// - "Analyze this service history for recurring issues..."
	// - "Based on these metrics, suggest 3 improvements..."
	// - "Draft a customer email explaining these findings..."
	Prompt string `json:"prompt,omitempty"`

	// Data is structured data to support the prompt (optional)
	// Claude can reference this when responding to the prompt
	Data interface{} `json:"data,omitempty"`

	// Metadata contains stats about the aggregation
	Metadata AggregationMetadata `json:"metadata"`
}

// AggregationMetadata provides transparency about data processing.
// It helps users understand the scope and performance of the aggregation.
type AggregationMetadata struct {
	// RecordsFetched is how many records were retrieved from the API
	RecordsFetched int `json:"records_fetched"`

	// RecordsProcessed is how many records passed filters and were included
	RecordsProcessed int `json:"records_processed"`

	// PagesTraversed is how many API pages were fetched
	PagesTraversed int `json:"pages_traversed"`

	// ExecutionTimeMs is total processing time in milliseconds
	ExecutionTimeMs int64 `json:"execution_time_ms"`
}

// BaseAnalysisTool provides common functionality for all analysis tools.
// Concrete tools should embed this struct to inherit helpers.
type BaseAnalysisTool struct {
	client *tekmetric.Client
	config *config.Config
	logger *slog.Logger
}

// NewBaseAnalysisTool creates a new base analysis tool with common dependencies
func NewBaseAnalysisTool(client *tekmetric.Client, cfg *config.Config, logger *slog.Logger) BaseAnalysisTool {
	return BaseAnalysisTool{
		client: client,
		config: cfg,
		logger: logger,
	}
}

// FetchAllPages fetches all pages of a paginated resource up to maxPages.
// It returns all fetched items and metadata about the operation.
//
// The fetcher function receives a page number (0-indexed) and should return
// the paginated response for that page.
//
// Example:
//
//	items, metadata, err := FetchAllPages(ctx, b.logger, func(page int) (*tekmetric.PaginatedResponse[tekmetric.RepairOrder], error) {
//	    return b.client.GetRepairOrdersWithParams(ctx, tekmetric.RepairOrderQueryParams{
//	        VehicleID: vehicleID,
//	        Page:      page,
//	        Size:      100,
//	    })
//	}, 10)
func FetchAllPages[T any](
	ctx context.Context,
	logger *slog.Logger,
	fetcher func(page int) (*tekmetric.PaginatedResponse[T], error),
	maxPages int,
) ([]T, AggregationMetadata, error) {
	startTime := time.Now()
	var allItems []T
	metadata := AggregationMetadata{}

	for page := 0; page < maxPages; page++ {
		resp, err := fetcher(page)
		if err != nil {
			return nil, metadata, fmt.Errorf("failed to fetch page %d: %w", page, err)
		}

		allItems = append(allItems, resp.Content...)
		metadata.PagesTraversed++
		metadata.RecordsFetched += len(resp.Content)

		logger.Debug("fetched page",
			"page", page,
			"items", len(resp.Content),
			"total_items", len(allItems))

		// Stop if this was the last page
		if resp.Last || len(resp.Content) == 0 {
			break
		}
	}

	metadata.RecordsProcessed = len(allItems)
	metadata.ExecutionTimeMs = time.Since(startTime).Milliseconds()

	return allItems, metadata, nil
}

// GetDefaultShopID returns the default shop ID from config or 0 if not set
func (b *BaseAnalysisTool) GetDefaultShopID() int {
	return b.config.Tekmetric.DefaultShopID
}

// MaxPages returns the page limit for one analysis. It is the safety limit on
// how much data a single tool call fetches.
func (b *BaseAnalysisTool) MaxPages() int {
	return b.config.Analysis.MaxPages
}

// MaxRecords returns the record limit for one analysis. It bounds the memory a
// single tool call uses.
func (b *BaseAnalysisTool) MaxRecords() int {
	return b.config.Analysis.MaxRecords
}

// AnalysisTimeout returns the time limit for one analysis.
func (b *BaseAnalysisTool) AnalysisTimeout() time.Duration {
	return time.Duration(b.config.Analysis.TimeoutSeconds) * time.Second
}

// AggregationError represents an error during aggregation with context
type AggregationError struct {
	Stage      string              // "fetch", "process", "format"
	Underlying error               // The underlying error
	Metadata   AggregationMetadata // What was completed before error
}

func (e *AggregationError) Error() string {
	return fmt.Sprintf("aggregation failed at %s stage: %v (fetched %d records across %d pages)",
		e.Stage, e.Underlying, e.Metadata.RecordsFetched, e.Metadata.PagesTraversed)
}

func (e *AggregationError) Unwrap() error {
	return e.Underlying
}
