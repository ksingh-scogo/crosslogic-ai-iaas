package monitor

import (
	"time"

	"github.com/crosslogic-ai-iaas/control-plane/pkg/database"
	"github.com/crosslogic-ai-iaas/control-plane/pkg/models"
)

// Monitor performs health tracking for nodes.
type Monitor struct {
	store database.Store
	now   func() time.Time
}

func NewMonitor(store database.Store) *Monitor {
	return &Monitor{store: store, now: time.Now}
}

// Heartbeat refreshes node metadata to keep registry current.
func (m *Monitor) Heartbeat(id string) {
	nodes := m.store.ListNodes()
	for _, n := range nodes {
		if n.ID == id {
			n.LastHeartbeat = m.now()
			m.store.SaveNode(n)
		}
	}
}

// StaleNodes lists nodes with outdated heartbeat data.
func (m *Monitor) StaleNodes(threshold time.Duration) []models.Node {
	nodes := m.store.ListNodes()
	stale := make([]models.Node, 0)
	for _, n := range nodes {
		if m.now().Sub(n.LastHeartbeat) > threshold {
			stale = append(stale, n)
		}
	}
	return stale
}
