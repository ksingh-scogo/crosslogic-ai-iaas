# Event Notification System - Design Document v2

## Executive Summary

This document outlines a comprehensive event notification system for CrossLogic Inference Cloud (CIC) with a pluggable architecture supporting Discord, Slack, Email, and generic webhook notifications.

**Key Objectives:**
- Real-time notifications for critical business events
- Pluggable architecture for multiple notification channels
- Production-ready with retry logic, idempotency, and observability
- Non-blocking, asynchronous delivery
- Extensible for future notification types

**Core Events:**
1. New organization signup
2. Successful payment processing
3. GPU node launches (SkyPilot)
4. Spot instance termination warnings

---

## Architecture Overview

### High-Level Design

```
┌─────────────────────────────────────────────────────────────────┐
│                     Application Layer                           │
│  (Tenants, Billing, Orchestrator, Scheduler)                   │
└────────────────┬────────────────────────────────────────────────┘
                 │ Publish Events
                 ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Event Bus (In-Memory)                       │
│  - Channel-based pub/sub                                        │
│  - Multiple subscribers per event type                          │
│  - Non-blocking async delivery                                  │
└────────────────┬────────────────────────────────────────────────┘
                 │ Subscribe
                 ▼
┌─────────────────────────────────────────────────────────────────┐
│                   Notification Service                           │
│  - Event routing                                                │
│  - Channel selection (Discord/Slack/Email/Webhook)              │
│  - Retry queue management                                       │
│  - Delivery tracking                                            │
└────────┬───────┬────────┬────────┬──────────────────────────────┘
         │       │        │        │
         ▼       ▼        ▼        ▼
    ┌────────┐ ┌─────┐ ┌──────┐ ┌─────────┐
    │Discord │ │Slack│ │Email │ │Webhook  │
    │Adapter │ │     │ │      │ │(Generic)│
    └────────┘ └─────┘ └──────┘ └─────────┘
```

### Design Principles

1. **Decoupling**: Event publishers don't know about notification channels
2. **Resilience**: Redis-backed retry queue for failed deliveries
3. **Observability**: Prometheus metrics for every delivery attempt
4. **Security**: Signature verification for webhooks, encrypted credentials
5. **Configuration**: Environment-based channel configuration
6. **Idempotency**: Prevent duplicate notifications using event IDs

---

## Component Design

### 1. Event Bus (`pkg/events/`)

**Purpose**: Decoupled pub/sub system for internal events

**Files to Create:**

#### `pkg/events/bus.go`
```go
package events

import (
    "context"
    "sync"
)

type EventType string

const (
    EventTenantCreated        EventType = "tenant.created"
    EventPaymentSucceeded     EventType = "payment.succeeded"
    EventNodeLaunched         EventType = "node.launched"
    EventNodeTerminated       EventType = "node.terminated"
    EventNodeHealthDegraded   EventType = "node.health_degraded"
    EventCostAnomalyDetected  EventType = "cost.anomaly_detected"
)

type Event struct {
    ID        string                 // Unique event ID for idempotency
    Type      EventType
    Timestamp time.Time
    TenantID  string                 // For tenant-scoped events
    Payload   map[string]interface{} // Event-specific data
}

type Handler func(ctx context.Context, event Event) error

type Bus struct {
    handlers map[EventType][]Handler
    mu       sync.RWMutex
    logger   *zap.Logger
}

func NewBus(logger *zap.Logger) *Bus
func (b *Bus) Subscribe(eventType EventType, handler Handler)
func (b *Bus) Publish(ctx context.Context, event Event) error
```

**Key Features:**
- Thread-safe subscriber management
- Multiple handlers per event type
- Async goroutine-based delivery
- Error logging without blocking publishers

---

### 2. Notification Service (`internal/notifications/`)

**Purpose**: Central notification orchestration and delivery

#### `internal/notifications/service.go`
```go
package notifications

type Service struct {
    config   *Config
    db       *database.Database
    cache    *cache.Cache
    logger   *zap.Logger
    bus      *events.Bus

    // Notification channels
    discord  *DiscordAdapter
    slack    *SlackAdapter
    email    *EmailAdapter
    webhook  *WebhookAdapter

    // Retry queue
    retryQueue chan *DeliveryTask
}

type DeliveryTask struct {
    ID           string
    EventID      string
    Channel      string
    Destination  string
    Payload      interface{}
    RetryCount   int
    MaxRetries   int
    CreatedAt    time.Time
}

func NewService(cfg *Config, db *database.Database, cache *cache.Cache, logger *zap.Logger, bus *events.Bus) *Service
func (s *Service) Start(ctx context.Context) error
func (s *Service) handleEvent(ctx context.Context, event events.Event) error
func (s *Service) deliver(ctx context.Context, task *DeliveryTask) error
func (s *Service) enqueueRetry(task *DeliveryTask) error
func (s *Service) processRetryQueue(ctx context.Context)
```

**Workflow:**
1. Subscribe to event bus
2. Route events to configured channels
3. Execute delivery via adapters
4. On failure: enqueue to retry queue (Redis)
5. Background worker processes retry queue
6. Track delivery status in database

---

### 3. Notification Channels

#### `internal/notifications/discord.go`

**Purpose**: Discord webhook integration with rich embeds

**Features:**
- Rich embeds with color-coded severity
- Inline fields for structured data
- Thumbnail/image support for branding
- Retry with exponential backoff

**Example Payload:**
```json
{
  "embeds": [{
    "title": "🎉 New Organization Signup",
    "description": "Acme Corp has joined CrossLogic!",
    "color": 3066993,
    "fields": [
      {"name": "Organization", "value": "Acme Corp", "inline": true},
      {"name": "Email", "value": "admin@acme.com", "inline": true},
      {"name": "Plan", "value": "Serverless", "inline": true}
    ],
    "timestamp": "2025-11-19T10:30:00Z"
  }]
}
```

#### `internal/notifications/slack.go`

**Purpose**: Slack webhook integration with Block Kit

**Features:**
- Block Kit for rich formatting
- Action buttons (future: approve/reject)
- Thread support for related events
- Mention support for urgent alerts

**Example Payload:**
```json
{
  "blocks": [
    {
      "type": "header",
      "text": {"type": "plain_text", "text": "💰 Payment Received"}
    },
    {
      "type": "section",
      "fields": [
        {"type": "mrkdwn", "text": "*Amount:*\n$1,234.56"},
        {"type": "mrkdwn", "text": "*Customer:*\nAcme Corp"}
      ]
    }
  ]
}
```

#### `internal/notifications/email.go`

**Purpose**: Email notifications via SMTP or SendGrid

**Features:**
- HTML templates with branding
- Plain text fallback
- Attachment support (invoices, reports)
- Configurable SMTP or SendGrid API

**Templates:**
- `tenant_created.html`
- `payment_succeeded.html`
- `node_launched.html`
- `node_terminated.html`

#### `internal/notifications/webhook.go`

**Purpose**: Generic webhook for custom integrations

**Features:**
- Configurable HTTP method (POST/PUT)
- Custom headers support
- HMAC signature for security
- Retry with exponential backoff
- Timeout configuration

**Example Request:**
```http
POST https://customer.example.com/webhooks/crosslogic
Content-Type: application/json
X-CrossLogic-Signature: sha256=abc123...
X-CrossLogic-Event-Type: payment.succeeded

{
  "event_id": "evt_123",
  "event_type": "payment.succeeded",
  "timestamp": "2025-11-19T10:30:00Z",
  "data": {
    "tenant_id": "tenant_123",
    "amount": 123456,
    "currency": "usd"
  }
}
```

---

### 4. Configuration (`internal/notifications/config.go`)

**Purpose**: Channel configuration and routing rules

```go
type Config struct {
    // Discord
    DiscordEnabled     bool
    DiscordWebhookURL  string

    // Slack
    SlackEnabled       bool
    SlackWebhookURL    string
    SlackChannel       string

    // Email
    EmailEnabled       bool
    EmailProvider      string // "smtp" or "sendgrid"
    EmailFrom          string
    EmailTo            []string
    SMTPHost           string
    SMTPPort           int
    SMTPUsername       string
    SMTPPassword       string
    SendGridAPIKey     string

    // Generic Webhook
    WebhookEnabled     bool
    WebhookURL         string
    WebhookSecret      string
    WebhookMethod      string
    WebhookHeaders     map[string]string

    // Retry Configuration
    MaxRetries         int
    RetryBackoffBase   time.Duration
    RetryQueueSize     int

    // Event Routing
    EventRouting       map[string][]string // event_type -> [channels]
}
```

**Environment Variables:**
```bash
# Discord
NOTIFICATIONS_DISCORD_ENABLED=true
NOTIFICATIONS_DISCORD_WEBHOOK_URL=https://discord.com/api/webhooks/...

# Slack
NOTIFICATIONS_SLACK_ENABLED=true
NOTIFICATIONS_SLACK_WEBHOOK_URL=https://hooks.slack.com/services/...

# Email
NOTIFICATIONS_EMAIL_ENABLED=true
NOTIFICATIONS_EMAIL_PROVIDER=sendgrid
NOTIFICATIONS_EMAIL_FROM=noreply@crosslogic.ai
NOTIFICATIONS_EMAIL_TO=ops@crosslogic.ai,billing@crosslogic.ai
NOTIFICATIONS_SENDGRID_API_KEY=SG.xxx

# Generic Webhook
NOTIFICATIONS_WEBHOOK_ENABLED=true
NOTIFICATIONS_WEBHOOK_URL=https://example.com/webhook
NOTIFICATIONS_WEBHOOK_SECRET=your-secret-key

# Event Routing (JSON)
NOTIFICATIONS_EVENT_ROUTING='{"payment.succeeded":["discord","slack","email"]}'
```

---

## Database Schema Changes

### New Table: `notification_deliveries`

**Purpose**: Track notification delivery status and audit trail

```sql
CREATE TABLE notification_deliveries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Event reference
    event_id VARCHAR(255) NOT NULL,
    event_type VARCHAR(100) NOT NULL,
    tenant_id UUID REFERENCES tenants(id),

    -- Delivery details
    channel VARCHAR(50) NOT NULL,  -- 'discord', 'slack', 'email', 'webhook'
    destination TEXT NOT NULL,      -- URL or email address

    -- Status tracking
    status VARCHAR(50) NOT NULL,    -- 'pending', 'sent', 'failed', 'retry'
    retry_count INT DEFAULT 0,
    max_retries INT DEFAULT 3,

    -- Request/Response
    request_payload JSONB,
    response_status INT,
    response_body TEXT,
    error_message TEXT,

    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    sent_at TIMESTAMP WITH TIME ZONE,
    failed_at TIMESTAMP WITH TIME ZONE,
    next_retry_at TIMESTAMP WITH TIME ZONE,

    -- Indexes
    INDEX idx_notification_deliveries_event_id (event_id),
    INDEX idx_notification_deliveries_status (status),
    INDEX idx_notification_deliveries_next_retry (next_retry_at) WHERE status = 'retry'
);
```

### New Table: `notification_config`

**Purpose**: Per-tenant notification preferences (future enhancement)

```sql
CREATE TABLE notification_config (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID REFERENCES tenants(id) NOT NULL,

    -- Channel configuration
    channel VARCHAR(50) NOT NULL,
    enabled BOOLEAN DEFAULT true,
    destination TEXT NOT NULL,

    -- Event filtering
    event_types TEXT[],  -- NULL = all events

    -- Custom settings
    settings JSONB,  -- Channel-specific settings

    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,

    UNIQUE(tenant_id, channel)
);
```

---

## Event Implementations

### Event 1: New Organization Signup

**Integration Point:** `control-plane/internal/gateway/tenants.go:61`

**Current Code:**
```go
func (g *Gateway) handleCreateTenant(w http.ResponseWriter, r *http.Request) {
    // ... existing code ...

    err := g.db.Pool.QueryRow(ctx, `
        INSERT INTO tenants (name, email, status, created_at, updated_at)
        VALUES ($1, $2, 'active', NOW(), NOW())
        RETURNING id
    `, req.Name, req.Email).Scan(&tenantID)

    // [INSERT NEW CODE HERE]
}
```

**New Code to Add:**
```go
// Publish tenant created event
event := events.Event{
    ID:        uuid.New().String(),
    Type:      events.EventTenantCreated,
    Timestamp: time.Now(),
    TenantID:  tenantID.String(),
    Payload: map[string]interface{}{
        "tenant_id":    tenantID.String(),
        "name":         req.Name,
        "email":        req.Email,
        "billing_plan": "serverless",
    },
}

if err := g.eventBus.Publish(ctx, event); err != nil {
    g.logger.Error("failed to publish tenant created event",
        zap.Error(err),
        zap.String("tenant_id", tenantID.String()),
    )
}
```

**Notification Payload:**
- Tenant ID
- Organization name
- Contact email
- Billing plan
- Creation timestamp

---

### Event 2: Successful Payment

**Integration Point:** `control-plane/internal/billing/webhooks.go:296`

**Current Code:**
```go
func (h *WebhookHandler) handlePaymentSucceeded(ctx context.Context, event stripe.Event) error {
    // ... existing code ...

    h.logger.Info("payment succeeded",
        zap.String("customer_id", customerID),
        zap.Int64("amount", paymentIntent.Amount),
    )

    // [INSERT NEW CODE HERE]

    return nil
}
```

**New Code to Add:**
```go
// Publish payment succeeded event
evt := events.Event{
    ID:        paymentIntent.ID,
    Type:      events.EventPaymentSucceeded,
    Timestamp: time.Now(),
    TenantID:  tenant.ID.String(),
    Payload: map[string]interface{}{
        "tenant_id":         tenant.ID.String(),
        "tenant_name":       tenant.Name,
        "amount":            paymentIntent.Amount,
        "currency":          string(paymentIntent.Currency),
        "amount_formatted":  fmt.Sprintf("$%.2f", float64(paymentIntent.Amount)/100),
        "payment_method":    paymentIntent.PaymentMethod,
        "stripe_payment_id": paymentIntent.ID,
    },
}

if err := h.eventBus.Publish(ctx, evt); err != nil {
    h.logger.Error("failed to publish payment succeeded event",
        zap.Error(err),
        zap.String("payment_id", paymentIntent.ID),
    )
}
```

**Notification Payload:**
- Tenant information
- Payment amount (formatted)
- Currency
- Payment method
- Stripe payment ID
- Timestamp

---

### Event 3: GPU Node Launched

**Integration Point:** `control-plane/internal/orchestrator/skypilot.go:365`

**Current Code:**
```go
func (o *Orchestrator) LaunchNode(ctx context.Context, config NodeConfig) (string, error) {
    // ... existing code ...

    o.logger.Info("GPU node launched successfully",
        zap.String("cluster_name", clusterName),
        zap.Duration("launch_duration", launchDuration),
        zap.String("node_id", config.NodeID),
    )

    // [INSERT NEW CODE HERE]

    if err := o.registerNode(ctx, config, clusterName); err != nil {
        // ...
    }
}
```

**New Code to Add:**
```go
// Publish node launched event
evt := events.Event{
    ID:        uuid.New().String(),
    Type:      events.EventNodeLaunched,
    Timestamp: time.Now(),
    Payload: map[string]interface{}{
        "node_id":         config.NodeID,
        "cluster_name":    clusterName,
        "provider":        config.Provider,
        "region":          config.Region,
        "instance_type":   config.InstanceType,
        "gpu_type":        config.AcceleratorType,
        "gpu_count":       config.AcceleratorCount,
        "spot_instance":   config.UseSpot,
        "spot_price":      config.SpotBidPrice,
        "model":           config.Model,
        "launch_duration": launchDuration.String(),
    },
}

if err := o.eventBus.Publish(ctx, evt); err != nil {
    o.logger.Error("failed to publish node launched event",
        zap.Error(err),
        zap.String("node_id", config.NodeID),
    )
}
```

**Notification Payload:**
- Node ID and cluster name
- Cloud provider and region
- Instance type and GPU details
- Spot vs on-demand status
- Model deployed
- Launch duration
- Cost estimate (future)

---

### Event 4: Spot Instance Termination

**Current Status:** NOT IMPLEMENTED - Needs new implementation

**Implementation Plan:**

#### Step 1: Create Spot Monitor in Node Agent

**New File:** `node-agent/internal/monitor/spot_monitor.go`

```go
package monitor

// SpotMonitor checks for spot termination warnings from cloud providers
type SpotMonitor struct {
    provider string  // aws, gcp, azure
    logger   *zap.Logger
    client   *http.Client
}

// AWS: Check metadata endpoint
// http://169.254.169.254/latest/meta-data/spot/instance-action
func (m *SpotMonitor) checkAWS(ctx context.Context) (*TerminationWarning, error)

// GCP: Check metadata endpoint
// http://metadata.google.internal/computeMetadata/v1/instance/preempted
func (m *SpotMonitor) checkGCP(ctx context.Context) (*TerminationWarning, error)

// Azure: Check scheduled events
// http://169.254.169.254/metadata/scheduledevents
func (m *SpotMonitor) checkAzure(ctx context.Context) (*TerminationWarning, error)

// Start monitoring loop (check every 5 seconds)
func (m *SpotMonitor) Start(ctx context.Context, callback func(*TerminationWarning))
```

#### Step 2: Add Termination Endpoint to Control Plane

**New Endpoint:** `POST /api/nodes/{node_id}/termination-warning`

**File:** `control-plane/internal/gateway/nodes.go` (new file)

```go
func (g *Gateway) handleNodeTerminationWarning(w http.ResponseWriter, r *http.Request) {
    nodeID := chi.URLParam(r, "node_id")

    var req struct {
        Provider        string    `json:"provider"`
        TerminationTime time.Time `json:"termination_time"`
        Reason          string    `json:"reason"`
    }

    // Decode request
    // Update node status to 'draining'
    // Redistribute traffic to other nodes
    // Publish termination event

    evt := events.Event{
        ID:        uuid.New().String(),
        Type:      events.EventNodeTerminated,
        Timestamp: time.Now(),
        Payload: map[string]interface{}{
            "node_id":          nodeID,
            "provider":         req.Provider,
            "termination_time": req.TerminationTime,
            "time_remaining":   time.Until(req.TerminationTime).String(),
            "reason":           req.Reason,
        },
    }

    g.eventBus.Publish(ctx, evt)
}
```

#### Step 3: Scheduler Integration

**File:** `control-plane/internal/scheduler/scheduler.go`

**Add Draining Logic:**
```go
// When node enters 'draining' state:
// 1. Stop routing new requests to this node
// 2. Wait for in-flight requests to complete
// 3. Mark node as 'dead' after grace period
// 4. Launch replacement node if needed
```

**Notification Payload:**
- Node ID and cluster name
- Cloud provider
- Termination time (ETA)
- Time remaining (urgency indicator)
- Reason (spot reclaim, maintenance, etc.)
- Current load on node
- Replacement status

---

## Security Considerations

### 1. Webhook Signature Verification

**Outbound Webhooks (Generic):**
```go
func (w *WebhookAdapter) sign(payload []byte) string {
    mac := hmac.New(sha256.New, []byte(w.secret))
    mac.Write(payload)
    return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}
```

**Recipients can verify:**
```python
import hmac
import hashlib

def verify_signature(payload, signature, secret):
    expected = "sha256=" + hmac.new(
        secret.encode(),
        payload,
        hashlib.sha256
    ).hexdigest()
    return hmac.compare_digest(signature, expected)
```

### 2. Credential Management

- Store webhook URLs and API keys in environment variables
- Encrypt sensitive configuration in database
- Use secret management service (AWS Secrets Manager, Vault)
- Rotate webhook secrets regularly

### 3. Rate Limiting

- Limit notification delivery rate per channel
- Prevent notification storms from misbehaving code
- Implement circuit breaker for failing channels

### 4. Access Control

- Admin-only endpoints for notification configuration
- Audit log for configuration changes
- Role-based access to notification settings

---

## Observability

### Prometheus Metrics

**File:** `internal/notifications/metrics.go`

```go
var (
    NotificationsPublished = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "notifications_published_total",
            Help: "Total number of notifications published",
        },
        []string{"event_type"},
    )

    NotificationsDelivered = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "notifications_delivered_total",
            Help: "Total number of notifications delivered",
        },
        []string{"event_type", "channel", "status"},
    )

    NotificationDeliveryDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "notification_delivery_duration_seconds",
            Help:    "Notification delivery duration",
            Buckets: prometheus.DefBuckets,
        },
        []string{"channel"},
    )

    NotificationRetries = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "notification_retries_total",
            Help: "Total number of notification retry attempts",
        },
        []string{"channel", "retry_count"},
    )
)
```

### Logging Standards

**Structured Logging with Zap:**
```go
logger.Info("notification delivered",
    zap.String("event_id", event.ID),
    zap.String("event_type", string(event.Type)),
    zap.String("channel", channel),
    zap.String("destination", destination),
    zap.Duration("duration", duration),
    zap.Int("retry_count", retryCount),
)
```

### Dashboard Metrics

- Notification delivery rate (per channel)
- Success/failure ratio
- Average delivery latency
- Retry queue depth
- Channel availability status

---

## Testing Strategy

### Unit Tests

**Files to Create:**
- `pkg/events/bus_test.go`
- `internal/notifications/service_test.go`
- `internal/notifications/discord_test.go`
- `internal/notifications/slack_test.go`
- `internal/notifications/email_test.go`
- `internal/notifications/webhook_test.go`

**Test Coverage:**
- Event publishing and subscription
- Notification routing logic
- Retry mechanism with exponential backoff
- Idempotency (duplicate event IDs)
- Error handling and fallback
- Signature verification

### Integration Tests

**Test Scenarios:**
1. End-to-end: Tenant creation → Discord notification
2. End-to-end: Payment → Multiple channels
3. Retry: Failed delivery → Retry queue → Success
4. Idempotency: Duplicate event → Single notification
5. Spot termination: Warning → Graceful shutdown

**Mock Services:**
- Mock Discord/Slack webhook endpoints
- Mock SMTP server for email testing
- Mock cloud metadata endpoints for spot monitoring

### Load Testing

**Test Notification Storm:**
- 1000 events published simultaneously
- Verify no blocking of event publishers
- Verify retry queue doesn't overflow
- Measure delivery latency at scale

---

## Additional Improvements & Future Enhancements

### 1. Real-Time WebSocket Notifications

**Purpose:** Push notifications to dashboard in real-time

**Implementation:**
- WebSocket endpoint: `ws://api.crosslogic.ai/ws`
- Subscribe to tenant-specific events
- Broadcast to connected dashboard clients
- Show toast notifications in UI

**Use Cases:**
- Show node launch progress in real-time
- Display payment confirmations instantly
- Alert on spot termination warnings

### 2. Cost Anomaly Detection

**Event:** `cost.anomaly_detected`

**Trigger Conditions:**
- Hourly cost exceeds 2x average
- Daily spend exceeds budget threshold
- Unexpected spike in token usage
- Spot price surge

**Notification Payload:**
```json
{
  "event_type": "cost.anomaly_detected",
  "tenant_id": "tenant_123",
  "anomaly_type": "hourly_spike",
  "current_cost": 245.67,
  "average_cost": 89.32,
  "threshold_exceeded": "2.75x",
  "recommendation": "Review recent API usage or check for runaway processes"
}
```

**Implementation:**
- Background job in billing engine
- Query cost metrics from billing_events table
- Compare against rolling averages
- Publish event if threshold exceeded

### 3. Node Health Degradation Warnings

**Event:** `node.health_degraded`

**Trigger Conditions:**
- Health score drops below 80
- Repeated request failures
- High latency (p95 > 5s)
- Memory/GPU utilization issues

**Notification Payload:**
```json
{
  "event_type": "node.health_degraded",
  "node_id": "node_123",
  "cluster_name": "gpu-cluster-us-west",
  "health_score": 65.5,
  "previous_score": 98.2,
  "issues": [
    "High GPU memory usage (95%)",
    "Elevated p95 latency (8.2s)"
  ],
  "recommendation": "Consider draining and replacing this node"
}
```

**Implementation:**
- Scheduler monitors node health scores
- Publish event when score drops significantly
- Include diagnostic information
- Suggest remediation actions

### 4. API Rate Limit Warnings

**Event:** `ratelimit.threshold_reached`

**Trigger Conditions:**
- API key usage reaches 80% of rate limit
- Tenant approaching quota limits
- Predicted to exceed limit within 1 hour

**Notification Payload:**
```json
{
  "event_type": "ratelimit.threshold_reached",
  "tenant_id": "tenant_123",
  "api_key_id": "key_abc",
  "current_usage": 8500,
  "limit": 10000,
  "threshold_percent": 85,
  "time_window": "1h",
  "recommendation": "Consider upgrading plan or optimizing request rate"
}
```

**Implementation:**
- Gateway tracks rate limit consumption
- Redis-backed counter with TTL
- Publish warning at 80%, 90%, 95% thresholds
- Include upgrade options in notification

### 5. Batch Notification Aggregation

**Purpose:** Avoid notification fatigue from high-frequency events

**Implementation:**
```go
type NotificationBatch struct {
    EventType      string
    Events         []events.Event
    AggregationWindow time.Duration
    MinBatchSize   int
}

// Example: Aggregate node launches
// Instead of 10 separate notifications for 10 nodes:
// Send 1 notification: "10 GPU nodes launched in the last 5 minutes"
```

**Aggregation Rules:**
- Node launches: Batch every 5 minutes
- Payment confirmations: Send immediately (high priority)
- Health degradations: Batch per node (max 1/hour)
- Cost anomalies: Send immediately (urgent)

### 6. Notification Templates & Customization

**Purpose:** Allow tenants to customize notification format

**Database Schema:**
```sql
CREATE TABLE notification_templates (
    id UUID PRIMARY KEY,
    tenant_id UUID REFERENCES tenants(id),
    event_type VARCHAR(100),
    channel VARCHAR(50),
    template_type VARCHAR(50),  -- 'discord_embed', 'slack_block', 'email_html'
    template_content JSONB,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
);
```

**Example Customization:**
- Custom Discord embed colors
- Custom email branding
- Custom message templates with variables
- Include/exclude specific fields

### 7. Multi-Channel Fallback Strategy

**Purpose:** Ensure critical notifications are delivered

**Strategy:**
```yaml
payment.succeeded:
  primary: discord
  fallback: [slack, email]
  retry_primary: true
  fallback_delay: 5m

node.terminated:
  primary: slack
  fallback: [email, webhook]
  retry_primary: false  # Skip retry, use fallback immediately
  urgent: true
```

**Implementation:**
- Try primary channel first
- On failure, wait fallback_delay
- Attempt fallback channels in order
- Mark as delivered once any channel succeeds

### 8. Notification Preferences API

**New Endpoints:**
```
GET    /api/notifications/config
PUT    /api/notifications/config
POST   /api/notifications/test
GET    /api/notifications/history
```

**Features:**
- Per-tenant notification configuration
- Enable/disable specific event types
- Configure channel preferences
- Test notification delivery
- View delivery history and logs

### 9. Incident Response Automation

**Event:** `incident.created`

**Trigger:** Critical system failures

**Automation:**
1. Create PagerDuty/Opsgenie incident
2. Send urgent Slack notification with @channel
3. Email on-call engineer
4. Create Jira ticket automatically
5. Update status page

**Integration Points:**
- Scheduler detects all nodes unhealthy
- Database connection failures
- Redis unavailability
- Critical API errors (500s > threshold)

### 10. Notification Analytics Dashboard

**Metrics to Track:**
- Delivery success rate by channel
- Average time to delivery
- Most common notification types
- Peak notification times
- Failed delivery reasons
- Cost per notification (for paid services)

**Visualization:**
- Grafana dashboard
- Real-time delivery status
- Historical trends
- Channel comparison

---

## Implementation Phases

### Phase 1: Core Infrastructure (Week 1)

**Tasks:**
1. Create event bus (`pkg/events/bus.go`)
2. Create notification service skeleton (`internal/notifications/service.go`)
3. Add configuration system (`internal/notifications/config.go`)
4. Create database tables
5. Write unit tests for event bus

**Deliverables:**
- Working pub/sub event system
- Configuration via environment variables
- Database schema deployed

### Phase 2: Notification Channels (Week 2)

**Tasks:**
1. Implement Discord adapter with embeds
2. Implement Slack adapter with Block Kit
3. Implement Email adapter (SMTP/SendGrid)
4. Implement generic webhook adapter
5. Add retry queue with Redis backing
6. Write integration tests

**Deliverables:**
- All 4 notification channels working
- Retry mechanism operational
- Test coverage >80%

### Phase 3: Event Integration (Week 3)

**Tasks:**
1. Hook into tenant creation (tenants.go)
2. Hook into payment success (webhooks.go)
3. Hook into node launches (skypilot.go)
4. Implement spot monitoring (node-agent)
5. Add spot termination endpoint
6. Test end-to-end flows

**Deliverables:**
- All 4 core events triggering notifications
- Spot termination detection working
- Production-ready code

### Phase 4: Observability & Operations (Week 4)

**Tasks:**
1. Add Prometheus metrics
2. Create Grafana dashboard
3. Add admin endpoints for configuration
4. Implement notification preferences API
5. Write operations documentation
6. Security audit and hardening

**Deliverables:**
- Comprehensive observability
- Admin API for management
- Production deployment guide
- Security review completed

### Phase 5: Enhanced Features (Future)

**Tasks:**
1. Cost anomaly detection
2. Node health warnings
3. WebSocket real-time notifications
4. Notification templates
5. Batch aggregation
6. Multi-channel fallback

**Deliverables:**
- Enhanced notification intelligence
- Reduced notification noise
- Improved user experience

---

## Deployment Strategy

### Step 1: Infrastructure Preparation

1. **Add Environment Variables:**
   ```bash
   # Add to .env or Kubernetes ConfigMap
   NOTIFICATIONS_DISCORD_ENABLED=true
   NOTIFICATIONS_DISCORD_WEBHOOK_URL=...
   NOTIFICATIONS_SLACK_ENABLED=true
   NOTIFICATIONS_SLACK_WEBHOOK_URL=...
   ```

2. **Deploy Database Schema:**
   ```bash
   psql -U postgres -d crosslogic < database/schemas/notification_deliveries.sql
   ```

3. **Configure Redis:**
   - Ensure Redis is available for retry queue
   - Test connection from control plane

### Step 2: Code Deployment

1. **Build and Test:**
   ```bash
   cd control-plane
   go test ./...
   go build -o bin/server cmd/server/main.go
   ```

2. **Deploy to Staging:**
   - Deploy control plane with new code
   - Verify event bus initialization
   - Test notification delivery

3. **Smoke Tests:**
   ```bash
   # Test tenant creation
   curl -X POST /admin/tenants -d '{"name":"Test","email":"test@example.com"}'

   # Verify Discord notification received
   ```

### Step 3: Production Rollout

1. **Enable for Single Tenant (Canary):**
   ```bash
   # Configure routing for test tenant only
   NOTIFICATIONS_EVENT_ROUTING='{"tenant.created":["discord"]}'
   ```

2. **Monitor Metrics:**
   - Watch Prometheus metrics
   - Check error logs
   - Verify delivery success rate

3. **Gradual Rollout:**
   - Enable Slack notifications
   - Enable Email notifications
   - Enable generic webhooks
   - Enable all event types

### Step 4: Documentation

1. **Operations Runbook:**
   - How to configure new channels
   - How to troubleshoot failed deliveries
   - How to test notifications

2. **User Guide:**
   - How to set up Discord webhooks
   - How to configure Slack integration
   - How to use notification preferences API

---

## Rollback Plan

### If Notifications Fail:

1. **Disable Notification Service:**
   ```bash
   NOTIFICATIONS_DISCORD_ENABLED=false
   NOTIFICATIONS_SLACK_ENABLED=false
   NOTIFICATIONS_EMAIL_ENABLED=false
   ```

2. **Event Bus Isolation:**
   - Event bus is non-blocking
   - Failure won't impact core functionality
   - Publishers continue working

3. **Database Rollback:**
   ```sql
   DROP TABLE notification_deliveries;
   DROP TABLE notification_config;
   ```

### If Performance Issues:

1. **Disable Retry Queue:**
   - Comment out retry worker goroutine
   - Prevent retry storms

2. **Increase Timeout:**
   - Increase HTTP client timeout
   - Reduce concurrent deliveries

3. **Circuit Breaker:**
   - Implement per-channel circuit breaker
   - Auto-disable failing channels

---

## Cost Analysis

### Free Tiers:

- **Discord Webhooks:** Free, unlimited
- **Slack Webhooks:** Free, unlimited
- **Generic Webhooks:** Free (your infrastructure)

### Paid Services:

- **SendGrid Email:**
  - Free: 100 emails/day
  - Essentials: $19.95/month for 50k emails
  - Estimated: ~$20/month for moderate usage

- **Infrastructure:**
  - Redis (if separate): ~$10/month (AWS ElastiCache)
  - Database storage: Minimal (<1GB for notifications)
  - Bandwidth: Minimal (small payloads)

**Total Estimated Cost:** $20-30/month

### Optimization:

- Use free tiers where possible
- Batch email notifications to reduce send count
- Implement notification aggregation
- Archive old deliveries after 90 days

---

## Success Metrics

### Technical Metrics:

- **Delivery Success Rate:** >99.5%
- **Average Delivery Latency:** <2 seconds
- **Event Publish Latency:** <10ms (non-blocking)
- **Retry Success Rate:** >95% after 3 retries
- **System Impact:** <5% CPU overhead

### Business Metrics:

- **Incident Response Time:** Reduced by 50%
- **Payment Visibility:** Real-time confirmation
- **Operational Awareness:** 100% coverage of critical events
- **Customer Satisfaction:** Improved with proactive notifications

### Reliability Metrics:

- **Uptime:** 99.9% notification system availability
- **Data Loss:** 0% (events persisted to database)
- **Duplicate Rate:** <0.1% (idempotency working)

---

## Conclusion

This notification system design provides:

✅ **Pluggable Architecture:** Easy to add new channels (SMS, PagerDuty, etc.)
✅ **Production-Ready:** Retry logic, idempotency, observability
✅ **Non-Blocking:** Async delivery doesn't impact core services
✅ **Extensible:** Event bus supports unlimited event types
✅ **Secure:** Signature verification, encrypted credentials
✅ **Observable:** Comprehensive metrics and logging
✅ **Cost-Effective:** Leverages free tiers, minimal infrastructure

The system integrates seamlessly with existing CrossLogic architecture patterns (middleware, background jobs, webhook handlers) and provides a foundation for future enhancements like AI-driven alerting, intelligent routing, and advanced analytics.

**Next Steps:**
1. Review and approve this design
2. Create GitHub issues for each phase
3. Begin Phase 1 implementation
4. Set up staging environment for testing
5. Plan production rollout timeline

**Questions or Concerns?** Please provide feedback and I'll refine the design accordingly.
