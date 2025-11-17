package scheduler

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/crosslogic/control-plane/pkg/models"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// TestNewVLLMProxy verifies the proxy is initialized with correct defaults
func TestNewVLLMProxy(t *testing.T) {
	logger := zap.NewNop()
	proxy := NewVLLMProxy(logger)

	if proxy == nil {
		t.Fatal("NewVLLMProxy returned nil")
	}

	if proxy.client == nil {
		t.Error("HTTP client not initialized")
	}

	if proxy.breakers == nil {
		t.Error("Circuit breakers map not initialized")
	}

	if proxy.logger == nil {
		t.Error("Logger not initialized")
	}

	// Verify client configuration
	if proxy.client.Timeout != 120*time.Second {
		t.Errorf("Expected timeout 120s, got %v", proxy.client.Timeout)
	}
}

// TestForwardRequest_Success tests successful request forwarding
func TestForwardRequest_Success(t *testing.T) {
	// Create mock vLLM server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST, got %s", r.Method)
		}

		// Return mock response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"chatcmpl-123","object":"chat.completion","choices":[{"message":{"role":"assistant","content":"Hello!"}}]}`))
	}))
	defer mockServer.Close()

	// Create proxy and node
	logger := zap.NewNop()
	proxy := NewVLLMProxy(logger)

	node := &models.Node{
		ID:          uuid.New(),
		EndpointURL: mockServer.URL,
		Status:      "active",
	}

	// Create request
	reqBody := []byte(`{"model":"test","messages":[{"role":"user","content":"Hi"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(string(reqBody)))
	req.Header.Set("Content-Type", "application/json")

	// Forward request
	ctx := context.Background()
	resp, err := proxy.ForwardRequest(ctx, node, req, reqBody)

	if err != nil {
		t.Fatalf("ForwardRequest failed: %v", err)
	}
	defer resp.Body.Close()

	// Verify response
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "chatcmpl-123") {
		t.Error("Response doesn't contain expected content")
	}
}

// TestForwardRequest_ServerError tests handling of server errors
func TestForwardRequest_ServerError(t *testing.T) {
	// Create mock server that returns 500
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"Internal server error"}`))
	}))
	defer mockServer.Close()

	logger := zap.NewNop()
	proxy := NewVLLMProxy(logger)

	node := &models.Node{
		ID:          uuid.New(),
		EndpointURL: mockServer.URL,
		Status:      "active",
	}

	reqBody := []byte(`{"model":"test"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(string(reqBody)))

	ctx := context.Background()
	resp, err := proxy.ForwardRequest(ctx, node, req, reqBody)

	// Should not error out, but return the error response
	if err != nil {
		t.Fatalf("ForwardRequest failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", resp.StatusCode)
	}
}

// TestForwardRequest_RetryOnTransientError tests retry logic
func TestForwardRequest_RetryOnTransientError(t *testing.T) {
	attempts := 0
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			// Fail first 2 attempts
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		// Succeed on 3rd attempt
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success":true}`))
	}))
	defer mockServer.Close()

	logger := zap.NewNop()
	proxy := NewVLLMProxy(logger)

	node := &models.Node{
		ID:          uuid.New(),
		EndpointURL: mockServer.URL,
		Status:      "active",
	}

	reqBody := []byte(`{"model":"test"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(string(reqBody)))

	ctx := context.Background()
	resp, err := proxy.ForwardRequest(ctx, node, req, reqBody)

	if err != nil {
		t.Fatalf("ForwardRequest failed after retries: %v", err)
	}
	defer resp.Body.Close()

	// Should succeed after retries
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 after retries, got %d", resp.StatusCode)
	}

	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

// TestForwardRequest_ContextCancellation tests context cancellation handling
func TestForwardRequest_ContextCancellation(t *testing.T) {
	// Create slow server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	logger := zap.NewNop()
	proxy := NewVLLMProxy(logger)

	node := &models.Node{
		ID:          uuid.New(),
		EndpointURL: mockServer.URL,
		Status:      "active",
	}

	reqBody := []byte(`{"model":"test"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(string(reqBody)))

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := proxy.ForwardRequest(ctx, node, req, reqBody)

	// Should error due to context cancellation
	if err == nil {
		t.Error("Expected error due to context cancellation")
	}

	if !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Errorf("Expected context deadline error, got: %v", err)
	}
}

// TestHandleStreaming_Success tests SSE streaming
func TestHandleStreaming_Success(t *testing.T) {
	// Create mock streaming server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Error("Response writer doesn't support flushing")
			return
		}

		// Send multiple chunks
		chunks := []string{
			`data: {"id":"1","choices":[{"delta":{"content":"Hello"}}]}`,
			`data: {"id":"2","choices":[{"delta":{"content":" World"}}]}`,
			`data: [DONE]`,
		}

		for _, chunk := range chunks {
			w.Write([]byte(chunk + "\n\n"))
			flusher.Flush()
			time.Sleep(10 * time.Millisecond)
		}
	}))
	defer mockServer.Close()

	logger := zap.NewNop()
	proxy := NewVLLMProxy(logger)

	node := &models.Node{
		ID:          uuid.New(),
		EndpointURL: mockServer.URL,
		Status:      "active",
	}

	reqBody := []byte(`{"model":"test","stream":true}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(string(reqBody)))

	// Create response recorder
	w := httptest.NewRecorder()

	ctx := context.Background()
	err := proxy.HandleStreaming(ctx, node, req, w, reqBody)

	if err != nil {
		t.Fatalf("HandleStreaming failed: %v", err)
	}

	// Verify streaming response
	result := w.Result()
	if result.Header.Get("Content-Type") != "text/event-stream" {
		t.Error("Expected text/event-stream content type")
	}

	body, _ := io.ReadAll(result.Body)
	if !strings.Contains(string(body), "Hello") {
		t.Error("Response doesn't contain streamed content")
	}
}

// TestCircuitBreaker_TripsAfterFailures tests circuit breaker functionality
func TestCircuitBreaker_TripsAfterFailures(t *testing.T) {
	logger := zap.NewNop()
	proxy := NewVLLMProxy(logger)

	node := &models.Node{
		ID:          uuid.New(),
		EndpointURL: "http://nonexistent-server:9999",
		Status:      "active",
	}

	reqBody := []byte(`{"model":"test"}`)

	// Make multiple failing requests to trip circuit breaker (5 failures opens the breaker)
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(string(reqBody)))
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		_, err := proxy.ForwardRequest(ctx, node, req, reqBody)
		cancel()

		if err == nil {
			t.Error("Expected error for failed request")
		}
	}

	// Circuit breaker should be open now - next request should fail immediately
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(string(reqBody)))
	ctx := context.Background()
	_, err := proxy.ForwardRequest(ctx, node, req, reqBody)

	if err == nil {
		t.Error("Expected circuit breaker to reject request")
	}
	if !strings.Contains(err.Error(), "circuit breaker open") {
		t.Errorf("Expected circuit breaker error, got: %v", err)
	}
}

// TestCircuitBreaker_ResetsAfterTimeout tests circuit breaker reset
// Note: This test is skipped by default due to 30s timeout. Run with -short=false to enable.
func TestCircuitBreaker_ResetsAfterTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping circuit breaker timeout test in short mode")
	}

	// Create a successful mock server for recovery testing
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success":true}`))
	}))
	defer mockServer.Close()

	logger := zap.NewNop()
	proxy := NewVLLMProxy(logger)

	node := &models.Node{
		ID:          uuid.New(),
		EndpointURL: "http://nonexistent-server:9999",
		Status:      "active",
	}

	reqBody := []byte(`{"model":"test"}`)

	// Trip circuit breaker with 5 failures
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(string(reqBody)))
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		proxy.ForwardRequest(ctx, node, req, reqBody)
		cancel()
	}

	// Verify circuit breaker is open
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(string(reqBody)))
	ctx := context.Background()
	_, err := proxy.ForwardRequest(ctx, node, req, reqBody)
	if err == nil || !strings.Contains(err.Error(), "circuit breaker open") {
		t.Error("Circuit breaker should be open")
	}

	// Wait for circuit breaker timeout (30 seconds) plus small buffer
	t.Log("Waiting 31 seconds for circuit breaker to transition to half-open...")
	time.Sleep(31 * time.Second)

	// Update node to point to working server
	node.EndpointURL = mockServer.URL

	// Circuit breaker should now be in half-open state and allow one request
	req = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(string(reqBody)))
	resp, err := proxy.ForwardRequest(ctx, node, req, reqBody)
	if err != nil {
		t.Errorf("Circuit breaker should allow request in half-open state, got error: %v", err)
	}
	if resp != nil {
		resp.Body.Close()
	}
}

// BenchmarkForwardRequest benchmarks request forwarding
func BenchmarkForwardRequest(b *testing.B) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success":true}`))
	}))
	defer mockServer.Close()

	logger := zap.NewNop()
	proxy := NewVLLMProxy(logger)

	node := &models.Node{
		ID:          uuid.New(),
		EndpointURL: mockServer.URL,
		Status:      "active",
	}

	reqBody := []byte(`{"model":"test"}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(string(reqBody)))
		ctx := context.Background()
		resp, err := proxy.ForwardRequest(ctx, node, req, reqBody)
		if err != nil {
			b.Fatal(err)
		}
		resp.Body.Close()
	}
}
