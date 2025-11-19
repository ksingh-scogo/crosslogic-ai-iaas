package billing

import (
	"context"
	"fmt"
	"time"

	"github.com/crosslogic/control-plane/pkg/database"
	"github.com/crosslogic/control-plane/pkg/metrics"
	"go.uber.org/zap"
)

// CostTracker tracks and aggregates costs per tenant
type CostTracker struct {
	db             *database.Database
	logger         *zap.Logger
	gpuPricing     *GPUPricingConfig
	aggregateInterval time.Duration
}

// TenantCostSummary represents cost summary for a tenant
type TenantCostSummary struct {
	TenantID string  `json:"tenant_id"`
	Period   string  `json:"period"` // e.g., "2025-01", "2025-01-19"

	// Compute costs
	ComputeCost     float64 `json:"compute_cost"`      // Total compute cost
	SpotCost        float64 `json:"spot_cost"`         // Cost from spot instances
	OnDemandCost    float64 `json:"ondemand_cost"`     // Cost from on-demand instances

	// Token costs
	TokenCost       float64 `json:"token_cost"`        // Cost from token usage
	InputTokens     int64   `json:"input_tokens"`      // Total input tokens
	OutputTokens    int64   `json:"output_tokens"`     // Total output tokens

	// Usage metrics
	TotalRequests   int64   `json:"total_requests"`    // Number of requests
	GPUHours        float64 `json:"gpu_hours"`         // Total GPU hours used
	SpotHours       float64 `json:"spot_hours"`        // Spot instance hours
	OnDemandHours   float64 `json:"ondemand_hours"`    // On-demand hours

	// Savings
	TotalCost       float64 `json:"total_cost"`        // Total cost
	PotentialCost   float64 `json:"potential_cost"`    // Cost if all on-demand
	Savings         float64 `json:"savings"`           // Amount saved using spot
	SavingsPercent  float64 `json:"savings_percent"`   // Savings percentage

	// Timestamps
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
}

// NewCostTracker creates a new cost tracker
func NewCostTracker(db *database.Database, logger *zap.Logger) *CostTracker {
	return &CostTracker{
		db:             db,
		logger:         logger,
		gpuPricing:     NewGPUPricingConfig(),
		aggregateInterval: 1 * time.Hour, // Aggregate costs every hour
	}
}

// Start begins the cost aggregation loop
func (ct *CostTracker) Start(ctx context.Context) {
	ct.logger.Info("starting cost tracker")
	go ct.aggregationLoop(ctx)
}

// aggregationLoop periodically aggregates costs
func (ct *CostTracker) aggregationLoop(ctx context.Context) {
	ticker := time.NewTicker(ct.aggregateInterval)
	defer ticker.Stop()

	// Run immediately on start
	ct.aggregateCosts(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ct.aggregateCosts(ctx)
		}
	}
}

// aggregateCosts aggregates costs for all tenants
func (ct *CostTracker) aggregateCosts(ctx context.Context) {
	ct.logger.Info("aggregating costs for all tenants")

	// Get all active tenants
	rows, err := ct.db.Pool.Query(ctx, "SELECT id FROM tenants WHERE status = 'active'")
	if err != nil {
		ct.logger.Error("failed to fetch tenants", zap.Error(err))
		return
	}
	defer rows.Close()

	for rows.Next() {
		var tenantID string
		if err := rows.Scan(&tenantID); err != nil {
			continue
		}

		// Calculate costs for this tenant
		if err := ct.calculateTenantCosts(ctx, tenantID); err != nil {
			ct.logger.Error("failed to calculate tenant costs",
				zap.String("tenant_id", tenantID),
				zap.Error(err),
			)
		}
	}
}

// calculateTenantCosts calculates and stores costs for a tenant
func (ct *CostTracker) calculateTenantCosts(ctx context.Context, tenantID string) error {
	// Calculate costs for current hour
	now := time.Now()
	startTime := now.Truncate(time.Hour)
	endTime := now

	summary, err := ct.GetTenantCosts(ctx, tenantID, startTime, endTime)
	if err != nil {
		return err
	}

	// Store in database
	return ct.storeCostSummary(ctx, summary)
}

// GetTenantCosts retrieves cost summary for a tenant in a time range
func (ct *CostTracker) GetTenantCosts(ctx context.Context, tenantID string, startTime, endTime time.Time) (*TenantCostSummary, error) {
	summary := &TenantCostSummary{
		TenantID:  tenantID,
		StartTime: startTime,
		EndTime:   endTime,
		Period:    startTime.Format("2006-01"),
	}

	// Query node usage for this tenant
	query := `
		SELECT
			n.gpu_type,
			n.gpu_count,
			n.spot_instance,
			EXTRACT(EPOCH FROM (COALESCE(n.terminated_at, NOW()) - n.created_at)) / 3600.0 as hours,
			COALESCE(SUM(ur.prompt_tokens), 0) as input_tokens,
			COALESCE(SUM(ur.completion_tokens), 0) as output_tokens,
			COALESCE(COUNT(ur.id), 0) as request_count
		FROM nodes n
		LEFT JOIN usage_records ur ON ur.node_id = n.id
			AND ur.created_at BETWEEN $2 AND $3
		WHERE n.tenant_id = $1
			AND n.created_at BETWEEN $2 AND $3
		GROUP BY n.id, n.gpu_type, n.gpu_count, n.spot_instance, n.created_at, n.terminated_at
	`

	rows, err := ct.db.Pool.Query(ctx, query, tenantID, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("failed to query node usage: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var gpuType string
		var gpuCount int
		var isSpot bool
		var hours float64
		var inputTokens, outputTokens, requests int64

		if err := rows.Scan(&gpuType, &gpuCount, &isSpot, &hours, &inputTokens, &outputTokens, &requests); err != nil {
			continue
		}

		// Calculate compute cost
		duration := time.Duration(hours * float64(time.Hour))
		computeCost := ct.gpuPricing.CalculateGPUHourlyCost(gpuType, duration, isSpot, gpuCount)

		// Calculate token cost
		totalTokens := inputTokens + outputTokens
		tokenCost := ct.gpuPricing.CalculateGPUTokenCost(gpuType, totalTokens)

		// Accumulate totals
		summary.ComputeCost += computeCost
		summary.TokenCost += tokenCost
		summary.InputTokens += inputTokens
		summary.OutputTokens += outputTokens
		summary.TotalRequests += requests
		summary.GPUHours += hours * float64(gpuCount)

		if isSpot {
			summary.SpotCost += computeCost
			summary.SpotHours += hours * float64(gpuCount)
		} else {
			summary.OnDemandCost += computeCost
			summary.OnDemandHours += hours * float64(gpuCount)
		}

		// Calculate potential cost if all on-demand
		onDemandCost := ct.gpuPricing.CalculateGPUHourlyCost(gpuType, duration, false, gpuCount)
		summary.PotentialCost += onDemandCost
	}

	// Calculate totals and savings
	summary.TotalCost = summary.ComputeCost + summary.TokenCost
	summary.Savings = summary.PotentialCost - summary.ComputeCost
	if summary.PotentialCost > 0 {
		summary.SavingsPercent = (summary.Savings / summary.PotentialCost) * 100
	}

	return summary, nil
}

// GetMonthlyTenantCosts retrieves monthly cost summary for a tenant
func (ct *CostTracker) GetMonthlyTenantCosts(ctx context.Context, tenantID string, year int, month int) (*TenantCostSummary, error) {
	startTime := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	endTime := startTime.AddDate(0, 1, 0)

	return ct.GetTenantCosts(ctx, tenantID, startTime, endTime)
}

// GetCurrentMonthCosts retrieves current month costs for a tenant
func (ct *CostTracker) GetCurrentMonthCosts(ctx context.Context, tenantID string) (*TenantCostSummary, error) {
	now := time.Now()
	return ct.GetMonthlyTenantCosts(ctx, tenantID, now.Year(), int(now.Month()))
}

// storeCostSummary stores the cost summary in the database
func (ct *CostTracker) storeCostSummary(ctx context.Context, summary *TenantCostSummary) error {
	query := `
		INSERT INTO tenant_cost_summary (
			tenant_id, period, start_time, end_time,
			compute_cost, spot_cost, ondemand_cost, token_cost,
			input_tokens, output_tokens, total_requests,
			gpu_hours, spot_hours, ondemand_hours,
			total_cost, potential_cost, savings, savings_percent,
			created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, NOW()
		)
		ON CONFLICT (tenant_id, period, start_time)
		DO UPDATE SET
			end_time = $4,
			compute_cost = $5,
			spot_cost = $6,
			ondemand_cost = $7,
			token_cost = $8,
			input_tokens = $9,
			output_tokens = $10,
			total_requests = $11,
			gpu_hours = $12,
			spot_hours = $13,
			ondemand_hours = $14,
			total_cost = $15,
			potential_cost = $16,
			savings = $17,
			savings_percent = $18,
			updated_at = NOW()
	`

	_, err := ct.db.Pool.Exec(ctx, query,
		summary.TenantID, summary.Period, summary.StartTime, summary.EndTime,
		summary.ComputeCost, summary.SpotCost, summary.OnDemandCost, summary.TokenCost,
		summary.InputTokens, summary.OutputTokens, summary.TotalRequests,
		summary.GPUHours, summary.SpotHours, summary.OnDemandHours,
		summary.TotalCost, summary.PotentialCost, summary.Savings, summary.SavingsPercent,
	)

	if err != nil {
		return fmt.Errorf("failed to store cost summary: %w", err)
	}

	ct.logger.Info("stored cost summary",
		zap.String("tenant_id", summary.TenantID),
		zap.String("period", summary.Period),
		zap.Float64("total_cost", summary.TotalCost),
		zap.Float64("savings", summary.Savings),
	)

	// Update Prometheus metrics for real-time visibility
	spotPercent := 0.0
	if summary.GPUHours > 0 {
		spotPercent = (summary.SpotHours / summary.GPUHours) * 100
	}

	metrics.UpdateCostMetrics(
		summary.TenantID,
		summary.TotalCost,
		summary.ComputeCost,
		summary.TokenCost,
		summary.Savings,
		spotPercent,
	)

	return nil
}

// GetTopSpendingTenants returns the top N tenants by spending
func (ct *CostTracker) GetTopSpendingTenants(ctx context.Context, startTime, endTime time.Time, limit int) ([]TenantCostSummary, error) {
	query := `
		SELECT
			tenant_id, period, start_time, end_time,
			compute_cost, spot_cost, ondemand_cost, token_cost,
			input_tokens, output_tokens, total_requests,
			gpu_hours, spot_hours, ondemand_hours,
			total_cost, potential_cost, savings, savings_percent
		FROM tenant_cost_summary
		WHERE start_time >= $1 AND end_time <= $2
		ORDER BY total_cost DESC
		LIMIT $3
	`

	rows, err := ct.db.Pool.Query(ctx, query, startTime, endTime, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var summaries []TenantCostSummary
	for rows.Next() {
		var s TenantCostSummary
		if err := rows.Scan(
			&s.TenantID, &s.Period, &s.StartTime, &s.EndTime,
			&s.ComputeCost, &s.SpotCost, &s.OnDemandCost, &s.TokenCost,
			&s.InputTokens, &s.OutputTokens, &s.TotalRequests,
			&s.GPUHours, &s.SpotHours, &s.OnDemandHours,
			&s.TotalCost, &s.PotentialCost, &s.Savings, &s.SavingsPercent,
		); err != nil {
			continue
		}
		summaries = append(summaries, s)
	}

	return summaries, nil
}
