-- Migration 041: Add PostgreSQL full-text search for transcripts
-- Replaces ILIKE fallback with native tsvector/tsquery FTS
-- Adds GIN index for fast search, trigger for auto-indexing new rows

-- Add tsvector column for full-text search
ALTER TABLE transcripts ADD COLUMN IF NOT EXISTS search_vector tsvector;

-- Backfill existing rows with tsvector data
UPDATE transcripts
SET search_vector = to_tsvector('english', COALESCE(content, ''))
WHERE search_vector IS NULL;

-- Create GIN index for fast full-text search
CREATE INDEX IF NOT EXISTS idx_transcripts_fts ON transcripts USING GIN (search_vector);

-- Create trigger function to auto-update search_vector on INSERT/UPDATE
CREATE OR REPLACE FUNCTION transcripts_search_vector_update() RETURNS trigger AS $$
BEGIN
    NEW.search_vector := to_tsvector('english', COALESCE(NEW.content, ''));
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create trigger (drop first for idempotency, then create)
DROP TRIGGER IF EXISTS trg_transcripts_search_vector ON transcripts;
CREATE TRIGGER trg_transcripts_search_vector
    BEFORE INSERT OR UPDATE OF content ON transcripts
    FOR EACH ROW
    EXECUTE FUNCTION transcripts_search_vector_update();

--------------------------------------------------------------------------------
-- Multi-language detection, scoped commands, and flexible phase gates
-- Supports polyglot projects (Go + TypeScript, Python + JavaScript, etc.)
--------------------------------------------------------------------------------

--------------------------------------------------------------------------------
-- PROJECT LANGUAGES: Multi-language detection support
-- Replaces single 'language' field in detection table
--------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS project_languages (
    id SERIAL PRIMARY KEY,
    language TEXT NOT NULL,              -- go, typescript, python, javascript, rust, etc.
    root_path TEXT NOT NULL DEFAULT '',  -- Relative path: '' = project root, 'web/' = subdir
    is_primary BOOLEAN NOT NULL DEFAULT FALSE,  -- User-designated primary language
    frameworks TEXT,                     -- JSON array of detected frameworks
    build_tool TEXT,                     -- npm, yarn, pnpm, bun, poetry, cargo, make
    test_command TEXT,                   -- Inferred test command for this language
    lint_command TEXT,                   -- Inferred lint command for this language
    build_command TEXT,                  -- Inferred build command for this language
    detected_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(language, root_path)          -- Same language can exist at different paths
);

CREATE INDEX IF NOT EXISTS idx_project_languages_language ON project_languages(language);
CREATE INDEX IF NOT EXISTS idx_project_languages_primary ON project_languages(is_primary) WHERE is_primary = TRUE;

--------------------------------------------------------------------------------
-- PROJECT COMMANDS: Add scope column and change primary key
-- Examples: tests:go, tests:frontend, lint:python, lint:frontend
--
-- PostgreSQL supports ALTER TABLE for PK changes (no table recreation needed)
--------------------------------------------------------------------------------

-- Add scope column
ALTER TABLE project_commands ADD COLUMN scope TEXT NOT NULL DEFAULT '';

-- Change primary key from (name) to (name, scope)
ALTER TABLE project_commands DROP CONSTRAINT project_commands_pkey;
ALTER TABLE project_commands ADD PRIMARY KEY (name, scope);

-- Recreate indexes (idempotent)
CREATE INDEX IF NOT EXISTS idx_project_commands_domain ON project_commands(domain);
CREATE INDEX IF NOT EXISTS idx_project_commands_enabled ON project_commands(enabled);
CREATE INDEX IF NOT EXISTS idx_project_commands_scope ON project_commands(name, scope);

--------------------------------------------------------------------------------
-- PHASE GATES: Per-phase gate configuration (supplements config.yaml)
-- Allows database-driven gate overrides without editing config files
--------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS phase_gates (
    id SERIAL PRIMARY KEY,
    phase_id TEXT NOT NULL UNIQUE,       -- Phase identifier (spec, implement, test, review, etc.)
    gate_type TEXT NOT NULL,             -- auto, human, ai, skip
    criteria TEXT,                       -- JSON array of criteria for auto gates
    enabled BOOLEAN NOT NULL DEFAULT TRUE,  -- Whether gate is active
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_phase_gates_enabled ON phase_gates(enabled) WHERE enabled = TRUE;

--------------------------------------------------------------------------------
-- TASK GATE OVERRIDES: Per-task gate configuration
-- Takes precedence over phase_gates and config.yaml
--------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS task_gate_overrides (
    id SERIAL PRIMARY KEY,
    task_id TEXT NOT NULL,
    phase_id TEXT NOT NULL,
    gate_type TEXT NOT NULL,             -- auto, human, ai, skip
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(task_id, phase_id),
    FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_task_gate_overrides_task ON task_gate_overrides(task_id);
