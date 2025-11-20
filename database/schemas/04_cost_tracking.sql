-- Cost Tracking Schema
-- Tracks per-tenant cost summaries for billing and analytics

-- Tenant Cost Summary Table
-- Stores aggregated cost data per tenant per time period
CREATE TABLE IF NOT EXISTS tenant_cost_summary (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Tenant identification
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    period VARCHAR(10) NOT NULL, -- e.g., "2025-01" for monthly, "2025-01-19" for daily

    -- Time range for this summary
    start_time TIMESTAMPTZ NOT NULL,
    end_time TIMESTAMPTZ NOT NULL,

    -- Compute costs (USD)
    compute_cost DECIMAL(10, 4) DEFAULT 0.0 NOT NULL,
    spot_cost DECIMAL(10, 4) DEFAULT 0.0 NOT NULL,
    ondemand_cost DECIMAL(10, 4) DEFAULT 0.0 NOT NULL,

    -- Token costs (USD)
    token_cost DECIMAL(10, 4) DEFAULT 0.0 NOT NULL,
    input_tokens BIGINT DEFAULT 0 NOT NULL,
    output_tokens BIGINT DEFAULT 0 NOT NULL,

    -- Usage metrics
    total_requests BIGINT DEFAULT 0 NOT NULL,
    gpu_hours DECIMAL(10, 2) DEFAULT 0.0 NOT NULL,
    spot_hours DECIMAL(10, 2) DEFAULT 0.0 NOT NULL,
    ondemand_hours DECIMAL(10, 2) DEFAULT 0.0 NOT NULL,

    -- Cost savings
    total_cost DECIMAL(10, 4) DEFAULT 0.0 NOT NULL,
    potential_cost DECIMAL(10, 4) DEFAULT 0.0 NOT NULL, -- Cost if all on-demand
    savings DECIMAL(10, 4) DEFAULT 0.0 NOT NULL,
    savings_percent DECIMAL(5, 2) DEFAULT 0.0 NOT NULL,

    -- Timestamps
    created_at TIMESTAMPTZ DEFAULT NOW() NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT NOW() NOT NULL,

    -- Unique constraint to prevent duplicate summaries
    UNIQUE(tenant_id, period, start_time)
);

-- Indexes for efficient querying
CREATE INDEX idx_tenant_cost_summary_tenant_id ON tenant_cost_summary(tenant_id);
CREATE INDEX idx_tenant_cost_summary_period ON tenant_cost_summary(period);
CREATE INDEX idx_tenant_cost_summary_start_time ON tenant_cost_summary(start_time);
CREATE INDEX idx_tenant_cost_summary_total_cost ON tenant_cost_summary(total_cost DESC);

-- Index for finding top spending tenants
CREATE INDEX idx_tenant_cost_summary_tenant_period ON tenant_cost_summary(tenant_id, period);

-- Comments for documentation
COMMENT ON TABLE tenant_cost_summary IS 'Aggregated cost summaries per tenant for billing and analytics';
COMMENT ON COLUMN tenant_cost_summary.period IS 'Billing period identifier (YYYY-MM for monthly, YYYY-MM-DD for daily)';
COMMENT ON COLUMN tenant_cost_summary.compute_cost IS 'Total compute cost (GPU hours) in USD';
COMMENT ON COLUMN tenant_cost_summary.spot_cost IS 'Cost from spot instances in USD';
COMMENT ON COLUMN tenant_cost_summary.ondemand_cost IS 'Cost from on-demand instances in USD';
COMMENT ON COLUMN tenant_cost_summary.token_cost IS 'Cost based on token usage in USD';
COMMENT ON COLUMN tenant_cost_summary.potential_cost IS 'What the cost would be if all instances were on-demand';
COMMENT ON COLUMN tenant_cost_summary.savings IS 'Amount saved by using spot instances';
COMMENT ON COLUMN tenant_cost_summary.savings_percent IS 'Percentage saved (0-100)';

-- GPU Pricing Configuration Table (optional, for dynamic pricing)
-- Allows runtime modification of pricing without code changes
CREATE TABLE IF NOT EXISTS gpu_pricing_tiers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    gpu_type VARCHAR(50) NOT NULL UNIQUE,

    -- Hourly rates (USD per GPU per hour)
    ondemand_rate DECIMAL(10, 4) NOT NULL,
    spot_rate DECIMAL(10, 4) NOT NULL,

    -- Token pricing (USD per million tokens)
    token_rate DECIMAL(10, 6) NOT NULL,

    -- Minimum charge per GPU per hour
    minimum_charge DECIMAL(10, 4) DEFAULT 0.0 NOT NULL,

    -- Spot discount percentage (informational)
    spot_discount DECIMAL(5, 2) DEFAULT 70.0 NOT NULL,

    -- Metadata
    description TEXT,
    is_active BOOLEAN DEFAULT true NOT NULL,

    -- Timestamps
    created_at TIMESTAMPTZ DEFAULT NOW() NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT NOW() NOT NULL
);

-- Index for GPU type lookups
CREATE INDEX idx_gpu_pricing_tiers_gpu_type ON gpu_pricing_tiers(gpu_type);
CREATE INDEX idx_gpu_pricing_tiers_active ON gpu_pricing_tiers(is_active);

COMMENT ON TABLE gpu_pricing_tiers IS 'GPU pricing configuration for different GPU types';

-- Insert default pricing tiers
INSERT INTO gpu_pricing_tiers (gpu_type, ondemand_rate, spot_rate, token_rate, minimum_charge, spot_discount, description) VALUES
    ('A10G', 1.20, 0.36, 0.30, 0.10, 70.0, 'NVIDIA A10G - Entry-level GPU for inference'),
    ('A100', 4.00, 1.20, 0.80, 0.30, 70.0, 'NVIDIA A100 40GB - High-performance GPU'),
    ('A100-80GB', 5.50, 1.65, 1.00, 0.40, 70.0, 'NVIDIA A100 80GB - High-memory GPU'),
    ('H100', 8.00, 2.40, 1.50, 0.60, 70.0, 'NVIDIA H100 - Latest generation GPU'),
    ('H100-NVL', 10.00, 3.00, 2.00, 0.80, 70.0, 'NVIDIA H100 NVL - High-bandwidth GPU'),
    ('L4', 0.80, 0.24, 0.20, 0.08, 70.0, 'NVIDIA L4 - Cost-effective inference GPU'),
    ('V100', 2.50, 0.75, 0.60, 0.20, 70.0, 'NVIDIA V100 - Legacy high-performance GPU')
ON CONFLICT (gpu_type) DO NOTHING;

-- Cost Anomaly Detection Table
-- Tracks unusual cost patterns for alerting
CREATE TABLE IF NOT EXISTS cost_anomalies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,

    -- Anomaly details
    anomaly_type VARCHAR(50) NOT NULL, -- 'spike', 'sustained_high', 'unusual_pattern'
    severity VARCHAR(20) NOT NULL, -- 'info', 'warning', 'critical'

    -- Cost information
    current_cost DECIMAL(10, 4) NOT NULL,
    baseline_cost DECIMAL(10, 4) NOT NULL,
    deviation_percent DECIMAL(5, 2) NOT NULL,

    -- Time period
    detection_time TIMESTAMPTZ DEFAULT NOW() NOT NULL,
    period_start TIMESTAMPTZ NOT NULL,
    period_end TIMESTAMPTZ NOT NULL,

    -- Status
    acknowledged BOOLEAN DEFAULT false NOT NULL,
    acknowledged_at TIMESTAMPTZ,
    acknowledged_by UUID REFERENCES tenants(id),

    -- Details
    description TEXT,
    metadata JSONB,

    created_at TIMESTAMPTZ DEFAULT NOW() NOT NULL
);

CREATE INDEX idx_cost_anomalies_tenant_id ON cost_anomalies(tenant_id);
CREATE INDEX idx_cost_anomalies_detection_time ON cost_anomalies(detection_time DESC);
CREATE INDEX idx_cost_anomalies_severity ON cost_anomalies(severity);
CREATE INDEX idx_cost_anomalies_acknowledged ON cost_anomalies(acknowledged, detection_time);

COMMENT ON TABLE cost_anomalies IS 'Detected cost anomalies for alerting and investigation';
COMMENT ON COLUMN cost_anomalies.anomaly_type IS 'Type of anomaly detected';
COMMENT ON COLUMN cost_anomalies.deviation_percent IS 'Percentage deviation from baseline';
