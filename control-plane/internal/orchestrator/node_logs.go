package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/crosslogic/control-plane/pkg/cache"
	"go.uber.org/zap"
)

// NodeLogPhase represents the current phase of node launch
type NodeLogPhase string

const (
	PhaseQueued        NodeLogPhase = "queued"
	PhaseProvisioning  NodeLogPhase = "provisioning"
	PhaseInstanceReady NodeLogPhase = "instance_ready"
	PhaseInstalling    NodeLogPhase = "installing"
	PhaseModelLoading  NodeLogPhase = "model_loading"
	PhaseHealthCheck   NodeLogPhase = "health_check"
	PhaseActive        NodeLogPhase = "active"
	PhaseFailed        NodeLogPhase = "failed"
)

// NodeLogLevel represents log severity
type NodeLogLevel string

const (
	LogLevelInfo  NodeLogLevel = "info"
	LogLevelWarn  NodeLogLevel = "warn"
	LogLevelError NodeLogLevel = "error"
	LogLevelDebug NodeLogLevel = "debug"
)

// NodeLogEntry represents a single log line for a node launch
type NodeLogEntry struct {
	Timestamp time.Time    `json:"timestamp"`
	Level     NodeLogLevel `json:"level"`
	Message   string       `json:"message"`
	Phase     NodeLogPhase `json:"phase"`
	Progress  int          `json:"progress,omitempty"` // 0-100
	Details   string       `json:"details,omitempty"`  // Additional context
}

// NodeStatusEvent represents a status update during node launch
type NodeStatusEvent struct {
	Phase    NodeLogPhase `json:"phase"`
	Progress int          `json:"progress"` // 0-100
	Message  string       `json:"message"`
}

// NodeErrorEvent represents an error during node launch
type NodeErrorEvent struct {
	Error   string `json:"error"`
	Details string `json:"details"`
	Phase   NodeLogPhase `json:"phase"`
}

// NodeDoneEvent represents successful node launch completion
type NodeDoneEvent struct {
	Status   string `json:"status"`
	Endpoint string `json:"endpoint"`
	Message  string `json:"message"`
}

// NodeLogStore manages node launch logs in Redis
type NodeLogStore struct {
	cache  *cache.Cache
	logger *zap.Logger
	ttl    time.Duration // Log retention time
}

// NewNodeLogStore creates a new log store
func NewNodeLogStore(cache *cache.Cache, logger *zap.Logger) *NodeLogStore {
	return &NodeLogStore{
		cache:  cache,
		logger: logger,
		ttl:    24 * time.Hour, // Retain logs for 24 hours
	}
}

// AppendLog appends a log entry for a node
func (s *NodeLogStore) AppendLog(ctx context.Context, nodeID string, entry NodeLogEntry) error {
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	// Serialize log entry
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal log entry: %w", err)
	}

	// Redis key for this node's logs
	key := s.logKey(nodeID)

	// Append to list (RPUSH adds to tail)
	if err := s.cache.Client.RPush(ctx, key, string(data)).Err(); err != nil {
		return fmt.Errorf("failed to append log: %w", err)
	}

	// Set expiration (refresh on each append)
	if err := s.cache.Expire(ctx, key, s.ttl); err != nil {
		s.logger.Warn("failed to set log expiration",
			zap.String("node_id", nodeID),
			zap.Error(err),
		)
	}

	s.logger.Debug("appended log entry",
		zap.String("node_id", nodeID),
		zap.String("level", string(entry.Level)),
		zap.String("phase", string(entry.Phase)),
		zap.String("message", entry.Message),
	)

	return nil
}

// GetLogs retrieves logs for a node with optional filtering
func (s *NodeLogStore) GetLogs(ctx context.Context, nodeID string, tail int, since *time.Time) ([]NodeLogEntry, error) {
	key := s.logKey(nodeID)

	// Get all logs from Redis list
	logs, err := s.cache.Client.LRange(ctx, key, 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve logs: %w", err)
	}

	var entries []NodeLogEntry
	for _, logStr := range logs {
		var entry NodeLogEntry
		if err := json.Unmarshal([]byte(logStr), &entry); err != nil {
			s.logger.Warn("failed to unmarshal log entry",
				zap.String("node_id", nodeID),
				zap.Error(err),
			)
			continue
		}

		// Filter by timestamp if provided
		if since != nil && entry.Timestamp.Before(*since) {
			continue
		}

		entries = append(entries, entry)
	}

	// Apply tail limit (return last N entries)
	if tail > 0 && len(entries) > tail {
		entries = entries[len(entries)-tail:]
	}

	return entries, nil
}

// StreamLogs streams logs for a node (blocking until context is canceled)
// Returns a channel of log entries
func (s *NodeLogStore) StreamLogs(ctx context.Context, nodeID string, tail int, since *time.Time) (<-chan NodeLogEntry, <-chan error) {
	logChan := make(chan NodeLogEntry, 10)
	errChan := make(chan error, 1)

	go func() {
		defer close(logChan)
		defer close(errChan)

		// First, send existing logs
		existingLogs, err := s.GetLogs(ctx, nodeID, tail, since)
		if err != nil {
			errChan <- err
			return
		}

		for _, entry := range existingLogs {
			select {
			case <-ctx.Done():
				return
			case logChan <- entry:
			}
		}

		// Then, poll for new logs every 500ms
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		var lastTimestamp time.Time
		if len(existingLogs) > 0 {
			lastTimestamp = existingLogs[len(existingLogs)-1].Timestamp
		}

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Get new logs since last timestamp
				newLogs, err := s.GetLogs(ctx, nodeID, 0, &lastTimestamp)
				if err != nil {
					s.logger.Error("failed to poll for new logs",
						zap.String("node_id", nodeID),
						zap.Error(err),
					)
					continue
				}

				for _, entry := range newLogs {
					// Skip entries we've already sent
					if !entry.Timestamp.After(lastTimestamp) {
						continue
					}

					select {
					case <-ctx.Done():
						return
					case logChan <- entry:
						lastTimestamp = entry.Timestamp
					}
				}
			}
		}
	}()

	return logChan, errChan
}

// ClearLogs removes all logs for a node
func (s *NodeLogStore) ClearLogs(ctx context.Context, nodeID string) error {
	key := s.logKey(nodeID)
	return s.cache.Delete(ctx, key)
}

// logKey generates the Redis key for a node's logs
func (s *NodeLogStore) logKey(nodeID string) string {
	return fmt.Sprintf("node_logs:%s", nodeID)
}

// Helper functions for common log operations

// LogInfo logs an info-level message
func (s *NodeLogStore) LogInfo(ctx context.Context, nodeID string, phase NodeLogPhase, message string, progress int) error {
	return s.AppendLog(ctx, nodeID, NodeLogEntry{
		Timestamp: time.Now(),
		Level:     LogLevelInfo,
		Phase:     phase,
		Message:   message,
		Progress:  progress,
	})
}

// LogError logs an error-level message
func (s *NodeLogStore) LogError(ctx context.Context, nodeID string, phase NodeLogPhase, message string, details string) error {
	return s.AppendLog(ctx, nodeID, NodeLogEntry{
		Timestamp: time.Now(),
		Level:     LogLevelError,
		Phase:     phase,
		Message:   message,
		Details:   details,
	})
}

// LogWarn logs a warning-level message
func (s *NodeLogStore) LogWarn(ctx context.Context, nodeID string, phase NodeLogPhase, message string) error {
	return s.AppendLog(ctx, nodeID, NodeLogEntry{
		Timestamp: time.Now(),
		Level:     LogLevelWarn,
		Phase:     phase,
		Message:   message,
	})
}

// LogDebug logs a debug-level message
func (s *NodeLogStore) LogDebug(ctx context.Context, nodeID string, phase NodeLogPhase, message string) error {
	return s.AppendLog(ctx, nodeID, NodeLogEntry{
		Timestamp: time.Now(),
		Level:     LogLevelDebug,
		Phase:     phase,
		Message:   message,
	})
}
