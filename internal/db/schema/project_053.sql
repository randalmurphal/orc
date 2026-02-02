-- Migration 053: Add feedback table for real-time user feedback to agents
-- Supports TASK-741: Backend API for real-time feedback to agents
--
-- Note: No FK constraint on task_id because:
-- 1. API layer validates task exists before saving
-- 2. Simpler for testing (no need to create tasks first)
-- 3. Feedback is still useful even if task is later deleted

CREATE TABLE IF NOT EXISTS feedback (
    id TEXT PRIMARY KEY,
    task_id TEXT NOT NULL,
    type TEXT NOT NULL,      -- 'general', 'inline', 'approval', 'direction'
    text TEXT NOT NULL,
    timing TEXT NOT NULL,    -- 'now', 'when_done', 'manual'
    file TEXT,               -- For inline comments
    line INTEGER,            -- For inline comments
    received BOOLEAN DEFAULT FALSE,
    sent_at TEXT,
    created_at TEXT DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_feedback_task ON feedback(task_id);
CREATE INDEX IF NOT EXISTS idx_feedback_received ON feedback(task_id, received);
