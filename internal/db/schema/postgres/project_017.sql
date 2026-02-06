-- Migration 017: Add updated_at column to tasks
-- Tracks when a task was last modified. Used by frontend for "Recent Activity" display.

-- Add the column
ALTER TABLE tasks ADD COLUMN updated_at TIMESTAMP WITH TIME ZONE;

-- Backfill existing tasks: use the most recent of completed_at, started_at, or created_at
UPDATE tasks SET updated_at = COALESCE(completed_at, started_at, created_at);

-- PostgreSQL trigger function for auto-updating updated_at
CREATE OR REPLACE FUNCTION tasks_set_updated_at() RETURNS TRIGGER AS $$
BEGIN
    IF NEW.updated_at IS NOT DISTINCT FROM OLD.updated_at OR NEW.updated_at IS NULL THEN
        NEW.updated_at = NOW();
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER tasks_updated_at
    BEFORE UPDATE ON tasks
    FOR EACH ROW
    EXECUTE FUNCTION tasks_set_updated_at();
