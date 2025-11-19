package notifications

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/crosslogic/control-plane/pkg/events"
	"go.uber.org/zap"
)

// WebhookAdapter sends notifications to generic webhooks with HMAC signatures
type WebhookAdapter struct {
	url     string
	secret  string
	method  string
	headers map[string]string
	client  *http.Client
	logger  *zap.Logger
}

// WebhookPayload represents the payload sent to generic webhooks
type WebhookPayload struct {
	EventID   string                 `json:"event_id"`
	EventType string                 `json:"event_type"`
	Timestamp string                 `json:"timestamp"`
	TenantID  string                 `json:"tenant_id,omitempty"`
	Data      map[string]interface{} `json:"data"`
}

// NewWebhookAdapter creates a new generic webhook adapter
func NewWebhookAdapter(url, secret, method string, headers map[string]string, logger *zap.Logger) *WebhookAdapter {
	return &WebhookAdapter{
		url:     url,
		secret:  secret,
		method:  method,
		headers: headers,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

// Send sends a notification to a generic webhook
func (w *WebhookAdapter) Send(ctx context.Context, event events.Event) error {
	payload := WebhookPayload{
		EventID:   event.ID,
		EventType: string(event.Type),
		Timestamp: event.Timestamp.Format(time.RFC3339),
		TenantID:  event.TenantID,
		Data:      event.Payload,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook payload: %w", err)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, w.method, w.url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "CrossLogic-Notifications/1.0")

	// Add custom headers
	for key, value := range w.headers {
		req.Header.Set(key, value)
	}

	// Add HMAC signature if secret is provided
	if w.secret != "" {
		signature := w.sign(jsonData)
		req.Header.Set("X-CrossLogic-Signature", signature)
		req.Header.Set("X-CrossLogic-Event-Type", string(event.Type))
		req.Header.Set("X-CrossLogic-Event-ID", event.ID)
		req.Header.Set("X-CrossLogic-Timestamp", event.Timestamp.Format(time.RFC3339))
	}

	// Send request
	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	w.logger.Debug("webhook sent successfully",
		zap.String("url", w.url),
		zap.String("event_id", event.ID),
		zap.Int("status_code", resp.StatusCode),
	)

	return nil
}

// sign creates an HMAC-SHA256 signature of the payload
func (w *WebhookAdapter) sign(payload []byte) string {
	mac := hmac.New(sha256.New, []byte(w.secret))
	mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// VerifySignature verifies an HMAC signature (utility for webhook receivers)
// This is not used by the adapter itself, but is provided as a helper
// for services that receive webhooks from CrossLogic
func VerifySignature(payload []byte, signature, secret string) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expectedSignature := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(signature), []byte(expectedSignature))
}
