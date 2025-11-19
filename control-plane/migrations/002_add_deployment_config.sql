-- Add configuration fields to deployments table
ALTER TABLE deployments
ADD COLUMN provider VARCHAR(50) DEFAULT 'aws',
ADD COLUMN region VARCHAR(50) DEFAULT 'us-west-2',
ADD COLUMN gpu_type VARCHAR(50) DEFAULT 'A100';
