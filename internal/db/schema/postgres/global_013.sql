-- Migration 013: Remove dead workflow columns
--
-- workflow_type and default_max_iterations no longer affect runtime behavior.

DROP INDEX IF EXISTS idx_workflows_type;

ALTER TABLE workflows DROP COLUMN IF EXISTS workflow_type;
ALTER TABLE workflows DROP COLUMN IF EXISTS default_max_iterations;
