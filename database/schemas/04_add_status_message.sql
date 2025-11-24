-- Add status_message column to nodes table
-- This column stores additional context about node status changes

ALTER TABLE nodes ADD COLUMN IF NOT EXISTS status_message VARCHAR(500);

CREATE INDEX IF NOT EXISTS idx_nodes_status_message ON nodes(status_message);

COMMENT ON COLUMN nodes.status_message IS 'Additional context about node status (e.g., spot_termination_warning, ghost_detected_by_reconciler, graceful_drain_initiated)';
