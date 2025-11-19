package notifications

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds all notification-related Prometheus metrics
type Metrics struct {
	deliveredTotal   *prometheus.CounterVec
	deliveryDuration *prometheus.HistogramVec
	retriesTotal     *prometheus.CounterVec
	queueDepth       prometheus.Gauge
	mu               sync.Mutex
}

var (
	metricsOnce     sync.Once
	metricsInstance *Metrics
)

// NewMetrics creates a new Metrics instance (singleton)
func NewMetrics() *Metrics {
	metricsOnce.Do(func() {
		metricsInstance = &Metrics{
			deliveredTotal: promauto.NewCounterVec(
				prometheus.CounterOpts{
					Name: "notifications_delivered_total",
					Help: "Total number of notifications delivered",
				},
				[]string{"channel", "event_type", "status"},
			),

			deliveryDuration: promauto.NewHistogramVec(
				prometheus.HistogramOpts{
					Name:    "notification_delivery_duration_seconds",
					Help:    "Notification delivery duration in seconds",
					Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30},
				},
				[]string{"channel"},
			),

			retriesTotal: promauto.NewCounterVec(
				prometheus.CounterOpts{
					Name: "notification_retries_total",
					Help: "Total number of notification retry attempts",
				},
				[]string{"channel", "retry_count"},
			),

			queueDepth: promauto.NewGauge(
				prometheus.GaugeOpts{
					Name: "notification_retry_queue_depth",
					Help: "Current depth of the notification retry queue",
				},
			),
		}
	})

	return metricsInstance
}

// RecordDelivery records a notification delivery attempt
func (m *Metrics) RecordDelivery(channel, eventType, status string, duration time.Duration) {
	m.deliveredTotal.WithLabelValues(channel, eventType, status).Inc()
	m.deliveryDuration.WithLabelValues(channel).Observe(duration.Seconds())
}

// RecordRetry records a retry attempt
func (m *Metrics) RecordRetry(channel string, retryCount int) {
	m.retriesTotal.WithLabelValues(channel, string(rune(retryCount))).Inc()
}

// SetQueueDepth sets the current retry queue depth
func (m *Metrics) SetQueueDepth(depth int) {
	m.queueDepth.Set(float64(depth))
}
