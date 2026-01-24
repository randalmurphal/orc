-- Migration 034: Rename perspective column to agent_id in review_findings
-- This reflects the shift from hardcoded perspectives to database-backed agents

-- SQLite doesn't support ALTER TABLE RENAME COLUMN directly in older versions
-- Use the standard pattern: create new table, copy data, drop old, rename new

CREATE TABLE IF NOT EXISTS review_findings_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id TEXT NOT NULL,
    review_round INTEGER NOT NULL,
    summary TEXT,
    issues_json TEXT,
    questions_json TEXT,
    positives_json TEXT,
    agent_id TEXT,  -- Renamed from perspective
    created_at TEXT DEFAULT (datetime('now')),
    UNIQUE(task_id, review_round),
    FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE
);

-- Copy data from old table to new (perspective -> agent_id)
-- Note: Original table used (task_id, review_round) as compound primary key, no id column
INSERT OR IGNORE INTO review_findings_new (task_id, review_round, summary, issues_json, questions_json, positives_json, agent_id, created_at)
SELECT task_id, review_round, summary, issues_json, questions_json, positives_json, perspective, created_at
FROM review_findings;

-- Drop old table
DROP TABLE IF EXISTS review_findings;

-- Rename new table
ALTER TABLE review_findings_new RENAME TO review_findings;

-- Recreate index
CREATE INDEX IF NOT EXISTS idx_review_findings_task ON review_findings(task_id);
