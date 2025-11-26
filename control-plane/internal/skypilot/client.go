package skypilot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// Client is a production-ready HTTP client for the SkyPilot API Server
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
	logger     *zap.Logger

	// Retry configuration
	maxRetries     int
	retryDelay     time.Duration
	retryMaxDelay  time.Duration
}

// Config holds SkyPilot API client configuration
type Config struct {
	BaseURL string        // e.g., "https://skypilot-api.example.com" or "http://localhost:8080"
	Token   string        // Service account token (Bearer token)
	Timeout time.Duration // HTTP request timeout (default: 5 minutes)

	// Advanced configuration
	MaxRetries        int           // Maximum number of retries for transient failures (default: 3)
	RetryDelay        time.Duration // Initial retry delay (default: 1s)
	RetryMaxDelay     time.Duration // Maximum retry delay with exponential backoff (default: 30s)
	MaxIdleConns      int           // Maximum idle connections (default: 100)
	IdleConnTimeout   time.Duration // Idle connection timeout (default: 90s)
}

// NewClient creates a new SkyPilot API client with production-ready defaults
func NewClient(cfg Config, logger *zap.Logger) *Client {
	// Set defaults
	if cfg.Timeout == 0 {
		cfg.Timeout = 5 * time.Minute
	}
	// Use default of 3 retries if not specified
	// A negative value means no retries (useful for testing)
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = 3
	} else if cfg.MaxRetries < 0 {
		cfg.MaxRetries = 0 // Disable retries
	}
	if cfg.RetryDelay == 0 {
		cfg.RetryDelay = 1 * time.Second
	}
	if cfg.RetryMaxDelay == 0 {
		cfg.RetryMaxDelay = 30 * time.Second
	}
	if cfg.MaxIdleConns == 0 {
		cfg.MaxIdleConns = 100
	}
	if cfg.IdleConnTimeout == 0 {
		cfg.IdleConnTimeout = 90 * time.Second
	}

	// Create HTTP client with connection pooling
	transport := &http.Transport{
		MaxIdleConns:        cfg.MaxIdleConns,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     cfg.IdleConnTimeout,
		DisableCompression:  false,
		DisableKeepAlives:   false,

		// Connection settings
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,

		// TLS and HTTP/2 settings
		ForceAttemptHTTP2:     true,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   cfg.Timeout,
	}

	return &Client{
		baseURL:       cfg.BaseURL,
		token:         cfg.Token,
		httpClient:    httpClient,
		logger:        logger,
		maxRetries:    cfg.MaxRetries,
		retryDelay:    cfg.RetryDelay,
		retryMaxDelay: cfg.RetryMaxDelay,
	}
}

// Launch starts a new cluster asynchronously
// Returns a request ID that can be used to poll for completion status
func (c *Client) Launch(ctx context.Context, req LaunchRequest) (*LaunchResponse, error) {
	c.logger.Info("launching cluster via SkyPilot API",
		zap.String("cluster_name", req.ClusterName),
		zap.Bool("detach", req.Detach),
		zap.Bool("retry_until_up", req.RetryUntilUp),
	)

	var result LaunchResponse
	err := c.doRequestWithRetry(ctx, "POST", "/api/v1/clusters/launch", req, &result)
	if err != nil {
		c.logger.Error("failed to launch cluster",
			zap.String("cluster_name", req.ClusterName),
			zap.Error(err),
		)
		return nil, err
	}

	c.logger.Info("cluster launch initiated",
		zap.String("cluster_name", req.ClusterName),
		zap.String("request_id", result.RequestID),
	)

	return &result, nil
}

// Terminate terminates a cluster asynchronously
// Returns a request ID that can be used to poll for completion status
func (c *Client) Terminate(ctx context.Context, clusterName string, purge bool) (*TerminateResponse, error) {
	c.logger.Info("terminating cluster via SkyPilot API",
		zap.String("cluster_name", clusterName),
		zap.Bool("purge", purge),
	)

	req := TerminateRequest{Purge: purge}
	var result TerminateResponse

	err := c.doRequestWithRetry(ctx, "DELETE", fmt.Sprintf("/api/v1/clusters/%s", clusterName), req, &result)
	if err != nil {
		c.logger.Error("failed to terminate cluster",
			zap.String("cluster_name", clusterName),
			zap.Error(err),
		)
		return nil, err
	}

	c.logger.Info("cluster termination initiated",
		zap.String("cluster_name", clusterName),
		zap.String("request_id", result.RequestID),
	)

	return &result, nil
}

// GetStatus retrieves the current status of a cluster
func (c *Client) GetStatus(ctx context.Context, clusterName string) (*ClusterStatus, error) {
	c.logger.Debug("getting cluster status",
		zap.String("cluster_name", clusterName),
	)

	var result ClusterStatus
	err := c.doRequestWithRetry(ctx, "GET", fmt.Sprintf("/api/v1/clusters/%s", clusterName), nil, &result)
	if err != nil {
		c.logger.Error("failed to get cluster status",
			zap.String("cluster_name", clusterName),
			zap.Error(err),
		)
		return nil, err
	}

	c.logger.Debug("retrieved cluster status",
		zap.String("cluster_name", clusterName),
		zap.String("status", result.Status),
	)

	return &result, nil
}

// ListClusters returns all clusters managed by the API server
func (c *Client) ListClusters(ctx context.Context) (*ClusterListResponse, error) {
	c.logger.Debug("listing all clusters")

	var result ClusterListResponse
	err := c.doRequestWithRetry(ctx, "GET", "/api/v1/clusters", nil, &result)
	if err != nil {
		c.logger.Error("failed to list clusters", zap.Error(err))
		return nil, err
	}

	c.logger.Debug("listed clusters",
		zap.Int("total", result.Total),
		zap.Int("returned", len(result.Clusters)),
	)

	return &result, nil
}

// GetRequestStatus polls the status of an async request
func (c *Client) GetRequestStatus(ctx context.Context, requestID string) (*RequestStatus, error) {
	c.logger.Debug("getting request status",
		zap.String("request_id", requestID),
	)

	var result RequestStatus
	err := c.doRequestWithRetry(ctx, "GET", fmt.Sprintf("/api/v1/requests/%s", requestID), nil, &result)
	if err != nil {
		c.logger.Error("failed to get request status",
			zap.String("request_id", requestID),
			zap.Error(err),
		)
		return nil, err
	}

	c.logger.Debug("retrieved request status",
		zap.String("request_id", requestID),
		zap.String("status", result.Status),
		zap.Int("progress", result.Progress),
	)

	return &result, nil
}

// WaitForRequest polls an async request until it completes or fails
// Uses exponential backoff for polling to reduce API load
func (c *Client) WaitForRequest(ctx context.Context, requestID string, pollInterval time.Duration) (*RequestStatus, error) {
	c.logger.Info("waiting for request completion",
		zap.String("request_id", requestID),
		zap.Duration("poll_interval", pollInterval),
	)

	if pollInterval == 0 {
		pollInterval = 5 * time.Second
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	// Track polling attempts for exponential backoff
	attempt := 0
	maxPollInterval := 30 * time.Second

	for {
		select {
		case <-ctx.Done():
			c.logger.Warn("request wait cancelled",
				zap.String("request_id", requestID),
				zap.Error(ctx.Err()),
			)
			return nil, ctx.Err()

		case <-ticker.C:
			attempt++
			status, err := c.GetRequestStatus(ctx, requestID)
			if err != nil {
				// Log error but continue polling for transient failures
				c.logger.Warn("failed to poll request status",
					zap.String("request_id", requestID),
					zap.Int("attempt", attempt),
					zap.Error(err),
				)
				continue
			}

			switch status.Status {
			case "completed":
				c.logger.Info("request completed successfully",
					zap.String("request_id", requestID),
					zap.Int("total_attempts", attempt),
				)
				return status, nil

			case "failed":
				c.logger.Error("request failed",
					zap.String("request_id", requestID),
					zap.String("error", status.Error),
					zap.String("error_detail", status.ErrorDetail),
				)
				return status, fmt.Errorf("request failed: %s", status.Error)

			case "cancelled":
				c.logger.Warn("request was cancelled",
					zap.String("request_id", requestID),
				)
				return status, fmt.Errorf("request cancelled")

			case "pending", "running":
				c.logger.Debug("request still in progress",
					zap.String("request_id", requestID),
					zap.String("status", status.Status),
					zap.String("phase", status.CurrentPhase),
					zap.Int("progress", status.Progress),
				)

				// Exponential backoff: increase poll interval up to max
				if attempt > 5 {
					newInterval := time.Duration(float64(pollInterval) * 1.5)
					if newInterval > maxPollInterval {
						newInterval = maxPollInterval
					}
					if newInterval != pollInterval {
						pollInterval = newInterval
						ticker.Reset(pollInterval)
						c.logger.Debug("increased poll interval",
							zap.Duration("new_interval", pollInterval),
						)
					}
				}

			default:
				c.logger.Warn("unknown request status",
					zap.String("request_id", requestID),
					zap.String("status", status.Status),
				)
			}
		}
	}
}

// Execute runs a command on a cluster
func (c *Client) Execute(ctx context.Context, req ExecuteRequest) (*ExecuteResponse, error) {
	c.logger.Info("executing command on cluster",
		zap.String("cluster_name", req.ClusterName),
		zap.String("command", req.Command),
	)

	var result ExecuteResponse
	err := c.doRequestWithRetry(ctx, "POST", "/api/v1/clusters/execute", req, &result)
	if err != nil {
		c.logger.Error("failed to execute command",
			zap.String("cluster_name", req.ClusterName),
			zap.Error(err),
		)
		return nil, err
	}

	c.logger.Info("command executed",
		zap.String("cluster_name", req.ClusterName),
		zap.Int("exit_code", result.ExitCode),
	)

	return &result, nil
}

// GetLogs retrieves logs from a cluster
func (c *Client) GetLogs(ctx context.Context, req LogsRequest) (*LogsResponse, error) {
	c.logger.Debug("getting cluster logs",
		zap.String("cluster_name", req.ClusterName),
		zap.Int("tail_lines", req.TailLines),
	)

	var result LogsResponse
	err := c.doRequestWithRetry(ctx, "POST", "/api/v1/clusters/logs", req, &result)
	if err != nil {
		c.logger.Error("failed to get logs",
			zap.String("cluster_name", req.ClusterName),
			zap.Error(err),
		)
		return nil, err
	}

	return &result, nil
}

// EstimateCost estimates the cost of running a task
func (c *Client) EstimateCost(ctx context.Context, req EstimateCostRequest) (*EstimateCostResponse, error) {
	c.logger.Debug("estimating cost",
		zap.Int("hours", req.Hours),
	)

	var result EstimateCostResponse
	err := c.doRequestWithRetry(ctx, "POST", "/api/v1/cost/estimate", req, &result)
	if err != nil {
		c.logger.Error("failed to estimate cost", zap.Error(err))
		return nil, err
	}

	c.logger.Debug("cost estimated",
		zap.Float64("estimated_cost", result.EstimatedCost),
		zap.String("currency", result.Currency),
	)

	return &result, nil
}

// Health checks the API server health
func (c *Client) Health(ctx context.Context) (*HealthResponse, error) {
	c.logger.Debug("checking API server health")

	var result HealthResponse
	err := c.doRequestWithRetry(ctx, "GET", "/api/health", nil, &result)
	if err != nil {
		c.logger.Error("health check failed", zap.Error(err))
		return nil, err
	}

	c.logger.Debug("health check successful",
		zap.String("status", result.Status),
		zap.String("version", result.Version),
	)

	return &result, nil
}

// doRequestWithRetry executes an HTTP request with exponential backoff retry logic
func (c *Client) doRequestWithRetry(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	var lastErr error

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			// Calculate exponential backoff delay
			delay := c.calculateBackoff(attempt)

			c.logger.Debug("retrying request",
				zap.String("method", method),
				zap.String("path", path),
				zap.Int("attempt", attempt),
				zap.Duration("delay", delay),
			)

			select {
			case <-time.After(delay):
				// Continue with retry
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		err := c.doRequest(ctx, method, path, body, result)
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if !c.isRetryable(err) {
			c.logger.Debug("error is not retryable, aborting",
				zap.String("method", method),
				zap.String("path", path),
				zap.Error(err),
			)
			return err
		}

		c.logger.Warn("request failed, will retry",
			zap.String("method", method),
			zap.String("path", path),
			zap.Int("attempt", attempt),
			zap.Int("max_retries", c.maxRetries),
			zap.Error(err),
		)
	}

	return fmt.Errorf("request failed after %d retries: %w", c.maxRetries, lastErr)
}

// doRequest executes a single HTTP request
func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	url := c.baseURL + path

	var bodyReader io.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)

		// Log request body for debugging (be careful with sensitive data)
		if c.logger.Core().Enabled(zap.DebugLevel) {
			c.logger.Debug("request body",
				zap.String("method", method),
				zap.String("path", path),
				zap.ByteString("body", bodyBytes),
			)
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	// Set headers
	c.setHeaders(req)

	// Execute request
	startTime := time.Now()
	resp, err := c.httpClient.Do(req)
	duration := time.Since(startTime)

	if err != nil {
		c.logger.Error("HTTP request failed",
			zap.String("method", method),
			zap.String("url", url),
			zap.Duration("duration", duration),
			zap.Error(err),
		)
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	// Log response for debugging
	c.logger.Debug("HTTP response received",
		zap.String("method", method),
		zap.String("url", url),
		zap.Int("status_code", resp.StatusCode),
		zap.Duration("duration", duration),
		zap.Int("body_size", len(respBody)),
	)

	// Check for HTTP errors
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Try to parse error response
		var apiErr ErrorResponse
		if err := json.Unmarshal(respBody, &apiErr); err == nil && apiErr.Error != "" {
			return &APIError{
				StatusCode: resp.StatusCode,
				Message:    apiErr.Error,
				ErrorCode:  apiErr.ErrorCode,
				Details:    apiErr.Details,
				RequestID:  apiErr.RequestID,
			}
		}

		// Fallback to generic error
		return &APIError{
			StatusCode: resp.StatusCode,
			Message:    string(respBody),
		}
	}

	// Parse successful response
	if result != nil {
		if err := json.Unmarshal(respBody, result); err != nil {
			c.logger.Error("failed to parse response",
				zap.String("method", method),
				zap.String("url", url),
				zap.ByteString("body", respBody),
				zap.Error(err),
			)
			return fmt.Errorf("decode response: %w", err)
		}
	}

	return nil
}

// setHeaders sets common HTTP headers for API requests
func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "crosslogic-control-plane/1.0")

	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
}

// calculateBackoff calculates exponential backoff delay
func (c *Client) calculateBackoff(attempt int) time.Duration {
	// Exponential backoff: delay * 2^(attempt-1)
	delay := time.Duration(float64(c.retryDelay) * math.Pow(2, float64(attempt-1)))

	// Cap at max delay
	if delay > c.retryMaxDelay {
		delay = c.retryMaxDelay
	}

	// Add jitter (±25%) to prevent thundering herd
	jitter := float64(delay) * 0.25
	jitterDelta := time.Duration(float64(jitter) * (2*getRandom() - 1))
	delay += jitterDelta

	return delay
}

// isRetryable determines if an error should trigger a retry
func (c *Client) isRetryable(err error) bool {
	// Check for context cancellation
	if err == context.Canceled || err == context.DeadlineExceeded {
		return false
	}

	// Check for API errors
	if apiErr, ok := err.(*APIError); ok {
		// Retry on server errors (5xx)
		if apiErr.StatusCode >= 500 {
			return true
		}

		// Retry on rate limiting (429)
		if apiErr.StatusCode == http.StatusTooManyRequests {
			return true
		}

		// Don't retry client errors (4xx)
		return false
	}

	// Network errors are retryable
	return true
}

// getRandom returns a pseudo-random number between 0 and 1
// This is a simple implementation for jitter; use crypto/rand for security-critical applications
func getRandom() float64 {
	// Use nanosecond timestamp for simple pseudo-random jitter
	return float64(time.Now().UnixNano()%1000) / 1000.0
}

// APIError represents an error returned by the SkyPilot API
type APIError struct {
	StatusCode int
	Message    string
	ErrorCode  string
	Details    string
	RequestID  string
}

// Error implements the error interface
func (e *APIError) Error() string {
	if e.ErrorCode != "" {
		return fmt.Sprintf("SkyPilot API error [%s]: %s (status: %d)", e.ErrorCode, e.Message, e.StatusCode)
	}
	return fmt.Sprintf("SkyPilot API error: %s (status: %d)", e.Message, e.StatusCode)
}

// IsNotFound returns true if the error is a 404 Not Found
func (e *APIError) IsNotFound() bool {
	return e.StatusCode == http.StatusNotFound
}

// IsUnauthorized returns true if the error is a 401 Unauthorized
func (e *APIError) IsUnauthorized() bool {
	return e.StatusCode == http.StatusUnauthorized
}

// IsRateLimited returns true if the error is a 429 Too Many Requests
func (e *APIError) IsRateLimited() bool {
	return e.StatusCode == http.StatusTooManyRequests
}

// Close closes the HTTP client and releases resources
func (c *Client) Close() {
	c.logger.Debug("closing SkyPilot client")
	c.httpClient.CloseIdleConnections()
}
