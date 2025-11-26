package skypilot

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestNewClient verifies client initialization with defaults
func TestNewClient(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	tests := []struct {
		name   string
		config Config
	}{
		{
			name: "default configuration",
			config: Config{
				BaseURL: "https://api.example.com",
				Token:   "test-token",
			},
		},
		{
			name: "custom configuration",
			config: Config{
				BaseURL:       "https://api.example.com",
				Token:         "test-token",
				Timeout:       10 * time.Minute,
				MaxRetries:    5,
				RetryDelay:    2 * time.Second,
				RetryMaxDelay: 60 * time.Second,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(tt.config, logger)
			require.NotNil(t, client)
			assert.Equal(t, tt.config.BaseURL, client.baseURL)
			assert.Equal(t, tt.config.Token, client.token)
			assert.NotNil(t, client.httpClient)
			assert.NotNil(t, client.logger)
		})
	}
}

// TestLaunch verifies cluster launch functionality
func TestLaunch(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/api/v1/clusters/launch", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"request_id": "req-123", "message": "Launch initiated"}`))
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL: server.URL,
		Token:   "test-token",
		Timeout: 10 * time.Second,
	}, logger)

	ctx := context.Background()
	req := LaunchRequest{
		ClusterName:  "test-cluster",
		TaskYAML:     "resources:\n  cloud: aws\n  instance_type: p3.2xlarge\nrun: echo hello",
		RetryUntilUp: true,
		Detach:       true,
	}

	resp, err := client.Launch(ctx, req)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "req-123", resp.RequestID)
	assert.Equal(t, "Launch initiated", resp.Message)
}

// TestTerminate verifies cluster termination functionality
func TestTerminate(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		assert.Equal(t, "/api/v1/clusters/test-cluster", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"request_id": "req-456", "message": "Termination initiated"}`))
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL: server.URL,
		Token:   "test-token",
	}, logger)

	ctx := context.Background()
	resp, err := client.Terminate(ctx, "test-cluster", false)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "req-456", resp.RequestID)
}

// TestGetStatus verifies cluster status retrieval
func TestGetStatus(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/api/v1/clusters/test-cluster", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"name": "test-cluster",
			"status": "UP",
			"cloud": "aws",
			"region": "us-east-1",
			"resources": {
				"accelerators": "A100:4",
				"cpus": 32,
				"memory": "128GB"
			},
			"cost_per_hour": 12.50
		}`))
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL: server.URL,
		Token:   "test-token",
	}, logger)

	ctx := context.Background()
	status, err := client.GetStatus(ctx, "test-cluster")
	require.NoError(t, err)
	assert.NotNil(t, status)
	assert.Equal(t, "test-cluster", status.Name)
	assert.Equal(t, "UP", status.Status)
	assert.Equal(t, "aws", status.Provider)
	assert.Equal(t, "us-east-1", status.Region)
	assert.Equal(t, 32, status.Resources.CPUs)
	assert.Equal(t, 12.50, status.CostPerHour)
}

// TestListClusters verifies cluster listing
func TestListClusters(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/api/v1/clusters", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"clusters": [
				{
					"name": "cluster-1",
					"status": "UP",
					"cloud": "aws",
					"region": "us-east-1",
					"resources": {"cpus": 16, "memory": "64GB"}
				},
				{
					"name": "cluster-2",
					"status": "STOPPED",
					"cloud": "azure",
					"region": "westus2",
					"resources": {"cpus": 8, "memory": "32GB"}
				}
			],
			"total": 2
		}`))
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL: server.URL,
		Token:   "test-token",
	}, logger)

	ctx := context.Background()
	resp, err := client.ListClusters(ctx)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 2, resp.Total)
	assert.Len(t, resp.Clusters, 2)
	assert.Equal(t, "cluster-1", resp.Clusters[0].Name)
	assert.Equal(t, "UP", resp.Clusters[0].Status)
}

// TestGetRequestStatus verifies async request status polling
func TestGetRequestStatus(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/api/v1/requests/req-123", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"request_id": "req-123",
			"status": "running",
			"progress": 75,
			"current_phase": "launching",
			"created_at": "2024-01-01T00:00:00Z"
		}`))
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL: server.URL,
		Token:   "test-token",
	}, logger)

	ctx := context.Background()
	status, err := client.GetRequestStatus(ctx, "req-123")
	require.NoError(t, err)
	assert.NotNil(t, status)
	assert.Equal(t, "req-123", status.RequestID)
	assert.Equal(t, "running", status.Status)
	assert.Equal(t, 75, status.Progress)
	assert.Equal(t, "launching", status.CurrentPhase)
}

// TestHealth verifies health check functionality
func TestHealth(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/api/health", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"status": "healthy",
			"version": "0.10.0",
			"uptime": 3600,
			"timestamp": "2024-01-01T12:00:00Z"
		}`))
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL: server.URL,
		Token:   "test-token",
	}, logger)

	ctx := context.Background()
	health, err := client.Health(ctx)
	require.NoError(t, err)
	assert.NotNil(t, health)
	assert.Equal(t, "healthy", health.Status)
	assert.Equal(t, "0.10.0", health.Version)
	assert.Equal(t, 3600, health.Uptime)
}

// TestAPIError verifies error handling
func TestAPIError(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	tests := []struct {
		name           string
		statusCode     int
		responseBody   string
		expectedError  string
		checkFunc      func(*testing.T, error)
	}{
		{
			name:          "404 Not Found",
			statusCode:    http.StatusNotFound,
			responseBody:  `{"error": "Cluster not found", "error_code": "NOT_FOUND"}`,
			expectedError: "Cluster not found",
			checkFunc: func(t *testing.T, err error) {
				var apiErr *APIError
				require.True(t, errors.As(err, &apiErr), "expected error to be or wrap *APIError")
				assert.True(t, apiErr.IsNotFound())
			},
		},
		{
			name:          "401 Unauthorized",
			statusCode:    http.StatusUnauthorized,
			responseBody:  `{"error": "Invalid token", "error_code": "UNAUTHORIZED"}`,
			expectedError: "Invalid token",
			checkFunc: func(t *testing.T, err error) {
				var apiErr *APIError
				require.True(t, errors.As(err, &apiErr), "expected error to be or wrap *APIError")
				assert.True(t, apiErr.IsUnauthorized())
			},
		},
		{
			name:          "429 Rate Limited",
			statusCode:    http.StatusTooManyRequests,
			responseBody:  `{"error": "Rate limit exceeded", "error_code": "RATE_LIMITED"}`,
			expectedError: "Rate limit exceeded",
			checkFunc: func(t *testing.T, err error) {
				var apiErr *APIError
				require.True(t, errors.As(err, &apiErr), "expected error to be or wrap *APIError")
				assert.True(t, apiErr.IsRateLimited())
			},
		},
		{
			name:          "500 Internal Server Error",
			statusCode:    http.StatusInternalServerError,
			responseBody:  `{"error": "Internal server error", "error_code": "INTERNAL_ERROR"}`,
			expectedError: "Internal server error",
			checkFunc: func(t *testing.T, err error) {
				var apiErr *APIError
				require.True(t, errors.As(err, &apiErr), "expected error to be or wrap *APIError")
				assert.Equal(t, 500, apiErr.StatusCode)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			client := NewClient(Config{
				BaseURL:    server.URL,
				Token:      "test-token",
				MaxRetries: -1, // Disable retries for error tests
			}, logger)

			ctx := context.Background()
			_, err := client.GetStatus(ctx, "test-cluster")
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)

			if tt.checkFunc != nil {
				tt.checkFunc(t, err)
			}
		})
	}
}

// TestRetryLogic verifies exponential backoff retry mechanism
func TestRetryLogic(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			// Fail first 2 attempts with retryable error
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error": "Temporary error"}`))
			return
		}
		// Succeed on 3rd attempt
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"status": "healthy",
			"version": "0.10.0",
			"uptime": 3600,
			"timestamp": "2024-01-01T12:00:00Z"
		}`))
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL:       server.URL,
		Token:         "test-token",
		MaxRetries:    3,
		RetryDelay:    10 * time.Millisecond,
		RetryMaxDelay: 100 * time.Millisecond,
	}, logger)

	ctx := context.Background()
	health, err := client.Health(ctx)
	require.NoError(t, err)
	assert.NotNil(t, health)
	assert.Equal(t, 3, attempts) // Should succeed after 2 retries
}

// TestContextCancellation verifies context cancellation handling
func TestContextCancellation(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow response
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL: server.URL,
		Token:   "test-token",
	}, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := client.Health(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context deadline exceeded")
}

// TestExecute verifies command execution functionality
func TestExecute(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/api/v1/clusters/execute", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"request_id": "exec-123",
			"stdout": "Hello World\n",
			"stderr": "",
			"exit_code": 0,
			"executed_at": "2024-01-01T12:00:00Z"
		}`))
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL: server.URL,
		Token:   "test-token",
	}, logger)

	ctx := context.Background()
	req := ExecuteRequest{
		ClusterName: "test-cluster",
		Command:     "echo 'Hello World'",
	}

	resp, err := client.Execute(ctx, req)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "exec-123", resp.RequestID)
	assert.Equal(t, "Hello World\n", resp.Stdout)
	assert.Equal(t, 0, resp.ExitCode)
}

// TestEstimateCost verifies cost estimation functionality
func TestEstimateCost(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/api/v1/cost/estimate", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"estimated_cost": 125.50,
			"cost_per_hour": 12.55,
			"currency": "USD",
			"provider": "aws",
			"region": "us-east-1",
			"instance_type": "p3.8xlarge",
			"estimated_at": "2024-01-01T12:00:00Z",
			"compute_cost": 100.00,
			"storage_cost": 20.00,
			"network_cost": 5.50,
			"confidence_level": "high"
		}`))
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL: server.URL,
		Token:   "test-token",
	}, logger)

	ctx := context.Background()
	req := EstimateCostRequest{
		TaskYAML: "resources:\n  cloud: aws\n  instance_type: p3.8xlarge",
		Hours:    10,
	}

	resp, err := client.EstimateCost(ctx, req)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 125.50, resp.EstimatedCost)
	assert.Equal(t, 12.55, resp.CostPerHour)
	assert.Equal(t, "USD", resp.Currency)
	assert.Equal(t, "high", resp.ConfidenceLevel)
}
