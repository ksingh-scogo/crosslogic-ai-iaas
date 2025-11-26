package gateway

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// TierLevel represents the tenant subscription tier
type TierLevel string

const (
	TierFree       TierLevel = "free"
	TierStarter    TierLevel = "starter"
	TierPro        TierLevel = "pro"
	TierEnterprise TierLevel = "enterprise"
)

// tierContextKey is the context key for tier information
type tierContextKey string

const tierKey tierContextKey = "tier"

// RequireProOrEnterprise is middleware that checks if the tenant is on PRO or ENTERPRISE tier
// Returns 403 Forbidden if the tenant is on free or starter tier
func (g *Gateway) RequireProOrEnterprise(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Get tenant ID from context (set by authMiddleware)
		tenantID, ok := ctx.Value("tenant_id").(uuid.UUID)
		if !ok {
			g.logger.Error("tenant_id not found in context")
			g.writeError(w, http.StatusInternalServerError, "authentication context invalid")
			return
		}

		// Get tenant tier from database
		tier, err := g.getTenantTier(ctx, tenantID)
		if err != nil {
			g.logger.Error("failed to get tenant tier",
				zap.Error(err),
				zap.String("tenant_id", tenantID.String()),
			)
			g.writeError(w, http.StatusInternalServerError, "failed to verify tenant tier")
			return
		}

		// Check if tenant is on PRO or ENTERPRISE tier
		if tier != TierPro && tier != TierEnterprise {
			g.logger.Warn("tenant attempted to access PRO+ feature",
				zap.String("tenant_id", tenantID.String()),
				zap.String("tier", string(tier)),
				zap.String("path", r.URL.Path),
			)

			g.writeJSON(w, http.StatusForbidden, map[string]interface{}{
				"error": map[string]string{
					"message": "This feature is only available on PRO and ENTERPRISE plans. Please upgrade your subscription.",
					"type":    "tier_restriction_error",
					"tier":    string(tier),
					"required_tier": "pro",
				},
			})
			return
		}

		// Add tier to context for handlers to use
		ctx = context.WithValue(ctx, tierKey, tier)

		g.logger.Debug("tier check passed",
			zap.String("tenant_id", tenantID.String()),
			zap.String("tier", string(tier)),
		)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// getTenantTier retrieves the tenant's subscription tier from the database
func (g *Gateway) getTenantTier(ctx context.Context, tenantID uuid.UUID) (TierLevel, error) {
	var billingPlan string
	query := `
		SELECT billing_plan
		FROM tenants
		WHERE id = $1 AND status = 'active' AND deleted_at IS NULL
	`

	err := g.db.Pool.QueryRow(ctx, query, tenantID).Scan(&billingPlan)
	if err != nil {
		return "", err
	}

	// Map billing_plan to TierLevel
	// The billing_plan field contains values like "free", "starter", "pro", "enterprise"
	switch billingPlan {
	case "free":
		return TierFree, nil
	case "starter":
		return TierStarter, nil
	case "pro":
		return TierPro, nil
	case "enterprise":
		return TierEnterprise, nil
	default:
		// Default to free if unknown
		g.logger.Warn("unknown billing plan, defaulting to free",
			zap.String("billing_plan", billingPlan),
			zap.String("tenant_id", tenantID.String()),
		)
		return TierFree, nil
	}
}

// GetTierFromContext retrieves the tier from the request context
func GetTierFromContext(ctx context.Context) (TierLevel, bool) {
	tier, ok := ctx.Value(tierKey).(TierLevel)
	return tier, ok
}
