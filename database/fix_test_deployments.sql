-- Fix Test Deployments Script
-- This script pauses the sample test deployments that are causing auto-scaling spam
-- Run this with: psql 'postgresql://crosslogic:cl%40123@localhost:5432/crosslogic_iaas?sslmode=disable' -f database/fix_test_deployments.sql

-- =============================================================================
-- Show current state of deployments
-- =============================================================================
\echo '=== Current Deployment Status ==='
SELECT
    name,
    model_name,
    min_replicas,
    max_replicas,
    current_replicas,
    provider,
    region,
    gpu_type,
    status,
    created_at
FROM deployments
ORDER BY created_at DESC;

-- =============================================================================
-- Pause sample deployments to stop auto-scaling
-- =============================================================================
\echo ''
\echo '=== Pausing Sample Deployments ==='

-- Pause llama-3-70b-prod (has no provider configured)
UPDATE deployments
SET status = 'paused',
    updated_at = NOW()
WHERE name = 'llama-3-70b-prod';

-- Pause mistral-7b-us-east (requires AWS which is not enabled)
UPDATE deployments
SET status = 'paused',
    updated_at = NOW()
WHERE name = 'mistral-7b-us-east';

-- =============================================================================
-- Show updated state
-- =============================================================================
\echo ''
\echo '=== Updated Deployment Status ==='
SELECT
    name,
    model_name,
    min_replicas,
    max_replicas,
    current_replicas,
    provider,
    region,
    gpu_type,
    status,
    updated_at
FROM deployments
ORDER BY created_at DESC;

-- =============================================================================
-- Optional: Delete sample deployments completely (uncomment if needed)
-- =============================================================================
-- \echo ''
-- \echo '=== Deleting Sample Deployments ==='
-- DELETE FROM deployments WHERE name = 'llama-3-70b-prod';
-- DELETE FROM deployments WHERE name = 'mistral-7b-us-east';

\echo ''
\echo '=== Fix Complete ==='
\echo 'Sample deployments have been paused to stop auto-scaling spam.'
\echo 'The deployment controller will now only process deployments with status = active.'
\echo ''
\echo 'To re-enable a deployment:'
\echo '  UPDATE deployments SET status = '\''active'\'' WHERE name = '\''deployment-name'\'';'
\echo ''
\echo 'To completely delete a deployment:'
\echo '  DELETE FROM deployments WHERE name = '\''deployment-name'\'';'
