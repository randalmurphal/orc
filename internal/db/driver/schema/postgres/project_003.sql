-- Project database schema extension for sub-task queue and team mode
-- PostgreSQL version

-- Sub-task queue (proposed sub-tasks awaiting review)
CREATE TABLE IF NOT EXISTS subtask_queue (
    id TEXT PRIMARY KEY,
    parent_task_id TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    description TEXT,
    proposed_by TEXT NOT NULL,
    proposed_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    status TEXT NOT NULL DEFAULT 'pending',
    approved_by TEXT,
    approved_at TIMESTAMP WITH TIME ZONE,
    rejected_reason TEXT,
    created_task_id TEXT
);

CREATE INDEX IF NOT EXISTS idx_subtask_queue_parent ON subtask_queue(parent_task_id);
CREATE INDEX IF NOT EXISTS idx_subtask_queue_status ON subtask_queue(status);

-- Team members (for team mode)
CREATE TABLE IF NOT EXISTS team_members (
    id TEXT PRIMARY KEY,
    email TEXT UNIQUE NOT NULL,
    display_name TEXT NOT NULL,
    initials TEXT NOT NULL,
    role TEXT DEFAULT 'member',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Task claims (which team member is working on which task)
CREATE TABLE IF NOT EXISTS task_claims (
    task_id TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    member_id TEXT NOT NULL REFERENCES team_members(id),
    claimed_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    released_at TIMESTAMP WITH TIME ZONE,
    PRIMARY KEY (task_id, member_id, claimed_at)
);

CREATE INDEX IF NOT EXISTS idx_task_claims_member ON task_claims(member_id);
CREATE INDEX IF NOT EXISTS idx_task_claims_active ON task_claims(task_id) WHERE released_at IS NULL;

-- Activity log (audit trail of actions)
CREATE TABLE IF NOT EXISTS activity_log (
    id SERIAL PRIMARY KEY,
    task_id TEXT REFERENCES tasks(id) ON DELETE SET NULL,
    member_id TEXT REFERENCES team_members(id),
    action TEXT NOT NULL,
    details JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_activity_log_task ON activity_log(task_id);
CREATE INDEX IF NOT EXISTS idx_activity_log_member ON activity_log(member_id);
CREATE INDEX IF NOT EXISTS idx_activity_log_created ON activity_log(created_at DESC);
