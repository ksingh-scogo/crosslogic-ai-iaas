package skypilot

import (
	"time"
)

// LaunchRequest represents a cluster launch request to the SkyPilot API
type LaunchRequest struct {
	// Core cluster configuration
	ClusterName string `json:"cluster_name"`
	TaskYAML    string `json:"task_yaml"` // Complete SkyPilot task YAML

	// Launch options
	RetryUntilUp      bool `json:"retry_until_up"`
	IdleMinutesToStop int  `json:"idle_minutes_to_autostop,omitempty"`
	Detach            bool `json:"detach"` // Run cluster launch in background

	// Environment variables to inject into the cluster
	Envs map[string]string `json:"envs,omitempty"`

	// Cloud credentials (for multi-tenant support)
	// These are dynamically injected per request rather than stored server-side
	CloudCredentials *CloudCredentials `json:"cloud_credentials,omitempty"`
}

// CloudCredentials contains dynamic cloud provider credentials
// This enables multi-tenant support where each tenant has their own cloud accounts
type CloudCredentials struct {
	// AWS credentials
	AWS *AWSCredentials `json:"aws,omitempty"`

	// Azure credentials
	Azure *AzureCredentials `json:"azure,omitempty"`

	// GCP credentials
	GCP *GCPCredentials `json:"gcp,omitempty"`
}

// AWSCredentials contains AWS-specific credentials
type AWSCredentials struct {
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
	Region          string `json:"region,omitempty"`
	SessionToken    string `json:"session_token,omitempty"` // For temporary credentials
}

// AzureCredentials contains Azure-specific credentials
type AzureCredentials struct {
	SubscriptionID string `json:"subscription_id"`
	TenantID       string `json:"tenant_id"`
	ClientID       string `json:"client_id"`
	ClientSecret   string `json:"client_secret"`
}

// GCPCredentials contains GCP-specific credentials
type GCPCredentials struct {
	ProjectID           string `json:"project_id"`
	ServiceAccountJSON  string `json:"service_account_json"` // JSON key file contents
	ServiceAccountEmail string `json:"service_account_email,omitempty"`
}

// LaunchResponse contains the async request ID for tracking cluster launch
type LaunchResponse struct {
	RequestID string `json:"request_id"` // Async request ID to poll for completion
	Message   string `json:"message,omitempty"`
}

// TerminateRequest represents a cluster termination request
type TerminateRequest struct {
	// Whether to purge all data (default: false keeps data for potential restart)
	Purge bool `json:"purge"`
}

// TerminateResponse contains the async request ID for tracking cluster termination
type TerminateResponse struct {
	RequestID string `json:"request_id"` // Async request ID to poll for completion
	Message   string `json:"message,omitempty"`
}

// ClusterStatus represents the detailed status of a cluster
type ClusterStatus struct {
	// Identity
	Name      string `json:"name"`
	ClusterID string `json:"cluster_id,omitempty"`

	// State
	Status     string     `json:"status"` // UP, INIT, STOPPED, TERMINATED, UNKNOWN
	LaunchedAt *time.Time `json:"launched_at,omitempty"`
	StoppedAt  *time.Time `json:"stopped_at,omitempty"`

	// Cloud provider details
	Provider string `json:"cloud"`          // aws, azure, gcp, etc.
	Region   string `json:"region"`         // us-east-1, westus2, us-central1, etc.
	Zone     string `json:"zone,omitempty"` // Availability zone if applicable

	// Resource allocation
	Resources ClusterResources `json:"resources"`

	// Network endpoints
	Endpoints ClusterEndpoints `json:"endpoints,omitempty"`

	// Cost information
	CostPerHour float64 `json:"cost_per_hour,omitempty"`

	// Metadata
	Metadata map[string]string `json:"metadata,omitempty"`
}

// ClusterResources describes allocated compute resources
type ClusterResources struct {
	// GPU configuration
	Accelerators     string `json:"accelerators,omitempty"`      // e.g., "A100:4"
	AcceleratorCount int    `json:"accelerator_count,omitempty"` // Number of GPUs/TPUs

	// CPU/Memory
	CPUs   int    `json:"cpus"`            // Number of vCPUs
	Memory string `json:"memory"`          // e.g., "64GB"
	Disk   string `json:"disk,omitempty"`  // e.g., "500GB"

	// Instance details
	InstanceType string `json:"instance_type,omitempty"` // e.g., "g4dn.12xlarge"

	// Image
	Image string `json:"image,omitempty"` // VM image used
}

// ClusterEndpoints contains network endpoints for the cluster
type ClusterEndpoints struct {
	// SSH connection
	SSHHost string `json:"ssh_host,omitempty"`
	SSHPort int    `json:"ssh_port,omitempty"`
	SSHUser string `json:"ssh_user,omitempty"`

	// HTTP/HTTPS endpoints
	HTTP  string `json:"http,omitempty"`  // e.g., "http://54.123.45.67:8080"
	HTTPS string `json:"https,omitempty"` // e.g., "https://54.123.45.67:8443"

	// Custom endpoints exposed by the workload
	Custom map[string]string `json:"custom,omitempty"` // e.g., {"vllm": "http://54.123.45.67:8000"}
}

// ClusterListResponse contains a list of all clusters
type ClusterListResponse struct {
	Clusters []ClusterStatus `json:"clusters"`
	Total    int             `json:"total"`
}

// RequestStatus represents the status of an async operation
type RequestStatus struct {
	// Request identity
	RequestID string `json:"request_id"`

	// Status: pending, running, completed, failed, cancelled
	Status string `json:"status"`

	// Progress tracking
	Progress     int    `json:"progress,omitempty"`      // 0-100 percentage
	CurrentPhase string `json:"current_phase,omitempty"` // e.g., "provisioning", "configuring", "launching"

	// Result data (populated when status is "completed")
	Result interface{} `json:"result,omitempty"`

	// Error information (populated when status is "failed")
	Error       string `json:"error,omitempty"`
	ErrorDetail string `json:"error_detail,omitempty"`

	// Timing information
	CreatedAt   time.Time  `json:"created_at"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`

	// Metadata
	Metadata map[string]string `json:"metadata,omitempty"`
}

// HealthResponse represents the API server health check response
type HealthResponse struct {
	Status    string    `json:"status"`    // "healthy", "degraded", "unhealthy"
	Version   string    `json:"version"`   // SkyPilot API version
	Uptime    int       `json:"uptime"`    // Seconds since server start
	Timestamp time.Time `json:"timestamp"` // Current server time

	// Component health
	Components map[string]ComponentHealth `json:"components,omitempty"`
}

// ComponentHealth represents health status of a specific component
type ComponentHealth struct {
	Status  string `json:"status"`            // "healthy", "degraded", "unhealthy"
	Message string `json:"message,omitempty"` // Additional health info
	Latency int    `json:"latency,omitempty"` // Response time in ms
}

// ErrorResponse represents an API error response
type ErrorResponse struct {
	Error       string `json:"error"`                  // Human-readable error message
	ErrorCode   string `json:"error_code,omitempty"`   // Machine-readable error code
	StatusCode  int    `json:"status_code"`            // HTTP status code
	RequestID   string `json:"request_id,omitempty"`   // Request ID for debugging
	Timestamp   string `json:"timestamp,omitempty"`    // Error timestamp
	Details     string `json:"details,omitempty"`      // Additional error details
	Remediation string `json:"remediation,omitempty"`  // Suggested fix
}

// ExecuteRequest represents a command execution request on a cluster
type ExecuteRequest struct {
	ClusterName string            `json:"cluster_name"`
	Command     string            `json:"command"`          // Shell command to execute
	Envs        map[string]string `json:"envs,omitempty"`   // Environment variables
	WorkingDir  string            `json:"working_dir,omitempty"` // Working directory
	Timeout     int               `json:"timeout,omitempty"` // Timeout in seconds
}

// ExecuteResponse contains the result of command execution
type ExecuteResponse struct {
	RequestID  string `json:"request_id"`
	Stdout     string `json:"stdout,omitempty"`
	Stderr     string `json:"stderr,omitempty"`
	ExitCode   int    `json:"exit_code"`
	ExecutedAt time.Time `json:"executed_at"`
}

// LogsRequest represents a request for cluster logs
type LogsRequest struct {
	ClusterName string `json:"cluster_name"`
	Follow      bool   `json:"follow,omitempty"`      // Stream logs
	TailLines   int    `json:"tail_lines,omitempty"`  // Number of lines to tail
	Since       string `json:"since,omitempty"`       // RFC3339 timestamp or duration (e.g., "1h")
}

// LogsResponse contains cluster logs
type LogsResponse struct {
	Logs      string    `json:"logs"`
	Timestamp time.Time `json:"timestamp"`
	More      bool      `json:"more"` // Whether more logs are available
}

// ResourceQuotaRequest represents cloud resource quota information
type ResourceQuotaRequest struct {
	Provider string `json:"provider"` // aws, azure, gcp
	Region   string `json:"region"`
}

// ResourceQuotaResponse contains quota information
type ResourceQuotaResponse struct {
	Provider string                       `json:"provider"`
	Region   string                       `json:"region"`
	Quotas   map[string]ResourceQuotaInfo `json:"quotas"`
}

// ResourceQuotaInfo represents quota details for a specific resource type
type ResourceQuotaInfo struct {
	ResourceType string  `json:"resource_type"` // e.g., "vcpu", "gpu-a100"
	Limit        int     `json:"limit"`
	Used         int     `json:"used"`
	Available    int     `json:"available"`
	Unit         string  `json:"unit,omitempty"` // e.g., "count", "GB"
}

// EstimateCostRequest represents a cost estimation request
type EstimateCostRequest struct {
	TaskYAML string `json:"task_yaml"`
	Hours    int    `json:"hours"` // Duration to estimate for
}

// EstimateCostResponse contains cost estimation
type EstimateCostResponse struct {
	EstimatedCost       float64 `json:"estimated_cost"`        // Total estimated cost
	CostPerHour         float64 `json:"cost_per_hour"`         // Hourly cost
	Currency            string  `json:"currency"`              // USD, EUR, etc.
	Provider            string  `json:"provider"`
	Region              string  `json:"region"`
	InstanceType        string  `json:"instance_type"`
	EstimatedAt         time.Time `json:"estimated_at"`

	// Breakdown
	ComputeCost         float64 `json:"compute_cost,omitempty"`
	StorageCost         float64 `json:"storage_cost,omitempty"`
	NetworkCost         float64 `json:"network_cost,omitempty"`

	// Confidence
	ConfidenceLevel     string  `json:"confidence_level,omitempty"` // high, medium, low
}
