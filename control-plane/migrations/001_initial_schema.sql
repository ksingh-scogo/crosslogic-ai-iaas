-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Create nodes table if it doesn't exist
CREATE TABLE IF NOT EXISTS nodes (
    id UUID PRIMARY KEY,
    cluster_name VARCHAR(255) UNIQUE,
    provider VARCHAR(50),
    region VARCHAR(50),
    gpu_type VARCHAR(50),
    model_name VARCHAR(255),
    status VARCHAR(50),
    endpoint VARCHAR(255),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Create deployments table
CREATE TABLE IF NOT EXISTS deployments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) UNIQUE,
    model_name VARCHAR(255),
    min_replicas INT DEFAULT 2,
    max_replicas INT DEFAULT 10,
    current_replicas INT DEFAULT 0,
    strategy VARCHAR(50) DEFAULT 'spread',
    created_at TIMESTAMP DEFAULT NOW()
);

-- Add deployment_id to nodes if not exists
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='nodes' AND column_name='deployment_id') THEN
        ALTER TABLE nodes ADD COLUMN deployment_id UUID REFERENCES deployments(id);
    END IF;
END $$;

-- Create deployment_nodes join table (optional, if many-to-many needed, but 1:N is likely sufficient for now as per plan which said 'nodes table: Add deployment_id FK')
-- The plan also showed `deployment_nodes` table in one section but `nodes` table FK in another. 
-- I will stick to `nodes` table FK as it's simpler for 1 node belonging to 1 deployment.
