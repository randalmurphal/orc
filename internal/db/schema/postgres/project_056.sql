-- Migration 056: Add initiative_notes table for knowledge sharing
-- Stores notes (patterns, warnings, learnings, handoffs) at initiative level.
-- Human notes always inject; agent notes must meet strict bar.

CREATE TABLE IF NOT EXISTS initiative_notes (
    id TEXT PRIMARY KEY,
    initiative_id TEXT NOT NULL,

    -- Author
    author TEXT NOT NULL,
    author_type TEXT NOT NULL,      -- 'human' | 'agent'
    source_task TEXT,               -- TASK-001 (if agent-generated)
    source_phase TEXT,              -- 'docs' (if agent-generated)

    -- Content
    note_type TEXT NOT NULL,        -- 'pattern' | 'warning' | 'learning' | 'handoff'
    content TEXT NOT NULL,
    relevant_files TEXT,            -- JSON array, optional

    -- Lifecycle
    graduated INTEGER DEFAULT 0,

    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    FOREIGN KEY (initiative_id) REFERENCES initiatives(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_initiative_notes_initiative ON initiative_notes(initiative_id);
CREATE INDEX IF NOT EXISTS idx_initiative_notes_type ON initiative_notes(initiative_id, note_type);
CREATE INDEX IF NOT EXISTS idx_initiative_notes_source_task ON initiative_notes(source_task);

-- Seed initial value for note sequence (start at 0)
INSERT INTO sequences (name, current_value) VALUES ('note', 0) ON CONFLICT DO NOTHING;
