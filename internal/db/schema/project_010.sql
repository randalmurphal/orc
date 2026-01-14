-- Task category field
-- Enables categorization of tasks by type (feature, bug, refactor, etc.)

-- Add category column
ALTER TABLE tasks ADD COLUMN category TEXT DEFAULT 'feature';

-- Index for category filtering
CREATE INDEX IF NOT EXISTS idx_tasks_category ON tasks(category);
