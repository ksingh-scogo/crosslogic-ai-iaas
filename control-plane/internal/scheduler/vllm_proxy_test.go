package scheduler

import (
	"context"
	"fmt"
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

func TestVLLMProxy_ForwardRequest(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	proxy := NewVLLMProxy(logger)

	// Mock vLLM server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("expected path /v1/chat/completions, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("expected Authorization header, got %s", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"chatcmpl-123","object":"chat.completion","created":1677652288,"model":"gpt-3.5-turbo","choices":[{"index":0,"message":{"role":"assistant","content":"Hello world"},"finish_reason":"stop"}],"usage":{"prompt_tokens":9,"completion_tokens":12,"total_tokens":21}}`))
	}))
	defer ts.Close()

	node := &models.Node{
		ID:          uuid.New(),
		EndpointURL: ts.URL,
	}

	reqBody := []byte(`{"model":"gpt-3.5-turbo","messages":[{"role":"user","content":"Hello"}]}`)
	req, _ := http.NewRequest("POST", "http://example.com/v1/chat/completions", strings.NewReader(string(reqBody)))
	req.Header.Set("Authorization", "Bearer test-key")

	resp, err := proxy.ForwardRequest(context.Background(), node, req, reqBody)
	if err != nil {
		t.Fatalf("ForwardRequest failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "Hello world") {
		t.Errorf("expected response to contain 'Hello world', got %s", string(body))
	}
}

func TestVLLMProxy_HandleStreaming(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	proxy := NewVLLMProxy(logger)

	// Mock vLLM server with SSE
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)

		events := []string{
			`data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1694268190,"model":"gpt-3.5-turbo","choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}]}`,
			`data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1694268190,"model":"gpt-3.5-turbo","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}`,
			`data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1694268190,"model":"gpt-3.5-turbo","choices":[{"index":0,"delta":{"content":" World"},"finish_reason":null}]}`,
			`data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1694268190,"model":"gpt-3.5-turbo","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
			`data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1694268190,"model":"gpt-3.5-turbo","choices":[],"usage":{"prompt_tokens":10,"completion_tokens":20,"total_tokens":30}}`,
			`data: [DONE]`,
		}

		for _, event := range events {
			fmt.Fprintf(w, "%s\n\n", event)
			flusher.Flush()
			time.Sleep(10 * time.Millisecond)
		}
	}))
	defer ts.Close()

	node := &models.Node{
		ID:          uuid.New(),
		EndpointURL: ts.URL,
	}

	reqBody := []byte(`{"model":"gpt-3.5-turbo","messages":[{"role":"user","content":"Hello"}],"stream":true}`)
	req, _ := http.NewRequest("POST", "http://example.com/v1/chat/completions", strings.NewReader(string(reqBody)))

	// Mock ResponseWriter
	w := httptest.NewRecorder()

	usage, err := proxy.HandleStreaming(context.Background(), node, req, w, reqBody)
	if err != nil {
		t.Fatalf("HandleStreaming failed: %v", err)
	}

	if usage == nil {
		t.Fatal("expected usage metrics, got nil")
	}

	if usage.TotalTokens != 30 {
		t.Errorf("expected 30 total tokens, got %d", usage.TotalTokens)
	}

	respBody := w.Body.String()
	if !strings.Contains(respBody, "Hello") {
		t.Errorf("expected response to contain 'Hello', got %s", respBody)
	}
}

func TestVLLMProxy_CircuitBreaker(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	proxy := NewVLLMProxy(logger)

	// Mock failing server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer ts.Close()

	node := &models.Node{
		ID:          uuid.New(),
		EndpointURL: ts.URL,
	}

	reqBody := []byte(`{}`)
	req, _ := http.NewRequest("POST", "http://example.com", strings.NewReader(string(reqBody)))

	// Trigger failures to open circuit breaker
	for i := 0; i < 6; i++ {
		proxy.ForwardRequest(context.Background(), node, req, reqBody)
	}

	// Next request should fail fast with circuit breaker error
	_, err := proxy.ForwardRequest(context.Background(), node, req, reqBody)
	if err == nil {
		t.Fatal("expected circuit breaker error, got nil")
	}
	if !strings.Contains(err.Error(), "circuit breaker open") {
		t.Errorf("expected 'circuit breaker open' error, got %v", err)
	}
}
