-- Migration 017: Add updated_at column to tasks
-- Tracks when a task was last modified. Used by frontend for "Recent Activity" display.

-- Add the column (SQLite requires literal default for ALTER TABLE)
ALTER TABLE tasks ADD COLUMN updated_at TEXT;

-- Backfill existing tasks: use the most recent of completed_at, started_at, or created_at
UPDATE tasks SET updated_at = COALESCE(completed_at, started_at, created_at);

-- Future inserts will set updated_at via application code or trigger
CREATE TRIGGER IF NOT EXISTS tasks_updated_at AFTER UPDATE ON tasks
WHEN NEW.updated_at = OLD.updated_at OR NEW.updated_at IS NULL
BEGIN
    UPDATE tasks SET updated_at = datetime('now') WHERE id = NEW.id;
END;
