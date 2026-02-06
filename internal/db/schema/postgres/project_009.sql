-- Task queue and priority fields
-- Enables backlog/active separation and priority-based ordering

-- Add queue column (active vs backlog)
ALTER TABLE tasks ADD COLUMN queue TEXT DEFAULT 'active';

-- Add priority column
ALTER TABLE tasks ADD COLUMN priority TEXT DEFAULT 'normal';

-- Index for efficient queue filtering
CREATE INDEX IF NOT EXISTS idx_tasks_queue ON tasks(queue);

-- Index for priority sorting within queue
CREATE INDEX IF NOT EXISTS idx_tasks_queue_priority ON tasks(queue, priority);
