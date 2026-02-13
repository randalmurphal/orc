-- Migration 063: Add phase_scratchpad table for persistent note-taking (TASK-020)
--
-- Stores structured observations, decisions, and blockers that agents produce
-- during phase execution. Entries survive retries and propagate to downstream
-- phases via PREV_SCRATCHPAD and RETRY_SCRATCHPAD template variables.

CREATE TABLE IF NOT EXISTS phase_scratchpad (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id TEXT NOT NULL,
    phase_id TEXT NOT NULL,
    category TEXT NOT NULL,
    content TEXT NOT NULL,
    attempt INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_phase_scratchpad_task_phase
    ON phase_scratchpad (task_id, phase_id);
