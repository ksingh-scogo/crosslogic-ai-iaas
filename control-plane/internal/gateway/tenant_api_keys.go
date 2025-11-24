package gateway

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// handleCreateTenantAPIKey creates a new API key for the authenticated tenant
// Tenant API - POST /v1/api-keys
// Extracts tenant_id from authentication context
func (g *Gateway) handleCreateTenantAPIKey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract tenant_id from context (set by authMiddleware)
	tenantID, ok := ctx.Value("tenant_id").(uuid.UUID)
	if !ok {
		g.writeError(w, http.StatusUnauthorized, "tenant ID not found in context")
		return
	}

	// Parse request body
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		g.writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	// Get the default environment for the tenant
	var envID uuid.UUID
	err := g.db.Pool.QueryRow(ctx, `
		SELECT id FROM environments
		WHERE tenant_id = $1 AND status = 'active'
		ORDER BY created_at ASC LIMIT 1
	`, tenantID).Scan(&envID)

	if err != nil {
		g.logger.Error("failed to find environment for tenant",
			zap.Error(err),
			zap.String("tenant_id", tenantID.String()),
		)
		g.writeError(w, http.StatusBadRequest, "no active environment found for tenant")
		return
	}

	// Create API key using Authenticator
	apiKey, err := g.authenticator.CreateAPIKey(ctx, tenantID, envID, req.Name)
	if err != nil {
		g.logger.Error("failed to create api key",
			zap.Error(err),
			zap.String("tenant_id", tenantID.String()),
		)
		g.writeError(w, http.StatusInternalServerError, "failed to create api key")
		return
	}

	// Query back the created key ID
	var keyID uuid.UUID
	var createdAt time.Time
	err = g.db.Pool.QueryRow(ctx, `
		SELECT id, created_at FROM api_keys
		WHERE tenant_id = $1 AND environment_id = $2 AND name = $3
		ORDER BY created_at DESC LIMIT 1
	`, tenantID, envID, req.Name).Scan(&keyID, &createdAt)

	if err != nil {
		g.logger.Warn("failed to query created key ID",
			zap.Error(err),
			zap.String("tenant_id", tenantID.String()),
		)
	}

	g.logger.Info("tenant API key created",
		zap.String("tenant_id", tenantID.String()),
		zap.String("key_name", req.Name),
	)

	g.writeJSON(w, http.StatusCreated, map[string]interface{}{
		"key":        apiKey,
		"id":         keyID,
		"name":       req.Name,
		"created_at": createdAt,
	})
}

// handleListTenantAPIKeys lists API keys for the authenticated tenant
// Tenant API - GET /v1/api-keys
func (g *Gateway) handleListTenantAPIKeys(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract tenant_id from context
	tenantID, ok := ctx.Value("tenant_id").(uuid.UUID)
	if !ok {
		g.writeError(w, http.StatusUnauthorized, "tenant ID not found in context")
		return
	}

	rows, err := g.db.Pool.Query(ctx, `
		SELECT id, name, key_prefix, created_at, last_used_at, status,
		       rate_limit_requests_per_min
		FROM api_keys
		WHERE tenant_id = $1 AND status != 'revoked'
		ORDER BY created_at DESC
	`, tenantID)
	if err != nil {
		g.logger.Error("failed to list api keys",
			zap.Error(err),
			zap.String("tenant_id", tenantID.String()),
		)
		g.writeError(w, http.StatusInternalServerError, "failed to list api keys")
		return
	}
	defer rows.Close()

	var keys []map[string]interface{}
	for rows.Next() {
		var id uuid.UUID
		var name, keyPrefix, status string
		var createdAt time.Time
		var lastUsedAt *time.Time
		var rateLimit int

		if err := rows.Scan(&id, &name, &keyPrefix, &createdAt, &lastUsedAt, &status, &rateLimit); err != nil {
			g.logger.Warn("failed to scan api key row", zap.Error(err))
			continue
		}

		keyData := map[string]interface{}{
			"id":                    id,
			"name":                  name,
			"prefix":                keyPrefix + "...",
			"created_at":            createdAt,
			"status":                status,
			"rate_limit_per_minute": rateLimit,
		}

		if lastUsedAt != nil {
			keyData["last_used_at"] = *lastUsedAt
		}

		keys = append(keys, keyData)
	}

	g.writeJSON(w, http.StatusOK, map[string]interface{}{
		"data": keys,
	})
}

// handleRevokeTenantAPIKey revokes an API key for the authenticated tenant
// Tenant API - DELETE /v1/api-keys/{key_id}
func (g *Gateway) handleRevokeTenantAPIKey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract tenant_id from context
	tenantID, ok := ctx.Value("tenant_id").(uuid.UUID)
	if !ok {
		g.writeError(w, http.StatusUnauthorized, "tenant ID not found in context")
		return
	}

	keyIDStr := chi.URLParam(r, "key_id")
	keyID, err := uuid.Parse(keyIDStr)
	if err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid key ID")
		return
	}

	// Verify the key belongs to this tenant before revoking
	var existingTenantID uuid.UUID
	err = g.db.Pool.QueryRow(ctx, `
		SELECT tenant_id FROM api_keys WHERE id = $1
	`, keyID).Scan(&existingTenantID)

	if err != nil {
		g.logger.Warn("api key not found",
			zap.Error(err),
			zap.String("key_id", keyID.String()),
		)
		g.writeError(w, http.StatusNotFound, "API key not found")
		return
	}

	if existingTenantID != tenantID {
		g.logger.Warn("attempt to revoke key from different tenant",
			zap.String("tenant_id", tenantID.String()),
			zap.String("key_tenant_id", existingTenantID.String()),
			zap.String("key_id", keyID.String()),
		)
		g.writeError(w, http.StatusForbidden, "access denied")
		return
	}

	// Revoke the key
	_, err = g.db.Pool.Exec(ctx, `
		UPDATE api_keys SET status = 'revoked', updated_at = NOW()
		WHERE id = $1
	`, keyID)
	if err != nil {
		g.logger.Error("failed to revoke api key",
			zap.Error(err),
			zap.String("key_id", keyID.String()),
		)
		g.writeError(w, http.StatusInternalServerError, "failed to revoke api key")
		return
	}

	g.logger.Info("tenant API key revoked",
		zap.String("tenant_id", tenantID.String()),
		zap.String("key_id", keyID.String()),
	)

	g.writeJSON(w, http.StatusOK, map[string]string{
		"status":  "revoked",
		"message": "API key revoked successfully",
	})
}
