package gateway

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// handleCreateTenant creates a new tenant
func (g *Gateway) handleCreateTenant(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" || req.Email == "" {
		g.writeError(w, http.StatusBadRequest, "name and email are required")
		return
	}

	ctx := r.Context()
	var tenantID uuid.UUID
	err := g.db.Pool.QueryRow(ctx, `
		INSERT INTO tenants (name, email, status, created_at, updated_at)
		VALUES ($1, $2, 'active', NOW(), NOW())
		RETURNING id
	`, req.Name, req.Email).Scan(&tenantID)

	if err != nil {
		g.logger.Error("failed to create tenant", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to create tenant")
		return
	}

	// Create default environment for the tenant
	_, err = g.db.Pool.Exec(ctx, `
		INSERT INTO environments (tenant_id, name, region, status, created_at, updated_at)
		VALUES ($1, 'production', 'us-east', 'active', NOW(), NOW())
	`, tenantID)

	if err != nil {
		g.logger.Error("failed to create default environment", zap.Error(err))
		// We don't fail the request here, but log the error. 
		// In a real system, we might want to use a transaction.
	}

	g.writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":     tenantID,
		"name":   req.Name,
		"email":  req.Email,
		"status": "active",
	})
}

// handleGetTenant retrieves tenant details
func (g *Gateway) handleGetTenant(w http.ResponseWriter, r *http.Request) {
	tenantIDStr := chi.URLParam(r, "tenant_id")
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid tenant ID")
		return
	}

	ctx := r.Context()
	var name, email, status string
	var createdAt time.Time

	err = g.db.Pool.QueryRow(ctx, `
		SELECT name, email, status, created_at
		FROM tenants
		WHERE id = $1
	`, tenantID).Scan(&name, &email, &status, &createdAt)

	if err != nil {
		g.logger.Error("failed to get tenant", zap.Error(err))
		g.writeError(w, http.StatusNotFound, "tenant not found")
		return
	}

	g.writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":         tenantID,
		"name":       name,
		"email":      email,
		"status":     status,
		"created_at": createdAt,
	})
}

