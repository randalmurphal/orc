-- Migration: Add position fields to workflow_phases, drop UNIQUE(workflow_id, sequence)
-- Supports visual workflow editor with draggable phase nodes and parallel phases

--------------------------------------------------------------------------------
-- WORKFLOW_PHASES: Drop unique constraint on (workflow_id, sequence), add position columns
-- PostgreSQL supports ALTER TABLE for constraint changes (no table recreation needed)
--------------------------------------------------------------------------------

-- Drop the unique constraint that prevented parallel phases at same sequence
ALTER TABLE workflow_phases DROP CONSTRAINT IF EXISTS workflow_phases_workflow_id_sequence_key;

-- Add visual editor position columns (NULL = auto-layout via dagre)
ALTER TABLE workflow_phases ADD COLUMN position_x REAL;
ALTER TABLE workflow_phases ADD COLUMN position_y REAL;

-- Recreate indexes (idempotent, already exist from project_028)
CREATE INDEX IF NOT EXISTS idx_workflow_phases_workflow ON workflow_phases(workflow_id);
CREATE INDEX IF NOT EXISTS idx_workflow_phases_sequence ON workflow_phases(workflow_id, sequence);
