package billing

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stripe/stripe-go/v76/webhook"
	"go.uber.org/zap"
)

func TestWebhookHandler_HandleWebhook_SignatureVerification(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	// Pass nil for DB and Cache as we are testing signature verification which happens before DB access
	handler := NewWebhookHandler("whsec_test_secret", nil, nil, logger)

	tests := []struct {
		name           string
		payload        []byte
		signature      string
		expectedStatus int
	}{
		{
			name:           "No signature",
			payload:        []byte(`{}`),
			signature:      "",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Invalid signature",
			payload:        []byte(`{}`),
			signature:      "t=123,v1=invalid",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Valid signature",
			payload:        []byte(`{"id": "evt_123", "object": "event", "api_version": "2023-10-16"}`),
			signature:      generateSignature(t, []byte(`{"id": "evt_123", "object": "event", "api_version": "2023-10-16"}`), "whsec_test_secret"),
			expectedStatus: http.StatusOK, // Unknown event type returns 200
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/webhooks/stripe", bytes.NewReader(tt.payload))
			if tt.signature != "" {
				req.Header.Set("Stripe-Signature", tt.signature)
			}
			w := httptest.NewRecorder()

			handler.HandleWebhook(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestWebhookHandler_Idempotency(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	handler := NewWebhookHandler("whsec_test_secret", nil, nil, logger)

	payload := []byte(`{"id": "evt_idempotency_test", "object": "event", "type": "unknown.event", "api_version": "2023-10-16"}`)
	signature := generateSignature(t, payload, "whsec_test_secret")

	// First request
	req1 := httptest.NewRequest("POST", "/api/webhooks/stripe", bytes.NewReader(payload))
	req1.Header.Set("Stripe-Signature", signature)
	w1 := httptest.NewRecorder()

	handler.HandleWebhook(w1, req1)

	if w1.Code != http.StatusOK {
		t.Errorf("first request failed: %d", w1.Code)
	}

	// Verify event is marked as processed in memory
	handler.mu.Lock()
	if _, exists := handler.processedEvents["evt_idempotency_test"]; !exists {
		t.Error("event not marked as processed")
	}
	handler.mu.Unlock()

	// Second request (should be idempotent)
	req2 := httptest.NewRequest("POST", "/api/webhooks/stripe", bytes.NewReader(payload))
	req2.Header.Set("Stripe-Signature", signature)
	w2 := httptest.NewRecorder()

	handler.HandleWebhook(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("second request failed: %d", w2.Code)
	}
}

func generateSignature(t *testing.T, payload []byte, secret string) string {
	t.Helper()
	now := time.Now().Unix()
	signature := webhook.ComputeSignature(time.Unix(now, 0), payload, secret)
	return fmt.Sprintf("t=%d,v1=%s", now, hex.EncodeToString(signature))
}
