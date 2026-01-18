-- Migration 019: Add review_findings table for storing structured review output
-- Stores extracted ReviewFindings (round summary, issues) between review rounds

CREATE TABLE IF NOT EXISTS review_findings (
    task_id TEXT NOT NULL,
    review_round INTEGER NOT NULL,
    summary TEXT NOT NULL,
    issues_json TEXT,       -- JSON array of ReviewFinding structs
    questions_json TEXT,    -- JSON array of strings
    positives_json TEXT,    -- JSON array of strings
    perspective TEXT,       -- Which reviewer perspective produced these
    created_at TEXT DEFAULT (datetime('now')),
    PRIMARY KEY (task_id, review_round),
    FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_review_findings_task ON review_findings(task_id);
