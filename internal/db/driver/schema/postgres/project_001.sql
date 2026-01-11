-- Project database schema: PostgreSQL version
-- Stores tasks, phases, transcripts for a single project

-- Project detection results (cached)
CREATE TABLE IF NOT EXISTS detection (
    id INTEGER PRIMARY KEY DEFAULT 1,
    language TEXT,
    frameworks JSONB,
    build_tools JSONB,
    has_tests BOOLEAN DEFAULT FALSE,
    test_command TEXT,
    lint_command TEXT,
    detected_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Tasks
CREATE TABLE IF NOT EXISTS tasks (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    description TEXT,
    weight TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'created',
    current_phase TEXT,
    branch TEXT,
    worktree_path TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    total_cost_usd DECIMAL(10, 6) DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
CREATE INDEX IF NOT EXISTS idx_tasks_created ON tasks(created_at DESC);

-- Phase execution state
CREATE TABLE IF NOT EXISTS phases (
    task_id TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    phase_id TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    iterations INTEGER DEFAULT 0,
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    input_tokens INTEGER DEFAULT 0,
    output_tokens INTEGER DEFAULT 0,
    cost_usd DECIMAL(10, 6) DEFAULT 0,
    error_message TEXT,
    PRIMARY KEY (task_id, phase_id)
);

-- Transcripts (Claude conversation logs)
CREATE TABLE IF NOT EXISTS transcripts (
    id SERIAL PRIMARY KEY,
    task_id TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    phase TEXT NOT NULL,
    iteration INTEGER NOT NULL DEFAULT 1,
    role TEXT,
    content TEXT NOT NULL,
    timestamp TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_transcripts_task ON transcripts(task_id, phase);

-- Full-text search using tsvector
ALTER TABLE transcripts ADD COLUMN IF NOT EXISTS search_vector tsvector;
CREATE INDEX IF NOT EXISTS idx_transcripts_fts ON transcripts USING gin(search_vector);

-- Function to update search vector
CREATE OR REPLACE FUNCTION update_transcript_search_vector()
RETURNS TRIGGER AS $$
BEGIN
    NEW.search_vector := to_tsvector('english', COALESCE(NEW.content, ''));
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger to update search vector on insert/update
DROP TRIGGER IF EXISTS transcripts_search_trigger ON transcripts;
CREATE TRIGGER transcripts_search_trigger
    BEFORE INSERT OR UPDATE ON transcripts
    FOR EACH ROW EXECUTE FUNCTION update_transcript_search_vector();
