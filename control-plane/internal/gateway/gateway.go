package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/crosslogic/control-plane/pkg/cache"
	"github.com/crosslogic/control-plane/pkg/database"
	"github.com/crosslogic/control-plane/pkg/models"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Gateway handles API requests
type Gateway struct {
	db            *database.Database
	cache         *cache.Cache
	logger        *zap.Logger
	authenticator *Authenticator
	rateLimiter   *RateLimiter
	router        *chi.Mux
}

// NewGateway creates a new API gateway
func NewGateway(db *database.Database, cache *cache.Cache, logger *zap.Logger) *Gateway {
	g := &Gateway{
		db:            db,
		cache:         cache,
		logger:        logger,
		authenticator: NewAuthenticator(db, cache, logger),
		rateLimiter:   NewRateLimiter(cache, logger),
		router:        chi.NewRouter(),
	}

	g.setupRoutes()
	return g
}

// setupRoutes configures the HTTP routes
func (g *Gateway) setupRoutes() {
	// Middleware
	g.router.Use(middleware.RequestID)
	g.router.Use(middleware.RealIP)
	g.router.Use(g.loggerMiddleware)
	g.router.Use(middleware.Recoverer)
	g.router.Use(middleware.Timeout(60 * time.Second))

	// Health check (no auth required)
	g.router.Get("/health", g.handleHealth)
	g.router.Get("/ready", g.handleReady)

	// OpenAI-compatible endpoints (require auth)
	g.router.Group(func(r chi.Router) {
		r.Use(g.authMiddleware)
		r.Use(g.rateLimitMiddleware)

		// Chat completions
		r.Post("/v1/chat/completions", g.handleChatCompletions)
		r.Post("/v1/completions", g.handleCompletions)

		// Embeddings
		r.Post("/v1/embeddings", g.handleEmbeddings)

		// Models
		r.Get("/v1/models", g.handleListModels)
		r.Get("/v1/models/{model}", g.handleGetModel)
	})

	// Admin endpoints
	g.router.Group(func(r chi.Router) {
		r.Use(g.adminAuthMiddleware)

		r.Get("/admin/nodes", g.handleListNodes)
		r.Get("/admin/usage/{tenant_id}", g.handleGetUsage)
	})
}

// ServeHTTP implements http.Handler
func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	g.router.ServeHTTP(w, r)
}

// Middleware implementations

func (g *Gateway) loggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)

		g.logger.Info("request",
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.Int("status", ww.Status()),
			zap.Duration("duration", time.Since(start)),
			zap.String("remote_addr", r.RemoteAddr),
		)
	})
}

func (g *Gateway) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract API key from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			g.writeError(w, http.StatusUnauthorized, "missing authorization header")
			return
		}

		// Support both "Bearer" and direct key
		apiKey := strings.TrimPrefix(authHeader, "Bearer ")
		apiKey = strings.TrimSpace(apiKey)

		// Validate API key
		ctx := r.Context()
		keyInfo, err := g.authenticator.ValidateAPIKey(ctx, apiKey)
		if err != nil {
			g.logger.Warn("authentication failed", zap.Error(err))
			g.writeError(w, http.StatusUnauthorized, "invalid API key")
			return
		}

		// Add key info to context
		ctx = context.WithValue(ctx, "api_key", keyInfo)
		ctx = context.WithValue(ctx, "tenant_id", keyInfo.TenantID)
		ctx = context.WithValue(ctx, "environment_id", keyInfo.EnvironmentID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (g *Gateway) rateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Get API key from context
		keyInfo, ok := ctx.Value("api_key").(*models.APIKey)
		if !ok {
			g.writeError(w, http.StatusInternalServerError, "missing API key in context")
			return
		}

		// Check rate limits
		allowed, err := g.rateLimiter.CheckRateLimit(ctx, keyInfo)
		if err != nil {
			g.logger.Error("rate limit check failed", zap.Error(err))
			g.writeError(w, http.StatusInternalServerError, "rate limit check failed")
			return
		}

		if !allowed {
			g.writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (g *Gateway) adminAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// For now, simple token-based auth
		// In production, use proper admin authentication
		adminToken := r.Header.Get("X-Admin-Token")
		// TODO: Validate admin token
		if adminToken == "" {
			g.writeError(w, http.StatusUnauthorized, "missing admin token")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Handler implementations

func (g *Gateway) handleHealth(w http.ResponseWriter, r *http.Request) {
	g.writeJSON(w, http.StatusOK, map[string]string{
		"status": "healthy",
		"time":   time.Now().Format(time.RFC3339),
	})
}

func (g *Gateway) handleReady(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Check database
	if err := g.db.Health(ctx); err != nil {
		g.writeError(w, http.StatusServiceUnavailable, "database not ready")
		return
	}

	// Check cache
	if err := g.cache.Health(ctx); err != nil {
		g.writeError(w, http.StatusServiceUnavailable, "cache not ready")
		return
	}

	g.writeJSON(w, http.StatusOK, map[string]string{
		"status": "ready",
	})
}

func (g *Gateway) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse request
	var req ChatCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate request
	if err := req.Validate(); err != nil {
		g.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Get tenant/env info from context
	tenantID := ctx.Value("tenant_id").(uuid.UUID)
	envID := ctx.Value("environment_id").(uuid.UUID)

	g.logger.Info("chat completion request",
		zap.String("tenant_id", tenantID.String()),
		zap.String("env_id", envID.String()),
		zap.String("model", req.Model),
	)

	// TODO: Forward to scheduler/router
	// For now, return a mock response
	g.writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":      fmt.Sprintf("chatcmpl-%s", uuid.New().String()),
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   req.Model,
		"choices": []map[string]interface{}{
			{
				"index": 0,
				"message": map[string]string{
					"role":    "assistant",
					"content": "This is a placeholder response. Full implementation pending.",
				},
				"finish_reason": "stop",
			},
		},
		"usage": map[string]int{
			"prompt_tokens":     10,
			"completion_tokens": 20,
			"total_tokens":      30,
		},
	})
}

func (g *Gateway) handleCompletions(w http.ResponseWriter, r *http.Request) {
	g.writeError(w, http.StatusNotImplemented, "completions endpoint not yet implemented")
}

func (g *Gateway) handleEmbeddings(w http.ResponseWriter, r *http.Request) {
	g.writeError(w, http.StatusNotImplemented, "embeddings endpoint not yet implemented")
}

func (g *Gateway) handleListModels(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Query models from database
	rows, err := g.db.Pool.Query(ctx, `
		SELECT id, name, family, type, context_length, status
		FROM models
		WHERE status = 'active'
		ORDER BY name
	`)
	if err != nil {
		g.logger.Error("failed to query models", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to query models")
		return
	}
	defer rows.Close()

	var modelsList []map[string]interface{}
	for rows.Next() {
		var m models.Model
		if err := rows.Scan(&m.ID, &m.Name, &m.Family, &m.Type, &m.ContextLength, &m.Status); err != nil {
			continue
		}

		modelsList = append(modelsList, map[string]interface{}{
			"id":      m.Name,
			"object":  "model",
			"created": time.Now().Unix(),
			"owned_by": "crosslogic",
		})
	}

	g.writeJSON(w, http.StatusOK, map[string]interface{}{
		"object": "list",
		"data":   modelsList,
	})
}

func (g *Gateway) handleGetModel(w http.ResponseWriter, r *http.Request) {
	modelName := chi.URLParam(r, "model")

	g.writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":      modelName,
		"object":  "model",
		"created": time.Now().Unix(),
		"owned_by": "crosslogic",
	})
}

func (g *Gateway) handleListNodes(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	rows, err := g.db.Pool.Query(ctx, `
		SELECT id, provider, status, endpoint_url, health_score, last_heartbeat_at
		FROM nodes
		WHERE status IN ('active', 'draining')
		ORDER BY created_at DESC
		LIMIT 100
	`)
	if err != nil {
		g.logger.Error("failed to query nodes", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to query nodes")
		return
	}
	defer rows.Close()

	var nodes []models.Node
	for rows.Next() {
		var n models.Node
		if err := rows.Scan(&n.ID, &n.Provider, &n.Status, &n.EndpointURL, &n.HealthScore, &n.LastHeartbeatAt); err != nil {
			continue
		}
		nodes = append(nodes, n)
	}

	g.writeJSON(w, http.StatusOK, nodes)
}

func (g *Gateway) handleGetUsage(w http.ResponseWriter, r *http.Request) {
	tenantIDStr := chi.URLParam(r, "tenant_id")
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid tenant ID")
		return
	}

	ctx := r.Context()

	rows, err := g.db.Pool.Query(ctx, `
		SELECT hour, total_tokens, total_requests, total_cost_microdollars
		FROM usage_hourly
		WHERE tenant_id = $1
		ORDER BY hour DESC
		LIMIT 168
	`, tenantID)
	if err != nil {
		g.logger.Error("failed to query usage", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to query usage")
		return
	}
	defer rows.Close()

	var usage []models.UsageHourly
	for rows.Next() {
		var u models.UsageHourly
		if err := rows.Scan(&u.Hour, &u.TotalTokens, &u.TotalRequests, &u.TotalCostMicrodollars); err != nil {
			continue
		}
		usage = append(usage, u)
	}

	g.writeJSON(w, http.StatusOK, usage)
}

// Utility methods

func (g *Gateway) writeJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

func (g *Gateway) writeError(w http.ResponseWriter, statusCode int, message string) {
	g.writeJSON(w, statusCode, map[string]interface{}{
		"error": map[string]string{
			"message": message,
			"type":    "invalid_request_error",
		},
	})
}

// Request/Response types

type ChatCompletionRequest struct {
	Model       string                   `json:"model"`
	Messages    []ChatCompletionMessage  `json:"messages"`
	Temperature *float64                 `json:"temperature,omitempty"`
	MaxTokens   *int                     `json:"max_tokens,omitempty"`
	Stream      bool                     `json:"stream,omitempty"`
}

type ChatCompletionMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func (r *ChatCompletionRequest) Validate() error {
	if r.Model == "" {
		return fmt.Errorf("model is required")
	}
	if len(r.Messages) == 0 {
		return fmt.Errorf("messages are required")
	}
	return nil
}
