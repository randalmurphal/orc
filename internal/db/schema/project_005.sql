-- Team mode infrastructure tables
-- Supports team member management, task claiming, and activity logging

-- Team members table
CREATE TABLE IF NOT EXISTS team_members (
    id TEXT PRIMARY KEY,
    email TEXT UNIQUE NOT NULL,
    display_name TEXT NOT NULL,
    initials TEXT NOT NULL,
    role TEXT DEFAULT 'member',  -- admin | member | viewer
    created_at TEXT DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_team_members_email ON team_members(email);

-- Task claims table
-- Tracks which team member is working on which task
CREATE TABLE IF NOT EXISTS task_claims (
    task_id TEXT NOT NULL,
    member_id TEXT NOT NULL,
    claimed_at TEXT DEFAULT (datetime('now')),
    released_at TEXT,
    PRIMARY KEY (task_id, member_id, claimed_at),
    FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE,
    FOREIGN KEY (member_id) REFERENCES team_members(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_task_claims_task ON task_claims(task_id);
CREATE INDEX IF NOT EXISTS idx_task_claims_member ON task_claims(member_id);
CREATE INDEX IF NOT EXISTS idx_task_claims_active ON task_claims(task_id, released_at) WHERE released_at IS NULL;

-- Activity log table
-- Records all team activity for audit and visibility
-- Note: SQLite uses INTEGER AUTOINCREMENT, PostgreSQL uses SERIAL
CREATE TABLE IF NOT EXISTS activity_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id TEXT,
    member_id TEXT,
    action TEXT NOT NULL,  -- created | started | paused | completed | failed | commented | claimed | released
    details TEXT,          -- JSON details for additional context
    created_at TEXT DEFAULT (datetime('now')),
    FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE SET NULL,
    FOREIGN KEY (member_id) REFERENCES team_members(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_activity_log_task ON activity_log(task_id);
CREATE INDEX IF NOT EXISTS idx_activity_log_member ON activity_log(member_id);
CREATE INDEX IF NOT EXISTS idx_activity_log_created ON activity_log(created_at);
CREATE INDEX IF NOT EXISTS idx_activity_log_action ON activity_log(action);
