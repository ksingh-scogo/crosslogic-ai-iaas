package billing

import (
	"context"
	"fmt"
	"strings"
	"time"

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

// =============================================================================
// GPU-Based Tiered Pricing (Phase 3)
// =============================================================================

// GPUPricingTier represents the pricing for a specific GPU type
type GPUPricingTier struct {
	GPUType        string  `json:"gpu_type"`
	OnDemandRate   float64 `json:"ondemand_rate"`   // USD per GPU per hour
	SpotRate       float64 `json:"spot_rate"`       // USD per GPU per hour
	TokenRate      float64 `json:"token_rate"`      // USD per million tokens
	MinimumCharge  float64 `json:"minimum_charge"`  // Minimum hourly charge per GPU
	SpotDiscount   float64 `json:"spot_discount"`   // Discount percentage (0-100)
}

// GPUPricingConfig holds the pricing configuration for all GPU types
type GPUPricingConfig struct {
	tiers map[string]*GPUPricingTier

	// Default rates for unknown GPU types
	defaultOnDemandRate float64
	defaultSpotRate     float64
	defaultTokenRate    float64
}

// NewGPUPricingConfig creates a new GPU pricing configuration with default tiers
func NewGPUPricingConfig() *GPUPricingConfig {
	config := &GPUPricingConfig{
		tiers:               make(map[string]*GPUPricingTier),
		defaultOnDemandRate: 3.00,  // $3/hour default
		defaultSpotRate:     0.90,  // $0.90/hour default (70% discount)
		defaultTokenRate:    0.50,  // $0.50 per million tokens
	}

	// Initialize default pricing tiers
	config.addDefaultTiers()
	return config
}

// addDefaultTiers adds the default pricing for common GPU types
func (c *GPUPricingConfig) addDefaultTiers() {
	// NVIDIA A10G - Entry level inference
	c.AddTier(&GPUPricingTier{
		GPUType:        "A10G",
		OnDemandRate:   1.20,  // $1.20/hour per GPU
		SpotRate:       0.36,  // $0.36/hour per GPU (70% discount)
		TokenRate:      0.30,  // $0.30 per million tokens
		MinimumCharge:  0.02,  // $0.02 minimum per GPU
		SpotDiscount:   70.0,
	})

	// NVIDIA A100 40GB - High performance
	c.AddTier(&GPUPricingTier{
		GPUType:        "A100",
		OnDemandRate:   4.00,  // $4.00/hour per GPU
		SpotRate:       1.20,  // $1.20/hour per GPU
		TokenRate:      0.80,  // $0.80 per million tokens
		MinimumCharge:  0.07,
		SpotDiscount:   70.0,
	})

	// NVIDIA A100 80GB - Large models
	c.AddTier(&GPUPricingTier{
		GPUType:        "A100-80GB",
		OnDemandRate:   5.50,
		SpotRate:       1.65,
		TokenRate:      1.00,
		MinimumCharge:  0.09,
		SpotDiscount:   70.0,
	})

	// NVIDIA H100 - Premium performance
	c.AddTier(&GPUPricingTier{
		GPUType:        "H100",
		OnDemandRate:   8.00,
		SpotRate:       2.40,
		TokenRate:      1.50,
		MinimumCharge:  0.13,
		SpotDiscount:   70.0,
	})

	// NVIDIA H100 NVL - Highest tier
	c.AddTier(&GPUPricingTier{
		GPUType:        "H100-NVL",
		OnDemandRate:   10.00,
		SpotRate:       3.00,
		TokenRate:      2.00,
		MinimumCharge:  0.17,
		SpotDiscount:   70.0,
	})

	// NVIDIA L4 - Cost-effective inference
	c.AddTier(&GPUPricingTier{
		GPUType:        "L4",
		OnDemandRate:   0.80,
		SpotRate:       0.24,
		TokenRate:      0.20,
		MinimumCharge:  0.01,
		SpotDiscount:   70.0,
	})

	// NVIDIA V100 - Legacy but still useful
	c.AddTier(&GPUPricingTier{
		GPUType:        "V100",
		OnDemandRate:   2.50,
		SpotRate:       0.75,
		TokenRate:      0.60,
		MinimumCharge:  0.04,
		SpotDiscount:   70.0,
	})
}

// AddTier adds or updates a pricing tier
func (c *GPUPricingConfig) AddTier(tier *GPUPricingTier) {
	// Normalize GPU type (uppercase, no spaces)
	normalizedType := strings.ToUpper(strings.TrimSpace(tier.GPUType))
	c.tiers[normalizedType] = tier
}

// GetTier retrieves the pricing tier for a GPU type
func (c *GPUPricingConfig) GetTier(gpuType string) *GPUPricingTier {
	normalizedType := strings.ToUpper(strings.TrimSpace(gpuType))

	if tier, exists := c.tiers[normalizedType]; exists {
		return tier
	}

	// Return default tier if not found
	return &GPUPricingTier{
		GPUType:        gpuType,
		OnDemandRate:   c.defaultOnDemandRate,
		SpotRate:       c.defaultSpotRate,
		TokenRate:      c.defaultTokenRate,
		MinimumCharge:  0.05,
		SpotDiscount:   70.0,
	}
}

// CalculateGPUHourlyCost calculates the compute cost for a given duration
func (c *GPUPricingConfig) CalculateGPUHourlyCost(gpuType string, duration time.Duration, isSpot bool, gpuCount int) float64 {
	tier := c.GetTier(gpuType)

	// Calculate hours (rounded up to nearest minute)
	hours := duration.Hours()
	if hours < 0.0167 { // Less than 1 minute
		hours = 0.0167
	}

	// Select rate based on spot vs on-demand
	var ratePerHour float64
	if isSpot {
		ratePerHour = tier.SpotRate
	} else {
		ratePerHour = tier.OnDemandRate
	}

	// Calculate cost for multiple GPUs
	cost := ratePerHour * hours * float64(gpuCount)

	// Apply minimum charge
	minimumForCount := tier.MinimumCharge * float64(gpuCount)
	if cost < minimumForCount {
		cost = minimumForCount
	}

	return cost
}

// CalculateGPUTokenCost calculates the cost for tokens used
func (c *GPUPricingConfig) CalculateGPUTokenCost(gpuType string, tokenCount int64) float64 {
	tier := c.GetTier(gpuType)

	// Cost per million tokens
	millions := float64(tokenCount) / 1_000_000.0
	return tier.TokenRate * millions
}

// CalculateTotalGPUCost calculates the total cost combining compute time and tokens
func (c *GPUPricingConfig) CalculateTotalGPUCost(gpuType string, duration time.Duration, isSpot bool, gpuCount int, tokenCount int64) float64 {
	computeCost := c.CalculateGPUHourlyCost(gpuType, duration, isSpot, gpuCount)
	tokenCost := c.CalculateGPUTokenCost(gpuType, tokenCount)

	return computeCost + tokenCost
}

// EstimateMonthlyCost estimates the monthly cost for continuous usage
func (c *GPUPricingConfig) EstimateMonthlyCost(gpuType string, isSpot bool, gpuCount int, utilizationPercent float64) float64 {
	tier := c.GetTier(gpuType)

	// Hours in a month (average)
	hoursPerMonth := 730.0

	// Select rate
	var ratePerHour float64
	if isSpot {
		ratePerHour = tier.SpotRate
	} else {
		ratePerHour = tier.OnDemandRate
	}

	// Calculate with utilization
	actualHours := hoursPerMonth * (utilizationPercent / 100.0)
	return ratePerHour * actualHours * float64(gpuCount)
}

// GetSavingsPercentage calculates the savings percentage when using spot instances
func (c *GPUPricingConfig) GetSavingsPercentage(gpuType string) float64 {
	tier := c.GetTier(gpuType)
	return tier.SpotDiscount
}

// GetAllTiers returns all available pricing tiers
func (c *GPUPricingConfig) GetAllTiers() []*GPUPricingTier {
	tiers := make([]*GPUPricingTier, 0, len(c.tiers))
	for _, tier := range c.tiers {
		tiers = append(tiers, tier)
	}
	return tiers
}

// FormatCost formats a cost value as a string with currency
func FormatCost(cost float64) string {
	return fmt.Sprintf("$%.4f", cost)
}

// FormatCostCompact formats a cost value in a compact way
func FormatCostCompact(cost float64) string {
	if cost >= 1000 {
		return fmt.Sprintf("$%.2fK", cost/1000)
	}
	if cost >= 1 {
		return fmt.Sprintf("$%.2f", cost)
	}
	return fmt.Sprintf("$%.4f", cost)
}
