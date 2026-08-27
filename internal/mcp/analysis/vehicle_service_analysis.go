package analysis

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/beetlebugorg/tekmetric-mcp/internal/config"
	"github.com/beetlebugorg/tekmetric-mcp/pkg/tekmetric"
)

// VehicleServiceAnalysis fetches and analyzes a vehicle's complete service history,
// returning a prompt for Claude to interpret with automotive expertise.
type VehicleServiceAnalysis struct {
	BaseAnalysisTool
}

// NewVehicleServiceAnalysis creates a new vehicle service analysis tool
func NewVehicleServiceAnalysis(client *tekmetric.Client, cfg *config.Config, logger *slog.Logger) *VehicleServiceAnalysis {
	return &VehicleServiceAnalysis{
		BaseAnalysisTool: NewBaseAnalysisTool(client, cfg, logger),
	}
}

func (v *VehicleServiceAnalysis) Name() string {
	return "vehicle_service_analysis"
}

func (v *VehicleServiceAnalysis) Description() string {
	return "📋 Vehicle Service History Analysis - Fetches complete service timeline for a vehicle, " +
		"including all repair orders, service events, parts replaced, and costs. " +
		"Returns formatted data ready for analysis with dates, mileage, and spending patterns. " +
		"Perfect for understanding maintenance history, identifying recurring issues, and assessing vehicle health."
}

func (v *VehicleServiceAnalysis) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"vehicle_id": map[string]interface{}{
				"type":        "number",
				"description": "Vehicle ID to get service timeline for",
			},
			"shop_id": map[string]interface{}{
				"type":        "number",
				"description": "Shop ID (optional, uses default if not specified)",
			},
			"start_date": map[string]interface{}{
				"type":        "string",
				"description": "Optional start date to filter history (YYYY-MM-DD)",
			},
			"end_date": map[string]interface{}{
				"type":        "string",
				"description": "Optional end date to filter history (YYYY-MM-DD)",
			},
			"max_pages": map[string]interface{}{
				"type":        "number",
				"description": "Maximum pages to fetch. The server config sets the ceiling.",
			},
		},
		"required": []string{"vehicle_id"},
	}
}

func (v *VehicleServiceAnalysis) Execute(
	ctx context.Context,
	params map[string]interface{},
) (*AnalysisResult, error) {
	// Parse parameters
	vehicleID, ok := params["vehicle_id"].(float64)
	if !ok {
		return nil, fmt.Errorf("vehicle_id is required and must be a number")
	}

	shopID := v.GetDefaultShopID()
	if sid, ok := params["shop_id"].(float64); ok {
		shopID = int(sid)
	}

	startDate := ""
	if sd, ok := params["start_date"].(string); ok {
		startDate = sd
	}

	endDate := ""
	if ed, ok := params["end_date"].(string); ok {
		endDate = ed
	}

	// The configured page limit is the ceiling. A caller may ask for fewer.
	maxPages := v.MaxPages()
	if mp, ok := params["max_pages"].(float64); ok && int(mp) < maxPages {
		maxPages = int(mp)
	}
	if maxPages < 1 {
		maxPages = 1
	}

	v.logger.Info("fetching vehicle service timeline",
		"vehicle_id", int(vehicleID),
		"shop_id", shopID,
		"start_date", startDate,
		"end_date", endDate,
		"max_pages", maxPages)

	// 1. Fetch vehicle info
	vehicle, err := v.client.GetVehicle(ctx, int(vehicleID))
	if err != nil {
		return nil, &AggregationError{
			Stage:      "fetch",
			Underlying: fmt.Errorf("failed to fetch vehicle: %w", err),
			Metadata:   AggregationMetadata{},
		}
	}

	// 2. Fetch all repair orders for this vehicle
	if timeout := v.AnalysisTimeout(); timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	repairOrders, metadata, err := FetchAllPages(ctx, v.logger, func(page int) (*tekmetric.PaginatedResponse[tekmetric.RepairOrder], error) {
		queryParams := tekmetric.RepairOrderQueryParams{
			Shop:          shopID,
			VehicleID:     int(vehicleID),
			Page:          page,
			Size:          100,
			Sort:          "createdDate",
			SortDirection: "DESC",
		}
		if startDate != "" {
			queryParams.Start = startDate
		}
		if endDate != "" {
			queryParams.End = endDate
		}
		return v.client.GetRepairOrdersWithParams(ctx, queryParams)
	}, maxPages)

	if err != nil {
		return nil, &AggregationError{
			Stage:      "fetch",
			Underlying: fmt.Errorf("failed to fetch repair orders: %w", err),
			Metadata:   metadata,
		}
	}

	// Drop the tail when the result is over the record limit.
	if limit := v.MaxRecords(); limit > 0 && len(repairOrders) > limit {
		v.logger.Warn("truncated analysis to the record limit",
			"fetched", len(repairOrders), "limit", limit)
		repairOrders = repairOrders[:limit]
		metadata.RecordsProcessed = limit
	}

	// Sort chronologically (oldest first for timeline)
	sort.Slice(repairOrders, func(i, j int) bool {
		return repairOrders[i].CreatedDate.Before(repairOrders[j].CreatedDate)
	})

	// 3. Process the data
	timeline := v.buildTimeline(repairOrders)
	stats := v.calculateStats(repairOrders)

	// 4. Generate summary
	summary := v.formatSummary(vehicle, len(repairOrders), stats)

	// 5. Create the analysis prompt for Claude
	prompt := v.createAnalysisPrompt(vehicle, timeline, stats)

	return &AnalysisResult{
		Summary: summary,
		Prompt:  prompt,
		Data: map[string]interface{}{
			"vehicle":  vehicle,
			"timeline": timeline,
			"stats":    stats,
		},
		Metadata: metadata,
	}, nil
}

// TimelineEvent represents a single service event - concise but complete
type TimelineEvent struct {
	Date             string   `json:"date"`               // YYYY-MM-DD format
	Mileage          int      `json:"mileage"`            // Odometer reading
	Services         []string `json:"services"`           // Services performed
	Parts            []string `json:"parts,omitempty"`    // Parts replaced (concise)
	Cost             float64  `json:"cost"`               // Total cost
	LaborHours       float64  `json:"labor_hours"`        // Labor time
	CustomerConcerns []string `json:"concerns,omitempty"` // What customer reported
	Status           string   `json:"status"`             // Order status
	RONumber         int      `json:"ro_number"`          // Reference number
}

// ServiceStats holds aggregate statistics about service history
type ServiceStats struct {
	TotalVisits       int     `json:"total_visits"`
	TotalSpent        float64 `json:"total_spent"`
	TotalLaborHours   float64 `json:"total_labor_hours"`
	AverageVisitCost  float64 `json:"average_visit_cost"`
	FirstVisitDate    string  `json:"first_visit_date"`
	LastVisitDate     string  `json:"last_visit_date"`
	MileageRange      string  `json:"mileage_range"`
	CompletedOrders   int     `json:"completed_orders"`
	EstimatesDeclined int     `json:"estimates_declined"`
}

func (v *VehicleServiceAnalysis) buildTimeline(ros []tekmetric.RepairOrder) []TimelineEvent {
	timeline := make([]TimelineEvent, 0, len(ros))

	for _, ro := range ros {
		// Extract services and parts (concise format)
		services := make([]string, 0)
		parts := make([]string, 0)
		totalLaborHours := 0.0

		for _, job := range ro.Jobs {
			if job.Name != "" {
				services = append(services, job.Name)
			}
			totalLaborHours += job.LaborHours

			// Extract key parts (concise - just part name)
			for _, part := range job.Parts {
				if part.Name != "" && part.Quantity > 0 {
					parts = append(parts, part.Name)
				}
			}
		}

		// Skip entries with no services
		if len(services) == 0 {
			continue
		}

		// Extract customer concerns
		concerns := make([]string, 0)
		for _, concern := range ro.CustomerConcerns {
			if concern.Concern != "" {
				concerns = append(concerns, concern.Concern)
			}
		}

		// Get mileage
		mileage := 0
		if ro.MilesIn != nil {
			mileage = int(*ro.MilesIn)
		}

		event := TimelineEvent{
			Date:             ro.CreatedDate.Format("2006-01-02"),
			Mileage:          mileage,
			Services:         services,
			Parts:            parts,
			Cost:             float64(ro.TotalSales) / 100.0,
			LaborHours:       totalLaborHours,
			CustomerConcerns: concerns,
			Status:           ro.RepairOrderStatus.Name,
			RONumber:         ro.RepairOrderNumber,
		}

		timeline = append(timeline, event)
	}

	return timeline
}

func (v *VehicleServiceAnalysis) calculateStats(ros []tekmetric.RepairOrder) ServiceStats {
	if len(ros) == 0 {
		return ServiceStats{}
	}

	stats := ServiceStats{
		TotalVisits: len(ros),
	}

	var totalSpent int64
	var totalLaborHours float64
	var minMileage, maxMileage float64
	var firstDate, lastDate time.Time
	completedCount := 0
	estimatesDeclined := 0

	for i, ro := range ros {
		totalSpent += int64(ro.TotalSales)

		// Count labor hours
		for _, job := range ro.Jobs {
			totalLaborHours += job.LaborHours
		}

		// Track mileage range
		if ro.MilesIn != nil {
			if i == 0 || *ro.MilesIn < minMileage {
				minMileage = *ro.MilesIn
			}
			if *ro.MilesIn > maxMileage {
				maxMileage = *ro.MilesIn
			}
		}

		// Track dates
		if i == 0 || ro.CreatedDate.Before(firstDate) {
			firstDate = ro.CreatedDate
		}
		if i == 0 || ro.CreatedDate.After(lastDate) {
			lastDate = ro.CreatedDate
		}

		// Count status types
		if ro.RepairOrderStatus.Code == "COMPLETE" || ro.RepairOrderStatus.Code == "POSTED" {
			completedCount++
		} else if ro.RepairOrderStatus.Code == "ESTIMATE" {
			estimatesDeclined++
		}
	}

	stats.TotalSpent = float64(totalSpent) / 100.0
	stats.TotalLaborHours = totalLaborHours
	stats.AverageVisitCost = stats.TotalSpent / float64(len(ros))
	stats.FirstVisitDate = firstDate.Format("2006-01-02")
	stats.LastVisitDate = lastDate.Format("2006-01-02")
	stats.MileageRange = fmt.Sprintf("%.0f - %.0f miles", minMileage, maxMileage)
	stats.CompletedOrders = completedCount
	stats.EstimatesDeclined = estimatesDeclined

	return stats
}

func (v *VehicleServiceAnalysis) formatSummary(vehicle *tekmetric.Vehicle, roCount int, stats ServiceStats) string {
	return fmt.Sprintf(
		"Vehicle Service Timeline for %d %s %s (VIN: %s)\n"+
			"Total service visits: %d\n"+
			"Total spent: $%.2f (avg $%.2f per visit)\n"+
			"Service period: %s to %s\n"+
			"Mileage range: %s",
		vehicle.Year, vehicle.Make, vehicle.Model,
		vehicle.VIN,
		roCount,
		stats.TotalSpent, stats.AverageVisitCost,
		stats.FirstVisitDate, stats.LastVisitDate,
		stats.MileageRange,
	)
}

func (v *VehicleServiceAnalysis) createAnalysisPrompt(
	vehicle *tekmetric.Vehicle,
	timeline []TimelineEvent,
	stats ServiceStats,
) string {
	return fmt.Sprintf(`📋 **Service History for %d %s %s**

Present this complete service timeline in a **concise, well-organized format**:

## 1. Service Timeline Table

Create a clean markdown table. Include ALL details but keep them **tight**:
| Date | Miles | Services & Parts | Hours | Cost | Notes |

- Combine services + key parts in one column (use • or commas)
- Add customer concerns in Notes if present
- Keep each row scannable

## 2. Analysis Summary

Brief overview (3-5 bullets max):
- Service frequency & spending patterns
- Most common service categories (use your automotive knowledge)
- Any recurring issues or notable patterns
- Maintenance schedule adherence (if obvious)

**Format for scannability**: Use markdown tables, bold headers, and bullet points. Be complete but concise.`,
		vehicle.Year, vehicle.Make, vehicle.Model)
}
