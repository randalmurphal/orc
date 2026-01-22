-- Phase artifacts table for storing non-spec artifact content directly in the database.
-- Part of the TDD-first workflow where artifacts (design, tdd_write, breakdown, docs)
-- are stored in DB instead of files to avoid worktree merge conflicts.
--
-- Note: Spec artifacts continue to use the existing 'specs' table.
-- This table handles: design, tdd_write, breakdown, research, docs

CREATE TABLE IF NOT EXISTS phase_artifacts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id TEXT NOT NULL,
    phase_id TEXT NOT NULL,  -- design, tdd_write, breakdown, research, docs
    content TEXT NOT NULL,
    content_hash TEXT,
    source TEXT,  -- 'executor', 'import', 'manual'
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now')),
    FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE,
    UNIQUE(task_id, phase_id)
);

-- Index for loading artifact by task and phase
CREATE INDEX IF NOT EXISTS idx_phase_artifacts_task ON phase_artifacts(task_id);
CREATE INDEX IF NOT EXISTS idx_phase_artifacts_task_phase ON phase_artifacts(task_id, phase_id);
