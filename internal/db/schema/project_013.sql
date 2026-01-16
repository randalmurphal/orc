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

-- ============================================================================
-- AUTOMATION TRIGGERS AND NOTIFICATIONS SYSTEM
-- Enables automated maintenance tasks based on configurable conditions
-- ============================================================================

-- Trigger definitions (loaded from config, but state persisted in DB)
CREATE TABLE IF NOT EXISTS automation_triggers (
    id TEXT PRIMARY KEY,
    type TEXT NOT NULL,                     -- 'count', 'initiative', 'event', 'threshold', 'schedule'
    description TEXT,
    enabled INTEGER DEFAULT 1,
    config TEXT NOT NULL,                   -- JSON: full trigger configuration
    last_triggered_at TEXT,
    trigger_count INTEGER DEFAULT 0,        -- Times this trigger has fired
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_automation_triggers_type ON automation_triggers(type);
CREATE INDEX IF NOT EXISTS idx_automation_triggers_enabled ON automation_triggers(enabled);

-- Trigger execution history
CREATE TABLE IF NOT EXISTS trigger_executions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    trigger_id TEXT NOT NULL,
    task_id TEXT,                           -- AUTO-XXX task created (nullable if skipped)
    triggered_at TEXT DEFAULT (datetime('now')),
    trigger_reason TEXT,                    -- Why it fired (e.g., "5 tasks completed")
    status TEXT DEFAULT 'pending',          -- pending, running, completed, failed, skipped
    completed_at TEXT,
    error_message TEXT,
    FOREIGN KEY (trigger_id) REFERENCES automation_triggers(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_trigger_executions_trigger ON trigger_executions(trigger_id);
CREATE INDEX IF NOT EXISTS idx_trigger_executions_status ON trigger_executions(status);
CREATE INDEX IF NOT EXISTS idx_trigger_executions_task ON trigger_executions(task_id);

-- Counters for count-based triggers (task count since last trigger)
CREATE TABLE IF NOT EXISTS trigger_counters (
    trigger_id TEXT NOT NULL,
    metric TEXT NOT NULL,                   -- 'tasks_completed', 'large_tasks_completed', 'phases_completed'
    count INTEGER DEFAULT 0,
    last_reset_at TEXT DEFAULT (datetime('now')),
    PRIMARY KEY (trigger_id, metric),
    FOREIGN KEY (trigger_id) REFERENCES automation_triggers(id) ON DELETE CASCADE
);

-- Metrics for threshold-based triggers
CREATE TABLE IF NOT EXISTS trigger_metrics (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    metric TEXT NOT NULL,                   -- 'test_coverage', 'doc_coverage', etc.
    value REAL NOT NULL,
    task_id TEXT,                           -- Source task if applicable
    recorded_at TEXT DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_trigger_metrics_metric ON trigger_metrics(metric);
CREATE INDEX IF NOT EXISTS idx_trigger_metrics_recorded ON trigger_metrics(recorded_at);

-- ============================================================================
-- NOTIFICATIONS
-- ============================================================================

-- General notification system (starts with automation, extensible)
CREATE TABLE IF NOT EXISTS notifications (
    id TEXT PRIMARY KEY,
    type TEXT NOT NULL,                     -- 'automation_pending', 'automation_failed', 'automation_blocked'
    title TEXT NOT NULL,
    message TEXT,
    source_type TEXT,                       -- 'trigger', 'task', etc.
    source_id TEXT,                         -- trigger_id, task_id, etc.
    dismissed INTEGER DEFAULT 0,
    created_at TEXT DEFAULT (datetime('now')),
    expires_at TEXT                         -- Auto-expire after period (nullable)
);

CREATE INDEX IF NOT EXISTS idx_notifications_type ON notifications(type);
CREATE INDEX IF NOT EXISTS idx_notifications_dismissed ON notifications(dismissed);
CREATE INDEX IF NOT EXISTS idx_notifications_source ON notifications(source_type, source_id);
CREATE INDEX IF NOT EXISTS idx_notifications_expires ON notifications(expires_at);

-- ============================================================================
-- AUTOMATION TASK TRACKING
-- ============================================================================

-- Add automation flag to tasks table (to identify AUTO-XXX tasks)
ALTER TABLE tasks ADD COLUMN is_automation INTEGER DEFAULT 0;

-- Add trigger reference to tasks (which trigger created this task)
ALTER TABLE tasks ADD COLUMN trigger_id TEXT;

CREATE INDEX IF NOT EXISTS idx_tasks_automation ON tasks(is_automation);
CREATE INDEX IF NOT EXISTS idx_tasks_trigger ON tasks(trigger_id);
-- Composite index for efficient automation task queries by status
CREATE INDEX IF NOT EXISTS idx_tasks_automation_status ON tasks(is_automation, status);
