-- Migration: Add tenant self-service features for PRO tier
-- Adds tenant_id to nodes table for tenant-owned instances
-- Adds spot_instance and terminated_at columns for better instance tracking

-- Add tenant_id column to nodes table for tenant ownership
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='nodes' AND column_name='tenant_id') THEN
        ALTER TABLE nodes ADD COLUMN tenant_id UUID REFERENCES tenants(id);
    END IF;
END $$;

-- Add spot_instance column to track spot vs on-demand instances
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='nodes' AND column_name='spot_instance') THEN
        ALTER TABLE nodes ADD COLUMN spot_instance BOOLEAN DEFAULT false;
    END IF;
END $$;

-- Add terminated_at column to track when instances were terminated
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='nodes' AND column_name='terminated_at') THEN
        ALTER TABLE nodes ADD COLUMN terminated_at TIMESTAMP;
    END IF;
END $$;

-- Add index on tenant_id for faster tenant instance queries
CREATE INDEX IF NOT EXISTS idx_nodes_tenant_id ON nodes(tenant_id);

-- Add composite index for tenant instance listing
CREATE INDEX IF NOT EXISTS idx_nodes_tenant_status ON nodes(tenant_id, status) WHERE status != 'deleted';

-- Create cloud_credentials table if it doesn't exist (should already exist from credentials service)
-- This is idempotent and safe to run
CREATE TABLE IF NOT EXISTS cloud_credentials (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    environment_id UUID REFERENCES environments(id),
    provider VARCHAR(50) NOT NULL,
    name VARCHAR(255) NOT NULL,
    credentials_encrypted BYTEA NOT NULL,
    encryption_key_id VARCHAR(255) NOT NULL,
    is_default BOOLEAN DEFAULT false,
    status VARCHAR(50) DEFAULT 'active',
    last_used_at TIMESTAMP,
    last_validated_at TIMESTAMP,
    validation_error TEXT,
    created_by_user_id UUID,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    deleted_at TIMESTAMP,
    CONSTRAINT unique_tenant_provider_name UNIQUE (tenant_id, provider, name)
);

-- Add indexes for cloud_credentials table
CREATE INDEX IF NOT EXISTS idx_cloud_credentials_tenant_id ON cloud_credentials(tenant_id);
CREATE INDEX IF NOT EXISTS idx_cloud_credentials_tenant_provider ON cloud_credentials(tenant_id, provider) WHERE status = 'active';
CREATE INDEX IF NOT EXISTS idx_cloud_credentials_default ON cloud_credentials(tenant_id, provider, is_default) WHERE is_default = true AND status = 'active';
