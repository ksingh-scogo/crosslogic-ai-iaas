package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/crosslogic/control-plane/internal/orchestrator"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

// handleStreamNodeLogs streams node launch logs in real-time via Server-Sent Events (SSE)
// Platform Admin Only - GET /admin/nodes/{id}/logs/stream
//
// Query Parameters:
//   - follow (bool): Keep connection open and stream new logs (default: true)
//   - tail (int): Number of recent lines to send initially (default: 100)
//   - since (timestamp): Only return logs after this timestamp (RFC3339 format)
//
// Response Format (SSE):
//   - event: log
//     data: {"timestamp": "...", "level": "info", "message": "...", "phase": "provisioning"}
//   - event: status
//     data: {"phase": "installing", "progress": 45, "message": "Installing vLLM..."}
//   - event: error
//     data: {"error": "Failed to provision", "details": "...", "phase": "provisioning"}
//   - event: done
//     data: {"status": "active", "endpoint": "http://10.0.0.1:8000", "message": "Node ready"}
func (g *Gateway) handleStreamNodeLogs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get node ID from URL
	nodeID := chi.URLParam(r, "id")
	if nodeID == "" {
		g.writeError(w, http.StatusBadRequest, "node ID is required")
		return
	}

	// Parse query parameters
	follow := true
	if followStr := r.URL.Query().Get("follow"); followStr != "" {
		if parsed, err := strconv.ParseBool(followStr); err == nil {
			follow = parsed
		}
	}

	tail := 100
	if tailStr := r.URL.Query().Get("tail"); tailStr != "" {
		if parsed, err := strconv.Atoi(tailStr); err == nil && parsed > 0 {
			tail = parsed
		}
	}

	var since *time.Time
	if sinceStr := r.URL.Query().Get("since"); sinceStr != "" {
		if parsed, err := time.Parse(time.RFC3339, sinceStr); err == nil {
			since = &parsed
		} else {
			g.writeError(w, http.StatusBadRequest, "invalid 'since' timestamp format (expected RFC3339)")
			return
		}
	}

	g.logger.Info("streaming node logs",
		zap.String("node_id", nodeID),
		zap.Bool("follow", follow),
		zap.Int("tail", tail),
	)

	// Verify node exists
	var clusterName, status string
	err := g.db.Pool.QueryRow(ctx, `
		SELECT cluster_name, status FROM nodes WHERE id = $1
	`, nodeID).Scan(&clusterName, &status)

	if err != nil {
		g.logger.Error("node not found",
			zap.String("node_id", nodeID),
			zap.Error(err),
		)
		g.writeError(w, http.StatusNotFound, "node not found")
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering

	// Create flusher for SSE
	flusher, ok := w.(http.Flusher)
	if !ok {
		g.writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	// Initialize log store
	logStore := orchestrator.NewNodeLogStore(g.cache, g.logger)

	// Create context with timeout for the stream
	streamCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	// Handle client disconnect
	go func() {
		<-streamCtx.Done()
		g.logger.Debug("client disconnected from log stream",
			zap.String("node_id", nodeID),
		)
	}()

	// Start streaming logs
	if follow {
		// Stream mode: Send existing logs + follow new ones
		logChan, errChan := logStore.StreamLogs(streamCtx, nodeID, tail, since)

		// Track if we've seen a terminal state
		terminated := false

		for {
			select {
			case <-streamCtx.Done():
				return

			case err := <-errChan:
				if err != nil {
					g.logger.Error("error streaming logs",
						zap.String("node_id", nodeID),
						zap.Error(err),
					)
					g.writeSSEEvent(w, "error", orchestrator.NodeErrorEvent{
						Error:   "Failed to stream logs",
						Details: err.Error(),
					})
					flusher.Flush()
				}
				return

			case entry, ok := <-logChan:
				if !ok {
					return
				}

				// Send log entry as SSE event
				g.writeSSEEvent(w, "log", entry)
				flusher.Flush()

				// Check for status updates
				if entry.Progress > 0 {
					g.writeSSEEvent(w, "status", orchestrator.NodeStatusEvent{
						Phase:    entry.Phase,
						Progress: entry.Progress,
						Message:  entry.Message,
					})
					flusher.Flush()
				}

				// Check for errors
				if entry.Level == orchestrator.LogLevelError {
					g.writeSSEEvent(w, "error", orchestrator.NodeErrorEvent{
						Error:   entry.Message,
						Details: entry.Details,
						Phase:   entry.Phase,
					})
					flusher.Flush()
				}

				// Check for terminal states
				if entry.Phase == orchestrator.PhaseActive && !terminated {
					// Query for endpoint URL
					var endpoint string
					if err := g.db.Pool.QueryRow(streamCtx, `
						SELECT COALESCE(endpoint_url, endpoint, '') FROM nodes WHERE id = $1
					`, nodeID).Scan(&endpoint); err != nil {
						g.logger.Warn("failed to get endpoint URL", zap.Error(err))
						endpoint = ""
					}

					g.writeSSEEvent(w, "done", orchestrator.NodeDoneEvent{
						Status:   "active",
						Endpoint: endpoint,
						Message:  "Node is ready and serving requests",
					})
					flusher.Flush()
					terminated = true

					// Close stream after sending done event
					time.Sleep(500 * time.Millisecond)
					return
				}

				if entry.Phase == orchestrator.PhaseFailed && !terminated {
					g.writeSSEEvent(w, "done", orchestrator.NodeDoneEvent{
						Status:  "failed",
						Message: "Node launch failed",
					})
					flusher.Flush()
					terminated = true

					// Close stream after sending done event
					time.Sleep(500 * time.Millisecond)
					return
				}
			}
		}
	} else {
		// Non-follow mode: Send existing logs and close
		logs, err := logStore.GetLogs(streamCtx, nodeID, tail, since)
		if err != nil {
			g.logger.Error("failed to get logs",
				zap.String("node_id", nodeID),
				zap.Error(err),
			)
			g.writeSSEEvent(w, "error", orchestrator.NodeErrorEvent{
				Error:   "Failed to retrieve logs",
				Details: err.Error(),
			})
			flusher.Flush()
			return
		}

		// Send all logs
		for _, entry := range logs {
			g.writeSSEEvent(w, "log", entry)
			flusher.Flush()
		}

		// Send final done event
		g.writeSSEEvent(w, "done", orchestrator.NodeDoneEvent{
			Status:  status,
			Message: "Log stream complete (follow=false)",
		})
		flusher.Flush()
	}
}

// writeSSEEvent writes a Server-Sent Event to the response
func (g *Gateway) writeSSEEvent(w http.ResponseWriter, event string, data interface{}) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		g.logger.Error("failed to marshal SSE data", zap.Error(err))
		return
	}

	fmt.Fprintf(w, "event: %s\n", event)
	fmt.Fprintf(w, "data: %s\n\n", string(jsonData))
}

// handleGetNodeLogs retrieves historical node logs (non-streaming, JSON response)
// Platform Admin Only - GET /admin/nodes/{id}/logs
//
// Query Parameters:
//   - tail (int): Number of recent lines to return (default: 100)
//   - since (timestamp): Only return logs after this timestamp (RFC3339 format)
//
// Response: JSON array of log entries
func (g *Gateway) handleGetNodeLogs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get node ID from URL
	nodeID := chi.URLParam(r, "id")
	if nodeID == "" {
		g.writeError(w, http.StatusBadRequest, "node ID is required")
		return
	}

	// Parse query parameters
	tail := 100
	if tailStr := r.URL.Query().Get("tail"); tailStr != "" {
		if parsed, err := strconv.Atoi(tailStr); err == nil && parsed > 0 {
			tail = parsed
		}
	}

	var since *time.Time
	if sinceStr := r.URL.Query().Get("since"); sinceStr != "" {
		if parsed, err := time.Parse(time.RFC3339, sinceStr); err == nil {
			since = &parsed
		} else {
			g.writeError(w, http.StatusBadRequest, "invalid 'since' timestamp format (expected RFC3339)")
			return
		}
	}

	// Verify node exists
	var exists bool
	err := g.db.Pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM nodes WHERE id = $1)
	`, nodeID).Scan(&exists)

	if err != nil || !exists {
		g.logger.Error("node not found",
			zap.String("node_id", nodeID),
			zap.Error(err),
		)
		g.writeError(w, http.StatusNotFound, "node not found")
		return
	}

	// Get logs from store
	logStore := orchestrator.NewNodeLogStore(g.cache, g.logger)
	logs, err := logStore.GetLogs(ctx, nodeID, tail, since)
	if err != nil {
		g.logger.Error("failed to retrieve logs",
			zap.String("node_id", nodeID),
			zap.Error(err),
		)
		g.writeError(w, http.StatusInternalServerError, "failed to retrieve logs")
		return
	}

	g.writeJSON(w, http.StatusOK, map[string]interface{}{
		"node_id": nodeID,
		"count":   len(logs),
		"logs":    logs,
	})
}
