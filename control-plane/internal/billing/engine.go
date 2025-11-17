package billing

import (
	"context"
	"fmt"
	"time"

	"github.com/crosslogic/control-plane/pkg/database"
	"github.com/crosslogic/control-plane/pkg/models"
	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/usagerecord"
	"go.uber.org/zap"
)

// Engine handles billing operations
type Engine struct {
	db         *database.Database
	logger     *zap.Logger
	meter      *TokenMeter
	pricer     *PricingCalculator
	stripeKey  string
}

// NewEngine creates a new billing engine
func NewEngine(db *database.Database, logger *zap.Logger, stripeKey string) *Engine {
	stripe.Key = stripeKey

	return &Engine{
		db:        db,
		logger:    logger,
		meter:     NewTokenMeter(db, logger),
		pricer:    NewPricingCalculator(db, logger),
		stripeKey: stripeKey,
	}
}

// RecordUsage records token usage for a request
func (e *Engine) RecordUsage(ctx context.Context, usage *UsageRecord) error {
	// Calculate cost
	cost, err := e.pricer.CalculateCost(ctx, usage)
	if err != nil {
		e.logger.Error("failed to calculate cost", zap.Error(err))
		cost = 0 // Don't fail on cost calculation
	}

	// Record to meter
	if err := e.meter.Record(ctx, usage, cost); err != nil {
		return fmt.Errorf("failed to record usage: %w", err)
	}

	e.logger.Debug("recorded usage",
		zap.String("tenant_id", usage.TenantID.String()),
		zap.String("model", usage.Model),
		zap.Int("total_tokens", usage.TotalTokens),
		zap.Int64("cost_microdollars", cost),
	)

	return nil
}

// ExportToStripe exports unbilled usage to Stripe
func (e *Engine) ExportToStripe(ctx context.Context) error {
	// Get unbilled usage grouped by tenant
	rows, err := e.db.Pool.Query(ctx, `
		SELECT
			tenant_id,
			SUM(total_tokens) as total_tokens,
			SUM(cost_microdollars) as total_cost
		FROM usage_records
		WHERE billed = false
			AND timestamp >= NOW() - INTERVAL '1 hour'
		GROUP BY tenant_id
	`)
	if err != nil {
		return fmt.Errorf("failed to query unbilled usage: %w", err)
	}
	defer rows.Close()

	successCount := 0
	failureCount := 0

	for rows.Next() {
		var tenantID uuid.UUID
		var totalTokens int64
		var totalCost int64

		if err := rows.Scan(&tenantID, &totalTokens, &totalCost); err != nil {
			e.logger.Error("failed to scan usage", zap.Error(err))
			continue
		}

		// Get Stripe customer ID
		var stripeCustomerID *string
		err := e.db.Pool.QueryRow(ctx, `
			SELECT stripe_customer_id
			FROM tenants
			WHERE id = $1 AND stripe_customer_id IS NOT NULL
		`, tenantID).Scan(&stripeCustomerID)
		if err != nil || stripeCustomerID == nil {
			e.logger.Warn("tenant has no Stripe customer ID",
				zap.String("tenant_id", tenantID.String()),
			)
			continue
		}

		// Create Stripe usage record
		params := &stripe.UsageRecordParams{
			Quantity:           stripe.Int64(totalTokens),
			Timestamp:          stripe.Int64(time.Now().Unix()),
			Action:             stripe.String(string(stripe.UsageRecordActionIncrement)),
		}

		// TODO: Use actual subscription item ID
		// For now, this is a placeholder
		subscriptionItemID := "si_placeholder"

		_, err = usagerecord.New(&stripe.UsageRecordParams{
			Params:             stripe.Params{Context: ctx},
			Quantity:           params.Quantity,
			Timestamp:          params.Timestamp,
			Action:             params.Action,
			SubscriptionItem:   stripe.String(subscriptionItemID),
		})
		if err != nil {
			e.logger.Error("failed to create Stripe usage record",
				zap.Error(err),
				zap.String("tenant_id", tenantID.String()),
			)
			failureCount++

			// Mark as billing failed
			e.markBillingFailed(ctx, tenantID)
			continue
		}

		// Mark usage as billed
		_, err = e.db.Pool.Exec(ctx, `
			UPDATE usage_records
			SET billed = true
			WHERE tenant_id = $1 AND billed = false
				AND timestamp >= NOW() - INTERVAL '1 hour'
		`, tenantID)
		if err != nil {
			e.logger.Error("failed to mark usage as billed", zap.Error(err))
		}

		// Record billing event
		e.recordBillingEvent(ctx, tenantID, totalTokens, totalCost)

		successCount++
	}

	e.logger.Info("exported usage to Stripe",
		zap.Int("success", successCount),
		zap.Int("failure", failureCount),
	)

	return nil
}

// markBillingFailed marks usage records as billing failed
func (e *Engine) markBillingFailed(ctx context.Context, tenantID uuid.UUID) {
	_, err := e.db.Pool.Exec(ctx, `
		UPDATE usage_records
		SET billing_failed = true, retry_count = retry_count + 1
		WHERE tenant_id = $1 AND billed = false
			AND timestamp >= NOW() - INTERVAL '1 hour'
	`, tenantID)
	if err != nil {
		e.logger.Error("failed to mark billing failed", zap.Error(err))
	}
}

// recordBillingEvent records a billing event
func (e *Engine) recordBillingEvent(ctx context.Context, tenantID uuid.UUID, tokens, cost int64) {
	_, err := e.db.Pool.Exec(ctx, `
		INSERT INTO billing_events (
			tenant_id, event_type, amount_microdollars, currency,
			description, period_start, period_end, status
		) VALUES (
			$1, 'usage', $2, 'USD', $3, $4, $5, 'processed'
		)
	`,
		tenantID,
		cost,
		fmt.Sprintf("Usage: %d tokens", tokens),
		time.Now().Add(-1*time.Hour),
		time.Now(),
	)
	if err != nil {
		e.logger.Error("failed to record billing event", zap.Error(err))
	}
}

// AggregateHourlyUsage aggregates usage into hourly buckets
func (e *Engine) AggregateHourlyUsage(ctx context.Context) error {
	_, err := e.db.Pool.Exec(ctx, `
		INSERT INTO usage_hourly (
			hour, tenant_id, environment_id, model_id, region_id,
			total_tokens, total_requests, total_cost_microdollars,
			avg_latency_ms
		)
		SELECT
			date_trunc('hour', timestamp) as hour,
			tenant_id,
			environment_id,
			model_id,
			region_id,
			SUM(total_tokens) as total_tokens,
			COUNT(*) as total_requests,
			SUM(cost_microdollars) as total_cost_microdollars,
			AVG(latency_ms)::int as avg_latency_ms
		FROM usage_records
		WHERE timestamp >= date_trunc('hour', NOW() - INTERVAL '2 hours')
			AND timestamp < date_trunc('hour', NOW())
		GROUP BY
			date_trunc('hour', timestamp),
			tenant_id,
			environment_id,
			model_id,
			region_id
		ON CONFLICT (hour, tenant_id, environment_id, model_id, region_id)
		DO UPDATE SET
			total_tokens = EXCLUDED.total_tokens,
			total_requests = EXCLUDED.total_requests,
			total_cost_microdollars = EXCLUDED.total_cost_microdollars,
			avg_latency_ms = EXCLUDED.avg_latency_ms
	`)
	if err != nil {
		return fmt.Errorf("failed to aggregate hourly usage: %w", err)
	}

	e.logger.Info("aggregated hourly usage")
	return nil
}

// StartBackgroundJobs starts background billing jobs
func (e *Engine) StartBackgroundJobs(ctx context.Context) {
	// Export to Stripe every 5 minutes
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := e.ExportToStripe(ctx); err != nil {
					e.logger.Error("failed to export to Stripe", zap.Error(err))
				}
			}
		}
	}()

	// Aggregate hourly usage every hour
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := e.AggregateHourlyUsage(ctx); err != nil {
					e.logger.Error("failed to aggregate hourly usage", zap.Error(err))
				}
			}
		}
	}()

	e.logger.Info("started billing background jobs")
}

// UsageRecord represents a usage record to be billed
type UsageRecord struct {
	RequestID        string
	TenantID         uuid.UUID
	EnvironmentID    uuid.UUID
	APIKeyID         uuid.UUID
	RegionID         uuid.UUID
	ModelID          uuid.UUID
	Model            string
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	LatencyMs        int
}
