package notifications

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/crosslogic/control-plane/pkg/cache"
	"github.com/crosslogic/control-plane/pkg/database"
	"github.com/crosslogic/control-plane/pkg/events"
	"go.uber.org/zap"
)

// Service is the main notification service that orchestrates delivery
type Service struct {
	config *Config
	db     *database.Database
	cache  *cache.Cache
	logger *zap.Logger
	bus    *events.Bus

	// Notification channel adapters
	discord *DiscordAdapter
	slack   *SlackAdapter
	email   *EmailAdapter
	webhook *WebhookAdapter

	// Retry queue
	retryQueue chan *DeliveryTask
	stopChan   chan struct{}
	wg         sync.WaitGroup

	// Metrics
	metrics *Metrics
}

// DeliveryTask represents a notification delivery task
type DeliveryTask struct {
	ID          string
	EventID     string
	EventType   string
	TenantID    string
	Channel     string
	Destination string
	Payload     interface{}
	RetryCount  int
	MaxRetries  int
	CreatedAt   time.Time
	LastAttempt time.Time
}

// NewService creates a new notification service
func NewService(
	config *Config,
	db *database.Database,
	cache *cache.Cache,
	logger *zap.Logger,
	bus *events.Bus,
) (*Service, error) {
	if !config.Enabled {
		logger.Info("notification service is disabled")
		return &Service{
			config: config,
			logger: logger,
		}, nil
	}

	s := &Service{
		config:     config,
		db:         db,
		cache:      cache,
		logger:     logger,
		bus:        bus,
		retryQueue: make(chan *DeliveryTask, config.RetryQueueSize),
		stopChan:   make(chan struct{}),
		metrics:    NewMetrics(),
	}

	// Initialize notification channel adapters
	if config.DiscordEnabled {
		s.discord = NewDiscordAdapter(config.DiscordWebhookURL, logger)
		logger.Info("discord notifications enabled", zap.String("webhook_url", maskURL(config.DiscordWebhookURL)))
	}

	if config.SlackEnabled {
		s.slack = NewSlackAdapter(config.SlackWebhookURL, config.SlackChannel, logger)
		logger.Info("slack notifications enabled", zap.String("webhook_url", maskURL(config.SlackWebhookURL)))
	}

	if config.EmailEnabled {
		var err error
		s.email, err = NewEmailAdapter(config.EmailFrom, config.EmailTo, config.ResendAPIKey, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize email adapter: %w", err)
		}
		logger.Info("email notifications enabled",
			zap.String("provider", "resend"),
			zap.String("from", config.EmailFrom),
			zap.Strings("to", config.EmailTo),
		)
	}

	if config.WebhookEnabled {
		s.webhook = NewWebhookAdapter(
			config.WebhookURL,
			config.WebhookSecret,
			config.WebhookMethod,
			config.WebhookHeaders,
			logger,
		)
		logger.Info("generic webhook notifications enabled", zap.String("url", maskURL(config.WebhookURL)))
	}

	logger.Info("notification service initialized",
		zap.Bool("discord", config.DiscordEnabled),
		zap.Bool("slack", config.SlackEnabled),
		zap.Bool("email", config.EmailEnabled),
		zap.Bool("webhook", config.WebhookEnabled),
		zap.Int("max_retries", config.MaxRetries),
		zap.Int("retry_workers", config.RetryWorkers),
	)

	return s, nil
}

// Start starts the notification service
func (s *Service) Start(ctx context.Context) error {
	if !s.config.Enabled {
		s.logger.Info("notification service is disabled, skipping start")
		return nil
	}

	s.logger.Info("starting notification service")

	// Subscribe to events
	s.subscribeToEvents()

	// Start retry workers
	for i := 0; i < s.config.RetryWorkers; i++ {
		s.wg.Add(1)
		go s.retryWorker(ctx, i)
	}

	s.logger.Info("notification service started",
		zap.Int("retry_workers", s.config.RetryWorkers),
	)

	return nil
}

// Stop stops the notification service gracefully
func (s *Service) Stop(ctx context.Context) error {
	if !s.config.Enabled {
		return nil
	}

	s.logger.Info("stopping notification service")

	// Signal workers to stop
	close(s.stopChan)

	// Wait for workers to finish
	s.wg.Wait()

	s.logger.Info("notification service stopped")
	return nil
}

// subscribeToEvents subscribes to all relevant events from the event bus
func (s *Service) subscribeToEvents() {
	// Subscribe to tenant events
	s.bus.Subscribe(events.EventTenantCreated, s.handleEvent)

	// Subscribe to payment events
	s.bus.Subscribe(events.EventPaymentSucceeded, s.handleEvent)
	s.bus.Subscribe(events.EventPaymentFailed, s.handleEvent)

	// Subscribe to node events
	s.bus.Subscribe(events.EventNodeLaunched, s.handleEvent)
	s.bus.Subscribe(events.EventNodeTerminated, s.handleEvent)
	s.bus.Subscribe(events.EventNodeHealthDegraded, s.handleEvent)

	// Subscribe to cost events
	s.bus.Subscribe(events.EventCostAnomalyDetected, s.handleEvent)

	// Subscribe to rate limit events
	s.bus.Subscribe(events.EventRateLimitThreshold, s.handleEvent)

	s.logger.Info("subscribed to event types",
		zap.Strings("events", []string{
			string(events.EventTenantCreated),
			string(events.EventPaymentSucceeded),
			string(events.EventPaymentFailed),
			string(events.EventNodeLaunched),
			string(events.EventNodeTerminated),
			string(events.EventNodeHealthDegraded),
			string(events.EventCostAnomalyDetected),
			string(events.EventRateLimitThreshold),
		}),
	)
}

// handleEvent is the main event handler that routes events to notification channels
func (s *Service) handleEvent(ctx context.Context, event events.Event) error {
	s.logger.Debug("handling event",
		zap.String("event_id", event.ID),
		zap.String("event_type", string(event.Type)),
		zap.String("tenant_id", event.TenantID),
	)

	// Check if this event was already processed (idempotency)
	if s.isDuplicate(ctx, event.ID) {
		s.logger.Debug("duplicate event, skipping",
			zap.String("event_id", event.ID),
		)
		return nil
	}

	// Get channels for this event type
	channels := s.config.GetChannelsForEvent(string(event.Type))
	if len(channels) == 0 {
		s.logger.Debug("no channels configured for event type",
			zap.String("event_type", string(event.Type)),
		)
		return nil
	}

	// Deliver to each channel
	for _, channel := range channels {
		task := &DeliveryTask{
			ID:          fmt.Sprintf("%s-%s", event.ID, channel),
			EventID:     event.ID,
			EventType:   string(event.Type),
			TenantID:    event.TenantID,
			Channel:     channel,
			Payload:     event,
			RetryCount:  0,
			MaxRetries:  s.config.MaxRetries,
			CreatedAt:   time.Now(),
			LastAttempt: time.Now(),
		}

		if err := s.deliver(ctx, task); err != nil {
			s.logger.Error("delivery failed, enqueuing for retry",
				zap.String("event_id", event.ID),
				zap.String("channel", channel),
				zap.Error(err),
			)
			s.enqueueRetry(task)
		}
	}

	// Mark event as processed
	s.markProcessed(ctx, event.ID)

	return nil
}

// deliver delivers a notification to the specified channel
func (s *Service) deliver(ctx context.Context, task *DeliveryTask) error {
	startTime := time.Now()

	// Add timeout to context
	ctx, cancel := context.WithTimeout(ctx, s.config.DeliveryTimeout)
	defer cancel()

	var err error
	event := task.Payload.(events.Event)

	switch task.Channel {
	case "discord":
		if s.discord != nil {
			err = s.discord.Send(ctx, event)
		} else {
			err = fmt.Errorf("discord adapter not initialized")
		}

	case "slack":
		if s.slack != nil {
			err = s.slack.Send(ctx, event)
		} else {
			err = fmt.Errorf("slack adapter not initialized")
		}

	case "email":
		if s.email != nil {
			err = s.email.Send(ctx, event)
		} else {
			err = fmt.Errorf("email adapter not initialized")
		}

	case "webhook":
		if s.webhook != nil {
			err = s.webhook.Send(ctx, event)
		} else {
			err = fmt.Errorf("webhook adapter not initialized")
		}

	default:
		err = fmt.Errorf("unknown channel: %s", task.Channel)
	}

	duration := time.Since(startTime)

	// Record metrics
	if err != nil {
		s.metrics.RecordDelivery(task.Channel, string(event.Type), "failed", duration)
		s.logger.Error("notification delivery failed",
			zap.String("event_id", event.ID),
			zap.String("channel", task.Channel),
			zap.Duration("duration", duration),
			zap.Error(err),
		)
		return err
	}

	s.metrics.RecordDelivery(task.Channel, string(event.Type), "success", duration)
	s.logger.Info("notification delivered",
		zap.String("event_id", event.ID),
		zap.String("event_type", string(event.Type)),
		zap.String("channel", task.Channel),
		zap.Duration("duration", duration),
	)

	// Persist delivery record
	if err := s.persistDelivery(ctx, task, "sent", ""); err != nil {
		s.logger.Error("failed to persist delivery record",
			zap.String("event_id", event.ID),
			zap.Error(err),
		)
	}

	return nil
}

// enqueueRetry adds a failed delivery to the retry queue
func (s *Service) enqueueRetry(task *DeliveryTask) {
	task.RetryCount++
	task.LastAttempt = time.Now()

	select {
	case s.retryQueue <- task:
		s.metrics.RecordRetry(task.Channel, task.RetryCount)
		s.logger.Debug("task enqueued for retry",
			zap.String("task_id", task.ID),
			zap.String("channel", task.Channel),
			zap.Int("retry_count", task.RetryCount),
		)
	default:
		s.logger.Error("retry queue full, dropping task",
			zap.String("task_id", task.ID),
			zap.String("channel", task.Channel),
		)
	}
}

// retryWorker processes the retry queue
func (s *Service) retryWorker(ctx context.Context, workerID int) {
	defer s.wg.Done()

	s.logger.Info("retry worker started", zap.Int("worker_id", workerID))

	for {
		select {
		case <-s.stopChan:
			s.logger.Info("retry worker stopping", zap.Int("worker_id", workerID))
			return

		case task := <-s.retryQueue:
			// Check if we've exceeded max retries
			if task.RetryCount > task.MaxRetries {
				s.logger.Error("max retries exceeded, giving up",
					zap.String("task_id", task.ID),
					zap.String("channel", task.Channel),
					zap.Int("retry_count", task.RetryCount),
				)
				_ = s.persistDelivery(ctx, task, "failed", "max retries exceeded")
				continue
			}

			// Calculate backoff delay
			backoff := s.calculateBackoff(task.RetryCount)
			s.logger.Debug("retrying after backoff",
				zap.String("task_id", task.ID),
				zap.Duration("backoff", backoff),
			)

			time.Sleep(backoff)

			// Retry delivery
			if err := s.deliver(ctx, task); err != nil {
				s.logger.Warn("retry failed, re-enqueuing",
					zap.String("task_id", task.ID),
					zap.String("channel", task.Channel),
					zap.Int("retry_count", task.RetryCount),
					zap.Error(err),
				)
				s.enqueueRetry(task)
			}
		}
	}
}

// calculateBackoff calculates exponential backoff duration
func (s *Service) calculateBackoff(retryCount int) time.Duration {
	// Exponential backoff: base * 2^retryCount
	// Max backoff: 5 minutes
	backoff := s.config.RetryBackoffBase * time.Duration(1<<uint(retryCount))
	maxBackoff := 5 * time.Minute
	if backoff > maxBackoff {
		backoff = maxBackoff
	}
	return backoff
}

// isDuplicate checks if an event was already processed
func (s *Service) isDuplicate(ctx context.Context, eventID string) bool {
	key := fmt.Sprintf("notification:processed:%s", eventID)
	exists, err := s.cache.Exists(ctx, key)
	if err != nil {
		s.logger.Error("failed to check duplicate", zap.Error(err))
		return false
	}
	return exists
}

// markProcessed marks an event as processed
func (s *Service) markProcessed(ctx context.Context, eventID string) {
	key := fmt.Sprintf("notification:processed:%s", eventID)
	// Store for 24 hours
	if err := s.cache.Set(ctx, key, "1", 24*time.Hour); err != nil {
		s.logger.Error("failed to mark event as processed", zap.Error(err))
	}
}

// persistDelivery stores the delivery record in the database
func (s *Service) persistDelivery(ctx context.Context, task *DeliveryTask, status, errorMsg string) error {
	query := `
		INSERT INTO notification_deliveries (
			event_id, event_type, tenant_id, channel, status, retry_count, error_message, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := s.db.Pool.Exec(ctx, query,
		task.EventID,
		task.EventType,
		task.TenantID,
		task.Channel,
		status,
		task.RetryCount,
		errorMsg,
		task.CreatedAt,
	)

	return err
}

// maskURL masks sensitive parts of a URL for logging
func maskURL(url string) string {
	if len(url) < 20 {
		return "***"
	}
	return url[:20] + "***"
}
