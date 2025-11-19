package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/crosslogic/control-plane/pkg/events"
	"go.uber.org/zap"
)

// SlackAdapter sends notifications to Slack via webhooks
type SlackAdapter struct {
	webhookURL string
	channel    string
	client     *http.Client
	logger     *zap.Logger
}

// SlackWebhookPayload represents a Slack webhook message
type SlackWebhookPayload struct {
	Channel  string       `json:"channel,omitempty"`
	Username string       `json:"username,omitempty"`
	IconURL  string       `json:"icon_url,omitempty"`
	Blocks   []SlackBlock `json:"blocks,omitempty"`
	Text     string       `json:"text,omitempty"` // Fallback text
}

// SlackBlock represents a Slack Block Kit block
type SlackBlock struct {
	Type string                 `json:"type"`
	Text *SlackTextObject       `json:"text,omitempty"`
	Fields []SlackTextObject    `json:"fields,omitempty"`
	Accessory interface{}       `json:"accessory,omitempty"`
}

// SlackTextObject represents a text object in Slack
type SlackTextObject struct {
	Type string `json:"type"` // "plain_text" or "mrkdwn"
	Text string `json:"text"`
	Emoji bool  `json:"emoji,omitempty"`
}

// NewSlackAdapter creates a new Slack notification adapter
func NewSlackAdapter(webhookURL, channel string, logger *zap.Logger) *SlackAdapter {
	return &SlackAdapter{
		webhookURL: webhookURL,
		channel:    channel,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

// Send sends a notification to Slack
func (s *SlackAdapter) Send(ctx context.Context, event events.Event) error {
	blocks := s.formatEvent(event)

	payload := SlackWebhookPayload{
		Channel:  s.channel,
		Username: "CrossLogic Notifications",
		IconURL:  "https://crosslogic.ai/icon.png", // Optional: replace with your icon
		Blocks:   blocks,
		Text:     fmt.Sprintf("Event: %s", event.Type), // Fallback text
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal slack payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", s.webhookURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send slack webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("slack webhook returned status %d", resp.StatusCode)
	}

	return nil
}

// formatEvent converts an event into Slack blocks
func (s *SlackAdapter) formatEvent(event events.Event) []SlackBlock {
	switch event.Type {
	case events.EventTenantCreated:
		return s.formatTenantCreated(event)
	case events.EventPaymentSucceeded:
		return s.formatPaymentSucceeded(event)
	case events.EventNodeLaunched:
		return s.formatNodeLaunched(event)
	case events.EventNodeTerminated:
		return s.formatNodeTerminated(event)
	case events.EventNodeHealthDegraded:
		return s.formatNodeHealthDegraded(event)
	case events.EventCostAnomalyDetected:
		return s.formatCostAnomaly(event)
	default:
		return s.formatGeneric(event)
	}
}

func (s *SlackAdapter) formatTenantCreated(event events.Event) []SlackBlock {
	return []SlackBlock{
		{
			Type: "header",
			Text: &SlackTextObject{
				Type:  "plain_text",
				Text:  "🎉 New Organization Signup",
				Emoji: true,
			},
		},
		{
			Type: "section",
			Fields: []SlackTextObject{
				{Type: "mrkdwn", Text: fmt.Sprintf("*Organization:*\n%s", getStringField(event.Payload, "name"))},
				{Type: "mrkdwn", Text: fmt.Sprintf("*Email:*\n%s", getStringField(event.Payload, "email"))},
				{Type: "mrkdwn", Text: fmt.Sprintf("*Plan:*\n%s", getStringField(event.Payload, "billing_plan"))},
				{Type: "mrkdwn", Text: fmt.Sprintf("*Tenant ID:*\n`%s`", event.TenantID)},
			},
		},
		{
			Type: "context",
			Fields: []SlackTextObject{
				{Type: "mrkdwn", Text: fmt.Sprintf("<!date^%d^{date_num} {time_secs}|%s>", event.Timestamp.Unix(), event.Timestamp.Format(time.RFC3339))},
			},
		},
	}
}

func (s *SlackAdapter) formatPaymentSucceeded(event events.Event) []SlackBlock {
	return []SlackBlock{
		{
			Type: "header",
			Text: &SlackTextObject{
				Type:  "plain_text",
				Text:  "💰 Payment Received",
				Emoji: true,
			},
		},
		{
			Type: "section",
			Fields: []SlackTextObject{
				{Type: "mrkdwn", Text: fmt.Sprintf("*Customer:*\n%s", getStringField(event.Payload, "tenant_name"))},
				{Type: "mrkdwn", Text: fmt.Sprintf("*Amount:*\n%s", getStringField(event.Payload, "amount_formatted"))},
				{Type: "mrkdwn", Text: fmt.Sprintf("*Currency:*\n%s", getStringField(event.Payload, "currency"))},
				{Type: "mrkdwn", Text: fmt.Sprintf("*Payment ID:*\n`%s`", getStringField(event.Payload, "stripe_payment_id"))},
			},
		},
		{
			Type: "context",
			Fields: []SlackTextObject{
				{Type: "mrkdwn", Text: fmt.Sprintf("<!date^%d^{date_num} {time_secs}|%s>", event.Timestamp.Unix(), event.Timestamp.Format(time.RFC3339))},
			},
		},
	}
}

func (s *SlackAdapter) formatNodeLaunched(event events.Event) []SlackBlock {
	return []SlackBlock{
		{
			Type: "header",
			Text: &SlackTextObject{
				Type:  "plain_text",
				Text:  "🚀 GPU Node Launched",
				Emoji: true,
			},
		},
		{
			Type: "section",
			Text: &SlackTextObject{
				Type: "mrkdwn",
				Text: fmt.Sprintf("*%s* GPU node launched in *%s* (%s)",
					getStringField(event.Payload, "gpu_type"),
					getStringField(event.Payload, "region"),
					getStringField(event.Payload, "provider"),
				),
			},
		},
		{
			Type: "section",
			Fields: []SlackTextObject{
				{Type: "mrkdwn", Text: fmt.Sprintf("*Node ID:*\n`%s`", getStringField(event.Payload, "node_id"))},
				{Type: "mrkdwn", Text: fmt.Sprintf("*Cluster:*\n%s", getStringField(event.Payload, "cluster_name"))},
				{Type: "mrkdwn", Text: fmt.Sprintf("*GPU Count:*\n%v", event.Payload["gpu_count"])},
				{Type: "mrkdwn", Text: fmt.Sprintf("*Instance Type:*\n%s", getStringField(event.Payload, "instance_type"))},
				{Type: "mrkdwn", Text: fmt.Sprintf("*Spot Instance:*\n%v", event.Payload["spot_instance"])},
				{Type: "mrkdwn", Text: fmt.Sprintf("*Model:*\n%s", getStringField(event.Payload, "model"))},
			},
		},
		{
			Type: "context",
			Fields: []SlackTextObject{
				{Type: "mrkdwn", Text: fmt.Sprintf("Launch duration: %s", getStringField(event.Payload, "launch_duration"))},
			},
		},
	}
}

func (s *SlackAdapter) formatNodeTerminated(event events.Event) []SlackBlock {
	return []SlackBlock{
		{
			Type: "header",
			Text: &SlackTextObject{
				Type:  "plain_text",
				Text:  "⚠️ Spot Instance Termination Warning",
				Emoji: true,
			},
		},
		{
			Type: "section",
			Text: &SlackTextObject{
				Type: "mrkdwn",
				Text: fmt.Sprintf("Spot instance *%s* will be terminated soon!",
					getStringField(event.Payload, "node_id"),
				),
			},
		},
		{
			Type: "section",
			Fields: []SlackTextObject{
				{Type: "mrkdwn", Text: fmt.Sprintf("*Provider:*\n%s", getStringField(event.Payload, "provider"))},
				{Type: "mrkdwn", Text: fmt.Sprintf("*Time Remaining:*\n%s", getStringField(event.Payload, "time_remaining"))},
				{Type: "mrkdwn", Text: fmt.Sprintf("*Reason:*\n%s", getStringField(event.Payload, "reason"))},
			},
		},
	}
}

func (s *SlackAdapter) formatNodeHealthDegraded(event events.Event) []SlackBlock {
	return []SlackBlock{
		{
			Type: "header",
			Text: &SlackTextObject{
				Type:  "plain_text",
				Text:  "🏥 Node Health Degraded",
				Emoji: true,
			},
		},
		{
			Type: "section",
			Fields: []SlackTextObject{
				{Type: "mrkdwn", Text: fmt.Sprintf("*Node ID:*\n`%s`", getStringField(event.Payload, "node_id"))},
				{Type: "mrkdwn", Text: fmt.Sprintf("*Health Score:*\n%.1f%%", event.Payload["health_score"])},
				{Type: "mrkdwn", Text: fmt.Sprintf("*Previous Score:*\n%.1f%%", event.Payload["previous_score"])},
			},
		},
	}
}

func (s *SlackAdapter) formatCostAnomaly(event events.Event) []SlackBlock {
	return []SlackBlock{
		{
			Type: "header",
			Text: &SlackTextObject{
				Type:  "plain_text",
				Text:  "💸 Cost Anomaly Detected",
				Emoji: true,
			},
		},
		{
			Type: "section",
			Text: &SlackTextObject{
				Type: "mrkdwn",
				Text: "*Unusual spending detected!*",
			},
		},
		{
			Type: "section",
			Fields: []SlackTextObject{
				{Type: "mrkdwn", Text: fmt.Sprintf("*Tenant:*\n`%s`", event.TenantID)},
				{Type: "mrkdwn", Text: fmt.Sprintf("*Anomaly Type:*\n%s", getStringField(event.Payload, "anomaly_type"))},
				{Type: "mrkdwn", Text: fmt.Sprintf("*Current Cost:*\n$%.2f", event.Payload["current_cost"])},
				{Type: "mrkdwn", Text: fmt.Sprintf("*Average Cost:*\n$%.2f", event.Payload["average_cost"])},
				{Type: "mrkdwn", Text: fmt.Sprintf("*Threshold:*\n%s", getStringField(event.Payload, "threshold_exceeded"))},
			},
		},
	}
}

func (s *SlackAdapter) formatGeneric(event events.Event) []SlackBlock {
	return []SlackBlock{
		{
			Type: "header",
			Text: &SlackTextObject{
				Type:  "plain_text",
				Text:  fmt.Sprintf("📬 Event: %s", event.Type),
				Emoji: true,
			},
		},
		{
			Type: "section",
			Fields: []SlackTextObject{
				{Type: "mrkdwn", Text: fmt.Sprintf("*Event ID:*\n`%s`", event.ID)},
				{Type: "mrkdwn", Text: fmt.Sprintf("*Tenant ID:*\n`%s`", event.TenantID)},
			},
		},
	}
}
