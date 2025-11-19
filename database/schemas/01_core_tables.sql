-- CrossLogic Inference Cloud - Core Database Schema
-- Version: 1.0.0
-- Description: Core tables for multi-tenant LLM inference platform

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ============================================================================
-- TENANTS & ORGANIZATIONS
-- ============================================================================

CREATE TABLE tenants (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255) NOT NULL UNIQUE,
    stripe_customer_id VARCHAR(255) UNIQUE,
    status VARCHAR(50) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'suspended', 'deleted')),
    billing_plan VARCHAR(50) NOT NULL DEFAULT 'serverless' CHECK (billing_plan IN ('serverless', 'reserved', 'enterprise')),
    reserved_capacity_tokens_per_sec INTEGER DEFAULT 0,
    region_preferences JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX idx_tenants_email ON tenants(email);
CREATE INDEX idx_tenants_stripe_customer_id ON tenants(stripe_customer_id);
CREATE INDEX idx_tenants_status ON tenants(status);

-- ============================================================================
-- ENVIRONMENTS (dev/staging/prod per org)
-- ============================================================================

CREATE TABLE environments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    region VARCHAR(50) NOT NULL,
    model_list JSONB DEFAULT '[]',
    quota_tokens_per_day BIGINT DEFAULT 1000000,
    quota_tokens_per_minute INTEGER DEFAULT 10000,
    concurrency_limit INTEGER DEFAULT 10,
    status VARCHAR(50) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'suspended')),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(tenant_id, name)
);

CREATE INDEX idx_environments_tenant_id ON environments(tenant_id);
CREATE INDEX idx_environments_region ON environments(region);
CREATE INDEX idx_environments_status ON environments(status);

-- ============================================================================
-- API KEYS
-- ============================================================================

CREATE TABLE api_keys (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    key_hash TEXT NOT NULL UNIQUE,
    key_prefix VARCHAR(20) NOT NULL,
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    environment_id UUID NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    user_id UUID,
    name VARCHAR(255),
    role VARCHAR(50) NOT NULL DEFAULT 'developer' CHECK (role IN ('admin', 'developer', 'read-only')),
    rate_limit_tokens_per_min INTEGER,
    rate_limit_requests_per_min INTEGER DEFAULT 60,
    concurrency_limit INTEGER DEFAULT 5,
    status VARCHAR(50) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'suspended', 'revoked')),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    last_used_at TIMESTAMP WITH TIME ZONE,
    expires_at TIMESTAMP WITH TIME ZONE,
    metadata JSONB DEFAULT '{}'
);

CREATE INDEX idx_api_keys_key_hash ON api_keys(key_hash);
CREATE INDEX idx_api_keys_tenant_id ON api_keys(tenant_id);
CREATE INDEX idx_api_keys_environment_id ON api_keys(environment_id);
CREATE INDEX idx_api_keys_status ON api_keys(status);
CREATE INDEX idx_api_keys_key_prefix ON api_keys(key_prefix);

-- ============================================================================
-- REGIONS & AVAILABILITY
-- ============================================================================

CREATE TABLE regions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    code VARCHAR(50) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    country VARCHAR(100),
    city VARCHAR(100),
    cloud_providers JSONB DEFAULT '[]',
    cost_multiplier DECIMAL(10, 4) DEFAULT 1.0,
    status VARCHAR(50) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'degraded', 'maintenance', 'offline')),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_regions_code ON regions(code);
CREATE INDEX idx_regions_status ON regions(status);

-- Insert default regions
INSERT INTO regions (code, name, country, city, cloud_providers, cost_multiplier) VALUES
('in-mumbai', 'Mumbai', 'India', 'Mumbai', '["aws", "gcp", "azure"]', 0.7),
('us-east', 'US East', 'USA', 'Virginia', '["aws", "gcp", "azure"]', 1.0),
('eu-west', 'EU West', 'Germany', 'Frankfurt', '["aws", "gcp", "azure"]', 1.1),
('ap-southeast', 'APAC', 'Singapore', 'Singapore', '["aws", "gcp", "azure"]', 0.9);

-- ============================================================================
-- MODEL CATALOG
-- ============================================================================

CREATE TABLE models (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL UNIQUE,
    family VARCHAR(100) NOT NULL,
    size VARCHAR(50),
    type VARCHAR(50) NOT NULL CHECK (type IN ('completion', 'chat', 'embedding')),
    context_length INTEGER NOT NULL,
    vram_required_gb INTEGER NOT NULL,
    price_input_per_million DECIMAL(10, 6) NOT NULL,
    price_output_per_million DECIMAL(10, 6) NOT NULL,
    tokens_per_second_capacity INTEGER,
    status VARCHAR(50) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'deprecated', 'beta')),
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_models_name ON models(name);
CREATE INDEX idx_models_family ON models(family);
CREATE INDEX idx_models_type ON models(type);
CREATE INDEX idx_models_status ON models(status);

-- Insert default models
INSERT INTO models (name, family, size, type, context_length, vram_required_gb, price_input_per_million, price_output_per_million, tokens_per_second_capacity) VALUES
('llama-3-8b', 'Llama', '8B', 'chat', 8192, 16, 0.05, 0.05, 100),
('llama-3-70b', 'Llama', '70B', 'chat', 8192, 80, 0.60, 0.60, 50),
('mistral-7b', 'Mistral', '7B', 'chat', 32768, 16, 0.04, 0.04, 100),
('qwen-2.5-7b', 'Qwen', '7B', 'chat', 32768, 16, 0.04, 0.04, 100),
('gemma-7b', 'Gemma', '7B', 'chat', 8192, 16, 0.03, 0.03, 100);

-- ============================================================================
-- GPU NODES
-- ============================================================================

CREATE TABLE nodes (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    node_id_external VARCHAR(255) UNIQUE,
    provider VARCHAR(50) NOT NULL CHECK (provider IN ('aws', 'gcp', 'azure', 'oci', 'on-prem')),
    region_id UUID REFERENCES regions(id),
    instance_type VARCHAR(100),
    gpu_type VARCHAR(100),
    vram_total_gb INTEGER,
    vram_free_gb INTEGER,
    model_id UUID REFERENCES models(id),
    endpoint_url VARCHAR(500) NOT NULL,
    internal_ip VARCHAR(50),
    spot_instance BOOLEAN DEFAULT false,
    spot_price DECIMAL(10, 4),
    throughput_tokens_per_sec INTEGER,
    status VARCHAR(50) NOT NULL DEFAULT 'initializing' CHECK (status IN ('initializing', 'active', 'draining', 'unhealthy', 'dead')),
    health_score DECIMAL(5, 2) DEFAULT 100.0,
    last_heartbeat_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    terminated_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX idx_nodes_provider ON nodes(provider);
CREATE INDEX idx_nodes_region_id ON nodes(region_id);
CREATE INDEX idx_nodes_model_id ON nodes(model_id);
CREATE INDEX idx_nodes_status ON nodes(status);
CREATE INDEX idx_nodes_health_score ON nodes(health_score);
CREATE INDEX idx_nodes_last_heartbeat_at ON nodes(last_heartbeat_at);

-- ============================================================================
-- USAGE RECORDS
-- ============================================================================

CREATE TABLE usage_records (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    request_id VARCHAR(255) UNIQUE,
    timestamp TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    environment_id UUID NOT NULL REFERENCES environments(id),
    api_key_id UUID REFERENCES api_keys(id),
    region_id UUID REFERENCES regions(id),
    model_id UUID REFERENCES models(id),
    node_id UUID REFERENCES nodes(id),
    prompt_tokens INTEGER NOT NULL DEFAULT 0,
    completion_tokens INTEGER NOT NULL DEFAULT 0,
    total_tokens INTEGER NOT NULL DEFAULT 0,
    cached_tokens INTEGER DEFAULT 0,
    latency_ms INTEGER,
    cost_microdollars BIGINT,
    billed BOOLEAN DEFAULT false,
    billing_failed BOOLEAN DEFAULT false,
    retry_count INTEGER DEFAULT 0,
    metadata JSONB DEFAULT '{}'
);

CREATE INDEX idx_usage_records_timestamp ON usage_records(timestamp DESC);
CREATE INDEX idx_usage_records_tenant_id ON usage_records(tenant_id);
CREATE INDEX idx_usage_records_environment_id ON usage_records(environment_id);
CREATE INDEX idx_usage_records_api_key_id ON usage_records(api_key_id);
CREATE INDEX idx_usage_records_model_id ON usage_records(model_id);
CREATE INDEX idx_usage_records_billed ON usage_records(billed);
CREATE INDEX idx_usage_records_request_id ON usage_records(request_id);

-- Partitioning for performance (optional for production)
-- CREATE TABLE usage_records_y2025m01 PARTITION OF usage_records
--     FOR VALUES FROM ('2025-01-01') TO ('2025-02-01');

-- ============================================================================
-- AGGREGATED USAGE (for billing & analytics)
-- ============================================================================

CREATE TABLE usage_hourly (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    hour TIMESTAMP WITH TIME ZONE NOT NULL,
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    environment_id UUID REFERENCES environments(id),
    model_id UUID REFERENCES models(id),
    region_id UUID REFERENCES regions(id),
    total_tokens BIGINT NOT NULL DEFAULT 0,
    total_requests INTEGER NOT NULL DEFAULT 0,
    total_cost_microdollars BIGINT DEFAULT 0,
    avg_latency_ms INTEGER,
    p50_latency_ms INTEGER,
    p95_latency_ms INTEGER,
    p99_latency_ms INTEGER,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(hour, tenant_id, environment_id, model_id, region_id)
);

CREATE INDEX idx_usage_hourly_hour ON usage_hourly(hour DESC);
CREATE INDEX idx_usage_hourly_tenant_id ON usage_hourly(tenant_id);
CREATE INDEX idx_usage_hourly_model_id ON usage_hourly(model_id);

-- ============================================================================
-- BILLING EVENTS
-- ============================================================================

CREATE TABLE billing_events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    event_type VARCHAR(100) NOT NULL CHECK (event_type IN ('usage', 'subscription', 'credit', 'refund')),
    amount_microdollars BIGINT NOT NULL,
    currency VARCHAR(10) DEFAULT 'USD',
    stripe_usage_record_id VARCHAR(255),
    stripe_invoice_id VARCHAR(255),
    description TEXT,
    period_start TIMESTAMP WITH TIME ZONE,
    period_end TIMESTAMP WITH TIME ZONE,
    status VARCHAR(50) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'processed', 'failed')),
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    processed_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX idx_billing_events_tenant_id ON billing_events(tenant_id);
CREATE INDEX idx_billing_events_event_type ON billing_events(event_type);
CREATE INDEX idx_billing_events_status ON billing_events(status);
CREATE INDEX idx_billing_events_created_at ON billing_events(created_at DESC);

-- ============================================================================
-- CREDITS & FREE TIER
-- ============================================================================

CREATE TABLE credits (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    amount_microdollars BIGINT NOT NULL,
    remaining_microdollars BIGINT NOT NULL,
    credit_type VARCHAR(50) NOT NULL CHECK (credit_type IN ('signup_bonus', 'referral', 'promotional', 'monthly_free')),
    description TEXT,
    expires_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_credits_tenant_id ON credits(tenant_id);
CREATE INDEX idx_credits_credit_type ON credits(credit_type);
CREATE INDEX idx_credits_expires_at ON credits(expires_at);

-- ============================================================================
-- CAPACITY RESERVATIONS
-- ============================================================================

CREATE TABLE reservations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    environment_id UUID REFERENCES environments(id),
    model_id UUID REFERENCES models(id),
    region_id UUID REFERENCES regions(id),
    tokens_per_sec INTEGER NOT NULL,
    priority INTEGER DEFAULT 1,
    status VARCHAR(50) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'expired', 'cancelled')),
    starts_at TIMESTAMP WITH TIME ZONE NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_reservations_tenant_id ON reservations(tenant_id);
CREATE INDEX idx_reservations_model_id ON reservations(model_id);
CREATE INDEX idx_reservations_status ON reservations(status);
CREATE INDEX idx_reservations_expires_at ON reservations(expires_at);

-- ============================================================================
-- AUDIT LOGS
-- ============================================================================

CREATE TABLE audit_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    timestamp TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    tenant_id UUID REFERENCES tenants(id),
    user_id UUID,
    action VARCHAR(255) NOT NULL,
    resource_type VARCHAR(100),
    resource_id UUID,
    ip_address INET,
    user_agent TEXT,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_audit_logs_timestamp ON audit_logs(timestamp DESC);
CREATE INDEX idx_audit_logs_tenant_id ON audit_logs(tenant_id);
CREATE INDEX idx_audit_logs_action ON audit_logs(action);

-- ============================================================================
-- HEALTH CHECK RECORDS
-- ============================================================================

CREATE TABLE health_checks (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    node_id UUID NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    timestamp TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    status VARCHAR(50) NOT NULL CHECK (status IN ('healthy', 'degraded', 'unhealthy')),
    response_time_ms INTEGER,
    gpu_temperature_celsius INTEGER,
    vram_free_gb INTEGER,
    error_message TEXT,
    metadata JSONB DEFAULT '{}'
);

CREATE INDEX idx_health_checks_node_id ON health_checks(node_id);
CREATE INDEX idx_health_checks_timestamp ON health_checks(timestamp DESC);
CREATE INDEX idx_health_checks_status ON health_checks(status);

-- ============================================================================
-- TRIGGERS FOR updated_at
-- ============================================================================

CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_tenants_updated_at BEFORE UPDATE ON tenants
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_environments_updated_at BEFORE UPDATE ON environments
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_regions_updated_at BEFORE UPDATE ON regions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_models_updated_at BEFORE UPDATE ON models
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_nodes_updated_at BEFORE UPDATE ON nodes
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_reservations_updated_at BEFORE UPDATE ON reservations
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_credits_updated_at BEFORE UPDATE ON credits
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- ============================================================================
-- WEBHOOK EVENTS TABLE (for idempotency and audit trail)
-- ============================================================================

CREATE TABLE webhook_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id VARCHAR(255) NOT NULL UNIQUE,
    event_type VARCHAR(100) NOT NULL,
    processed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    payload JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_webhook_events_event_id ON webhook_events(event_id);
CREATE INDEX idx_webhook_events_event_type ON webhook_events(event_type);
CREATE INDEX idx_webhook_events_processed_at ON webhook_events(processed_at DESC);
