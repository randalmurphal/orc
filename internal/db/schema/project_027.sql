-- Add covering index for event_log pagination performance
-- The ORDER BY clause uses (created_at DESC, id DESC), so we need a composite index
CREATE INDEX IF NOT EXISTS idx_event_log_created_id ON event_log(created_at DESC, id DESC);

-- Remove redundant index on phase_artifacts
-- idx_phase_artifacts_task(task_id) is covered by idx_phase_artifacts_task_phase(task_id, phase_id)
-- SQLite can use a composite index for queries that only filter on the first column
DROP INDEX IF EXISTS idx_phase_artifacts_task;
