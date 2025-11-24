package gateway

import "github.com/go-chi/chi/v5"

// setupExtendedRoutes registers all new extended API routes
// Call this from setupRoutes() to add the new handlers
func (g *Gateway) setupExtendedRoutes(r chi.Router) {
	// === ADMIN TENANT MANAGEMENT (Extended) ===
	r.Delete("/admin/tenants/{id}", g.handleDeleteTenant)
	r.Post("/admin/tenants/{id}/suspend", g.handleSuspendTenant)
	r.Post("/admin/tenants/{id}/activate", g.handleActivateTenant)
	r.Get("/admin/tenants/{id}/api-keys", g.handleGetTenantAPIKeys)
	r.Get("/admin/tenants/{id}/deployments", g.handleGetTenantDeployments)
	r.Get("/admin/tenants/{id}/usage/detailed", g.handleGetTenantDetailedUsage)

	// === ADMIN REGIONS MANAGEMENT ===
	r.Post("/admin/regions", g.handleCreateRegion)
	r.Put("/admin/regions/{id}", g.handleUpdateRegion)
	r.Delete("/admin/regions/{id}", g.handleDeleteRegion)
	r.Get("/admin/regions/{id}/availability", g.handleGetRegionAvailability)

	// === ADMIN INSTANCE TYPES MANAGEMENT ===
	r.Post("/admin/instance-types", g.handleCreateInstanceType)
	r.Put("/admin/instance-types/{id}", g.handleUpdateInstanceType)
	r.Delete("/admin/instance-types/{id}", g.handleDeleteInstanceType)
	r.Post("/admin/instance-types/{id}/regions", g.handleAssociateInstanceTypeRegions)
	r.Get("/admin/instance-types/{id}/pricing", g.handleGetInstanceTypePricing)
}

// setupExtendedTenantRoutes registers all new tenant API routes
// Call this from setupRoutes() to add the new tenant handlers
func (g *Gateway) setupExtendedTenantRoutes(r chi.Router) {
	// === TENANT USAGE (Extended) ===
	r.Get("/v1/usage/detailed", g.handleGetUsageDetailed)
	r.Get("/v1/usage/by-hour", g.handleGetUsageByHour)
	r.Get("/v1/usage/by-day", g.handleGetUsageByDay)
	r.Get("/v1/usage/by-week", g.handleGetUsageByWeek)
	r.Get("/v1/usage/by-month", g.handleGetUsageByMonth)

	// === TENANT METRICS (Extended) ===
	r.Get("/v1/metrics/performance", g.handleGetPerformanceMetrics)
	r.Get("/v1/metrics/throughput", g.handleGetThroughputMetrics)
	r.Get("/v1/metrics/by-model", g.handleGetModelMetrics)
}
