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

// DiscordAdapter sends notifications to Discord via webhooks
type DiscordAdapter struct {
	webhookURL string
	client     *http.Client
	logger     *zap.Logger
}

// DiscordWebhookPayload represents a Discord webhook message
type DiscordWebhookPayload struct {
	Content string          `json:"content,omitempty"`
	Embeds  []DiscordEmbed  `json:"embeds,omitempty"`
}

// DiscordEmbed represents a Discord embed
type DiscordEmbed struct {
	Title       string              `json:"title,omitempty"`
	Description string              `json:"description,omitempty"`
	Color       int                 `json:"color,omitempty"`
	Fields      []DiscordEmbedField `json:"fields,omitempty"`
	Timestamp   string              `json:"timestamp,omitempty"`
	Footer      *DiscordEmbedFooter `json:"footer,omitempty"`
}

// DiscordEmbedField represents a field in a Discord embed
type DiscordEmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

// DiscordEmbedFooter represents the footer of a Discord embed
type DiscordEmbedFooter struct {
	Text string `json:"text"`
}

// Discord color constants
const (
	DiscordColorGreen  = 3066993  // Success (green)
	DiscordColorBlue   = 3447003  // Info (blue)
	DiscordColorYellow = 16776960 // Warning (yellow)
	DiscordColorRed    = 15158332 // Error (red)
	DiscordColorPurple = 10181046 // Special (purple)
)

// NewDiscordAdapter creates a new Discord notification adapter
func NewDiscordAdapter(webhookURL string, logger *zap.Logger) *DiscordAdapter {
	return &DiscordAdapter{
		webhookURL: webhookURL,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

// Send sends a notification to Discord
func (d *DiscordAdapter) Send(ctx context.Context, event events.Event) error {
	embed := d.formatEvent(event)

	payload := DiscordWebhookPayload{
		Embeds: []DiscordEmbed{embed},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal discord payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", d.webhookURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send discord webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("discord webhook returned status %d", resp.StatusCode)
	}

	return nil
}

// formatEvent converts an event into a Discord embed
func (d *DiscordAdapter) formatEvent(event events.Event) DiscordEmbed {
	switch event.Type {
	case events.EventTenantCreated:
		return d.formatTenantCreated(event)
	case events.EventPaymentSucceeded:
		return d.formatPaymentSucceeded(event)
	case events.EventNodeLaunched:
		return d.formatNodeLaunched(event)
	case events.EventNodeTerminated:
		return d.formatNodeTerminated(event)
	case events.EventNodeHealthDegraded:
		return d.formatNodeHealthDegraded(event)
	case events.EventCostAnomalyDetected:
		return d.formatCostAnomaly(event)
	default:
		return d.formatGeneric(event)
	}
}

func (d *DiscordAdapter) formatTenantCreated(event events.Event) DiscordEmbed {
	return DiscordEmbed{
		Title:       "🎉 New Organization Signup",
		Description: "A new organization has joined CrossLogic!",
		Color:       DiscordColorGreen,
		Fields: []DiscordEmbedField{
			{Name: "Organization", Value: getStringField(event.Payload, "name"), Inline: true},
			{Name: "Email", Value: getStringField(event.Payload, "email"), Inline: true},
			{Name: "Plan", Value: getStringField(event.Payload, "billing_plan"), Inline: true},
			{Name: "Tenant ID", Value: event.TenantID, Inline: false},
		},
		Timestamp: event.Timestamp.Format(time.RFC3339),
		Footer:    &DiscordEmbedFooter{Text: "CrossLogic Notifications"},
	}
}

func (d *DiscordAdapter) formatPaymentSucceeded(event events.Event) DiscordEmbed {
	return DiscordEmbed{
		Title:       "💰 Payment Received",
		Description: "A payment has been successfully processed!",
		Color:       DiscordColorGreen,
		Fields: []DiscordEmbedField{
			{Name: "Customer", Value: getStringField(event.Payload, "tenant_name"), Inline: true},
			{Name: "Amount", Value: getStringField(event.Payload, "amount_formatted"), Inline: true},
			{Name: "Currency", Value: getStringField(event.Payload, "currency"), Inline: true},
			{Name: "Payment ID", Value: getStringField(event.Payload, "stripe_payment_id"), Inline: false},
		},
		Timestamp: event.Timestamp.Format(time.RFC3339),
		Footer:    &DiscordEmbedFooter{Text: "CrossLogic Notifications"},
	}
}

func (d *DiscordAdapter) formatNodeLaunched(event events.Event) DiscordEmbed {
	return DiscordEmbed{
		Title:       "🚀 GPU Node Launched",
		Description: "A new GPU node has been successfully launched!",
		Color:       DiscordColorBlue,
		Fields: []DiscordEmbedField{
			{Name: "Node ID", Value: getStringField(event.Payload, "node_id"), Inline: true},
			{Name: "Cluster", Value: getStringField(event.Payload, "cluster_name"), Inline: true},
			{Name: "Provider", Value: getStringField(event.Payload, "provider"), Inline: true},
			{Name: "Region", Value: getStringField(event.Payload, "region"), Inline: true},
			{Name: "GPU Type", Value: getStringField(event.Payload, "gpu_type"), Inline: true},
			{Name: "GPU Count", Value: fmt.Sprintf("%v", event.Payload["gpu_count"]), Inline: true},
			{Name: "Instance Type", Value: getStringField(event.Payload, "instance_type"), Inline: true},
			{Name: "Spot Instance", Value: fmt.Sprintf("%v", event.Payload["spot_instance"]), Inline: true},
			{Name: "Model", Value: getStringField(event.Payload, "model"), Inline: true},
			{Name: "Launch Duration", Value: getStringField(event.Payload, "launch_duration"), Inline: false},
		},
		Timestamp: event.Timestamp.Format(time.RFC3339),
		Footer:    &DiscordEmbedFooter{Text: "CrossLogic Notifications"},
	}
}

func (d *DiscordAdapter) formatNodeTerminated(event events.Event) DiscordEmbed {
	return DiscordEmbed{
		Title:       "⚠️ Spot Instance Termination Warning",
		Description: "A spot instance is scheduled for termination!",
		Color:       DiscordColorYellow,
		Fields: []DiscordEmbedField{
			{Name: "Node ID", Value: getStringField(event.Payload, "node_id"), Inline: true},
			{Name: "Provider", Value: getStringField(event.Payload, "provider"), Inline: true},
			{Name: "Time Remaining", Value: getStringField(event.Payload, "time_remaining"), Inline: true},
			{Name: "Reason", Value: getStringField(event.Payload, "reason"), Inline: false},
		},
		Timestamp: event.Timestamp.Format(time.RFC3339),
		Footer:    &DiscordEmbedFooter{Text: "CrossLogic Notifications"},
	}
}

func (d *DiscordAdapter) formatNodeHealthDegraded(event events.Event) DiscordEmbed {
	return DiscordEmbed{
		Title:       "🏥 Node Health Degraded",
		Description: "A GPU node's health score has dropped significantly!",
		Color:       DiscordColorYellow,
		Fields: []DiscordEmbedField{
			{Name: "Node ID", Value: getStringField(event.Payload, "node_id"), Inline: true},
			{Name: "Health Score", Value: fmt.Sprintf("%.1f%%", event.Payload["health_score"]), Inline: true},
			{Name: "Previous Score", Value: fmt.Sprintf("%.1f%%", event.Payload["previous_score"]), Inline: true},
		},
		Timestamp: event.Timestamp.Format(time.RFC3339),
		Footer:    &DiscordEmbedFooter{Text: "CrossLogic Notifications"},
	}
}

func (d *DiscordAdapter) formatCostAnomaly(event events.Event) DiscordEmbed {
	return DiscordEmbed{
		Title:       "💸 Cost Anomaly Detected",
		Description: "Unusual spending detected!",
		Color:       DiscordColorRed,
		Fields: []DiscordEmbedField{
			{Name: "Tenant ID", Value: event.TenantID, Inline: true},
			{Name: "Anomaly Type", Value: getStringField(event.Payload, "anomaly_type"), Inline: true},
			{Name: "Current Cost", Value: fmt.Sprintf("$%.2f", event.Payload["current_cost"]), Inline: true},
			{Name: "Average Cost", Value: fmt.Sprintf("$%.2f", event.Payload["average_cost"]), Inline: true},
			{Name: "Threshold", Value: getStringField(event.Payload, "threshold_exceeded"), Inline: true},
		},
		Timestamp: event.Timestamp.Format(time.RFC3339),
		Footer:    &DiscordEmbedFooter{Text: "CrossLogic Notifications"},
	}
}

func (d *DiscordAdapter) formatGeneric(event events.Event) DiscordEmbed {
	return DiscordEmbed{
		Title:       fmt.Sprintf("📬 Event: %s", event.Type),
		Description: "A new event occurred in CrossLogic",
		Color:       DiscordColorBlue,
		Fields: []DiscordEmbedField{
			{Name: "Event ID", Value: event.ID, Inline: true},
			{Name: "Tenant ID", Value: event.TenantID, Inline: true},
		},
		Timestamp: event.Timestamp.Format(time.RFC3339),
		Footer:    &DiscordEmbedFooter{Text: "CrossLogic Notifications"},
	}
}

// Helper function to safely get string fields from payload
func getStringField(payload map[string]interface{}, key string) string {
	if val, ok := payload[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
		return fmt.Sprintf("%v", val)
	}
	return "N/A"
}
