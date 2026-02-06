-- Dashboard performance indexes (TASK-531)
-- Speeds up time-filtered dashboard queries by avoiding full table scans.
CREATE INDEX IF NOT EXISTS idx_tasks_completed_at ON tasks(completed_at);
CREATE INDEX IF NOT EXISTS idx_tasks_updated_at ON tasks(updated_at);
