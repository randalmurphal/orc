-- Global database schema: ~/.orc/orc.db
-- Stores projects registry, cost tracking, and templates

-- Projects registry
CREATE TABLE IF NOT EXISTS projects (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    path TEXT UNIQUE NOT NULL,
    language TEXT,
    created_at TEXT DEFAULT (datetime('now'))
);

-- Cost tracking across all projects
-- No foreign key on project_id to allow orphan cost entries
CREATE TABLE IF NOT EXISTS cost_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id TEXT,
    task_id TEXT,
    phase TEXT,
    cost_usd REAL,
    input_tokens INTEGER,
    output_tokens INTEGER,
    timestamp TEXT DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_cost_project ON cost_log(project_id);
CREATE INDEX IF NOT EXISTS idx_cost_timestamp ON cost_log(timestamp);

-- User-defined templates
CREATE TABLE IF NOT EXISTS templates (
    name TEXT PRIMARY KEY,
    weight TEXT,
    phases TEXT,  -- JSON array
    created_at TEXT DEFAULT (datetime('now'))
);
