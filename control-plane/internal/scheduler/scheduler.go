package scheduler

import (
	"errors"
	"sync/atomic"

	"github.com/crosslogic-ai-iaas/control-plane/pkg/database"
	"github.com/crosslogic-ai-iaas/control-plane/pkg/models"
	"github.com/crosslogic-ai-iaas/control-plane/pkg/telemetry"
)

// Scheduler selects nodes using strategies described in the PRD.
type Scheduler struct {
	store   database.Store
	logger  *telemetry.Logger
	counter uint64
}

// NewScheduler initializes a round-robin scheduler.
func NewScheduler(store database.Store, logger *telemetry.Logger) *Scheduler {
	return &Scheduler{store: store, logger: logger}
}

// SelectNode chooses a candidate node using round-robin among healthy nodes.
func (s *Scheduler) SelectNode(req models.Request) (models.Node, error) {
	candidates := s.store.ListNodesByRegionAndModel(req.Region, req.Model)
	if len(candidates) == 0 {
		candidates = s.store.ListNodes() // allow cross-region fallback for MVP
	}
	if len(candidates) == 0 {
		return models.Node{}, errors.New("no capacity available")
	}

	idx := int(atomic.AddUint64(&s.counter, 1)) % len(candidates)
	node := candidates[idx]
	s.logger.Info("scheduler", "selected", node.ID, "region", node.Region, "model", node.Model)
	return node, nil
}
