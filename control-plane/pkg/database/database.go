package database

import (
	"errors"
	"sync"
	"time"

	"github.com/crosslogic-ai-iaas/control-plane/pkg/models"
)

// Store defines persistence operations required by the control plane.
type Store interface {
	SaveTenant(models.Tenant)
	FindTenantByKey(string) (models.Tenant, bool)
	SaveAPIKey(models.APIKey)
	FindAPIKey(string) (models.APIKey, bool)
	SaveNode(models.Node)
	ListNodes() []models.Node
	ListNodesByRegionAndModel(region, model string) []models.Node
	SaveUsage(models.UsageRecord)
	ListUsageByTenant(string) []models.UsageRecord
}

// InMemoryStore provides a SQLite-like developer experience without dependencies.
type InMemoryStore struct {
	mu      sync.RWMutex
	tenants map[string]models.Tenant
	keys    map[string]models.APIKey
	nodes   map[string]models.Node
	usage   []models.UsageRecord
}

// NewInMemoryStore returns a thread-safe in-memory store.
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		tenants: make(map[string]models.Tenant),
		keys:    make(map[string]models.APIKey),
		nodes:   make(map[string]models.Node),
		usage:   make([]models.UsageRecord, 0),
	}
}

func (s *InMemoryStore) SaveTenant(t models.Tenant) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if t.CreatedAt.IsZero() {
		t.CreatedAt = time.Now()
	}
	s.tenants[t.ID] = t
}

func (s *InMemoryStore) FindTenantByKey(key string) (models.Tenant, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	k, ok := s.keys[key]
	if !ok {
		return models.Tenant{}, false
	}
	t, ok := s.tenants[k.TenantID]
	return t, ok
}

func (s *InMemoryStore) SaveAPIKey(k models.APIKey) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if k.CreatedAt.IsZero() {
		k.CreatedAt = time.Now()
	}
	s.keys[k.Key] = k
}

func (s *InMemoryStore) FindAPIKey(key string) (models.APIKey, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	k, ok := s.keys[key]
	if !ok {
		return models.APIKey{}, false
	}
	return k, true
}

func (s *InMemoryStore) SaveNode(n models.Node) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n.LastHeartbeat.IsZero() {
		n.LastHeartbeat = time.Now()
	}
	s.nodes[n.ID] = n
}

func (s *InMemoryStore) ListNodes() []models.Node {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]models.Node, 0, len(s.nodes))
	for _, n := range s.nodes {
		result = append(result, n)
	}
	return result
}

func (s *InMemoryStore) ListNodesByRegionAndModel(region, model string) []models.Node {
	s.mu.RLock()
	defer s.mu.RUnlock()
	filtered := make([]models.Node, 0)
	for _, n := range s.nodes {
		if n.Region == region && n.Model == model && n.Status == models.NodeStatusHealthy {
			filtered = append(filtered, n)
		}
	}
	return filtered
}

func (s *InMemoryStore) SaveUsage(u models.UsageRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if u.Timestamp.IsZero() {
		u.Timestamp = time.Now()
	}
	s.usage = append(s.usage, u)
}

func (s *InMemoryStore) ListUsageByTenant(tenantID string) []models.UsageRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	records := make([]models.UsageRecord, 0)
	for _, u := range s.usage {
		if u.TenantID == tenantID {
			records = append(records, u)
		}
	}
	return records
}

// ErrNotFound indicates an expected resource is missing.
var ErrNotFound = errors.New("not found")
