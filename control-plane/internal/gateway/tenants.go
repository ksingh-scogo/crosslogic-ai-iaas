package gateway

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/crosslogic/control-plane/pkg/events"
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

	// Publish tenant created event
	if g.eventBus != nil {
		evt := events.NewEvent(
			events.EventTenantCreated,
			tenantID.String(),
			map[string]interface{}{
				"name":         req.Name,
				"email":        req.Email,
				"billing_plan": "serverless",
			},
		)
		if err := g.eventBus.Publish(ctx, evt); err != nil {
			g.logger.Error("failed to publish tenant created event",
				zap.Error(err),
				zap.String("tenant_id", tenantID.String()),
			)
		}
	}

	g.writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":     tenantID,
		"name":   req.Name,
		"email":  req.Email,
		"status": "active",
	})
}

// handleResolveTenant resolves a tenant by email, creating one if it doesn't exist
func (g *Gateway) handleResolveTenant(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Email == "" {
		g.writeError(w, http.StatusBadRequest, "email is required")
		return
	}

	ctx := r.Context()
	var tenantID uuid.UUID
	var status string

	// Check if tenant exists
	err := g.db.Pool.QueryRow(ctx, `
		SELECT id, status FROM tenants WHERE email = $1
	`, req.Email).Scan(&tenantID, &status)

	if err == nil {
		// Tenant exists
		g.writeJSON(w, http.StatusOK, map[string]interface{}{
			"id":     tenantID,
			"status": status,
			"new":    false,
		})
		return
	}

	// Tenant does not exist, create new one
	// Use provided name or fallback to email part
	name := req.Name
	if name == "" {
		parts := strings.Split(req.Email, "@")
		if len(parts) > 0 {
			name = parts[0]
		} else {
			name = "New Tenant"
		}
	}

	err = g.db.Pool.QueryRow(ctx, `
		INSERT INTO tenants (name, email, status, created_at, updated_at)
		VALUES ($1, $2, 'active', NOW(), NOW())
		RETURNING id
	`, name, req.Email).Scan(&tenantID)

	if err != nil {
		g.logger.Error("failed to create tenant", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to create tenant")
		return
	}

	// Create default environment
	_, err = g.db.Pool.Exec(ctx, `
		INSERT INTO environments (tenant_id, name, region, status, created_at, updated_at)
		VALUES ($1, 'production', 'us-east', 'active', NOW(), NOW())
	`, tenantID)

	if err != nil {
		g.logger.Error("failed to create default environment", zap.Error(err))
	}

	// Publish tenant created event
	if g.eventBus != nil {
		evt := events.NewEvent(
			events.EventTenantCreated,
			tenantID.String(),
			map[string]interface{}{
				"name":         name,
				"email":        req.Email,
				"billing_plan": "serverless",
			},
		)
		if err := g.eventBus.Publish(ctx, evt); err != nil {
			g.logger.Error("failed to publish tenant created event",
				zap.Error(err),
				zap.String("tenant_id", tenantID.String()),
			)
		}
	}

	g.writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":     tenantID,
		"status": "active",
		"new":    true,
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

