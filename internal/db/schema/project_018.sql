-- Migration 018: Fix decision ID collision across initiatives
-- Changes initiative_decisions to use composite primary key (id, initiative_id)
-- instead of just id, allowing each initiative to have its own DEC-001, etc.

-- Create new table with composite primary key
CREATE TABLE initiative_decisions_new (
    id TEXT NOT NULL,
    initiative_id TEXT NOT NULL,
    decision TEXT NOT NULL,
    rationale TEXT,
    decided_by TEXT,
    decided_at TEXT DEFAULT (datetime('now')),
    PRIMARY KEY (id, initiative_id),
    FOREIGN KEY (initiative_id) REFERENCES initiatives(id) ON DELETE CASCADE
);

-- Copy existing data
INSERT INTO initiative_decisions_new
SELECT id, initiative_id, decision, rationale, decided_by, decided_at
FROM initiative_decisions;

-- Drop old table
DROP TABLE initiative_decisions;

-- Rename new table
ALTER TABLE initiative_decisions_new RENAME TO initiative_decisions;

-- Recreate index
CREATE INDEX IF NOT EXISTS idx_initiative_decisions_init ON initiative_decisions(initiative_id);
