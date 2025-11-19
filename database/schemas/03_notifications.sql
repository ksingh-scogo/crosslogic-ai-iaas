-- Notification Deliveries Table
-- Tracks all notification delivery attempts with status and retry information

CREATE TABLE IF NOT EXISTS notification_deliveries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Event reference
    event_id VARCHAR(255) NOT NULL,
    event_type VARCHAR(100) NOT NULL,
    tenant_id UUID REFERENCES tenants(id) ON DELETE CASCADE,

    -- Delivery details
    channel VARCHAR(50) NOT NULL,  -- 'discord', 'slack', 'email', 'webhook'
    destination TEXT,               -- URL or email address (optional, for logging)

    -- Status tracking
    status VARCHAR(50) NOT NULL DEFAULT 'pending',  -- 'pending', 'sent', 'failed', 'retry'
    retry_count INT DEFAULT 0,
    max_retries INT DEFAULT 3,

    -- Request/Response (optional, for debugging)
    request_payload JSONB,
    response_status INT,
    response_body TEXT,
    error_message TEXT,

    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    sent_at TIMESTAMP WITH TIME ZONE,
    failed_at TIMESTAMP WITH TIME ZONE,
    next_retry_at TIMESTAMP WITH TIME ZONE
);

-- Indexes for efficient querying
CREATE INDEX IF NOT EXISTS idx_notification_deliveries_event_id ON notification_deliveries(event_id);
CREATE INDEX IF NOT EXISTS idx_notification_deliveries_tenant_id ON notification_deliveries(tenant_id);
CREATE INDEX IF NOT EXISTS idx_notification_deliveries_status ON notification_deliveries(status);
CREATE INDEX IF NOT EXISTS idx_notification_deliveries_channel ON notification_deliveries(channel);
CREATE INDEX IF NOT EXISTS idx_notification_deliveries_next_retry ON notification_deliveries(next_retry_at) WHERE status = 'retry';
CREATE INDEX IF NOT EXISTS idx_notification_deliveries_created_at ON notification_deliveries(created_at DESC);

-- Composite index for common queries
CREATE INDEX IF NOT EXISTS idx_notification_deliveries_event_channel ON notification_deliveries(event_id, channel);

-- Comments for documentation
COMMENT ON TABLE notification_deliveries IS 'Tracks all notification delivery attempts across different channels';
COMMENT ON COLUMN notification_deliveries.event_id IS 'Unique identifier for the event that triggered this notification';
COMMENT ON COLUMN notification_deliveries.event_type IS 'Type of event (e.g., tenant.created, payment.succeeded)';
COMMENT ON COLUMN notification_deliveries.channel IS 'Notification channel used (discord, slack, email, webhook)';
COMMENT ON COLUMN notification_deliveries.status IS 'Current delivery status (pending, sent, failed, retry)';
COMMENT ON COLUMN notification_deliveries.retry_count IS 'Number of retry attempts made';

-- Optional: Notification Configuration Table (for future per-tenant customization)
CREATE TABLE IF NOT EXISTS notification_config (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID REFERENCES tenants(id) ON DELETE CASCADE NOT NULL,

    -- Channel configuration
    channel VARCHAR(50) NOT NULL,
    enabled BOOLEAN DEFAULT true,
    destination TEXT,  -- Channel-specific destination (webhook URL, email, etc.)

    -- Event filtering
    event_types TEXT[],  -- NULL means all events, otherwise specific event types

    -- Custom settings (channel-specific JSON configuration)
    settings JSONB DEFAULT '{}'::jsonb,

    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,

    -- Ensure one config per tenant per channel
    UNIQUE(tenant_id, channel)
);

-- Indexes for notification_config
CREATE INDEX IF NOT EXISTS idx_notification_config_tenant_id ON notification_config(tenant_id);
CREATE INDEX IF NOT EXISTS idx_notification_config_enabled ON notification_config(enabled) WHERE enabled = true;

-- Comments
COMMENT ON TABLE notification_config IS 'Per-tenant notification channel configuration and preferences';
COMMENT ON COLUMN notification_config.event_types IS 'Array of event types to notify for (NULL = all events)';
COMMENT ON COLUMN notification_config.settings IS 'Channel-specific settings in JSON format';

-- Function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_notification_config_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger to automatically update updated_at
CREATE TRIGGER trigger_notification_config_updated_at
    BEFORE UPDATE ON notification_config
    FOR EACH ROW
    EXECUTE FUNCTION update_notification_config_updated_at();
