-- Migration 037: Consolidate status columns
-- The state_status column duplicated task status, causing sync bugs.
-- All code now uses the status column directly.

-- Ensure status column has the correct value for any rows where state_status
-- was the source of truth (e.g., running tasks that got out of sync)
UPDATE tasks
SET status = state_status
WHERE state_status IN ('running', 'failed', 'interrupted')
  AND status NOT IN ('running', 'failed', 'paused', 'blocked');

-- Map 'interrupted' state_status to 'paused' status (closest equivalent)
UPDATE tasks
SET status = 'paused'
WHERE state_status = 'interrupted'
  AND status NOT IN ('paused', 'blocked', 'failed', 'completed', 'resolved');

-- SQLite doesn't support DROP COLUMN easily. The column remains but is ignored.
-- Code stops reading/writing state_status after this migration.
