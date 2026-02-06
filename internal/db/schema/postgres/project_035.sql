-- Migration 035: Add workflow_id to tasks and loop_config to workflow_phases
-- Enables direct workflow assignment on tasks (not just via weight)
-- Enables configurable loop behavior for iterative workflows like QA E2E

-- Add workflow_id to tasks (optional, defaults based on weight when null)
ALTER TABLE tasks ADD COLUMN workflow_id TEXT;

-- Add loop_config to workflow_phases (JSON defining loop behavior)
-- Example: {"condition": "has_findings", "loop_to_phase": "qa_e2e_test", "max_iterations": 3}
ALTER TABLE workflow_phases ADD COLUMN loop_config TEXT;

-- Index for workflow_id filtering
CREATE INDEX IF NOT EXISTS idx_tasks_workflow ON tasks(workflow_id);
