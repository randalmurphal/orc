-- Global database migration 006: Hook scripts and skills tables
-- Stores reusable hook scripts and skills for phase configuration.

CREATE TABLE IF NOT EXISTS hook_scripts (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    content TEXT NOT NULL,
    event_type TEXT NOT NULL DEFAULT '',
    is_builtin BOOLEAN NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS skills (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    content TEXT NOT NULL,
    supporting_files TEXT,  -- JSON map[string]string, null when empty
    is_builtin BOOLEAN NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
