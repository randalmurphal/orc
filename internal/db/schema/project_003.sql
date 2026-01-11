-- Project database schema extension for sub-task queue
-- Adds subtask_queue table for agent-proposed sub-tasks

-- Sub-task queue (proposed sub-tasks awaiting review)
CREATE TABLE IF NOT EXISTS subtask_queue (
    id TEXT PRIMARY KEY,
    parent_task_id TEXT NOT NULL,
    title TEXT NOT NULL,
    description TEXT,
    proposed_by TEXT,           -- Task ID that proposed this
    proposed_at TEXT NOT NULL DEFAULT (datetime('now')),
    status TEXT DEFAULT 'pending',  -- pending, approved, rejected
    approved_by TEXT,
    approved_at TEXT,
    rejected_reason TEXT,
    created_task_id TEXT,       -- Task ID if approved and created
    FOREIGN KEY (parent_task_id) REFERENCES tasks(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_subtask_queue_parent ON subtask_queue(parent_task_id);
CREATE INDEX IF NOT EXISTS idx_subtask_queue_status ON subtask_queue(status);
