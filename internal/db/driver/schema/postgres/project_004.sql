-- Task queue, priority, and category fields
-- PostgreSQL version

-- Add queue column (active vs backlog)
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS queue TEXT DEFAULT 'active';

-- Add priority column
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS priority TEXT DEFAULT 'normal';

-- Add category column
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS category TEXT DEFAULT 'feature';

-- Indexes for efficient filtering
CREATE INDEX IF NOT EXISTS idx_tasks_queue ON tasks(queue);
CREATE INDEX IF NOT EXISTS idx_tasks_queue_priority ON tasks(queue, priority);
CREATE INDEX IF NOT EXISTS idx_tasks_category ON tasks(category);
