-- Branch targeting: Add target branch fields to tasks and initiatives
-- Enables per-task and per-initiative branch targeting for feature branches,
-- hotfix branches, and initiative-based isolation.

-- ============================================================================
-- TASK TABLE: Add target_branch column
-- ============================================================================

-- target_branch overrides where this task's PR targets
-- Takes precedence over initiative branch and project config
ALTER TABLE tasks ADD COLUMN target_branch TEXT;

-- ============================================================================
-- INITIATIVES TABLE: Add branch configuration columns
-- ============================================================================

-- branch_base is the target branch for tasks in this initiative
-- When set, tasks in this initiative target this branch instead of project default
ALTER TABLE initiatives ADD COLUMN branch_base TEXT;

-- branch_prefix overrides task branch naming for tasks in this initiative
-- Example: "feature/auth-" creates branches like "feature/auth-TASK-001"
ALTER TABLE initiatives ADD COLUMN branch_prefix TEXT;

-- ============================================================================
-- BRANCHES TABLE: Track orc-managed branches for lifecycle management
-- ============================================================================

CREATE TABLE IF NOT EXISTS branches (
    name TEXT PRIMARY KEY,
    -- Type: 'initiative' (feature branch), 'staging' (personal dev), 'task' (work branch)
    type TEXT NOT NULL,
    -- Owner: initiative ID, developer name, or task ID depending on type
    owner_id TEXT,
    -- Base branch this was created from (e.g., 'main')
    base_branch TEXT,
    -- Status: 'active', 'merged', 'stale', 'orphaned'
    status TEXT DEFAULT 'active',
    -- Tracking timestamps
    created_at TEXT DEFAULT (datetime('now')),
    last_activity TEXT DEFAULT (datetime('now')),
    -- Merge info (when merged)
    merged_at TEXT,
    merged_to TEXT,
    merge_commit_sha TEXT
);

CREATE INDEX IF NOT EXISTS idx_branches_type ON branches(type);
CREATE INDEX IF NOT EXISTS idx_branches_status ON branches(status);
CREATE INDEX IF NOT EXISTS idx_branches_owner ON branches(owner_id);
