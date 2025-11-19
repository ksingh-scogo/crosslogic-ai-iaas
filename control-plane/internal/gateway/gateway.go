package gateway

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/crosslogic/control-plane/internal/billing"
	"github.com/crosslogic/control-plane/internal/orchestrator"
	"github.com/crosslogic/control-plane/pkg/cache"
	"github.com/crosslogic/control-plane/pkg/database"
	"github.com/crosslogic/control-plane/pkg/events"
	"github.com/crosslogic/control-plane/pkg/models"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const defaultEstimatedTokens = 1024

// Gateway handles API requests
type Gateway struct {
	db             *database.Database
	cache          *cache.Cache
	logger         *zap.Logger
	authenticator  *Authenticator
	rateLimiter    *RateLimiter
	router         *chi.Mux
	webhookHandler *billing.WebhookHandler
	orchestrator   *orchestrator.SkyPilotOrchestrator
	monitor        *orchestrator.TripleSafetyMonitor
	adminToken     string
	eventBus       *events.Bus
	loadBalancer   *IntelligentLoadBalancer
}

// NewGateway creates a new API gateway
func NewGateway(db *database.Database, cache *cache.Cache, logger *zap.Logger, webhookHandler *billing.WebhookHandler, orch *orchestrator.SkyPilotOrchestrator, monitor *orchestrator.TripleSafetyMonitor, adminToken string, eventBus *events.Bus) *Gateway {
	g := &Gateway{
		db:             db,
		cache:          cache,
		logger:         logger,
		authenticator:  NewAuthenticator(db, cache, logger),
		rateLimiter:    NewRateLimiter(cache, logger),
		router:         chi.NewRouter(),
		webhookHandler: webhookHandler,
		orchestrator:   orch,
		monitor:        monitor,
		adminToken:     adminToken,
		eventBus:       eventBus,
		loadBalancer:   NewIntelligentLoadBalancer(db, logger),
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
	g.router.Use(g.metricsMiddleware) // Add metrics middleware
	g.router.Use(middleware.Recoverer)
	g.router.Use(middleware.Timeout(60 * time.Second))

	// CORS
	g.router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000", "https://*.crosslogic.ai"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-Admin-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300, // Maximum value not ignored by any of major browsers
	}))

	// Metrics endpoint
	g.registerMetrics()

	// Health check (no auth required)
	g.router.Get("/health", g.handleHealth)
	g.router.Get("/ready", g.handleReady)

	// Stripe webhook endpoint (no auth - uses signature verification)
	g.router.Post("/api/webhooks/stripe", g.webhookHandler.HandleWebhook)

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

		// Node management
		r.Get("/admin/nodes", g.handleListNodes)
		r.Post("/admin/nodes/launch", g.handleLaunchNode)
		r.Post("/admin/nodes/{cluster_name}/terminate", g.handleTerminateNode)
		r.Get("/admin/nodes/{cluster_name}/status", g.handleNodeStatus)
		r.Post("/admin/nodes/{node_id}/heartbeat", g.handleHeartbeat)
		r.Post("/admin/nodes/{node_id}/termination-warning", g.handleTerminationWarning)

		// Usage and billing
		r.Get("/admin/usage/{tenant_id}", g.handleGetUsage)

		// API Keys
		r.Get("/admin/api-keys/{tenant_id}", g.handleListAPIKeys)
		r.Post("/admin/api-keys", g.handleCreateAPIKey)
		r.Delete("/admin/api-keys/{key_id}", g.handleRevokeAPIKey)

		// Tenant management
		r.Post("/admin/tenants", g.handleCreateTenant)
		r.Post("/admin/tenants/resolve", g.handleResolveTenant)
		r.Get("/admin/tenants/{tenant_id}", g.handleGetTenant)
	})
}

func (g *Gateway) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	nodeID := chi.URLParam(r, "node_id")
	if nodeID == "" {
		g.writeError(w, http.StatusBadRequest, "node_id is required")
		return
	}

	var req struct {
		HealthScore float64 `json:"health_score"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := g.monitor.RecordHeartbeat(r.Context(), nodeID, req.HealthScore); err != nil {
		g.logger.Error("failed to record heartbeat", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to record heartbeat")
		return
	}

	g.writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (g *Gateway) handleTerminationWarning(w http.ResponseWriter, r *http.Request) {
	nodeID := chi.URLParam(r, "node_id")
	if nodeID == "" {
		g.writeError(w, http.StatusBadRequest, "node_id is required")
		return
	}

	g.logger.Warn("received spot termination warning", zap.String("node_id", nodeID))

	// Mark node as terminating
	query := `UPDATE nodes SET status = 'terminating', status_message = 'spot_termination_warning' WHERE id = $1`
	_, err := g.db.Pool.Exec(r.Context(), query, nodeID)
	if err != nil {
		g.logger.Error("failed to update node status", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to process warning")
		return
	}

	// Publish event
	if g.eventBus != nil {
		g.eventBus.Publish(r.Context(), events.NewEvent(events.EventNodeTerminated, "", map[string]interface{}{
			"node_id": nodeID,
			"reason":  "spot_termination",
		}))
	}

	g.writeJSON(w, http.StatusOK, map[string]string{"status": "received"})
}

// StartHealthMetrics starts a background goroutine to update dependency health metrics
func (g *Gateway) StartHealthMetrics(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				g.updateHealthMetrics(ctx)
			}
		}
	}()
}

func (g *Gateway) updateHealthMetrics(ctx context.Context) {
	// Check Database
	dbStatus := 0.0
	if err := g.db.Health(ctx); err == nil {
		dbStatus = 1.0
	}
	dependencyUp.WithLabelValues("postgres").Set(dbStatus)

	// Check Redis
	redisStatus := 0.0
	if err := g.cache.Health(ctx); err == nil {
		redisStatus = 1.0
	}
	dependencyUp.WithLabelValues("redis").Set(redisStatus)
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
			zap.String("request_id", middleware.GetReqID(r.Context())),
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

		defer func(keyID string) {
			releaseCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if err := g.rateLimiter.DecrementConcurrency(releaseCtx, keyID); err != nil {
				g.logger.Debug("failed to decrement concurrency",
					zap.String("key_id", keyID),
					zap.Error(err),
				)
			}
		}(keyInfo.ID.String())

		next.ServeHTTP(w, r)
	})
}

func (g *Gateway) adminAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		adminToken := r.Header.Get("X-Admin-Token")
		if adminToken == "" {
			g.writeError(w, http.StatusUnauthorized, "missing admin token")
			return
		}

		// Constant-time comparison to prevent timing attacks
		if subtle.ConstantTimeCompare([]byte(adminToken), []byte(g.adminToken)) != 1 {
			g.logger.Warn("invalid admin token attempt",
				zap.String("remote_addr", r.RemoteAddr),
				zap.String("path", r.URL.Path),
			)
			g.writeError(w, http.StatusUnauthorized, "invalid admin token")
			return
		}

		// Audit log for admin actions
		g.logger.Info("admin action authenticated",
			zap.String("request_id", middleware.GetReqID(r.Context())),
			zap.String("remote_addr", r.RemoteAddr),
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
		)

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

	// Read request body for parsing and forwarding
	body, err := io.ReadAll(r.Body)
	if err != nil {
		g.writeError(w, http.StatusBadRequest, "failed to read request body")
		return
	}
	r.Body.Close()

	// Parse request for validation and routing
	var req ChatCompletionRequest
	if err := json.Unmarshal(body, &req); err != nil {
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
		zap.Bool("streaming", req.Stream),
	)

	// Get environment details for region preference
	var envRegion string
	err = g.db.Pool.QueryRow(ctx, `
		SELECT region FROM environments
		WHERE id = $1 AND tenant_id = $2 AND status = 'active'
	`, envID, tenantID).Scan(&envRegion)
	if err != nil {
		g.logger.Error("failed to get environment",
			zap.Error(err),
			zap.String("env_id", envID.String()),
		)
		// Continue without region preference
	}

	// Select best endpoint
	endpoint, err := g.loadBalancer.SelectEndpoint(ctx, req.Model)
	if err != nil {
		g.logger.Error("failed to select endpoint", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to select endpoint")
		return
	}

	if endpoint == "" {
		g.writeError(w, http.StatusServiceUnavailable, "no healthy nodes for model")
		return
	}

	// Proxy request to endpoint
	// Re-create body reader for proxying
	r.Body = io.NopCloser(bytes.NewBuffer(body))

	start := time.Now()
	resp, err := g.proxyRequest(endpoint, r)
	duration := time.Since(start)

	// Record stats
	isError := err != nil || (resp != nil && resp.StatusCode >= 500)
	g.loadBalancer.RecordRequest(endpoint, duration, isError)

	if err != nil {
		g.logger.Error("failed to proxy request", zap.Error(err))
		g.writeError(w, http.StatusBadGateway, "failed to proxy request")
		return
	}
	defer resp.Body.Close()

	// Copy response headers
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func (g *Gateway) handleCompletions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Read request body for parsing and forwarding
	body, err := io.ReadAll(r.Body)
	if err != nil {
		g.writeError(w, http.StatusBadRequest, "failed to read request body")
		return
	}
	r.Body.Close()

	// Parse request for validation
	var req CompletionRequest
	if err := json.Unmarshal(body, &req); err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate request
	if req.Model == "" {
		g.writeError(w, http.StatusBadRequest, "model is required")
		return
	}
	if req.Prompt == "" {
		g.writeError(w, http.StatusBadRequest, "prompt is required")
		return
	}

	// Get tenant/env info from context
	tenantID := ctx.Value("tenant_id").(uuid.UUID)
	envID := ctx.Value("environment_id").(uuid.UUID)

	g.logger.Info("completion request",
		zap.String("tenant_id", tenantID.String()),
		zap.String("env_id", envID.String()),
		zap.String("model", req.Model),
		zap.Bool("streaming", req.Stream),
	)

	// Select best endpoint
	endpoint, err := g.loadBalancer.SelectEndpoint(ctx, req.Model)
	if err != nil {
		g.logger.Error("failed to select endpoint", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to select endpoint")
		return
	}

	if endpoint == "" {
		g.writeError(w, http.StatusServiceUnavailable, "no healthy nodes for model")
		return
	}

	// Proxy request to endpoint
	// Re-create body reader for proxying
	r.Body = io.NopCloser(bytes.NewBuffer(body))

	start := time.Now()
	resp, err := g.proxyRequest(endpoint, r)
	duration := time.Since(start)

	// Record stats
	isError := err != nil || (resp != nil && resp.StatusCode >= 500)
	g.loadBalancer.RecordRequest(endpoint, duration, isError)

	if err != nil {
		g.logger.Error("failed to proxy request", zap.Error(err))
		g.writeError(w, http.StatusBadGateway, "failed to proxy request")
		return
	}
	defer resp.Body.Close()

	// Copy response headers
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func (g *Gateway) handleEmbeddings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Read request body for parsing and forwarding
	body, err := io.ReadAll(r.Body)
	if err != nil {
		g.writeError(w, http.StatusBadRequest, "failed to read request body")
		return
	}
	r.Body.Close()

	// Parse request for validation
	var req EmbeddingRequest
	if err := json.Unmarshal(body, &req); err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate request
	if err := req.Validate(); err != nil {
		g.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	g.logger.Info("embedding request",
		zap.String("model", req.Model),
	)

	// Select best endpoint
	endpoint, err := g.loadBalancer.SelectEndpoint(ctx, req.Model)
	if err != nil {
		g.logger.Error("failed to select endpoint", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to select endpoint")
		return
	}

	if endpoint == "" {
		g.writeError(w, http.StatusServiceUnavailable, "no healthy nodes for model")
		return
	}

	// Proxy request to endpoint
	// Re-create body reader for proxying
	r.Body = io.NopCloser(bytes.NewBuffer(body))

	start := time.Now()
	resp, err := g.proxyRequest(endpoint, r)
	duration := time.Since(start)

	// Record stats
	isError := err != nil || (resp != nil && resp.StatusCode >= 500)
	g.loadBalancer.RecordRequest(endpoint, duration, isError)

	if err != nil {
		g.logger.Error("failed to proxy request", zap.Error(err))
		g.writeError(w, http.StatusBadGateway, "failed to proxy request")
		return
	}
	defer resp.Body.Close()

	// Copy response headers
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
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
			"id":       m.Name,
			"object":   "model",
			"created":  time.Now().Unix(),
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
		"id":       modelName,
		"object":   "model",
		"created":  time.Now().Unix(),
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

// handleLaunchNode launches a new GPU node using SkyPilot
func (g *Gateway) handleLaunchNode(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse request body
	var req orchestrator.NodeConfig
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Launch node
	clusterName, err := g.orchestrator.LaunchNode(ctx, req)
	if err != nil {
		g.logger.Error("failed to launch node", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to launch node: %v", err))
		return
	}

	g.writeJSON(w, http.StatusOK, map[string]interface{}{
		"cluster_name": clusterName,
		"node_id":      req.NodeID,
		"status":       "launching",
	})
}

// handleTerminateNode terminates a GPU node
func (g *Gateway) handleTerminateNode(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	clusterName := chi.URLParam(r, "cluster_name")

	if clusterName == "" {
		g.writeError(w, http.StatusBadRequest, "cluster_name is required")
		return
	}

	// Terminate node
	if err := g.orchestrator.TerminateNode(ctx, clusterName); err != nil {
		g.logger.Error("failed to terminate node", zap.Error(err), zap.String("cluster_name", clusterName))
		g.writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to terminate node: %v", err))
		return
	}

	g.writeJSON(w, http.StatusOK, map[string]interface{}{
		"cluster_name": clusterName,
		"status":       "terminated",
	})
}

// handleNodeStatus retrieves the status of a GPU node
func (g *Gateway) handleNodeStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	clusterName := chi.URLParam(r, "cluster_name")

	if clusterName == "" {
		g.writeError(w, http.StatusBadRequest, "cluster_name is required")
		return
	}

	// Get status
	status, err := g.orchestrator.GetNodeStatus(ctx, clusterName)
	if err != nil {
		g.logger.Error("failed to get node status", zap.Error(err), zap.String("cluster_name", clusterName))
		g.writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get node status: %v", err))
		return
	}

	g.writeJSON(w, http.StatusOK, map[string]interface{}{
		"cluster_name": clusterName,
		"status":       status,
	})
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

// recordUsage records token usage for billing
func (g *Gateway) recordUsage(ctx context.Context, usage models.UsageRecord) {
	// Store usage record asynchronously
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err := g.db.Pool.Exec(ctx, `
			INSERT INTO usage_records (
				id, request_id, timestamp, tenant_id, environment_id,
				api_key_id, node_id, prompt_tokens, completion_tokens,
				total_tokens, latency_ms
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		`,
			usage.ID, usage.RequestID, usage.Timestamp,
			usage.TenantID, usage.EnvironmentID, usage.APIKeyID,
			usage.NodeID, usage.PromptTokens, usage.CompletionTokens,
			usage.TotalTokens, usage.LatencyMs,
		)
		if err != nil {
			g.logger.Error("failed to record usage",
				zap.Error(err),
				zap.String("request_id", *usage.RequestID),
			)
		}
	}()
}

// Helper functions

// getFloat64 safely extracts a float64 from an interface{}
func getFloat64(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case nil:
		return 0
	default:
		return 0
	}
}

// intPtr returns a pointer to an int
func intPtr(i int) *int {
	return &i
}

func estimateTokens(maxTokens *int) int {
	if maxTokens == nil {
		return defaultEstimatedTokens
	}
	if *maxTokens <= 0 {
		return defaultEstimatedTokens
	}
	return *maxTokens
}

// Request/Response types

type ChatCompletionRequest struct {
	Model       string                  `json:"model"`
	Messages    []ChatCompletionMessage `json:"messages"`
	Temperature *float64                `json:"temperature,omitempty"`
	MaxTokens   *int                    `json:"max_tokens,omitempty"`
	Stream      bool                    `json:"stream,omitempty"`
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

type CompletionRequest struct {
	Model       string   `json:"model"`
	Prompt      string   `json:"prompt"`
	Temperature *float64 `json:"temperature,omitempty"`
	MaxTokens   *int     `json:"max_tokens,omitempty"`
	TopP        *float64 `json:"top_p,omitempty"`
	N           *int     `json:"n,omitempty"`
	Stream      bool     `json:"stream,omitempty"`
	Stop        []string `json:"stop,omitempty"`
}

type EmbeddingRequest struct {
	Model      string   `json:"model"`
	Input      string   `json:"input,omitempty"` // Single input string
	InputArray []string `json:"input,omitempty"` // Array of input strings (OpenAI supports both)
	User       string   `json:"user,omitempty"`  // Optional user identifier
}

// Validate checks if the request is valid
func (r *EmbeddingRequest) Validate() error {
	if r.Model == "" {
		return fmt.Errorf("model is required")
	}
	if r.Input == "" && len(r.InputArray) == 0 {
		return fmt.Errorf("input is required")
	}
	return nil
}

func (g *Gateway) proxyRequest(endpoint string, r *http.Request) (*http.Response, error) {
	// Construct target URL
	targetURL := endpoint + r.URL.Path
	if !strings.HasPrefix(endpoint, "http") {
		targetURL = "http://" + endpoint + r.URL.Path
	}

	// Create new request
	proxyReq, err := http.NewRequestWithContext(r.Context(), r.Method, targetURL, r.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to create proxy request: %w", err)
	}

	// Copy headers
	for k, v := range r.Header {
		proxyReq.Header[k] = v
	}

	// Execute request
	client := &http.Client{
		Timeout: 10 * time.Minute, // Long timeout for LLM generation
	}
	resp, err := client.Do(proxyReq)
	if err != nil {
		return nil, fmt.Errorf("proxy request failed: %w", err)
	}

	return resp, nil
}
