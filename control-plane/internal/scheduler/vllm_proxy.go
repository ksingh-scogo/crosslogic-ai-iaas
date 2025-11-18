package scheduler

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/crosslogic/control-plane/pkg/models"
	"go.uber.org/zap"
)

// VLLMProxy handles HTTP proxy operations to vLLM nodes
// It manages connection pooling, request forwarding, and SSE streaming
type VLLMProxy struct {
	// HTTP client with connection pooling optimized for high-throughput inference
	client *http.Client

	// Logger for debugging and monitoring proxy operations
	logger *zap.Logger

	// Metrics tracking for monitoring proxy performance
	metrics *ProxyMetrics

	// Circuit breaker pattern for node failures
	breakers  map[string]*CircuitBreaker
	breakerMu sync.RWMutex
}

// ProxyMetrics tracks operational metrics for the proxy
type ProxyMetrics struct {
	RequestsTotal   int64
	RequestsFailed  int64
	StreamingTotal  int64
	StreamingFailed int64
	TotalLatencyMs  int64
	mu              sync.RWMutex
}

// UsageMetrics captures token usage emitted by vLLM streams
type UsageMetrics struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// CircuitBreaker implements circuit breaker pattern for node failures
type CircuitBreaker struct {
	failures     int
	lastFailTime time.Time
	state        string // "closed", "open", "half-open"
	mu           sync.RWMutex
}

// NewVLLMProxy creates a new vLLM proxy instance with optimized settings
// The proxy is configured for high-throughput, low-latency inference operations
func NewVLLMProxy(logger *zap.Logger) *VLLMProxy {
	// Configure HTTP client with connection pooling optimized for LLM inference
	// These settings are based on production experience with vLLM deployments
	transport := &http.Transport{
		// Connection pool settings for efficient connection reuse
		MaxIdleConns:        100,              // Maximum idle connections across all hosts
		MaxIdleConnsPerHost: 10,               // Maximum idle connections per host
		IdleConnTimeout:     90 * time.Second, // How long idle connections are kept

		// Custom dialer with timeout settings
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second, // Connection timeout
			KeepAlive: 30 * time.Second, // Keep-alive period
		}).DialContext,

		// Timeout settings for establishing connections
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,

		// Connection settings for long-running inference requests
		ResponseHeaderTimeout: 30 * time.Second, // Time to wait for response headers
		DisableCompression:    false,            // Allow compression for efficiency
		DisableKeepAlives:     false,            // Keep connections alive for reuse
		MaxConnsPerHost:       0,                // No limit on connections per host

		// Buffer settings for streaming responses
		WriteBufferSize: 4096, // 4KB write buffer
		ReadBufferSize:  4096, // 4KB read buffer
	}

	return &VLLMProxy{
		client: &http.Client{
			Timeout:   120 * time.Second, // 2-minute timeout for long inference requests
			Transport: transport,
			// Custom redirect policy - follow up to 3 redirects
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 3 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
		logger:   logger,
		metrics:  &ProxyMetrics{},
		breakers: make(map[string]*CircuitBreaker),
	}
}

// ForwardRequest forwards a non-streaming request to a vLLM node
// It handles request proxying, header forwarding, and response copying
//
// Parameters:
//   - ctx: Context for cancellation and timeouts
//   - node: Target vLLM node to forward the request to
//   - originalReq: Original HTTP request from the client
//   - body: Request body bytes (pre-read for modification if needed)
//
// Returns:
//   - *http.Response: Response from the vLLM node (caller must close Body)
//   - error: Any error that occurred during forwarding
//
// Example usage:
//
//	resp, err := proxy.ForwardRequest(ctx, node, req, bodyBytes)
//	if err != nil {
//	    return fmt.Errorf("proxy failed: %w", err)
//	}
//	defer resp.Body.Close()
func (p *VLLMProxy) ForwardRequest(ctx context.Context, node *models.Node, originalReq *http.Request, body []byte) (*http.Response, error) {
	startTime := time.Now()

	// Check circuit breaker for this node
	if !p.isNodeHealthy(node.EndpointURL) {
		p.recordFailure(node.EndpointURL)
		return nil, fmt.Errorf("node %s circuit breaker open", node.ID)
	}

	// Build the target URL by combining node endpoint with request path
	targetURL := fmt.Sprintf("%s%s", strings.TrimSuffix(node.EndpointURL, "/"), originalReq.URL.Path)

	// Preserve query parameters if present
	if originalReq.URL.RawQuery != "" {
		targetURL += "?" + originalReq.URL.RawQuery
	}

	p.logger.Debug("forwarding request",
		zap.String("method", originalReq.Method),
		zap.String("target_url", targetURL),
		zap.Int("body_size", len(body)),
		zap.String("node_id", node.ID.String()),
	)

	// Create new request with context for proper cancellation
	proxyReq, err := http.NewRequestWithContext(ctx, originalReq.Method, targetURL, bytes.NewReader(body))
	if err != nil {
		p.recordFailure(node.EndpointURL)
		return nil, fmt.Errorf("failed to create proxy request: %w", err)
	}

	// Copy headers from original request, filtering out hop-by-hop headers
	p.copyHeaders(originalReq.Header, proxyReq.Header)

	// Add proxy-specific headers for debugging and tracing
	proxyReq.Header.Set("X-Forwarded-For", originalReq.RemoteAddr)
	proxyReq.Header.Set("X-Forwarded-Host", originalReq.Host)
	proxyReq.Header.Set("X-Forwarded-Proto", "https")
	proxyReq.Header.Set("X-Proxy-Request-ID", originalReq.Header.Get("X-Request-ID"))

	// Execute the request with retries for transient failures
	resp, err := p.executeWithRetry(ctx, proxyReq, node.EndpointURL)
	if err != nil {
		p.recordFailure(node.EndpointURL)
		p.logger.Error("request forwarding failed",
			zap.String("node_id", node.ID.String()),
			zap.String("target_url", targetURL),
			zap.Error(err),
			zap.Duration("latency", time.Since(startTime)),
		)
		return nil, fmt.Errorf("failed to forward request to node %s: %w", node.ID, err)
	}

	// Record success metrics
	p.recordSuccess(node.EndpointURL)
	p.recordLatency(time.Since(startTime))

	p.logger.Info("request forwarded successfully",
		zap.String("node_id", node.ID.String()),
		zap.Int("status_code", resp.StatusCode),
		zap.Duration("latency", time.Since(startTime)),
	)

	return resp, nil
}

// HandleStreaming handles Server-Sent Events (SSE) streaming from vLLM nodes
// It sets up proper SSE headers and streams chunks from the backend to the client
//
// Parameters:
//   - ctx: Context for cancellation
//   - node: Target vLLM node for streaming
//   - originalReq: Original HTTP request from client
//   - w: HTTP response writer to stream to
//   - body: Request body bytes
//
// Returns:
//   - *UsageMetrics: token usage emitted at stream completion (if provided)
//   - error: Any error that occurred during streaming
//
// Example usage:
//
//	err := proxy.HandleStreaming(ctx, node, req, w, bodyBytes)
//	if err != nil {
//	    http.Error(w, "Streaming failed", 500)
//	}
func (p *VLLMProxy) HandleStreaming(ctx context.Context, node *models.Node, originalReq *http.Request, w http.ResponseWriter, body []byte) (*UsageMetrics, error) {
	startTime := time.Now()

	p.logger.Debug("starting streaming proxy",
		zap.String("node_id", node.ID.String()),
		zap.String("endpoint", node.EndpointURL),
	)

	// Forward the request to the vLLM node
	resp, err := p.ForwardRequest(ctx, node, originalReq, body)
	if err != nil {
		p.metrics.recordStreamingFailure()
		return nil, fmt.Errorf("failed to initiate streaming: %w", err)
	}
	defer resp.Body.Close()

	// Verify the response is suitable for streaming
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/event-stream") && !strings.Contains(contentType, "application/json") {
		p.logger.Warn("unexpected content type for streaming",
			zap.String("content_type", contentType),
			zap.String("node_id", node.ID.String()),
		)
	}

	// Set up SSE headers for the client
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering

	// Copy any custom headers from the vLLM response
	for key, values := range resp.Header {
		if p.shouldForwardHeader(key) {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
	}

	// Write headers to initiate the response
	w.WriteHeader(resp.StatusCode)

	// Get flusher for real-time streaming
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("response writer does not support flushing")
	}

	// Stream the response with proper chunking
	usage, err := p.streamResponse(ctx, resp.Body, w, flusher)
	if err != nil {
		p.metrics.recordStreamingFailure()
		p.logger.Error("streaming failed",
			zap.String("node_id", node.ID.String()),
			zap.Error(err),
			zap.Duration("duration", time.Since(startTime)),
		)
		return nil, err
	}

	p.metrics.recordStreamingSuccess()
	p.logger.Info("streaming completed",
		zap.String("node_id", node.ID.String()),
		zap.Duration("duration", time.Since(startTime)),
	)

	return usage, nil
}

// streamResponse handles the actual streaming of data from source to destination
// It uses a 4KB buffer for efficient chunked transfer
func (p *VLLMProxy) streamResponse(ctx context.Context, source io.Reader, dest io.Writer, flusher http.Flusher) (*UsageMetrics, error) {
	// Use a 4KB buffer for chunked streaming (optimal for SSE)
	buffer := make([]byte, 4096)

	// Create a reader for better error handling
	reader := bufio.NewReaderSize(source, 4096)
	parser := newSSEUsageParser()
	var lastUsage *UsageMetrics

	for {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Read chunk from source
		n, err := reader.Read(buffer)
		if n > 0 {
			// Write chunk to destination
			written, writeErr := dest.Write(buffer[:n])
			if writeErr != nil {
				return nil, fmt.Errorf("failed to write chunk: %w", writeErr)
			}
			if written != n {
				return nil, fmt.Errorf("partial write: wrote %d of %d bytes", written, n)
			}

			// Flush immediately for real-time streaming
			flusher.Flush()

			if usages := parser.Append(buffer[:n]); len(usages) > 0 {
				lastUsage = usages[len(usages)-1]
			}
		}

		// Handle read errors
		if err == io.EOF {
			// Normal completion
			return lastUsage, nil
		}
		if err != nil {
			// Check if it's a timeout or context cancellation
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, fmt.Errorf("failed to read chunk: %w", err)
		}
	}
}

// sseUsageParser extracts usage information emitted in SSE data payloads
type sseUsageParser struct {
	buffer []byte
}

func newSSEUsageParser() *sseUsageParser {
	return &sseUsageParser{
		buffer: make([]byte, 0, 4096),
	}
}

func (p *sseUsageParser) Append(chunk []byte) []*UsageMetrics {
	p.buffer = append(p.buffer, chunk...)
	var metrics []*UsageMetrics

	for {
		idx, delimiterLen := findSSEDelimiter(p.buffer)
		if idx == -1 {
			break
		}

		event := make([]byte, idx)
		copy(event, p.buffer[:idx])
		p.buffer = p.buffer[idx+delimiterLen:]

		if usage := extractUsageMetrics(event); usage != nil {
			metrics = append(metrics, usage)
		}
	}

	return metrics
}

func findSSEDelimiter(buffer []byte) (int, int) {
	if idx := bytes.Index(buffer, []byte("\r\n\r\n")); idx != -1 {
		return idx, 4
	}
	if idx := bytes.Index(buffer, []byte("\n\n")); idx != -1 {
		return idx, 2
	}
	return -1, 0
}

func extractUsageMetrics(event []byte) *UsageMetrics {
	lines := bytes.Split(event, []byte("\n"))
	var dataParts []string

	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 || line[0] == ':' {
			continue
		}
		if bytes.HasPrefix(line, []byte("data:")) {
			dataParts = append(dataParts, strings.TrimSpace(string(line[5:])))
		}
	}

	if len(dataParts) == 0 {
		return nil
	}

	payload := strings.TrimSpace(strings.Join(dataParts, "\n"))
	if payload == "" || payload == "[DONE]" {
		return nil
	}

	usage, err := parseUsagePayload(payload)
	if err != nil {
		return nil
	}
	return usage
}

func parseUsagePayload(payload string) (*UsageMetrics, error) {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal([]byte(payload), &envelope); err != nil {
		return nil, err
	}

	rawUsage, ok := envelope["usage"]
	if !ok || len(rawUsage) == 0 {
		return nil, nil
	}

	var parsed struct {
		PromptTokens     *int `json:"prompt_tokens"`
		CompletionTokens *int `json:"completion_tokens"`
		TotalTokens      *int `json:"total_tokens"`
	}

	if err := json.Unmarshal(rawUsage, &parsed); err != nil {
		return nil, err
	}

	return &UsageMetrics{
		PromptTokens:     derefInt(parsed.PromptTokens),
		CompletionTokens: derefInt(parsed.CompletionTokens),
		TotalTokens:      derefInt(parsed.TotalTokens),
	}, nil
}

func derefInt(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

// executeWithRetry performs the HTTP request with exponential backoff retry logic
// It retries on transient failures (network errors, 502, 503, 504)
func (p *VLLMProxy) executeWithRetry(ctx context.Context, req *http.Request, nodeEndpoint string) (*http.Response, error) {
	maxRetries := 3
	baseDelay := 100 * time.Millisecond

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		// Check context before each attempt
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		// Execute the request
		resp, err := p.client.Do(req)

		// Success case
		if err == nil && resp.StatusCode < 500 {
			return resp, nil
		}

		// Determine if error is retryable
		if err != nil {
			lastErr = err
			if !p.isRetryableError(err) {
				return nil, err
			}
		} else {
			// Check if HTTP status is retryable
			if !p.isRetryableStatus(resp.StatusCode) {
				return resp, nil
			}
			resp.Body.Close()
			lastErr = fmt.Errorf("HTTP %d response", resp.StatusCode)
		}

		// Calculate backoff delay
		if attempt < maxRetries-1 {
			delay := time.Duration(attempt+1) * baseDelay
			p.logger.Debug("retrying request",
				zap.Int("attempt", attempt+1),
				zap.Duration("delay", delay),
				zap.Error(lastErr),
			)

			select {
			case <-time.After(delay):
				// Continue to next attempt
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}

	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

// copyHeaders copies headers from source to destination, filtering out hop-by-hop headers
func (p *VLLMProxy) copyHeaders(source, dest http.Header) {
	// List of hop-by-hop headers that should not be forwarded
	hopByHopHeaders := map[string]bool{
		"Connection":          true,
		"Keep-Alive":          true,
		"Proxy-Authenticate":  true,
		"Proxy-Authorization": true,
		"TE":                  true,
		"Trailers":            true,
		"Transfer-Encoding":   true,
		"Upgrade":             true,
	}

	for key, values := range source {
		// Skip hop-by-hop headers
		if hopByHopHeaders[key] {
			continue
		}

		// Skip host header (will be set by HTTP client)
		if key == "Host" {
			continue
		}

		// Copy all values for this header
		for _, value := range values {
			dest.Add(key, value)
		}
	}
}

// shouldForwardHeader determines if a header should be forwarded to the client
func (p *VLLMProxy) shouldForwardHeader(key string) bool {
	// Headers that should not be forwarded
	skipHeaders := map[string]bool{
		"Connection":        true,
		"Content-Length":    true, // Let Go handle this
		"Transfer-Encoding": true, // Let Go handle this
		"Content-Encoding":  true, // Let Go handle this
	}

	return !skipHeaders[key]
}

// isRetryableError checks if an error is retryable
func (p *VLLMProxy) isRetryableError(err error) bool {
	// Network errors are generally retryable
	if strings.Contains(err.Error(), "connection refused") ||
		strings.Contains(err.Error(), "connection reset") ||
		strings.Contains(err.Error(), "broken pipe") ||
		strings.Contains(err.Error(), "timeout") ||
		strings.Contains(err.Error(), "temporary failure") ||
		strings.Contains(err.Error(), "no such host") {
		return true
	}
	return false
}

// isRetryableStatus checks if an HTTP status code indicates a retryable error
func (p *VLLMProxy) isRetryableStatus(statusCode int) bool {
	switch statusCode {
	case http.StatusBadGateway, // 502
		http.StatusServiceUnavailable, // 503
		http.StatusGatewayTimeout,     // 504
		http.StatusTooManyRequests:    // 429
		return true
	default:
		return false
	}
}

// Circuit breaker methods for node health management

// isNodeHealthy checks if a node is healthy based on circuit breaker state
func (p *VLLMProxy) isNodeHealthy(endpoint string) bool {
	p.breakerMu.RLock()
	breaker, exists := p.breakers[endpoint]
	p.breakerMu.RUnlock()

	if !exists {
		// No breaker exists, node is considered healthy
		return true
	}

	breaker.mu.Lock()
	defer breaker.mu.Unlock()

	// Check circuit breaker state
	switch breaker.state {
	case "open":
		// Check if enough time has passed to try again
		if time.Since(breaker.lastFailTime) > 30*time.Second {
			breaker.state = "half-open"
			return true
		}
		return false
	case "half-open":
		// Allow one request through
		return true
	default:
		// Closed state, node is healthy
		return true
	}
}

// recordFailure records a failure for circuit breaker management
func (p *VLLMProxy) recordFailure(endpoint string) {
	p.breakerMu.Lock()
	breaker, exists := p.breakers[endpoint]
	if !exists {
		breaker = &CircuitBreaker{
			state: "closed",
		}
		p.breakers[endpoint] = breaker
	}
	p.breakerMu.Unlock()

	breaker.mu.Lock()
	defer breaker.mu.Unlock()

	breaker.failures++
	breaker.lastFailTime = time.Now()

	// Open circuit if too many failures
	if breaker.failures >= 5 {
		breaker.state = "open"
		p.logger.Warn("circuit breaker opened",
			zap.String("endpoint", endpoint),
			zap.Int("failures", breaker.failures),
		)
	}

	// Update metrics
	p.metrics.recordRequestFailure()
}

// recordSuccess records a successful request for circuit breaker management
func (p *VLLMProxy) recordSuccess(endpoint string) {
	p.breakerMu.RLock()
	breaker, exists := p.breakers[endpoint]
	p.breakerMu.RUnlock()

	if exists {
		breaker.mu.Lock()
		defer breaker.mu.Unlock()

		// Reset failures on success
		if breaker.state == "half-open" {
			breaker.state = "closed"
			breaker.failures = 0
			p.logger.Info("circuit breaker closed",
				zap.String("endpoint", endpoint),
			)
		}
	}

	// Update metrics
	p.metrics.recordRequestSuccess()
}

// recordLatency records request latency for metrics
func (p *VLLMProxy) recordLatency(duration time.Duration) {
	p.metrics.mu.Lock()
	defer p.metrics.mu.Unlock()
	p.metrics.TotalLatencyMs += duration.Milliseconds()
}

// Metrics methods

func (m *ProxyMetrics) recordRequestSuccess() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.RequestsTotal++
}

func (m *ProxyMetrics) recordRequestFailure() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.RequestsTotal++
	m.RequestsFailed++
}

func (m *ProxyMetrics) recordStreamingSuccess() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.StreamingTotal++
}

func (m *ProxyMetrics) recordStreamingFailure() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.StreamingTotal++
	m.StreamingFailed++
}

// GetMetrics returns current proxy metrics
func (p *VLLMProxy) GetMetrics() map[string]interface{} {
	p.metrics.mu.RLock()
	defer p.metrics.mu.RUnlock()

	avgLatency := int64(0)
	if p.metrics.RequestsTotal > 0 {
		avgLatency = p.metrics.TotalLatencyMs / p.metrics.RequestsTotal
	}

	return map[string]interface{}{
		"requests_total":   p.metrics.RequestsTotal,
		"requests_failed":  p.metrics.RequestsFailed,
		"streaming_total":  p.metrics.StreamingTotal,
		"streaming_failed": p.metrics.StreamingFailed,
		"avg_latency_ms":   avgLatency,
		"success_rate":     float64(p.metrics.RequestsTotal-p.metrics.RequestsFailed) / float64(p.metrics.RequestsTotal),
	}
}

// Close cleans up proxy resources
func (p *VLLMProxy) Close() {
	// Close idle connections
	p.client.CloseIdleConnections()

	p.logger.Info("vLLM proxy closed",
		zap.Any("metrics", p.GetMetrics()),
	)
}
