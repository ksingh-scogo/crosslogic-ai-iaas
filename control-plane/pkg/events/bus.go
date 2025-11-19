package events

import (
	"context"
	"fmt"
	"sync"

	"go.uber.org/zap"
)

// Handler is a function that handles an event
type Handler func(ctx context.Context, event Event) error

// Bus is an in-memory event bus for pub/sub messaging
type Bus struct {
	handlers map[EventType][]Handler
	mu       sync.RWMutex
	logger   *zap.Logger
}

// NewBus creates a new event bus
func NewBus(logger *zap.Logger) *Bus {
	return &Bus{
		handlers: make(map[EventType][]Handler),
		logger:   logger,
	}
}

// Subscribe registers a handler for a specific event type
// Multiple handlers can be registered for the same event type
func (b *Bus) Subscribe(eventType EventType, handler Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.handlers[eventType] = append(b.handlers[eventType], handler)
	b.logger.Info("event handler subscribed",
		zap.String("event_type", string(eventType)),
		zap.Int("total_handlers", len(b.handlers[eventType])),
	)
}

// Publish publishes an event to all registered handlers
// Handlers are called asynchronously in separate goroutines
// Errors from handlers are logged but don't block the publisher
func (b *Bus) Publish(ctx context.Context, event Event) error {
	b.mu.RLock()
	handlers := b.handlers[event.Type]
	b.mu.RUnlock()

	if len(handlers) == 0 {
		b.logger.Debug("no handlers registered for event type",
			zap.String("event_type", string(event.Type)),
			zap.String("event_id", event.ID),
		)
		return nil
	}

	b.logger.Debug("publishing event",
		zap.String("event_type", string(event.Type)),
		zap.String("event_id", event.ID),
		zap.Int("handler_count", len(handlers)),
	)

	// Call handlers asynchronously
	var wg sync.WaitGroup
	for _, handler := range handlers {
		wg.Add(1)
		go func(h Handler) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					b.logger.Error("event handler panicked",
						zap.String("event_type", string(event.Type)),
						zap.String("event_id", event.ID),
						zap.Any("panic", r),
					)
				}
			}()

			if err := h(ctx, event); err != nil {
				b.logger.Error("event handler failed",
					zap.String("event_type", string(event.Type)),
					zap.String("event_id", event.ID),
					zap.Error(err),
				)
			}
		}(handler)
	}

	// Don't block the publisher - handlers run async
	// We could optionally wait for handlers: wg.Wait()
	// But for now, fire and forget with error logging

	return nil
}

// PublishAndWait publishes an event and waits for all handlers to complete
// Returns the first error encountered from any handler
func (b *Bus) PublishAndWait(ctx context.Context, event Event) error {
	b.mu.RLock()
	handlers := b.handlers[event.Type]
	b.mu.RUnlock()

	if len(handlers) == 0 {
		return nil
	}

	var (
		wg     sync.WaitGroup
		errMu  sync.Mutex
		errOut error
	)

	for _, handler := range handlers {
		wg.Add(1)
		go func(h Handler) {
			defer wg.Done()
			if err := h(ctx, event); err != nil {
				errMu.Lock()
				if errOut == nil {
					errOut = err
				}
				errMu.Unlock()
			}
		}(handler)
	}

	wg.Wait()
	return errOut
}

// Unsubscribe removes all handlers for a specific event type (useful for testing)
func (b *Bus) Unsubscribe(eventType EventType) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.handlers, eventType)
}

// Stats returns statistics about the event bus
func (b *Bus) Stats() map[string]interface{} {
	b.mu.RLock()
	defer b.mu.RUnlock()

	stats := make(map[string]interface{})
	stats["total_event_types"] = len(b.handlers)

	handlerCounts := make(map[string]int)
	for eventType, handlers := range b.handlers {
		handlerCounts[string(eventType)] = len(handlers)
	}
	stats["handlers_per_type"] = handlerCounts

	return stats
}

// String returns a string representation of the bus state
func (b *Bus) String() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return fmt.Sprintf("EventBus{types=%d, handlers=%v}", len(b.handlers), b.handlers)
}
