package integration_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/crosslogic/control-plane/internal/billing"
	"github.com/crosslogic/control-plane/internal/config"
	"github.com/crosslogic/control-plane/internal/gateway"
	"github.com/crosslogic/control-plane/internal/orchestrator"
	"github.com/crosslogic/control-plane/pkg/cache"
	"github.com/crosslogic/control-plane/pkg/database"
	"github.com/crosslogic/control-plane/pkg/events"
	"go.uber.org/zap"
)

func TestEndToEndAPI(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") == "" {
		t.Skip("Skipping integration test; set INTEGRATION_TEST=1 to run")
	}

	// Setup dependencies
	logger, _ := zap.NewDevelopment()
	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Connect to DB
	db, err := database.NewDatabase(cfg.Database)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	// Connect to Redis
	redisCache, err := cache.NewCache(cfg.Redis)
	if err != nil {
		t.Fatalf("failed to connect to Redis: %v", err)
	}
	defer redisCache.Close()

	// Initialize event bus
	eventBus := events.NewBus(logger)

	// Setup components
	webhookHandler := billing.NewWebhookHandler("whsec_test", db, redisCache, logger, eventBus)
	orch, _ := orchestrator.NewSkyPilotOrchestrator(db, logger, "http://localhost:8080", "0.6.2", "2.4.0", eventBus, config.JuiceFSConfig{})

	monitor := orchestrator.NewTripleSafetyMonitor(db, logger, orch)
	gw := gateway.NewGateway(db, redisCache, logger, webhookHandler, orch, monitor, "admin-token", eventBus)

	// Create test server
	ts := httptest.NewServer(gw)
	defer ts.Close()

	// Test 1: Health Check
	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("health check failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	// Test 2: Create Tenant (Admin)
	tenantReq := map[string]string{
		"name":  "Integration Test Org",
		"email": fmt.Sprintf("test-%d@example.com", time.Now().Unix()),
	}
	tenantBody, _ := json.Marshal(tenantReq)
	req, _ := http.NewRequest("POST", ts.URL+"/admin/tenants", bytes.NewReader(tenantBody))
	req.Header.Set("X-Admin-Token", "admin-token")
	req.Header.Set("Content-Type", "application/json")

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("create tenant failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var tenantResp struct {
		ID string `json:"id"`
	}
	json.NewDecoder(resp.Body).Decode(&tenantResp)
	tenantID := tenantResp.ID

	// Test 3: Create API Key (Admin)
	keyReq := map[string]interface{}{
		"tenant_id": tenantID,
		"name":      "test-key",
	}
	keyBody, _ := json.Marshal(keyReq)
	req, _ = http.NewRequest("POST", ts.URL+"/admin/api-keys", bytes.NewReader(keyBody))
	req.Header.Set("X-Admin-Token", "admin-token")
	req.Header.Set("Content-Type", "application/json")

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("create api key failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var keyResp struct {
		Key string `json:"key"`
	}
	json.NewDecoder(resp.Body).Decode(&keyResp)
	apiKey := keyResp.Key

	// Test 4: Chat Completion (User)
	// Note: This will fail if no nodes are available, but we check for 503 or 200
	chatReq := map[string]interface{}{
		"model": "llama-3-8b",
		"messages": []map[string]string{
			{"role": "user", "content": "Hello"},
		},
	}
	chatBody, _ := json.Marshal(chatReq)
	req, _ = http.NewRequest("POST", ts.URL+"/v1/chat/completions", bytes.NewReader(chatBody))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("chat completion failed: %v", err)
	}

	// We expect either 200 (if nodes exist) or 503 (if no nodes)
	// 401 or 403 would indicate auth failure
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		t.Errorf("auth failed with status %d", resp.StatusCode)
	}
}
