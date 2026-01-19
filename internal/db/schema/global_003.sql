-- Global database migration 003: Add duration tracking to cost_log
-- Enables timing analytics for phase execution

-- Add duration_ms column for phase execution timing
ALTER TABLE cost_log ADD COLUMN duration_ms INTEGER DEFAULT 0;

-- Index for duration-based queries (e.g., slow phase detection)
CREATE INDEX IF NOT EXISTS idx_cost_duration ON cost_log(duration_ms);
