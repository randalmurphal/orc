-- Migration: Multi-language detection, scoped commands, and flexible phase gates
-- Supports polyglot projects (Go + TypeScript, Python + JavaScript, etc.)

--------------------------------------------------------------------------------
-- PROJECT LANGUAGES: Multi-language detection support
-- Replaces single 'language' field in detection table
--------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS project_languages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    language TEXT NOT NULL,              -- go, typescript, python, javascript, rust, etc.
    root_path TEXT NOT NULL DEFAULT '',  -- Relative path: '' = project root, 'web/' = subdir
    is_primary INTEGER NOT NULL DEFAULT 0,  -- User-designated primary language (1 = true)
    frameworks TEXT,                     -- JSON array of detected frameworks
    build_tool TEXT,                     -- npm, yarn, pnpm, bun, poetry, cargo, make
    test_command TEXT,                   -- Inferred test command for this language
    lint_command TEXT,                   -- Inferred lint command for this language
    build_command TEXT,                  -- Inferred build command for this language
    detected_at TEXT DEFAULT (datetime('now')),
    UNIQUE(language, root_path)          -- Same language can exist at different paths
);

CREATE INDEX IF NOT EXISTS idx_project_languages_language ON project_languages(language);
CREATE INDEX IF NOT EXISTS idx_project_languages_primary ON project_languages(is_primary) WHERE is_primary = 1;

--------------------------------------------------------------------------------
-- PROJECT COMMANDS: Migrate to support scope for language/stack-specific commands
-- Examples: tests:go, tests:frontend, lint:python, lint:frontend
--
-- SQLite can't add columns to primary keys, so we migrate the table
--------------------------------------------------------------------------------

-- Create new table with scope support
CREATE TABLE IF NOT EXISTS project_commands_new (
    name TEXT NOT NULL,                  -- 'tests', 'lint', 'build', 'typecheck', or custom
    scope TEXT NOT NULL DEFAULT '',      -- '', 'go', 'frontend', 'python', etc.
    domain TEXT NOT NULL DEFAULT 'code', -- 'code', 'custom'
    command TEXT NOT NULL,               -- Full command: 'go test ./...'
    short_command TEXT,                  -- Optional short variant: 'go test -short ./...'
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    description TEXT,
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now')),
    PRIMARY KEY (name, scope)
);

-- Copy existing data (all existing commands become global scope='')
INSERT OR IGNORE INTO project_commands_new (name, scope, domain, command, short_command, enabled, description, created_at, updated_at)
SELECT name, '', domain, command, short_command, enabled, description, created_at, updated_at
FROM project_commands;

-- Drop old table and rename new one
DROP TABLE IF EXISTS project_commands;
ALTER TABLE project_commands_new RENAME TO project_commands;

-- Recreate indexes on new table
CREATE INDEX IF NOT EXISTS idx_project_commands_domain ON project_commands(domain);
CREATE INDEX IF NOT EXISTS idx_project_commands_enabled ON project_commands(enabled);
CREATE INDEX IF NOT EXISTS idx_project_commands_scope ON project_commands(name, scope);

--------------------------------------------------------------------------------
-- PHASE GATES: Per-phase gate configuration (supplements config.yaml)
-- Allows database-driven gate overrides without editing config files
--------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS phase_gates (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    phase_id TEXT NOT NULL UNIQUE,       -- Phase identifier (spec, implement, test, review, etc.)
    gate_type TEXT NOT NULL,             -- auto, human, ai, skip
    criteria TEXT,                       -- JSON array of criteria for auto gates
    enabled INTEGER NOT NULL DEFAULT 1,  -- Whether gate is active
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_phase_gates_enabled ON phase_gates(enabled) WHERE enabled = 1;

--------------------------------------------------------------------------------
-- TASK GATE OVERRIDES: Per-task gate configuration
-- Takes precedence over phase_gates and config.yaml
--------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS task_gate_overrides (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id TEXT NOT NULL,
    phase_id TEXT NOT NULL,
    gate_type TEXT NOT NULL,             -- auto, human, ai, skip
    created_at TEXT DEFAULT (datetime('now')),
    UNIQUE(task_id, phase_id),
    FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_task_gate_overrides_task ON task_gate_overrides(task_id);
