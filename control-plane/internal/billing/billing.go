package billing

import (
	"time"

	"github.com/crosslogic-ai-iaas/control-plane/pkg/database"
	"github.com/crosslogic-ai-iaas/control-plane/pkg/models"
)

// Meter records usage for later export to Stripe or webhooks.
type Meter struct {
	store database.Store
}

func NewMeter(store database.Store) *Meter {
	return &Meter{store: store}
}

// RecordUsage captures a billing event aligned with the PRD schema.
func (m *Meter) RecordUsage(req models.Request, resp models.Response) {
	m.store.SaveUsage(models.UsageRecord{
		ID:           time.Now().Format("20060102150405"),
		TenantID:     req.TenantID,
		Model:        req.Model,
		Environment:  req.Environment,
		Region:       resp.Region,
		InputTokens:  resp.InputTokens,
		OutputTokens: resp.OutputTokens,
		CachedTokens: resp.CachedTokens,
		LatencyMs:    resp.LatencyMs,
		Timestamp:    time.Now(),
	})
}
