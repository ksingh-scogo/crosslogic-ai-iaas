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
	"github.com/crosslogic/control-plane/internal/credentials"
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
	db                *database.Database
	cache             *cache.Cache
	logger            *zap.Logger
	authenticator     *Authenticator
	rateLimiter       *RateLimiter
	router            *chi.Mux
	webhookHandler    *billing.WebhookHandler
	orchestrator      *orchestrator.SkyPilotOrchestrator
	monitor           *orchestrator.TripleSafetyMonitor
	adminToken        string
	eventBus          *events.Bus
	credentialService *credentials.Service
	// LoadBalancer handles intelligent request routing
	LoadBalancer *IntelligentLoadBalancer
}

// NewGateway creates a new API gateway
func NewGateway(db *database.Database, cache *cache.Cache, logger *zap.Logger, webhookHandler *billing.WebhookHandler, orch *orchestrator.SkyPilotOrchestrator, monitor *orchestrator.TripleSafetyMonitor, adminToken string, eventBus *events.Bus, credentialService *credentials.Service) *Gateway {
	g := &Gateway{
		db:                db,
		cache:             cache,
		logger:            logger,
		authenticator:     NewAuthenticator(db, cache, logger),
		rateLimiter:       NewRateLimiter(cache, logger),
		router:            chi.NewRouter(),
		webhookHandler:    webhookHandler,
		orchestrator:      orch,
		monitor:           monitor,
		adminToken:        adminToken,
		eventBus:          eventBus,
		credentialService: credentialService,
		LoadBalancer:      NewIntelligentLoadBalancer(db, logger),
	}

	g.setupRoutes()
	return g
}

// setupRoutes configures the HTTP routes
func (g *Gateway) setupRoutes() {
	// Security middleware (should be first to set headers on all responses)
	securityConfig := DefaultSecurityConfig()
	g.router.Use(SecurityMiddleware(securityConfig))
	g.router.Use(APISecurityMiddleware())

	// Request size limit (10MB max for inference requests)
	g.router.Use(RequestSizeLimitMiddleware(10 * 1024 * 1024))

	// Standard middleware
	g.router.Use(middleware.RequestID)
	g.router.Use(middleware.RealIP)
	g.router.Use(g.requestIDResponseMiddleware) // Add request ID to responses
	g.router.Use(g.loggerMiddleware)
	g.router.Use(g.metricsMiddleware) // Add metrics middleware
	g.router.Use(middleware.Recoverer)
	g.router.Use(middleware.Timeout(60 * time.Second))

	// CORS - Updated with rate limit headers exposed
	g.router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000", "https://*.crosslogic.ai"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-Admin-Token"},
		ExposedHeaders:   []string{"Link", "X-Request-ID", "X-RateLimit-Limit", "X-RateLimit-Remaining", "X-RateLimit-Reset", "Retry-After"},
		AllowCredentials: true,
		MaxAge:           300, // Maximum value not ignored by any of major browsers
	}))

	// Metrics endpoint
	g.registerMetrics()

	// === PUBLIC ENDPOINTS (No Auth) ===
	// Health check
	g.router.Get("/health", g.handleHealth)
	g.router.Get("/ready", g.handleReady)

	// API documentation
	g.router.Get("/api-docs", g.handleSwaggerUI)
	g.router.Get("/api/v1/admin/openapi.yaml", g.handleOpenAPISpec)

	// Stripe webhook endpoint (no auth - uses signature verification)
	if g.webhookHandler != nil {
		g.router.Post("/api/webhooks/stripe", g.webhookHandler.HandleWebhook)
	} else {
		g.router.Post("/api/webhooks/stripe", func(w http.ResponseWriter, r *http.Request) {
			g.writeError(w, http.StatusServiceUnavailable, "billing webhooks disabled")
		})
	}

	// === PLATFORM ADMIN APIs (X-Admin-Token auth) ===
	g.router.Group(func(r chi.Router) {
		r.Use(g.adminAuthMiddleware)

		// Admin - Models (RESTful CRUD)
		r.Get("/api/v1/admin/models", g.HandleListModels)
		r.Post("/api/v1/admin/models", g.HandleCreateModel)
		r.Get("/api/v1/admin/models/search", g.HandleSearchModels)
		r.Get("/api/v1/admin/models/{id}", g.HandleGetModel)
		r.Put("/api/v1/admin/models/{id}", g.HandleUpdateModel)
		r.Patch("/api/v1/admin/models/{id}", g.HandlePatchModel)
		r.Delete("/api/v1/admin/models/{id}", g.HandleDeleteModel)

		// Admin - Nodes
		r.Get("/admin/nodes", g.handleListNodes)
		r.Post("/admin/nodes/launch", g.handleLaunchNode)
		r.Post("/admin/nodes/register", g.handleRegisterNode)
		r.Get("/admin/nodes/{cluster_name}", g.handleNodeStatus)
		r.Post("/admin/nodes/{cluster_name}/terminate", g.handleTerminateNode)
		r.Get("/admin/nodes/{cluster_name}/status", g.handleNodeStatus)
		r.Post("/admin/nodes/{node_id}/heartbeat", g.handleHeartbeat)
		r.Post("/admin/nodes/{node_id}/drain", g.handleDrainNode)
		r.Post("/admin/nodes/{node_id}/termination-warning", g.handleTerminationWarning)

		// Admin - Node Logs (Real-time streaming)
		r.Get("/admin/nodes/{id}/logs", g.handleGetNodeLogs)
		r.Get("/admin/nodes/{id}/logs/stream", g.handleStreamNodeLogs)

		// Admin - Deployments
		r.Post("/admin/deployments", g.handleCreateDeployment)
		r.Get("/admin/deployments", g.handleListDeployments)
		r.Get("/admin/deployments/{id}", g.handleGetDeployment)
		r.Put("/admin/deployments/{id}/scale", g.handleScaleDeployment)
		r.Delete("/admin/deployments/{id}", g.handleDeleteDeployment)

		// Admin - Routing
		r.Get("/admin/routes", g.handleListRoutes)
		r.Get("/admin/routes/{model_id}", g.handleGetRoute)
		r.Put("/admin/routes/{model_id}", g.handleUpdateRoute)

		// Admin - Tenants
		r.Post("/admin/tenants", g.handleCreateTenant)
		r.Post("/admin/tenants/resolve", g.handleResolveTenant)
		r.Get("/admin/tenants", g.handleListTenants)
		r.Get("/admin/tenants/{tenant_id}", g.handleGetTenant)
		r.Put("/admin/tenants/{id}", g.handleUpdateTenant)
		r.Get("/admin/tenants/{id}/usage", g.handleGetTenantUsageAdmin)

		// Admin - Platform
		r.Get("/admin/platform/health", g.handlePlatformHealth)
		r.Get("/admin/platform/metrics", g.handlePlatformMetrics)

		// Admin - API Keys (admin view - all keys for a tenant)
		r.Get("/admin/api-keys/{tenant_id}", g.handleListAPIKeys)
		r.Post("/admin/api-keys", g.handleCreateAPIKey)
		r.Delete("/admin/api-keys/{key_id}", g.handleRevokeAPIKey)

		// Admin - Credentials
		r.Post("/admin/credentials", g.handleCreateCredential)
		r.Get("/admin/credentials", g.handleListCredentials)
		r.Get("/admin/credentials/{id}", g.handleGetCredential)
		r.Put("/admin/credentials/{id}", g.handleUpdateCredential)
		r.Delete("/admin/credentials/{id}", g.handleDeleteCredential)
		r.Post("/admin/credentials/{id}/validate", g.handleValidateCredential)
		r.Post("/admin/credentials/{id}/default", g.handleSetDefaultCredential)

		// Admin - Model/Instance management (UI-driven, legacy)
		r.Get("/admin/models/r2", g.ListR2ModelsHandler)
		r.Post("/admin/instances/launch", g.LaunchModelInstanceHandler)
		r.Get("/admin/instances/status", g.GetLaunchStatusHandler)
		r.Get("/admin/regions", g.ListRegionsHandler)
		r.Get("/admin/instance-types", g.ListInstanceTypesHandler)

		// === EXTENDED ADMIN ROUTES ===
		g.setupExtendedRoutes(r)
	})

	// === TENANT (CUSTOMER) APIs (Bearer token auth) ===
	g.router.Group(func(r chi.Router) {
		r.Use(g.authMiddleware)
		r.Use(g.rateLimitMiddleware)

		// Tenant - API Keys (self-service)
		r.Post("/v1/api-keys", g.handleCreateTenantAPIKey)
		r.Get("/v1/api-keys", g.handleListTenantAPIKeys)
		r.Delete("/v1/api-keys/{key_id}", g.handleRevokeTenantAPIKey)

		// Tenant - Endpoints (discovery)
		r.Get("/v1/endpoints", g.handleListTenantEndpoints)
		r.Get("/v1/endpoints/{model_id}", g.handleGetTenantEndpoint)

		// Tenant - Inference (OpenAI-compatible)
		r.Post("/v1/chat/completions", g.handleChatCompletions)
		r.Post("/v1/completions", g.handleCompletions)
		r.Post("/v1/embeddings", g.handleEmbeddings)
		r.Get("/v1/models", g.handleListModels)
		r.Get("/v1/models/{model}", g.handleGetModel)

		// Tenant - Usage & Billing
		r.Get("/v1/usage", g.handleGetUsage)
		r.Get("/v1/usage/by-model", g.handleGetUsageByModel)
		r.Get("/v1/usage/by-key", g.handleGetUsageByKey)
		r.Get("/v1/usage/by-date", g.handleGetUsageByDate)

		// Tenant - Metrics
		r.Get("/v1/metrics/latency", g.handleGetLatencyMetrics)
		r.Get("/v1/metrics/tokens", g.handleGetTokenMetrics)

		// === SELF-SERVICE FEATURES (PRO & ENTERPRISE ONLY) ===
		r.Group(func(proRouter chi.Router) {
			proRouter.Use(g.RequireProOrEnterprise)

			// Tenant - Cloud Credentials (self-service)
			proRouter.Post("/v1/credentials", g.handleCreateTenantCredential)
			proRouter.Get("/v1/credentials", g.handleListTenantCredentials)
			proRouter.Get("/v1/credentials/{id}", g.handleGetTenantCredential)
			proRouter.Put("/v1/credentials/{id}", g.handleUpdateTenantCredential)
			proRouter.Delete("/v1/credentials/{id}", g.handleDeleteTenantCredential)
			proRouter.Post("/v1/credentials/{id}/validate", g.handleValidateTenantCredential)
			proRouter.Post("/v1/credentials/{id}/default", g.handleSetDefaultTenantCredential)

			// Tenant - vLLM Instances (self-service)
			proRouter.Post("/v1/instances", g.handleLaunchTenantInstance)
			proRouter.Get("/v1/instances", g.handleListTenantInstances)
			proRouter.Get("/v1/instances/{id}", g.handleGetTenantInstance)
			proRouter.Delete("/v1/instances/{id}", g.handleTerminateTenantInstance)
			proRouter.Get("/v1/instances/{id}/logs/stream", g.handleStreamTenantInstanceLogs)
		})

		// === EXTENDED TENANT ROUTES ===
		g.setupExtendedTenantRoutes(r)
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

func (g *Gateway) handleRegisterNode(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ClusterName   string  `json:"cluster_name"`
		Provider      string  `json:"provider"`
		Region        string  `json:"region"`
		InstanceType  string  `json:"instance_type"`
		GPUType       string  `json:"gpu_type"`
		VRAMTotalGB   int     `json:"vram_total_gb"`
		ModelName     string  `json:"model_name"`
		EndpointURL   string  `json:"endpoint_url"`
		InternalIP    string  `json:"internal_ip"`
		SpotInstance  bool    `json:"spot_instance"`
		SpotPrice     float64 `json:"spot_price"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate required fields
	if req.ClusterName == "" || req.Provider == "" || req.EndpointURL == "" {
		g.writeError(w, http.StatusBadRequest, "cluster_name, provider, and endpoint_url are required")
		return
	}

	g.logger.Info("registering node",
		zap.String("cluster_name", req.ClusterName),
		zap.String("provider", req.Provider),
		zap.String("endpoint", req.EndpointURL),
	)

	// Check if node already exists
	var existingID string
	checkQuery := `SELECT id FROM nodes WHERE cluster_name = $1`
	err := g.db.Pool.QueryRow(r.Context(), checkQuery, req.ClusterName).Scan(&existingID)

	if err == nil {
		// Node exists, update it
		updateQuery := `
			UPDATE nodes SET
				endpoint_url = $1,
				internal_ip = $2,
				status = 'active',
				health_score = 100.0,
				last_heartbeat_at = NOW(),
				updated_at = NOW()
			WHERE cluster_name = $3
			RETURNING id
		`
		var nodeID string
		err = g.db.Pool.QueryRow(r.Context(), updateQuery,
			req.EndpointURL, req.InternalIP, req.ClusterName,
		).Scan(&nodeID)

		if err != nil {
			g.logger.Error("failed to update node", zap.Error(err))
			g.writeError(w, http.StatusInternalServerError, "failed to update node")
			return
		}

		g.writeJSON(w, http.StatusOK, map[string]interface{}{
			"status":  "updated",
			"node_id": nodeID,
		})
		return
	}

	// Create new node
	insertQuery := `
		INSERT INTO nodes (
			cluster_name, provider, instance_type, gpu_type, vram_total_gb,
			model_name, endpoint_url, internal_ip, spot_instance, spot_price,
			status, health_score, last_heartbeat_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, 'active', 100.0, NOW())
		RETURNING id
	`

	var nodeID string
	err = g.db.Pool.QueryRow(r.Context(), insertQuery,
		req.ClusterName, req.Provider, req.InstanceType, req.GPUType, req.VRAMTotalGB,
		req.ModelName, req.EndpointURL, req.InternalIP, req.SpotInstance, req.SpotPrice,
	).Scan(&nodeID)

	if err != nil {
		g.logger.Error("failed to register node", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to register node")
		return
	}

	g.logger.Info("node registered successfully", zap.String("node_id", nodeID))

	g.writeJSON(w, http.StatusCreated, map[string]interface{}{
		"status":  "registered",
		"node_id": nodeID,
	})
}

func (g *Gateway) handleDrainNode(w http.ResponseWriter, r *http.Request) {
	nodeID := chi.URLParam(r, "node_id")
	if nodeID == "" {
		g.writeError(w, http.StatusBadRequest, "node_id is required")
		return
	}

	g.logger.Info("draining node", zap.String("node_id", nodeID))

	// Mark node as draining
	query := `UPDATE nodes SET status = 'draining', status_message = 'graceful_drain_initiated' WHERE id = $1`
	_, err := g.db.Pool.Exec(r.Context(), query, nodeID)
	if err != nil {
		g.logger.Error("failed to update node status", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to drain node")
		return
	}

	// Publish event
	if g.eventBus != nil {
		g.eventBus.Publish(r.Context(), events.NewEvent(events.EventNodeDraining, "", map[string]interface{}{
			"node_id": nodeID,
		}))
	}

	g.writeJSON(w, http.StatusOK, map[string]string{"status": "draining"})
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

		// Anonymize API key in logs for security
		authHeader := r.Header.Get("Authorization")
		anonymizedAuth := ""
		if authHeader != "" {
			anonymizedAuth = AnonymizeAPIKey(strings.TrimPrefix(authHeader, "Bearer "))
		}

		g.logger.Info("request",
			zap.String("request_id", middleware.GetReqID(r.Context())),
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.Int("status", ww.Status()),
			zap.Duration("duration", time.Since(start)),
			zap.String("remote_addr", r.RemoteAddr),
			zap.String("api_key_prefix", anonymizedAuth),
		)
	})
}

// requestIDResponseMiddleware adds the request ID to response headers
func (g *Gateway) requestIDResponseMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := middleware.GetReqID(r.Context())
		if reqID != "" {
			w.Header().Set("X-Request-ID", reqID)
		}
		next.ServeHTTP(w, r)
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

		// Check rate limits with info for headers
		allowed, rateLimitInfo, err := g.rateLimiter.CheckRateLimitWithInfo(ctx, keyInfo)
		if err != nil {
			g.logger.Error("rate limit check failed", zap.Error(err))
			g.writeError(w, http.StatusInternalServerError, "rate limit check failed")
			return
		}

		// Always add rate limit headers (even when rejected)
		if rateLimitInfo != nil {
			for key, value := range rateLimitInfo.GetRateLimitHeaders() {
				w.Header().Set(key, value)
			}
		}

		if !allowed {
			g.writeRateLimitError(w, rateLimitInfo)
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

// writeRateLimitError writes a rate limit exceeded error with proper headers
func (g *Gateway) writeRateLimitError(w http.ResponseWriter, info *RateLimitInfo) {
	retryAfter := "60"
	if info != nil && info.RetryAfter > 0 {
		retryAfter = fmt.Sprintf("%d", info.RetryAfter)
	}
	w.Header().Set("Retry-After", retryAfter)

	g.writeJSON(w, http.StatusTooManyRequests, map[string]interface{}{
		"error": map[string]interface{}{
			"message":     "Rate limit exceeded. Please retry after the specified time.",
			"type":        "rate_limit_error",
			"retry_after": retryAfter,
		},
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
	endpoint, err := g.LoadBalancer.SelectEndpoint(ctx, req.Model)
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
	g.LoadBalancer.RecordRequest(endpoint, duration, isError)

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
	endpoint, err := g.LoadBalancer.SelectEndpoint(ctx, req.Model)
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
	g.LoadBalancer.RecordRequest(endpoint, duration, isError)

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
	endpoint, err := g.LoadBalancer.SelectEndpoint(ctx, req.Model)
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
	g.LoadBalancer.RecordRequest(endpoint, duration, isError)

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

func (g *Gateway) handleGetTenantUsageAdmin(w http.ResponseWriter, r *http.Request) {
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
	Model string      `json:"model"`
	Input interface{} `json:"input,omitempty"` // Can be string or []string (OpenAI supports both)
	User  string      `json:"user,omitempty"`  // Optional user identifier
}

// Validate checks if the request is valid
func (r *EmbeddingRequest) Validate() error {
	if r.Model == "" {
		return fmt.Errorf("model is required")
	}
	if r.Input == nil {
		return fmt.Errorf("input is required")
	}
	// Check for empty input (both string and []string cases)
	switch v := r.Input.(type) {
	case string:
		if v == "" {
			return fmt.Errorf("input is required")
		}
	case []interface{}:
		if len(v) == 0 {
			return fmt.Errorf("input is required")
		}
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
