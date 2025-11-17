package models

import "time"

// Tenant represents an organization using the platform.
type Tenant struct {
	ID          string
	Name        string
	Email       string
	Environment string
	CreatedAt   time.Time
}

// APIKey represents credentials to access APIs.
type APIKey struct {
	Key         string
	TenantID    string
	Environment string
	RateLimit   int
	CreatedAt   time.Time
	LastUsedAt  time.Time
}

// NodeStatus indicates the health of a worker node.
type NodeStatus string

const (
	NodeStatusHealthy   NodeStatus = "healthy"
	NodeStatusDraining  NodeStatus = "draining"
	NodeStatusUnhealthy NodeStatus = "unhealthy"
)

// Node describes a GPU worker running vLLM/SGLang.
type Node struct {
	ID            string
	Provider      string
	Region        string
	InstanceType  string
	Model         string
	Endpoint      string
	SpotPrice     float64
	Status        NodeStatus
	LastHeartbeat time.Time
}

// UsageRecord tracks a single billing event.
type UsageRecord struct {
	ID           string
	TenantID     string
	Model        string
	Environment  string
	Region       string
	InputTokens  int64
	OutputTokens int64
	CachedTokens int64
	LatencyMs    int64
	Timestamp    time.Time
}

// Request represents an OpenAI-compatible payload after validation.
type Request struct {
	APIKey      string
	TenantID    string
	Environment string
	Region      string
	Model       string
	Prompt      string
}

// Response summarizes the gateway result.
type Response struct {
	Model        string         `json:"model"`
	Provider     string         `json:"provider"`
	Region       string         `json:"region"`
	NodeID       string         `json:"node_id"`
	Message      string         `json:"message"`
	InputTokens  int64          `json:"input_tokens"`
	OutputTokens int64          `json:"output_tokens"`
	CachedTokens int64          `json:"cached_tokens"`
	LatencyMs    int64          `json:"latency_ms"`
	Timestamp    time.Time      `json:"timestamp"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}
