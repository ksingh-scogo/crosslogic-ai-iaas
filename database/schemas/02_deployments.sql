-- Deployments and Node Management Schema
-- Version: 1.1.0
-- Description: Support for deployment abstraction and 1:1 cluster-node architecture

-- ============================================================================
-- DEPLOYMENTS TABLE
-- ============================================================================
-- Represents a managed set of GPU nodes serving a specific model
CREATE TABLE IF NOT EXISTS deployments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) UNIQUE NOT NULL,
    model_name VARCHAR(255) NOT NULL,

    -- Replica configuration
    min_replicas INT DEFAULT 2,
    max_replicas INT DEFAULT 10,
    current_replicas INT DEFAULT 0,

    -- Placement strategy
    strategy VARCHAR(50) DEFAULT 'spread' CHECK (strategy IN ('spread', 'packed')),
    provider VARCHAR(50),  -- Optional: force specific provider
    region VARCHAR(50),    -- Optional: force specific region
    gpu_type VARCHAR(100), -- Optional: force specific GPU type (or 'auto')

    -- Status tracking
    status VARCHAR(50) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'paused', 'deleted')),

    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_deployments_name ON deployments(name);
CREATE INDEX IF NOT EXISTS idx_deployments_model_name ON deployments(model_name);
CREATE INDEX IF NOT EXISTS idx_deployments_status ON deployments(status);

COMMENT ON TABLE deployments IS 'Managed deployments with auto-scaling for LLM models';
COMMENT ON COLUMN deployments.strategy IS 'Placement strategy: spread (multi-region) or packed (same region)';
COMMENT ON COLUMN deployments.gpu_type IS 'GPU type or "auto" for automatic selection based on model size';

-- ============================================================================
-- ALTER NODES TABLE FOR 1:1 CLUSTER-NODE ARCHITECTURE
-- ============================================================================

-- Add cluster_name column (unique cluster identifier from SkyPilot)
ALTER TABLE nodes ADD COLUMN IF NOT EXISTS cluster_name VARCHAR(255) UNIQUE;
CREATE INDEX IF NOT EXISTS idx_nodes_cluster_name ON nodes(cluster_name);

-- Add model_name column (denormalized for faster queries)
ALTER TABLE nodes ADD COLUMN IF NOT EXISTS model_name VARCHAR(255);
CREATE INDEX IF NOT EXISTS idx_nodes_model_name ON nodes(model_name);

-- Add deployment_id to link nodes to deployments
ALTER TABLE nodes ADD COLUMN IF NOT EXISTS deployment_id UUID REFERENCES deployments(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_nodes_deployment_id ON nodes(deployment_id);

-- Add endpoint column if it doesn't exist (some schemas may have endpoint_url instead)
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                   WHERE table_name = 'nodes' AND column_name = 'endpoint') THEN
        ALTER TABLE nodes ADD COLUMN endpoint VARCHAR(500);
    END IF;
END $$;

COMMENT ON COLUMN nodes.cluster_name IS 'SkyPilot cluster name (format: cic-{provider}-{region}-{gpu}-{spot|od}-{id})';
COMMENT ON COLUMN nodes.model_name IS 'Model being served on this node (denormalized from deployments)';
COMMENT ON COLUMN nodes.deployment_id IS 'Link to parent deployment for managed nodes (NULL for standalone nodes)';

-- ============================================================================
-- DEPLOYMENT_NODES JUNCTION TABLE (Optional - for future use)
-- ============================================================================
-- This provides a clear many-to-many relationship tracking
-- Currently we use deployment_id on nodes table, but this gives us more flexibility
CREATE TABLE IF NOT EXISTS deployment_nodes (
    deployment_id UUID NOT NULL REFERENCES deployments(id) ON DELETE CASCADE,
    node_id UUID NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    status VARCHAR(50) DEFAULT 'healthy' CHECK (status IN ('healthy', 'unhealthy', 'draining')),
    joined_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (deployment_id, node_id)
);

CREATE INDEX IF NOT EXISTS idx_deployment_nodes_deployment_id ON deployment_nodes(deployment_id);
CREATE INDEX IF NOT EXISTS idx_deployment_nodes_node_id ON deployment_nodes(node_id);
CREATE INDEX IF NOT EXISTS idx_deployment_nodes_status ON deployment_nodes(status);

COMMENT ON TABLE deployment_nodes IS 'Junction table tracking node membership in deployments';

-- ============================================================================
-- TRIGGER FOR DEPLOYMENTS updated_at
-- ============================================================================

CREATE TRIGGER update_deployments_updated_at BEFORE UPDATE ON deployments
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- ============================================================================
-- SAMPLE DATA FOR TESTING
-- ============================================================================

-- Example deployment: Llama 3 70B with auto-scaling
INSERT INTO deployments (name, model_name, min_replicas, max_replicas, strategy, gpu_type)
VALUES ('llama-3-70b-prod', 'meta-llama/Llama-3-70b-instruct', 2, 8, 'spread', 'auto')
ON CONFLICT (name) DO NOTHING;

-- Example deployment: Mistral 7B with fixed GPU type
INSERT INTO deployments (name, model_name, min_replicas, max_replicas, strategy, provider, region, gpu_type)
VALUES ('mistral-7b-us-east', 'mistralai/Mistral-7B-Instruct-v0.2', 2, 5, 'packed', 'aws', 'us-east-1', 'A10G')
ON CONFLICT (name) DO NOTHING;
