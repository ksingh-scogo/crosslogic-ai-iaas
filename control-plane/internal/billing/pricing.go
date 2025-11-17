package billing

import (
	"context"
	"fmt"

	"github.com/crosslogic/control-plane/pkg/database"
	"go.uber.org/zap"
)

// PricingCalculator calculates costs
type PricingCalculator struct {
	db     *database.Database
	logger *zap.Logger
}

// NewPricingCalculator creates a new pricing calculator
func NewPricingCalculator(db *database.Database, logger *zap.Logger) *PricingCalculator {
	return &PricingCalculator{
		db:     db,
		logger: logger,
	}
}

// CalculateCost calculates the cost of a usage record in microdollars
func (pc *PricingCalculator) CalculateCost(ctx context.Context, usage *UsageRecord) (int64, error) {
	// Get model pricing
	var priceInputPerMillion, priceOutputPerMillion float64
	err := pc.db.Pool.QueryRow(ctx, `
		SELECT price_input_per_million, price_output_per_million
		FROM models
		WHERE id = $1
	`, usage.ModelID).Scan(&priceInputPerMillion, &priceOutputPerMillion)
	if err != nil {
		return 0, fmt.Errorf("failed to get model pricing: %w", err)
	}

	// Get region multiplier
	var regionMultiplier float64 = 1.0
	err = pc.db.Pool.QueryRow(ctx, `
		SELECT cost_multiplier
		FROM regions
		WHERE id = $1
	`, usage.RegionID).Scan(&regionMultiplier)
	if err != nil {
		// If region not found, use default multiplier
		pc.logger.Warn("region not found, using default multiplier", zap.Error(err))
	}

	// Calculate cost
	// Cost = (prompt_tokens * input_price + completion_tokens * output_price) / 1,000,000 * region_multiplier
	// Convert to microdollars (multiply by 1,000,000)
	inputCost := float64(usage.PromptTokens) * priceInputPerMillion * regionMultiplier
	outputCost := float64(usage.CompletionTokens) * priceOutputPerMillion * regionMultiplier

	totalCostMicrodollars := int64(inputCost + outputCost)

	return totalCostMicrodollars, nil
}
