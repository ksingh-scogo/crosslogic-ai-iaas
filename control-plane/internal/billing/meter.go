package billing

import (
	"context"
	"fmt"
	"time"

	"github.com/crosslogic/control-plane/pkg/database"
	"go.uber.org/zap"
)

// TokenMeter records token usage
type TokenMeter struct {
	db     *database.Database
	logger *zap.Logger
}

// NewTokenMeter creates a new token meter
func NewTokenMeter(db *database.Database, logger *zap.Logger) *TokenMeter {
	return &TokenMeter{
		db:     db,
		logger: logger,
	}
}

// Record records token usage
func (tm *TokenMeter) Record(ctx context.Context, usage *UsageRecord, cost int64) error {
	_, err := tm.db.Pool.Exec(ctx, `
		INSERT INTO usage_records (
			request_id, tenant_id, environment_id, api_key_id,
			region_id, model_id, prompt_tokens, completion_tokens,
			total_tokens, latency_ms, cost_microdollars, billed
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, false
		)
	`,
		usage.RequestID,
		usage.TenantID,
		usage.EnvironmentID,
		usage.APIKeyID,
		usage.RegionID,
		usage.ModelID,
		usage.PromptTokens,
		usage.CompletionTokens,
		usage.TotalTokens,
		usage.LatencyMs,
		cost,
	)
	if err != nil {
		return fmt.Errorf("failed to insert usage record: %w", err)
	}

	return nil
}
