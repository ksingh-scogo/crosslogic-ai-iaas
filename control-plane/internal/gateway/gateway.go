package gateway

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/crosslogic-ai-iaas/control-plane/internal/router"
	"github.com/crosslogic-ai-iaas/control-plane/pkg/cache"
	"github.com/crosslogic-ai-iaas/control-plane/pkg/database"
	"github.com/crosslogic-ai-iaas/control-plane/pkg/models"
	"github.com/crosslogic-ai-iaas/control-plane/pkg/telemetry"
)

// Gateway wires authentication, validation, and routing.
type Gateway struct {
	authenticator *Authenticator
	rateLimiter   *RateLimiter
	validator     *RequestValidator
	router        *router.Router
	logger        *telemetry.Logger
}

// NewGateway constructs the gateway stack for MVP usage.
func NewGateway(store database.Store, cache *cache.LocalCache, router *router.Router, logger *telemetry.Logger) *Gateway {
	return &Gateway{
		authenticator: NewAuthenticator(store, cache),
		rateLimiter:   NewRateLimiter(cache, time.Second),
		validator:     &RequestValidator{},
		router:        router,
		logger:        logger,
	}
}

// ParseRequest extracts API key and basic fields from an HTTP request.
func (g *Gateway) ParseRequest(r *http.Request) models.Request {
	key := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	region := r.URL.Query().Get("region")
	model := r.URL.Query().Get("model")
	prompt := r.URL.Query().Get("prompt")

	return models.Request{
		APIKey: key,
		Region: region,
		Model:  model,
		Prompt: prompt,
	}
}

// HandleRequest executes authentication, rate limiting, routing, and accounting.
func (g *Gateway) HandleRequest(ctx context.Context, req models.Request) (models.Response, error) {
	key, tenant, err := g.authenticator.ValidateAPIKey(req.APIKey)
	if err != nil {
		return models.Response{}, err
	}

	if err := g.rateLimiter.Allow(key.Key); err != nil {
		return models.Response{}, err
	}

	enriched, err := g.validator.ValidateRequest(req, tenant)
	if err != nil {
		return models.Response{}, err
	}

	start := time.Now()
	response, err := g.router.Route(ctx, enriched)
	if err != nil {
		return models.Response{}, err
	}

	response.LatencyMs = time.Since(start).Milliseconds()
	response.InputTokens = int64(len(enriched.Prompt))
	response.OutputTokens = response.InputTokens / 2
	response.Timestamp = time.Now()

	return response, nil
}

// RequestValidator performs lightweight sanity checks.
type RequestValidator struct{}

// ValidateRequest ensures tenant/environment align with API key.
func (v *RequestValidator) ValidateRequest(req models.Request, tenant models.Tenant) (models.Request, error) {
	if req.Model == "" {
		return models.Request{}, errors.New("model is required")
	}
	if req.Region == "" {
		req.Region = tenant.Environment // simple assumption for MVP
	}
	req.TenantID = tenant.ID
	req.Environment = tenant.Environment
	return req, nil
}
