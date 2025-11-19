package billing

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/crosslogic/control-plane/pkg/cache"
	"github.com/crosslogic/control-plane/pkg/database"
	"github.com/crosslogic/control-plane/pkg/events"
	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/webhook"
	"go.uber.org/zap"
)

const (
	webhookProcessedTTL  = 24 * time.Hour
	webhookProcessingTTL = 5 * time.Minute
)

// WebhookHandler processes Stripe webhook events for payment automation.
//
// It handles critical payment lifecycle events including:
// - Payment confirmation (payment_intent.succeeded)
// - Payment failures (payment_intent.payment_failed)
// - Subscription updates (customer.subscription.updated)
// - Invoice payments (invoice.payment_succeeded)
//
// All webhook events are verified using Stripe's signature verification
// to ensure authenticity and prevent replay attacks.
//
// Architecture:
// - Idempotent event processing with database-backed deduplication
// - Transactional database updates for consistency
// - Comprehensive error logging and monitoring
// - Graceful degradation for non-critical failures
//
// Production Considerations:
// - Webhook endpoints should use HTTPS with valid certificates
// - Configure Stripe webhook secret in environment variables
// - Monitor webhook processing latency and failure rates
// - Set up alerting for payment failures requiring manual intervention
type WebhookHandler struct {
	// webhookSecret is the Stripe webhook signing secret used to verify event authenticity.
	// This value is obtained from the Stripe Dashboard under Developers > Webhooks.
	webhookSecret string

	// db provides access to the PostgreSQL database for tenant and billing record updates
	db *database.Database

	// logger provides structured logging for observability and debugging
	logger *zap.Logger

	// cache provides distributed idempotency tracking
	cache *cache.Cache

	// eventBus for publishing payment events
	eventBus *events.Bus

	// processedEvents tracks processed webhook IDs to ensure idempotency.
	// In production, this should be backed by a distributed cache (Redis) or database table.
	processedEvents map[string]time.Time

	mu sync.Mutex
}

// webhookEvent represents a processed webhook event stored in the database
// for audit trail and idempotency checking.
type webhookEvent struct {
	ID          uuid.UUID `json:"id"`
	EventID     string    `json:"event_id"`     // Stripe event ID
	EventType   string    `json:"event_type"`   // e.g., "payment_intent.succeeded"
	ProcessedAt time.Time `json:"processed_at"` // Timestamp when event was processed
	Payload     []byte    `json:"payload"`      // Raw event payload for debugging
}

// NewWebhookHandler creates a new Stripe webhook handler with the provided configuration.
//
// Parameters:
// - webhookSecret: The Stripe webhook signing secret from the Stripe Dashboard
// - db: Database connection for tenant and billing record updates
// - logger: Structured logger for observability
//
// Returns:
// - *WebhookHandler: Configured webhook handler ready to process events
//
// Example:
//
//	handler := NewWebhookHandler(
//	    os.Getenv("STRIPE_WEBHOOK_SECRET"),
//	    database,
//	    logger,
//	)
//	http.HandleFunc("/api/webhooks/stripe", handler.HandleWebhook)
func NewWebhookHandler(webhookSecret string, db *database.Database, cacheClient *cache.Cache, logger *zap.Logger, eventBus *events.Bus) *WebhookHandler {
	return &WebhookHandler{
		webhookSecret:   webhookSecret,
		db:              db,
		cache:           cacheClient,
		eventBus:        eventBus,
		logger:          logger,
		processedEvents: make(map[string]time.Time),
	}
}

// HandleWebhook processes incoming Stripe webhook events.
//
// This is the main entry point for all Stripe webhook events. It performs:
// 1. Request body reading and validation
// 2. Stripe signature verification (prevents replay attacks)
// 3. Event routing to appropriate handlers
// 4. Idempotency checking to prevent duplicate processing
// 5. Error handling and logging
//
// HTTP Response Codes:
// - 200 OK: Event processed successfully
// - 400 Bad Request: Invalid request body or signature
// - 500 Internal Server Error: Database or processing error
//
// Security:
// - All events MUST pass Stripe signature verification
// - Events are deduplicated using Stripe event IDs
// - Unknown event types are safely ignored (logged but not failed)
//
// Example Stripe webhook configuration:
//
//	Endpoint URL: https://api.crosslogic.ai/api/webhooks/stripe
//	Events to send:
//	  - payment_intent.succeeded
//	  - payment_intent.payment_failed
//	  - customer.subscription.updated
//	  - invoice.payment_succeeded
func (h *WebhookHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Step 1: Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Error("failed to read webhook body",
			zap.Error(err),
		)
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	// Step 2: Verify Stripe signature to ensure authenticity
	signature := r.Header.Get("Stripe-Signature")
	event, err := webhook.ConstructEvent(body, signature, h.webhookSecret)
	if err != nil {
		h.logger.Warn("webhook signature verification failed",
			zap.Error(err),
			zap.String("signature", signature),
		)
		http.Error(w, "Invalid signature", http.StatusBadRequest)
		return
	}

	// Step 3: Acquire idempotency lock
	lockAcquired, err := h.reserveEvent(ctx, event.ID)
	if err != nil {
		h.logger.Error("failed to reserve webhook event",
			zap.Error(err),
			zap.String("event_id", event.ID),
		)
		http.Error(w, "Failed to reserve event", http.StatusInternalServerError)
		return
	}
	if !lockAcquired {
		h.logger.Info("webhook event already in progress or processed",
			zap.String("event_id", event.ID),
			zap.String("event_type", string(event.Type)),
		)
		w.WriteHeader(http.StatusOK)
		return
	}
	var handlerErr error

	defer func() {
		if handlerErr != nil {
			h.finalizeEvent(ctx, event.ID, false)
		} else {
			h.finalizeEvent(ctx, event.ID, true)
		}
	}()

	// Step 4: Log incoming event for observability
	h.logger.Info("processing webhook event",
		zap.String("event_id", event.ID),
		zap.String("event_type", string(event.Type)),
		zap.Time("created", time.Unix(event.Created, 0)),
	)

	// Step 5: Route to appropriate handler based on event type
	switch event.Type {
	case "payment_intent.succeeded":
		handlerErr = h.handlePaymentSucceeded(ctx, event)
	case "payment_intent.payment_failed":
		handlerErr = h.handlePaymentFailed(ctx, event)
	case "customer.subscription.updated":
		handlerErr = h.handleSubscriptionUpdated(ctx, event)
	case "invoice.payment_succeeded":
		handlerErr = h.handleInvoicePaymentSucceeded(ctx, event)
	default:
		// Unknown event type - log but don't fail (allows Stripe to add new events)
		h.logger.Info("received unknown webhook event type",
			zap.String("event_id", event.ID),
			zap.String("event_type", string(event.Type)),
		)
	}

	// Step 6: Handle processing errors
	if handlerErr != nil {
		h.logger.Error("webhook event processing failed",
			zap.Error(handlerErr),
			zap.String("event_id", event.ID),
			zap.String("event_type", string(event.Type)),
		)
		http.Error(w, "Event processing failed", http.StatusInternalServerError)
		return
	}

	// Step 7: Mark event as processed and persist to database
	if err := h.markEventProcessed(ctx, event, body); err != nil {
		h.logger.Error("failed to mark event as processed",
			zap.Error(err),
			zap.String("event_id", event.ID),
		)
		// Don't fail the webhook - event was processed successfully
	}

	// Step 8: Return success to Stripe
	w.WriteHeader(http.StatusOK)
}

// handlePaymentSucceeded processes successful payment events.
//
// When a payment is successfully completed:
// 1. Extract customer ID from payment intent
// 2. Update tenant status to 'active'
// 3. Log success for audit trail
//
// Database updates:
// - tenants.status = 'active'
// - tenants.updated_at = NOW()
//
// This event typically occurs after:
// - Initial subscription payment
// - Recurring subscription payment
// - One-time payment for credits
func (h *WebhookHandler) handlePaymentSucceeded(ctx context.Context, event stripe.Event) error {
	// Parse payment intent from event data
	var paymentIntent stripe.PaymentIntent
	if err := json.Unmarshal(event.Data.Raw, &paymentIntent); err != nil {
		return fmt.Errorf("failed to unmarshal payment intent: %w", err)
	}

	// Extract customer ID
	customerID := paymentIntent.Customer.ID
	if customerID == "" {
		return fmt.Errorf("payment intent missing customer ID")
	}

	// Start transaction
	tx, err := h.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Update tenant status to active
	query := `
		UPDATE tenants
		SET status = 'active', updated_at = NOW()
		WHERE stripe_customer_id = $1
		RETURNING id, name
	`

	var tenantID uuid.UUID
	var tenantName string
	err = tx.QueryRow(ctx, query, customerID).Scan(&tenantID, &tenantName)
	if err != nil {
		return fmt.Errorf("failed to update tenant status: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	h.logger.Info("payment succeeded - tenant activated",
		zap.String("tenant_id", tenantID.String()),
		zap.String("tenant_name", tenantName),
		zap.String("customer_id", customerID),
		zap.Int64("amount", paymentIntent.Amount),
		zap.String("currency", string(paymentIntent.Currency)),
	)

	// Publish payment succeeded event
	if h.eventBus != nil {
		evt := events.NewEvent(
			events.EventPaymentSucceeded,
			tenantID.String(),
			map[string]interface{}{
				"tenant_id":         tenantID.String(),
				"tenant_name":       tenantName,
				"amount":            paymentIntent.Amount,
				"currency":          string(paymentIntent.Currency),
				"amount_formatted":  fmt.Sprintf("$%.2f", float64(paymentIntent.Amount)/100),
				"payment_method":    paymentIntent.PaymentMethod,
				"stripe_payment_id": paymentIntent.ID,
			},
		)
		if err := h.eventBus.Publish(ctx, evt); err != nil {
			h.logger.Error("failed to publish payment succeeded event",
				zap.Error(err),
				zap.String("payment_id", paymentIntent.ID),
			)
		}
	}

	return nil
}

// handlePaymentFailed processes failed payment events.
//
// When a payment fails:
// 1. Extract customer ID and failure reason
// 2. Suspend tenant account to prevent unauthorized usage
// 3. Log failure details for customer support
// 4. Trigger email notification (TODO: implement email service)
//
// Database updates:
// - tenants.status = 'suspended'
// - tenants.updated_at = NOW()
//
// Common failure reasons:
// - Insufficient funds
// - Expired card
// - Card declined by issuer
// - Authentication required (3D Secure)
//
// Action items:
// - Send email to customer with payment update link
// - Create support ticket for manual follow-up if needed
// - Log detailed failure reason for analytics
func (h *WebhookHandler) handlePaymentFailed(ctx context.Context, event stripe.Event) error {
	// Parse payment intent from event data
	var paymentIntent stripe.PaymentIntent
	if err := json.Unmarshal(event.Data.Raw, &paymentIntent); err != nil {
		return fmt.Errorf("failed to unmarshal payment intent: %w", err)
	}

	// Extract customer ID
	customerID := paymentIntent.Customer.ID
	if customerID == "" {
		return fmt.Errorf("payment intent missing customer ID")
	}

	// Extract failure details
	failureCode := ""
	failureMessage := ""
	if paymentIntent.LastPaymentError != nil {
		failureCode = string(paymentIntent.LastPaymentError.Code)
		failureMessage = paymentIntent.LastPaymentError.Msg
	}

	// Start transaction
	tx, err := h.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Suspend tenant account
	query := `
		UPDATE tenants
		SET status = 'suspended', updated_at = NOW()
		WHERE stripe_customer_id = $1
		RETURNING id, name, email
	`

	var tenantID uuid.UUID
	var tenantName, tenantEmail string
	err = tx.QueryRow(ctx, query, customerID).Scan(&tenantID, &tenantName, &tenantEmail)
	if err != nil {
		return fmt.Errorf("failed to suspend tenant: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	h.logger.Warn("payment failed - tenant suspended",
		zap.String("tenant_id", tenantID.String()),
		zap.String("tenant_name", tenantName),
		zap.String("tenant_email", tenantEmail),
		zap.String("customer_id", customerID),
		zap.String("failure_code", failureCode),
		zap.String("failure_message", failureMessage),
	)

	// TODO: Send email notification to customer
	// Example:
	// h.emailService.SendPaymentFailedNotification(ctx, &EmailPayload{
	//     To: tenantEmail,
	//     TenantName: tenantName,
	//     FailureReason: failureMessage,
	//     UpdatePaymentURL: fmt.Sprintf("https://app.crosslogic.ai/billing?customer=%s", customerID),
	// })

	return nil
}

// handleSubscriptionUpdated processes subscription lifecycle events.
//
// When a subscription changes:
// 1. Extract subscription details (plan, status, billing interval)
// 2. Update tenant billing plan
// 3. Update tenant status based on subscription state
// 4. Log subscription change for audit trail
//
// Database updates:
// - tenants.billing_plan = subscription.items[0].price.id
// - tenants.status = subscription.status (active, canceled, past_due, etc.)
// - tenants.updated_at = NOW()
//
// Subscription statuses:
// - active: Subscription is active and paid
// - past_due: Payment failed, retry in progress
// - canceled: Subscription canceled by customer
// - unpaid: Payment failed after all retries
// - trialing: In free trial period
//
// This event is triggered by:
// - Plan upgrades/downgrades
// - Subscription cancellations
// - Trial start/end
// - Billing interval changes
func (h *WebhookHandler) handleSubscriptionUpdated(ctx context.Context, event stripe.Event) error {
	// Parse subscription from event data
	var subscription stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &subscription); err != nil {
		return fmt.Errorf("failed to unmarshal subscription: %w", err)
	}

	// Extract customer ID
	customerID := subscription.Customer.ID
	if customerID == "" {
		return fmt.Errorf("subscription missing customer ID")
	}

	// Extract subscription details
	status := string(subscription.Status)
	var priceID string
	if len(subscription.Items.Data) > 0 {
		priceID = subscription.Items.Data[0].Price.ID
	}

	// Map subscription status to tenant status
	tenantStatus := mapSubscriptionStatus(subscription.Status)

	// Start transaction
	tx, err := h.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Update tenant billing plan and status
	query := `
		UPDATE tenants
		SET billing_plan = $1, status = $2, updated_at = NOW()
		WHERE stripe_customer_id = $3
		RETURNING id, name
	`

	var tenantID uuid.UUID
	var tenantName string
	err = tx.QueryRow(ctx, query, priceID, tenantStatus, customerID).Scan(&tenantID, &tenantName)
	if err != nil {
		return fmt.Errorf("failed to update tenant subscription: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	h.logger.Info("subscription updated",
		zap.String("tenant_id", tenantID.String()),
		zap.String("tenant_name", tenantName),
		zap.String("customer_id", customerID),
		zap.String("subscription_status", status),
		zap.String("price_id", priceID),
		zap.String("tenant_status", tenantStatus),
	)

	return nil
}

// handleInvoicePaymentSucceeded processes successful invoice payment events.
//
// When an invoice is paid:
// 1. Extract customer ID from invoice
// 2. Mark all unbilled usage records as billed
// 3. Update billing_events table with export record
// 4. Log success for accounting audit trail
//
// Database updates:
// - usage_records.billed = true (for all unbilled records of this tenant)
// - billing_events: Insert new record with invoice details
//
// This event occurs after:
// - Monthly subscription invoice payment
// - Usage-based billing invoice payment
// - One-time invoice payment
//
// Usage records are marked as billed to prevent double-charging
// in the next billing cycle.
func (h *WebhookHandler) handleInvoicePaymentSucceeded(ctx context.Context, event stripe.Event) error {
	// Parse invoice from event data
	var invoice stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &invoice); err != nil {
		return fmt.Errorf("failed to unmarshal invoice: %w", err)
	}

	// Extract customer ID
	customerID := invoice.Customer.ID
	if customerID == "" {
		return fmt.Errorf("invoice missing customer ID")
	}

	// Start a transaction for atomic updates
	tx, err := h.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Get tenant ID
	var tenantID uuid.UUID
	err = tx.QueryRow(ctx, "SELECT id FROM tenants WHERE stripe_customer_id = $1", customerID).Scan(&tenantID)
	if err != nil {
		return fmt.Errorf("failed to get tenant ID: %w", err)
	}

	// Mark usage records as billed
	usageQuery := `
		UPDATE usage_records
		SET billed = true
		WHERE tenant_id = $1 AND billed = false
	`
	result, err := tx.Exec(ctx, usageQuery, tenantID)
	if err != nil {
		return fmt.Errorf("failed to mark usage as billed: %w", err)
	}

	rowsAffected := result.RowsAffected()

	// Insert billing event record
	billingEventQuery := `
		INSERT INTO billing_events (
			id, tenant_id, event_type, stripe_invoice_id,
			amount, currency, created_at
		) VALUES ($1, $2, 'invoice_paid', $3, $4, $5, NOW())
	`
	_, err = tx.Exec(ctx, billingEventQuery,
		uuid.New(),
		tenantID,
		invoice.ID,
		invoice.AmountPaid,
		string(invoice.Currency),
	)
	if err != nil {
		return fmt.Errorf("failed to insert billing event: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	h.logger.Info("invoice payment succeeded - usage marked as billed",
		zap.String("tenant_id", tenantID.String()),
		zap.String("customer_id", customerID),
		zap.String("invoice_id", invoice.ID),
		zap.Int64("amount_paid", invoice.AmountPaid),
		zap.String("currency", string(invoice.Currency)),
		zap.Int64("usage_records_billed", rowsAffected),
	)

	return nil
}

// markEventProcessed marks a webhook event as processed and persists to database.
//
// This stores the event in both:
// 1. In-memory cache for fast duplicate detection
// 2. Database for audit trail and recovery
//
// The event payload is stored for debugging and support purposes.
//
// Production considerations:
// - Periodically clean up old events (older than 30 days)
// - Monitor storage growth
// - Use compressed payload storage for large events
func (h *WebhookHandler) markEventProcessed(ctx context.Context, event stripe.Event, payload []byte) error {
	if h.db == nil {
		return nil // Skip persistence if DB is not configured (e.g. testing)
	}

	// Persist to database for audit trail
	query := `
		INSERT INTO webhook_events (
			id, event_id, event_type, processed_at, payload
		) VALUES ($1, $2, $3, NOW(), $4)
		ON CONFLICT (event_id) DO NOTHING
	`

	_, err := h.db.Pool.Exec(ctx, query,
		uuid.New(),
		event.ID,
		event.Type,
		payload,
	)

	if err != nil {
		return fmt.Errorf("failed to persist webhook event: %w", err)
	}

	return nil
}

func (h *WebhookHandler) reserveEvent(ctx context.Context, eventID string) (bool, error) {
	if h.cache != nil {
		key := h.redisKeyForEvent(eventID)
		acquired, err := h.cache.SetNX(ctx, key, "processing", webhookProcessingTTL)
		return acquired, err
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	h.cleanupExpiredEvents(time.Now())
	if _, exists := h.processedEvents[eventID]; exists {
		return false, nil
	}
	h.processedEvents[eventID] = time.Now()
	return true, nil
}

func (h *WebhookHandler) finalizeEvent(ctx context.Context, eventID string, success bool) {
	if h.cache != nil {
		key := h.redisKeyForEvent(eventID)
		if success {
			if err := h.cache.Set(ctx, key, "processed", webhookProcessedTTL); err != nil {
				h.logger.Warn("failed to persist webhook completion in cache",
					zap.String("event_id", eventID),
					zap.Error(err),
				)
			}
		} else {
			if err := h.cache.Delete(ctx, key); err != nil {
				h.logger.Warn("failed to release webhook lock",
					zap.String("event_id", eventID),
					zap.Error(err),
				)
			}
		}
		return
	}

	if !success {
		h.mu.Lock()
		delete(h.processedEvents, eventID)
		h.mu.Unlock()
	}
}

func (h *WebhookHandler) redisKeyForEvent(eventID string) string {
	return fmt.Sprintf("webhooks:stripe:%s", eventID)
}

func (h *WebhookHandler) cleanupExpiredEvents(now time.Time) {
	for id, ts := range h.processedEvents {
		if now.Sub(ts) > webhookProcessedTTL {
			delete(h.processedEvents, id)
		}
	}
}

// mapSubscriptionStatus maps Stripe subscription status to tenant status.
//
// Stripe subscription statuses:
// - active: Subscription is current and paid
// - past_due: Payment failed, but still in retry period
// - unpaid: Payment failed after all retries
// - canceled: Customer canceled subscription
// - incomplete: Initial payment not completed
// - incomplete_expired: Initial payment expired
// - trialing: In free trial period
//
// Tenant statuses:
// - active: Can use the service
// - suspended: Temporarily disabled (payment issues, can be restored)
// - canceled: Permanently disabled (requires new subscription)
func mapSubscriptionStatus(stripeStatus stripe.SubscriptionStatus) string {
	switch stripeStatus {
	case stripe.SubscriptionStatusActive:
		return "active"
	case stripe.SubscriptionStatusTrialing:
		return "active" // Allow usage during trial
	case stripe.SubscriptionStatusPastDue:
		return "suspended" // Temporary suspension while retrying payment
	case stripe.SubscriptionStatusUnpaid:
		return "suspended" // Payment failed, awaiting customer action
	case stripe.SubscriptionStatusCanceled:
		return "canceled" // Permanent cancellation
	case stripe.SubscriptionStatusIncomplete:
		return "suspended" // Awaiting initial payment
	case stripe.SubscriptionStatusIncompleteExpired:
		return "canceled" // Initial payment window expired
	default:
		return "suspended" // Safe default for unknown statuses
	}
}
