package events

import "time"

// EventType represents the type of event being published
type EventType string

const (
	// Tenant events
	EventTenantCreated EventType = "tenant.created"
	EventTenantUpdated EventType = "tenant.updated"
	EventTenantDeleted EventType = "tenant.deleted"

	// Payment events
	EventPaymentSucceeded EventType = "payment.succeeded"
	EventPaymentFailed    EventType = "payment.failed"
	EventSubscriptionUpdated EventType = "subscription.updated"

	// Node events
	EventNodeLaunched         EventType = "node.launched"
	EventNodeTerminated       EventType = "node.terminated"
	EventNodeHealthChanged    EventType = "node.health_changed"
	EventNodeHealthDegraded   EventType = "node.health_degraded"
	EventNodeDraining         EventType = "node.draining"

	// Cost events
	EventCostAnomalyDetected EventType = "cost.anomaly_detected"

	// Rate limit events
	EventRateLimitThreshold EventType = "ratelimit.threshold_reached"

	// API key events
	EventAPIKeyCreated EventType = "apikey.created"
	EventAPIKeyRevoked EventType = "apikey.revoked"
)

// Event represents a single event in the system
type Event struct {
	// ID is a unique identifier for this event (for idempotency)
	ID string

	// Type is the event type
	Type EventType

	// Timestamp is when the event occurred
	Timestamp time.Time

	// TenantID is the tenant this event belongs to (optional for system events)
	TenantID string

	// Payload contains event-specific data
	Payload map[string]interface{}
}

// NewEvent creates a new event with the given type and payload
func NewEvent(eventType EventType, tenantID string, payload map[string]interface{}) Event {
	return Event{
		ID:        generateEventID(),
		Type:      eventType,
		Timestamp: time.Now().UTC(),
		TenantID:  tenantID,
		Payload:   payload,
	}
}

// generateEventID generates a unique event ID
func generateEventID() string {
	// Using timestamp + random suffix for uniqueness
	return time.Now().Format("20060102150405") + "-" + randString(8)
}

// randString generates a random alphanumeric string
func randString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
	}
	return string(b)
}
