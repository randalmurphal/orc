-- Project database migration 058: Add atomic user claim fields and claim history table
-- Replaces the old team-based claim system (team_members + task_claims tables) with
-- direct user claim fields on the tasks table and an append-only history table.

-- Add user claim columns to tasks table
ALTER TABLE tasks ADD COLUMN claimed_by TEXT;
ALTER TABLE tasks ADD COLUMN claimed_at TEXT;

-- Create append-only claim history table
CREATE TABLE IF NOT EXISTS task_claim_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    claimed_at TEXT NOT NULL,
    released_at TEXT,
    stolen_from TEXT,
    FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE
);

-- Index for efficient lookup by task_id
CREATE INDEX IF NOT EXISTS idx_task_claim_history_task_id ON task_claim_history(task_id);
