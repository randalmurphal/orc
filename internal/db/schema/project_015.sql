-- Branch targeting: Add branches table for tracking orc-managed branches
CREATE TABLE IF NOT EXISTS branches (
    name TEXT PRIMARY KEY,         -- Branch name (e.g., "feature/auth", "dev/randy")
    type TEXT NOT NULL,            -- 'initiative' | 'staging' | 'task'
    owner_id TEXT,                 -- INIT-001, TASK-XXX, or developer name
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_activity TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    status TEXT DEFAULT 'active'   -- 'active' | 'merged' | 'stale' | 'orphaned'
);

-- Index for quick lookups by type and status
CREATE INDEX IF NOT EXISTS idx_branches_type ON branches(type);
CREATE INDEX IF NOT EXISTS idx_branches_status ON branches(status);
CREATE INDEX IF NOT EXISTS idx_branches_owner ON branches(owner_id);
