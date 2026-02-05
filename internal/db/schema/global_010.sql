-- Global database migration 010: Add users table and user_id to cost_log
-- Users table stores team members with unique names.
-- All user attribution columns in other tables reference users.id.

CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    name TEXT UNIQUE NOT NULL,
    email TEXT,
    created_at TEXT DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_users_name ON users(name);

-- Add user_id column to cost_log for cost attribution
ALTER TABLE cost_log ADD COLUMN user_id TEXT;

CREATE INDEX IF NOT EXISTS idx_cost_log_user ON cost_log(user_id);
CREATE INDEX IF NOT EXISTS idx_cost_log_project_user ON cost_log(project_id, user_id);
