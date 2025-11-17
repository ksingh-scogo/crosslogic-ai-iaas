package router

import (
	"context"
	"errors"
	"fmt"

	"github.com/crosslogic-ai-iaas/control-plane/internal/scheduler"
	"github.com/crosslogic-ai-iaas/control-plane/pkg/database"
	"github.com/crosslogic-ai-iaas/control-plane/pkg/models"
	"github.com/crosslogic-ai-iaas/control-plane/pkg/telemetry"
)

// Router makes region/model aware routing decisions.
type Router struct {
	store     database.Store
	scheduler *scheduler.Scheduler
	logger    *telemetry.Logger
}

// NewRouter returns a request router with attached scheduler.
func NewRouter(store database.Store, scheduler *scheduler.Scheduler, logger *telemetry.Logger) *Router {
	return &Router{store: store, scheduler: scheduler, logger: logger}
}

// Route selects a target node and fabricates a response for MVP demonstration.
func (r *Router) Route(ctx context.Context, req models.Request) (models.Response, error) {
	node, err := r.scheduler.SelectNode(req)
	if err != nil {
		return models.Response{}, err
	}

	return models.Response{
		Model:    req.Model,
		Provider: node.Provider,
		Region:   node.Region,
		NodeID:   node.ID,
		Message:  fmt.Sprintf("routed to %s (%s)", node.ID, node.Region),
		Metadata: map[string]any{"endpoint": node.Endpoint},
	}, nil
}

// ModelRegistry placeholder for future expansion.
type ModelRegistry struct{}

// FallbackHandler placeholder to align with PRD requirements.
type FallbackHandler struct{}

// Fallback returns a well-formed fallback response.
func (f *FallbackHandler) Fallback(req models.Request) models.Response {
	return models.Response{
		Model:   req.Model,
		Region:  req.Region,
		Message: "fallback response",
	}
}

// RouterError helps categorize router failures.
type RouterError struct {
	Code    string
	Message string
}

func (e *RouterError) Error() string { return fmt.Sprintf("%s: %s", e.Code, e.Message) }

// ErrNoHealthyNodes is returned when no candidates exist.
var ErrNoHealthyNodes = errors.New("no healthy nodes found")
