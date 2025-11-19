package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"time"

	"github.com/crosslogic/control-plane/pkg/events"
	"go.uber.org/zap"
)

// EmailAdapter sends notifications via email using Resend
type EmailAdapter struct {
	from    string
	to      []string
	apiKey  string
	client  *http.Client
	logger  *zap.Logger
}

// ResendEmailRequest represents a Resend API email request
type ResendEmailRequest struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	HTML    string   `json:"html"`
	Text    string   `json:"text,omitempty"`
}

// ResendEmailResponse represents a Resend API response
type ResendEmailResponse struct {
	ID string `json:"id"`
}

// NewEmailAdapter creates a new Email notification adapter using Resend
func NewEmailAdapter(from string, to []string, apiKey string, logger *zap.Logger) (*EmailAdapter, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("resend API key is required")
	}

	return &EmailAdapter{
		from:   from,
		to:     to,
		apiKey: apiKey,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}, nil
}

// Send sends an email notification using Resend
func (e *EmailAdapter) Send(ctx context.Context, event events.Event) error {
	subject, htmlBody, textBody := e.formatEvent(event)

	emailReq := ResendEmailRequest{
		From:    e.from,
		To:      e.to,
		Subject: subject,
		HTML:    htmlBody,
		Text:    textBody,
	}

	jsonData, err := json.Marshal(emailReq)
	if err != nil {
		return fmt.Errorf("failed to marshal email request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.resend.com/emails", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", e.apiKey))

	resp, err := e.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send email via resend: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("resend API returned status %d", resp.StatusCode)
	}

	var resendResp ResendEmailResponse
	if err := json.NewDecoder(resp.Body).Decode(&resendResp); err != nil {
		return fmt.Errorf("failed to decode resend response: %w", err)
	}

	e.logger.Info("email sent via resend",
		zap.String("email_id", resendResp.ID),
		zap.String("event_id", event.ID),
	)

	return nil
}

// formatEvent converts an event into email subject and body
func (e *EmailAdapter) formatEvent(event events.Event) (subject, htmlBody, textBody string) {
	switch event.Type {
	case events.EventTenantCreated:
		return e.formatTenantCreated(event)
	case events.EventPaymentSucceeded:
		return e.formatPaymentSucceeded(event)
	case events.EventNodeLaunched:
		return e.formatNodeLaunched(event)
	case events.EventNodeTerminated:
		return e.formatNodeTerminated(event)
	case events.EventNodeHealthDegraded:
		return e.formatNodeHealthDegraded(event)
	case events.EventCostAnomalyDetected:
		return e.formatCostAnomaly(event)
	default:
		return e.formatGeneric(event)
	}
}

func (e *EmailAdapter) formatTenantCreated(event events.Event) (string, string, string) {
	subject := "🎉 New Organization Signup - CrossLogic"

	htmlBody := fmt.Sprintf(`
		<!DOCTYPE html>
		<html>
		<head>
			<style>
				body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
				.container { max-width: 600px; margin: 0 auto; padding: 20px; }
				.header { background-color: #4CAF50; color: white; padding: 20px; text-align: center; }
				.content { background-color: #f9f9f9; padding: 20px; margin-top: 20px; }
				.field { margin-bottom: 10px; }
				.label { font-weight: bold; color: #555; }
				.value { color: #333; }
				.footer { text-align: center; margin-top: 30px; color: #888; font-size: 12px; }
			</style>
		</head>
		<body>
			<div class="container">
				<div class="header">
					<h1>🎉 New Organization Signup</h1>
				</div>
				<div class="content">
					<p>A new organization has joined CrossLogic!</p>
					<div class="field">
						<span class="label">Organization:</span>
						<span class="value">%s</span>
					</div>
					<div class="field">
						<span class="label">Email:</span>
						<span class="value">%s</span>
					</div>
					<div class="field">
						<span class="label">Billing Plan:</span>
						<span class="value">%s</span>
					</div>
					<div class="field">
						<span class="label">Tenant ID:</span>
						<span class="value">%s</span>
					</div>
					<div class="field">
						<span class="label">Signup Time:</span>
						<span class="value">%s</span>
					</div>
				</div>
				<div class="footer">
					<p>CrossLogic Notifications</p>
				</div>
			</div>
		</body>
		</html>
	`,
		getStringField(event.Payload, "name"),
		getStringField(event.Payload, "email"),
		getStringField(event.Payload, "billing_plan"),
		event.TenantID,
		event.Timestamp.Format(time.RFC1123),
	)

	textBody := fmt.Sprintf(`New Organization Signup

Organization: %s
Email: %s
Billing Plan: %s
Tenant ID: %s
Signup Time: %s

--
CrossLogic Notifications`,
		getStringField(event.Payload, "name"),
		getStringField(event.Payload, "email"),
		getStringField(event.Payload, "billing_plan"),
		event.TenantID,
		event.Timestamp.Format(time.RFC1123),
	)

	return subject, htmlBody, textBody
}

func (e *EmailAdapter) formatPaymentSucceeded(event events.Event) (string, string, string) {
	subject := "💰 Payment Received - CrossLogic"

	htmlBody := fmt.Sprintf(`
		<!DOCTYPE html>
		<html>
		<head>
			<style>
				body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
				.container { max-width: 600px; margin: 0 auto; padding: 20px; }
				.header { background-color: #4CAF50; color: white; padding: 20px; text-align: center; }
				.content { background-color: #f9f9f9; padding: 20px; margin-top: 20px; }
				.field { margin-bottom: 10px; }
				.label { font-weight: bold; color: #555; }
				.value { color: #333; }
				.amount { font-size: 24px; color: #4CAF50; font-weight: bold; }
				.footer { text-align: center; margin-top: 30px; color: #888; font-size: 12px; }
			</style>
		</head>
		<body>
			<div class="container">
				<div class="header">
					<h1>💰 Payment Received</h1>
				</div>
				<div class="content">
					<p>A payment has been successfully processed!</p>
					<div class="field">
						<span class="label">Customer:</span>
						<span class="value">%s</span>
					</div>
					<div class="field">
						<span class="label">Amount:</span>
						<span class="amount">%s</span>
					</div>
					<div class="field">
						<span class="label">Currency:</span>
						<span class="value">%s</span>
					</div>
					<div class="field">
						<span class="label">Payment ID:</span>
						<span class="value">%s</span>
					</div>
					<div class="field">
						<span class="label">Payment Time:</span>
						<span class="value">%s</span>
					</div>
				</div>
				<div class="footer">
					<p>CrossLogic Notifications</p>
				</div>
			</div>
		</body>
		</html>
	`,
		getStringField(event.Payload, "tenant_name"),
		getStringField(event.Payload, "amount_formatted"),
		getStringField(event.Payload, "currency"),
		getStringField(event.Payload, "stripe_payment_id"),
		event.Timestamp.Format(time.RFC1123),
	)

	textBody := fmt.Sprintf(`Payment Received

Customer: %s
Amount: %s
Currency: %s
Payment ID: %s
Payment Time: %s

--
CrossLogic Notifications`,
		getStringField(event.Payload, "tenant_name"),
		getStringField(event.Payload, "amount_formatted"),
		getStringField(event.Payload, "currency"),
		getStringField(event.Payload, "stripe_payment_id"),
		event.Timestamp.Format(time.RFC1123),
	)

	return subject, htmlBody, textBody
}

func (e *EmailAdapter) formatNodeLaunched(event events.Event) (string, string, string) {
	subject := "🚀 GPU Node Launched - CrossLogic"

	htmlBody := fmt.Sprintf(`
		<!DOCTYPE html>
		<html>
		<head>
			<style>
				body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
				.container { max-width: 600px; margin: 0 auto; padding: 20px; }
				.header { background-color: #2196F3; color: white; padding: 20px; text-align: center; }
				.content { background-color: #f9f9f9; padding: 20px; margin-top: 20px; }
				.field { margin-bottom: 10px; }
				.label { font-weight: bold; color: #555; }
				.value { color: #333; }
				.footer { text-align: center; margin-top: 30px; color: #888; font-size: 12px; }
			</style>
		</head>
		<body>
			<div class="container">
				<div class="header">
					<h1>🚀 GPU Node Launched</h1>
				</div>
				<div class="content">
					<p>A new GPU node has been successfully launched!</p>
					<div class="field">
						<span class="label">Node ID:</span>
						<span class="value">%s</span>
					</div>
					<div class="field">
						<span class="label">Cluster Name:</span>
						<span class="value">%s</span>
					</div>
					<div class="field">
						<span class="label">Provider:</span>
						<span class="value">%s</span>
					</div>
					<div class="field">
						<span class="label">Region:</span>
						<span class="value">%s</span>
					</div>
					<div class="field">
						<span class="label">GPU Type:</span>
						<span class="value">%s</span>
					</div>
					<div class="field">
						<span class="label">GPU Count:</span>
						<span class="value">%v</span>
					</div>
					<div class="field">
						<span class="label">Model:</span>
						<span class="value">%s</span>
					</div>
					<div class="field">
						<span class="label">Launch Duration:</span>
						<span class="value">%s</span>
					</div>
				</div>
				<div class="footer">
					<p>CrossLogic Notifications</p>
				</div>
			</div>
		</body>
		</html>
	`,
		getStringField(event.Payload, "node_id"),
		getStringField(event.Payload, "cluster_name"),
		getStringField(event.Payload, "provider"),
		getStringField(event.Payload, "region"),
		getStringField(event.Payload, "gpu_type"),
		event.Payload["gpu_count"],
		getStringField(event.Payload, "model"),
		getStringField(event.Payload, "launch_duration"),
	)

	textBody := fmt.Sprintf(`GPU Node Launched

Node ID: %s
Cluster Name: %s
Provider: %s
Region: %s
GPU Type: %s
GPU Count: %v
Model: %s
Launch Duration: %s

--
CrossLogic Notifications`,
		getStringField(event.Payload, "node_id"),
		getStringField(event.Payload, "cluster_name"),
		getStringField(event.Payload, "provider"),
		getStringField(event.Payload, "region"),
		getStringField(event.Payload, "gpu_type"),
		event.Payload["gpu_count"],
		getStringField(event.Payload, "model"),
		getStringField(event.Payload, "launch_duration"),
	)

	return subject, htmlBody, textBody
}

func (e *EmailAdapter) formatNodeTerminated(event events.Event) (string, string, string) {
	subject := "⚠️ Spot Instance Termination Warning - CrossLogic"

	htmlBody := fmt.Sprintf(`
		<!DOCTYPE html>
		<html>
		<head>
			<style>
				body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
				.container { max-width: 600px; margin: 0 auto; padding: 20px; }
				.header { background-color: #FF9800; color: white; padding: 20px; text-align: center; }
				.content { background-color: #f9f9f9; padding: 20px; margin-top: 20px; }
				.field { margin-bottom: 10px; }
				.label { font-weight: bold; color: #555; }
				.value { color: #333; }
				.warning { background-color: #fff3cd; padding: 15px; border-left: 4px solid #FF9800; margin-bottom: 20px; }
				.footer { text-align: center; margin-top: 30px; color: #888; font-size: 12px; }
			</style>
		</head>
		<body>
			<div class="container">
				<div class="header">
					<h1>⚠️ Spot Instance Termination Warning</h1>
				</div>
				<div class="content">
					<div class="warning">
						<strong>Action Required:</strong> A spot instance is scheduled for termination!
					</div>
					<div class="field">
						<span class="label">Node ID:</span>
						<span class="value">%s</span>
					</div>
					<div class="field">
						<span class="label">Provider:</span>
						<span class="value">%s</span>
					</div>
					<div class="field">
						<span class="label">Time Remaining:</span>
						<span class="value">%s</span>
					</div>
					<div class="field">
						<span class="label">Reason:</span>
						<span class="value">%s</span>
					</div>
				</div>
				<div class="footer">
					<p>CrossLogic Notifications</p>
				</div>
			</div>
		</body>
		</html>
	`,
		getStringField(event.Payload, "node_id"),
		getStringField(event.Payload, "provider"),
		getStringField(event.Payload, "time_remaining"),
		getStringField(event.Payload, "reason"),
	)

	textBody := fmt.Sprintf(`⚠️ Spot Instance Termination Warning

ACTION REQUIRED: A spot instance is scheduled for termination!

Node ID: %s
Provider: %s
Time Remaining: %s
Reason: %s

--
CrossLogic Notifications`,
		getStringField(event.Payload, "node_id"),
		getStringField(event.Payload, "provider"),
		getStringField(event.Payload, "time_remaining"),
		getStringField(event.Payload, "reason"),
	)

	return subject, htmlBody, textBody
}

func (e *EmailAdapter) formatNodeHealthDegraded(event events.Event) (string, string, string) {
	subject := "🏥 Node Health Degraded - CrossLogic"

	htmlBody := fmt.Sprintf(`
		<!DOCTYPE html>
		<html>
		<body>
			<h2>🏥 Node Health Degraded</h2>
			<p><strong>Node ID:</strong> %s</p>
			<p><strong>Health Score:</strong> %.1f%%</p>
			<p><strong>Previous Score:</strong> %.1f%%</p>
			<p>--<br>CrossLogic Notifications</p>
		</body>
		</html>
	`,
		getStringField(event.Payload, "node_id"),
		event.Payload["health_score"],
		event.Payload["previous_score"],
	)

	textBody := fmt.Sprintf(`Node Health Degraded

Node ID: %s
Health Score: %.1f%%
Previous Score: %.1f%%`,
		getStringField(event.Payload, "node_id"),
		event.Payload["health_score"],
		event.Payload["previous_score"],
	)

	return subject, htmlBody, textBody
}

func (e *EmailAdapter) formatCostAnomaly(event events.Event) (string, string, string) {
	subject := "💸 Cost Anomaly Detected - CrossLogic"

	htmlBody := fmt.Sprintf(`
		<!DOCTYPE html>
		<html>
		<body>
			<h2>💸 Cost Anomaly Detected</h2>
			<p><strong>Tenant ID:</strong> %s</p>
			<p><strong>Anomaly Type:</strong> %s</p>
			<p><strong>Current Cost:</strong> $%.2f</p>
			<p><strong>Average Cost:</strong> $%.2f</p>
			<p><strong>Threshold Exceeded:</strong> %s</p>
			<p>--<br>CrossLogic Notifications</p>
		</body>
		</html>
	`,
		event.TenantID,
		getStringField(event.Payload, "anomaly_type"),
		event.Payload["current_cost"],
		event.Payload["average_cost"],
		getStringField(event.Payload, "threshold_exceeded"),
	)

	textBody := fmt.Sprintf(`Cost Anomaly Detected

Tenant ID: %s
Anomaly Type: %s
Current Cost: $%.2f
Average Cost: $%.2f
Threshold Exceeded: %s`,
		event.TenantID,
		getStringField(event.Payload, "anomaly_type"),
		event.Payload["current_cost"],
		event.Payload["average_cost"],
		getStringField(event.Payload, "threshold_exceeded"),
	)

	return subject, htmlBody, textBody
}

func (e *EmailAdapter) formatGeneric(event events.Event) (string, string, string) {
	subject := fmt.Sprintf("📬 Event: %s - CrossLogic", event.Type)

	htmlBody := fmt.Sprintf(`
		<!DOCTYPE html>
		<html>
		<body>
			<h2>📬 New Event: %s</h2>
			<p><strong>Event ID:</strong> %s</p>
			<p><strong>Tenant ID:</strong> %s</p>
			<p><strong>Timestamp:</strong> %s</p>
			<p>--<br>CrossLogic Notifications</p>
		</body>
		</html>
	`,
		event.Type,
		event.ID,
		event.TenantID,
		event.Timestamp.Format(time.RFC1123),
	)

	textBody := fmt.Sprintf(`New Event: %s

Event ID: %s
Tenant ID: %s
Timestamp: %s`,
		event.Type,
		event.ID,
		event.TenantID,
		event.Timestamp.Format(time.RFC1123),
	)

	return subject, htmlBody, textBody
}

// Helper function to render HTML templates (optional, for more complex templates)
func renderTemplate(tmpl string, data interface{}) (string, error) {
	t, err := template.New("email").Parse(tmpl)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}
