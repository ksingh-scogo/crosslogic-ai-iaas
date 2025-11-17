package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/crosslogic-ai-iaas/control-plane/internal/gateway"
	"github.com/crosslogic-ai-iaas/control-plane/internal/router"
	"github.com/crosslogic-ai-iaas/control-plane/internal/scheduler"
	"github.com/crosslogic-ai-iaas/control-plane/pkg/cache"
	"github.com/crosslogic-ai-iaas/control-plane/pkg/database"
	"github.com/crosslogic-ai-iaas/control-plane/pkg/models"
	"github.com/crosslogic-ai-iaas/control-plane/pkg/telemetry"
)

func main() {
	store := database.NewInMemoryStore()
	cacheClient := cache.NewLocalCache()
	logger := telemetry.NewLogger()

	// seed demo data
	seedDemoData(store)

	sched := scheduler.NewScheduler(store, logger)
	route := router.NewRouter(store, sched, logger)
	gw := gateway.NewGateway(store, cacheClient, route, logger)

	http.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	http.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		resp, err := gw.HandleRequest(r.Context(), gw.ParseRequest(r))
		if err != nil {
			logger.Error("gateway", "err", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		encoder := json.NewEncoder(w)
		if err := encoder.Encode(resp); err != nil {
			http.Error(w, "failed to encode response", http.StatusInternalServerError)
		}
	})

	srv := &http.Server{
		Addr:              ":8080",
		ReadHeaderTimeout: 5 * time.Second,
	}

	logger.Info("server", "msg", "control plane running", "addr", srv.Addr)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

func seedDemoData(store database.Store) {
	tenant := models.Tenant{ID: "tenant-demo", Name: "Demo Org", Email: "demo@example.com", Environment: "dev"}
	store.SaveTenant(tenant)
	key := models.APIKey{Key: "sk_dev_demo", TenantID: tenant.ID, Environment: "dev", RateLimit: 5}
	store.SaveAPIKey(key)

	store.SaveNode(models.Node{
		ID:           "node-india",
		Provider:     "aws",
		Region:       "ap-south-1",
		Model:        "llama-7b",
		Endpoint:     "https://india.crosslogic.ai",
		SpotPrice:    0.12,
		InstanceType: "g4dn.xlarge",
		Status:       models.NodeStatusHealthy,
	})

	store.SaveNode(models.Node{
		ID:           "node-us",
		Provider:     "gcp",
		Region:       "us-central1",
		Model:        "mistral-7b",
		Endpoint:     "https://us.crosslogic.ai",
		SpotPrice:    0.11,
		InstanceType: "a2-highgpu-1g",
		Status:       models.NodeStatusHealthy,
	})
}
