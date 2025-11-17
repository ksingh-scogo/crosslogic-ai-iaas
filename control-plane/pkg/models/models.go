package models

import (
	"time"

	"github.com/google/uuid"
)

// Tenant represents an organization in the system
type Tenant struct {
	ID                           uuid.UUID  `json:"id" db:"id"`
	Name                         string     `json:"name" db:"name"`
	Email                        string     `json:"email" db:"email"`
	StripeCustomerID             *string    `json:"stripe_customer_id,omitempty" db:"stripe_customer_id"`
	Status                       string     `json:"status" db:"status"`
	BillingPlan                  string     `json:"billing_plan" db:"billing_plan"`
	ReservedCapacityTokensPerSec int        `json:"reserved_capacity_tokens_per_sec" db:"reserved_capacity_tokens_per_sec"`
	RegionPreferences            string     `json:"region_preferences" db:"region_preferences"` // JSON
	CreatedAt                    time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt                    time.Time  `json:"updated_at" db:"updated_at"`
	DeletedAt                    *time.Time `json:"deleted_at,omitempty" db:"deleted_at"`
}

// Environment represents an environment (dev/staging/prod) within a tenant
type Environment struct {
	ID                   uuid.UUID `json:"id" db:"id"`
	TenantID             uuid.UUID `json:"tenant_id" db:"tenant_id"`
	Name                 string    `json:"name" db:"name"`
	Region               string    `json:"region" db:"region"`
	ModelList            string    `json:"model_list" db:"model_list"` // JSON array
	QuotaTokensPerDay    int64     `json:"quota_tokens_per_day" db:"quota_tokens_per_day"`
	QuotaTokensPerMinute int       `json:"quota_tokens_per_minute" db:"quota_tokens_per_minute"`
	ConcurrencyLimit     int       `json:"concurrency_limit" db:"concurrency_limit"`
	Status               string    `json:"status" db:"status"`
	CreatedAt            time.Time `json:"created_at" db:"created_at"`
	UpdatedAt            time.Time `json:"updated_at" db:"updated_at"`
}

// APIKey represents an API key for authentication
type APIKey struct {
	ID                      uuid.UUID  `json:"id" db:"id"`
	KeyHash                 string     `json:"-" db:"key_hash"`
	KeyPrefix               string     `json:"key_prefix" db:"key_prefix"`
	TenantID                uuid.UUID  `json:"tenant_id" db:"tenant_id"`
	EnvironmentID           uuid.UUID  `json:"environment_id" db:"environment_id"`
	UserID                  *uuid.UUID `json:"user_id,omitempty" db:"user_id"`
	Name                    *string    `json:"name,omitempty" db:"name"`
	Role                    string     `json:"role" db:"role"`
	RateLimitTokensPerMin   *int       `json:"rate_limit_tokens_per_min,omitempty" db:"rate_limit_tokens_per_min"`
	RateLimitRequestsPerMin int        `json:"rate_limit_requests_per_min" db:"rate_limit_requests_per_min"`
	ConcurrencyLimit        int        `json:"concurrency_limit" db:"concurrency_limit"`
	Status                  string     `json:"status" db:"status"`
	CreatedAt               time.Time  `json:"created_at" db:"created_at"`
	LastUsedAt              *time.Time `json:"last_used_at,omitempty" db:"last_used_at"`
	ExpiresAt               *time.Time `json:"expires_at,omitempty" db:"expires_at"`
	Metadata                string     `json:"metadata" db:"metadata"` // JSON
}

// Region represents a geographical region
type Region struct {
	ID             uuid.UUID `json:"id" db:"id"`
	Code           string    `json:"code" db:"code"`
	Name           string    `json:"name" db:"name"`
	Country        *string   `json:"country,omitempty" db:"country"`
	City           *string   `json:"city,omitempty" db:"city"`
	CloudProviders string    `json:"cloud_providers" db:"cloud_providers"` // JSON array
	CostMultiplier float64   `json:"cost_multiplier" db:"cost_multiplier"`
	Status         string    `json:"status" db:"status"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
}

// Model represents an LLM model
type Model struct {
	ID                      uuid.UUID `json:"id" db:"id"`
	Name                    string    `json:"name" db:"name"`
	Family                  string    `json:"family" db:"family"`
	Size                    *string   `json:"size,omitempty" db:"size"`
	Type                    string    `json:"type" db:"type"`
	ContextLength           int       `json:"context_length" db:"context_length"`
	VRAMRequiredGB          int       `json:"vram_required_gb" db:"vram_required_gb"`
	PriceInputPerMillion    float64   `json:"price_input_per_million" db:"price_input_per_million"`
	PriceOutputPerMillion   float64   `json:"price_output_per_million" db:"price_output_per_million"`
	TokensPerSecondCapacity *int      `json:"tokens_per_second_capacity,omitempty" db:"tokens_per_second_capacity"`
	Status                  string    `json:"status" db:"status"`
	Metadata                string    `json:"metadata" db:"metadata"` // JSON
	CreatedAt               time.Time `json:"created_at" db:"created_at"`
	UpdatedAt               time.Time `json:"updated_at" db:"updated_at"`
}

// Node represents a GPU worker node
type Node struct {
	ID                     uuid.UUID  `json:"id" db:"id"`
	NodeIDExternal         *string    `json:"node_id_external,omitempty" db:"node_id_external"`
	Provider               string     `json:"provider" db:"provider"`
	RegionID               *uuid.UUID `json:"region_id,omitempty" db:"region_id"`
	InstanceType           *string    `json:"instance_type,omitempty" db:"instance_type"`
	GPUType                *string    `json:"gpu_type,omitempty" db:"gpu_type"`
	VRAMTotalGB            *int       `json:"vram_total_gb,omitempty" db:"vram_total_gb"`
	VRAMFreeGB             *int       `json:"vram_free_gb,omitempty" db:"vram_free_gb"`
	ModelID                *uuid.UUID `json:"model_id,omitempty" db:"model_id"`
	EndpointURL            string     `json:"endpoint_url" db:"endpoint_url"`
	InternalIP             *string    `json:"internal_ip,omitempty" db:"internal_ip"`
	SpotInstance           bool       `json:"spot_instance" db:"spot_instance"`
	SpotPrice              *float64   `json:"spot_price,omitempty" db:"spot_price"`
	ThroughputTokensPerSec *int       `json:"throughput_tokens_per_sec,omitempty" db:"throughput_tokens_per_sec"`
	Status                 string     `json:"status" db:"status"`
	HealthScore            float64    `json:"health_score" db:"health_score"`
	LastHeartbeatAt        *time.Time `json:"last_heartbeat_at,omitempty" db:"last_heartbeat_at"`
	CreatedAt              time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt              time.Time  `json:"updated_at" db:"updated_at"`
	TerminatedAt           *time.Time `json:"terminated_at,omitempty" db:"terminated_at"`
}

// UsageRecord represents a single inference request
type UsageRecord struct {
	ID               uuid.UUID  `json:"id" db:"id"`
	RequestID        *string    `json:"request_id,omitempty" db:"request_id"`
	Timestamp        time.Time  `json:"timestamp" db:"timestamp"`
	TenantID         uuid.UUID  `json:"tenant_id" db:"tenant_id"`
	EnvironmentID    uuid.UUID  `json:"environment_id" db:"environment_id"`
	APIKeyID         *uuid.UUID `json:"api_key_id,omitempty" db:"api_key_id"`
	RegionID         *uuid.UUID `json:"region_id,omitempty" db:"region_id"`
	ModelID          *uuid.UUID `json:"model_id,omitempty" db:"model_id"`
	NodeID           *uuid.UUID `json:"node_id,omitempty" db:"node_id"`
	PromptTokens     int        `json:"prompt_tokens" db:"prompt_tokens"`
	CompletionTokens int        `json:"completion_tokens" db:"completion_tokens"`
	TotalTokens      int        `json:"total_tokens" db:"total_tokens"`
	CachedTokens     *int       `json:"cached_tokens,omitempty" db:"cached_tokens"`
	LatencyMs        *int       `json:"latency_ms,omitempty" db:"latency_ms"`
	CostMicrodollars *int64     `json:"cost_microdollars,omitempty" db:"cost_microdollars"`
	Billed           bool       `json:"billed" db:"billed"`
	BillingFailed    bool       `json:"billing_failed" db:"billing_failed"`
	RetryCount       int        `json:"retry_count" db:"retry_count"`
	Metadata         string     `json:"metadata" db:"metadata"` // JSON
}

// UsageHourly represents aggregated hourly usage
type UsageHourly struct {
	ID                    uuid.UUID  `json:"id" db:"id"`
	Hour                  time.Time  `json:"hour" db:"hour"`
	TenantID              uuid.UUID  `json:"tenant_id" db:"tenant_id"`
	EnvironmentID         *uuid.UUID `json:"environment_id,omitempty" db:"environment_id"`
	ModelID               *uuid.UUID `json:"model_id,omitempty" db:"model_id"`
	RegionID              *uuid.UUID `json:"region_id,omitempty" db:"region_id"`
	TotalTokens           int64      `json:"total_tokens" db:"total_tokens"`
	TotalRequests         int        `json:"total_requests" db:"total_requests"`
	TotalCostMicrodollars *int64     `json:"total_cost_microdollars,omitempty" db:"total_cost_microdollars"`
	AvgLatencyMs          *int       `json:"avg_latency_ms,omitempty" db:"avg_latency_ms"`
	P50LatencyMs          *int       `json:"p50_latency_ms,omitempty" db:"p50_latency_ms"`
	P95LatencyMs          *int       `json:"p95_latency_ms,omitempty" db:"p95_latency_ms"`
	P99LatencyMs          *int       `json:"p99_latency_ms,omitempty" db:"p99_latency_ms"`
	CreatedAt             time.Time  `json:"created_at" db:"created_at"`
}

// BillingEvent represents a billing transaction
type BillingEvent struct {
	ID                  uuid.UUID  `json:"id" db:"id"`
	TenantID            uuid.UUID  `json:"tenant_id" db:"tenant_id"`
	EventType           string     `json:"event_type" db:"event_type"`
	AmountMicrodollars  int64      `json:"amount_microdollars" db:"amount_microdollars"`
	Currency            string     `json:"currency" db:"currency"`
	StripeUsageRecordID *string    `json:"stripe_usage_record_id,omitempty" db:"stripe_usage_record_id"`
	StripeInvoiceID     *string    `json:"stripe_invoice_id,omitempty" db:"stripe_invoice_id"`
	Description         *string    `json:"description,omitempty" db:"description"`
	PeriodStart         *time.Time `json:"period_start,omitempty" db:"period_start"`
	PeriodEnd           *time.Time `json:"period_end,omitempty" db:"period_end"`
	Status              string     `json:"status" db:"status"`
	Metadata            string     `json:"metadata" db:"metadata"` // JSON
	CreatedAt           time.Time  `json:"created_at" db:"created_at"`
	ProcessedAt         *time.Time `json:"processed_at,omitempty" db:"processed_at"`
}

// Credit represents promotional or free credits
type Credit struct {
	ID                    uuid.UUID  `json:"id" db:"id"`
	TenantID              uuid.UUID  `json:"tenant_id" db:"tenant_id"`
	AmountMicrodollars    int64      `json:"amount_microdollars" db:"amount_microdollars"`
	RemainingMicrodollars int64      `json:"remaining_microdollars" db:"remaining_microdollars"`
	CreditType            string     `json:"credit_type" db:"credit_type"`
	Description           *string    `json:"description,omitempty" db:"description"`
	ExpiresAt             *time.Time `json:"expires_at,omitempty" db:"expires_at"`
	CreatedAt             time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at" db:"updated_at"`
}

// Reservation represents reserved capacity
type Reservation struct {
	ID            uuid.UUID  `json:"id" db:"id"`
	TenantID      uuid.UUID  `json:"tenant_id" db:"tenant_id"`
	EnvironmentID *uuid.UUID `json:"environment_id,omitempty" db:"environment_id"`
	ModelID       *uuid.UUID `json:"model_id,omitempty" db:"model_id"`
	RegionID      *uuid.UUID `json:"region_id,omitempty" db:"region_id"`
	TokensPerSec  int        `json:"tokens_per_sec" db:"tokens_per_sec"`
	Priority      int        `json:"priority" db:"priority"`
	Status        string     `json:"status" db:"status"`
	StartsAt      time.Time  `json:"starts_at" db:"starts_at"`
	ExpiresAt     time.Time  `json:"expires_at" db:"expires_at"`
	CreatedAt     time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at" db:"updated_at"`
}

// HealthCheck represents a node health check record
type HealthCheck struct {
	ID                    uuid.UUID `json:"id" db:"id"`
	NodeID                uuid.UUID `json:"node_id" db:"node_id"`
	Timestamp             time.Time `json:"timestamp" db:"timestamp"`
	Status                string    `json:"status" db:"status"`
	ResponseTimeMs        *int      `json:"response_time_ms,omitempty" db:"response_time_ms"`
	GPUTemperatureCelsius *int      `json:"gpu_temperature_celsius,omitempty" db:"gpu_temperature_celsius"`
	VRAMFreeGB            *int      `json:"vram_free_gb,omitempty" db:"vram_free_gb"`
	ErrorMessage          *string   `json:"error_message,omitempty" db:"error_message"`
	Metadata              string    `json:"metadata" db:"metadata"` // JSON
}

// AuditLog represents an audit trail entry
type AuditLog struct {
	ID           uuid.UUID  `json:"id" db:"id"`
	Timestamp    time.Time  `json:"timestamp" db:"timestamp"`
	TenantID     *uuid.UUID `json:"tenant_id,omitempty" db:"tenant_id"`
	UserID       *uuid.UUID `json:"user_id,omitempty" db:"user_id"`
	Action       string     `json:"action" db:"action"`
	ResourceType *string    `json:"resource_type,omitempty" db:"resource_type"`
	ResourceID   *uuid.UUID `json:"resource_id,omitempty" db:"resource_id"`
	IPAddress    *string    `json:"ip_address,omitempty" db:"ip_address"`
	UserAgent    *string    `json:"user_agent,omitempty" db:"user_agent"`
	Metadata     string     `json:"metadata" db:"metadata"` // JSON
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
}
