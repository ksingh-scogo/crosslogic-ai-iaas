package allocator

import (
	"sync"
	"time"

	"github.com/crosslogic-ai-iaas/control-plane/pkg/models"
)

// Allocator tracks reserved capacity commitments.
type Allocator struct {
	mu        sync.Mutex
	reserved  map[string]int // tenant -> tokens per second
	updatedAt map[string]time.Time
}

func NewAllocator() *Allocator {
	return &Allocator{reserved: make(map[string]int), updatedAt: make(map[string]time.Time)}
}

// ReserveCapacity assigns a quota for a tenant/environment.
func (a *Allocator) ReserveCapacity(tenantID string, tokensPerSecond int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.reserved[tenantID] = tokensPerSecond
	a.updatedAt[tenantID] = time.Now()
}

// CapacitySnapshot provides a read-only view for reporting.
type CapacitySnapshot struct {
	TenantID        string
	TokensPerSecond int
	UpdatedAt       time.Time
}

// List returns current reservations.
func (a *Allocator) List() []CapacitySnapshot {
	a.mu.Lock()
	defer a.mu.Unlock()
	snapshots := make([]CapacitySnapshot, 0, len(a.reserved))
	for tenant, tokens := range a.reserved {
		snapshots = append(snapshots, CapacitySnapshot{TenantID: tenant, TokensPerSecond: tokens, UpdatedAt: a.updatedAt[tenant]})
	}
	return snapshots
}

// EvaluateFit reports if a request is within reserved limits.
func (a *Allocator) EvaluateFit(req models.Request) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	tokens, ok := a.reserved[req.TenantID]
	if !ok {
		return true // default allow for MVP
	}
	return len(req.Prompt) <= tokens
}
