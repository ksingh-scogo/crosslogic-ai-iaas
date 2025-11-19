package gateway

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// handleListAPIKeys lists API keys for a tenant
func (g *Gateway) handleListAPIKeys(w http.ResponseWriter, r *http.Request) {
	tenantIDStr := chi.URLParam(r, "tenant_id")
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid tenant ID")
		return
	}

	ctx := r.Context()
	rows, err := g.db.Pool.Query(ctx, `
		SELECT id, name, key_hash, created_at, last_used_at, status, rate_limit
		FROM api_keys
		WHERE tenant_id = $1 AND status != 'revoked'
		ORDER BY created_at DESC
	`, tenantID)
	if err != nil {
		g.logger.Error("failed to list api keys", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to list api keys")
		return
	}
	defer rows.Close()

	var keys []map[string]interface{}
	for rows.Next() {
		var id uuid.UUID
		var name, keyHash, status string
		var createdAt, lastUsedAt time.Time
		var rateLimit int

		if err := rows.Scan(&id, &name, &keyHash, &createdAt, &lastUsedAt, &status, &rateLimit); err != nil {
			continue
		}

		keys = append(keys, map[string]interface{}{
			"id":           id,
			"name":         name,
			"prefix":       "sk-..." + keyHash[:8], // Show hash prefix as proxy for key prefix
			"created_at":   createdAt,
			"last_used_at": lastUsedAt,
			"status":       status,
			"rate_limit":   rateLimit,
		})
	}

	g.writeJSON(w, http.StatusOK, keys)
}

// handleCreateAPIKey creates a new API key
func (g *Gateway) handleCreateAPIKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TenantID uuid.UUID `json:"tenant_id"`
		Name     string    `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ctx := r.Context()

	// Get the default environment for the tenant (assuming 'production' or first active one)
	var envID uuid.UUID
	err := g.db.Pool.QueryRow(ctx, `
		SELECT id FROM environments 
		WHERE tenant_id = $1 AND status = 'active' 
		ORDER BY created_at ASC LIMIT 1
	`, req.TenantID).Scan(&envID)

	if err != nil {
		g.logger.Error("failed to find environment for tenant", zap.Error(err))
		g.writeError(w, http.StatusBadRequest, "tenant has no active environment")
		return
	}

	// Use Authenticator to create the key (handles hashing and storage)
	apiKey, err := g.authenticator.CreateAPIKey(ctx, req.TenantID, envID, req.Name)
	if err != nil {
		g.logger.Error("failed to create api key", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to create api key")
		return
	}

	// We need to get the ID of the created key to return it
	// Since CreateAPIKey doesn't return the ID, we query it back using the hash
	// In a real high-concurrency scenario, this might be slightly race-prone if we don't return ID from CreateAPIKey
	// But for now, let's modify CreateAPIKey or just query by name/tenant/created_at
	// Actually, let's just return the key. The UI might need the ID for revocation list updates.
	// Let's query the ID based on the hash we just generated.
	// Wait, we don't have the hash here, Authenticator did it.
	// Let's just return the key and let the UI refresh the list.
	
	g.writeJSON(w, http.StatusOK, map[string]interface{}{
		"key": apiKey,
	})
}

// handleRevokeAPIKey revokes an API key
func (g *Gateway) handleRevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	keyIDStr := chi.URLParam(r, "key_id")
	keyID, err := uuid.Parse(keyIDStr)
	if err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid key ID")
		return
	}

	ctx := r.Context()
	_, err = g.db.Pool.Exec(ctx, `
		UPDATE api_keys SET status = 'revoked', updated_at = NOW()
		WHERE id = $1
	`, keyID)
	if err != nil {
		g.logger.Error("failed to revoke api key", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to revoke api key")
		return
	}

	g.writeJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}

