-- Constitution tables for project-level principles and spec validation
-- Part of the spec-kit TDD-first workflow integration

-- Constitution: project-level principles that guide all task execution
-- Singleton pattern enforced by CHECK constraint (only id=1 allowed)
CREATE TABLE IF NOT EXISTS constitutions (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    content TEXT NOT NULL,
    version TEXT NOT NULL DEFAULT '1.0.0',
    content_hash TEXT,
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);

-- Constitution checks: tracks validation of spec against constitution
-- Records whether a spec phase output complies with project principles
CREATE TABLE IF NOT EXISTS constitution_checks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id TEXT NOT NULL,
    phase TEXT NOT NULL,
    passed INTEGER NOT NULL DEFAULT 0,
    violations TEXT,  -- JSON array of violation descriptions
    checked_at TEXT DEFAULT (datetime('now')),
    FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE
);

-- Index for querying checks by task
CREATE INDEX IF NOT EXISTS idx_constitution_checks_task ON constitution_checks(task_id);

-- Add session_id column to phases table
-- Stores Claude CLI session UUID for --resume support per phase
ALTER TABLE phases ADD COLUMN session_id TEXT;
