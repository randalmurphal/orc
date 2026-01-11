-- Review comments for code review UI
-- Supports inline comments on diffs with severity levels

CREATE TABLE IF NOT EXISTS review_comments (
    id TEXT PRIMARY KEY,
    task_id TEXT NOT NULL,
    review_round INTEGER NOT NULL DEFAULT 1,
    file_path TEXT,
    line_number INTEGER,
    content TEXT NOT NULL,
    severity TEXT DEFAULT 'suggestion',  -- suggestion, issue, blocker
    status TEXT DEFAULT 'open',          -- open, resolved, wont_fix
    created_at TEXT DEFAULT (datetime('now')),
    resolved_at TEXT,
    resolved_by TEXT,
    FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_review_comments_task ON review_comments(task_id);
CREATE INDEX IF NOT EXISTS idx_review_comments_status ON review_comments(status);
CREATE INDEX IF NOT EXISTS idx_review_comments_file ON review_comments(task_id, file_path);
