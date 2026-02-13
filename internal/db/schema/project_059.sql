-- Project database migration 059: Add initiative acceptance criteria table
-- Stores acceptance criteria for initiatives with status tracking and task mapping.

CREATE TABLE IF NOT EXISTS initiative_criteria (
    id TEXT NOT NULL,
    initiative_id TEXT NOT NULL,
    description TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'uncovered',
    task_ids TEXT DEFAULT '[]',
    verified_at TEXT,
    verified_by TEXT,
    evidence TEXT,
    PRIMARY KEY (id, initiative_id),
    FOREIGN KEY (initiative_id) REFERENCES initiatives(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_initiative_criteria_init ON initiative_criteria(initiative_id);
