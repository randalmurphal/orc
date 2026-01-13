-- Task comments/notes system
-- General discussion about tasks - human feedback, Claude's notes, context preservation
-- Distinct from review_comments which are code-specific (file paths, line numbers, severity)

CREATE TABLE IF NOT EXISTS task_comments (
    id TEXT PRIMARY KEY,
    task_id TEXT NOT NULL,
    author TEXT NOT NULL,                 -- e.g., 'claude', 'human', specific username
    author_type TEXT DEFAULT 'human',     -- 'human', 'agent', 'system'
    content TEXT NOT NULL,
    phase TEXT,                           -- optional: which phase the comment relates to
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now')),
    FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE
);

-- Indexes for efficient queries
CREATE INDEX IF NOT EXISTS idx_task_comments_task ON task_comments(task_id);
CREATE INDEX IF NOT EXISTS idx_task_comments_created ON task_comments(task_id, created_at);
CREATE INDEX IF NOT EXISTS idx_task_comments_author_type ON task_comments(author_type);
