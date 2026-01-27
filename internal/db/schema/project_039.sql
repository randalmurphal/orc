-- Migration 039: Add unique index to prevent duplicate events
-- Events with the same (task_id, event_type, phase, created_at) are duplicates.
-- The INSERT OR IGNORE in SaveEvent/SaveEvents relies on this constraint.

-- Create unique index for deduplication
-- COALESCE handles NULL phase values (task-level events)
CREATE UNIQUE INDEX IF NOT EXISTS idx_event_log_dedup
ON event_log(task_id, event_type, COALESCE(phase, ''), created_at);
