-- Project database schema: .orc/orc.db
-- Stores tasks, phases, transcripts for a single project

-- Project detection results (cached)
CREATE TABLE IF NOT EXISTS detection (
    id INTEGER PRIMARY KEY,
    language TEXT,
    frameworks TEXT,  -- JSON array
    build_tools TEXT, -- JSON array
    has_tests INTEGER DEFAULT 0,
    test_command TEXT,
    lint_command TEXT,
    detected_at TEXT DEFAULT (datetime('now'))
);

-- Tasks
CREATE TABLE IF NOT EXISTS tasks (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    description TEXT,
    weight TEXT,
    status TEXT,
    current_phase TEXT,
    branch TEXT,
    worktree_path TEXT,
    created_at TEXT DEFAULT (datetime('now')),
    started_at TEXT,
    completed_at TEXT,
    total_cost_usd REAL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
CREATE INDEX IF NOT EXISTS idx_tasks_created ON tasks(created_at DESC);

-- Phase execution state
CREATE TABLE IF NOT EXISTS phases (
    task_id TEXT NOT NULL,
    phase_id TEXT NOT NULL,
    status TEXT DEFAULT 'pending',
    iterations INTEGER DEFAULT 0,
    started_at TEXT,
    completed_at TEXT,
    input_tokens INTEGER DEFAULT 0,
    output_tokens INTEGER DEFAULT 0,
    cost_usd REAL DEFAULT 0,
    error_message TEXT,
    PRIMARY KEY (task_id, phase_id),
    FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE
);

-- Transcripts (Claude conversation logs)
CREATE TABLE IF NOT EXISTS transcripts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id TEXT NOT NULL,
    phase TEXT NOT NULL,
    iteration INTEGER DEFAULT 1,
    role TEXT,  -- 'user', 'assistant', 'system'
    content TEXT,
    timestamp TEXT DEFAULT (datetime('now')),
    FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_transcripts_task ON transcripts(task_id, phase);

-- Full-text search on transcript content
CREATE VIRTUAL TABLE IF NOT EXISTS transcripts_fts USING fts5(
    content,
    task_id UNINDEXED,
    phase UNINDEXED,
    content=transcripts,
    content_rowid=id
);

-- Triggers to keep FTS in sync
CREATE TRIGGER IF NOT EXISTS transcripts_ai AFTER INSERT ON transcripts BEGIN
    INSERT INTO transcripts_fts(rowid, content, task_id, phase)
    VALUES (NEW.id, NEW.content, NEW.task_id, NEW.phase);
END;

CREATE TRIGGER IF NOT EXISTS transcripts_ad AFTER DELETE ON transcripts BEGIN
    INSERT INTO transcripts_fts(transcripts_fts, rowid, content, task_id, phase)
    VALUES('delete', OLD.id, OLD.content, OLD.task_id, OLD.phase);
END;

CREATE TRIGGER IF NOT EXISTS transcripts_au AFTER UPDATE ON transcripts BEGIN
    INSERT INTO transcripts_fts(transcripts_fts, rowid, content, task_id, phase)
    VALUES('delete', OLD.id, OLD.content, OLD.task_id, OLD.phase);
    INSERT INTO transcripts_fts(rowid, content, task_id, phase)
    VALUES (NEW.id, NEW.content, NEW.task_id, NEW.phase);
END;
