-- orc:disable_fk
-- Project database migration 060: Remove cross-database FK on workflow_run_phases
-- The phase_template_id FK references phase_templates which lives in GlobalDB,
-- not ProjectDB. This causes FK constraint failures (error 787) when new phase
-- templates are added to GlobalDB but the stale ProjectDB copy is out of sync.
-- Referential integrity is enforced at the application level (executor validates
-- phase templates exist in GlobalDB before saving run phases).

-- Recreate workflow_run_phases without the phase_templates FK
CREATE TABLE workflow_run_phases_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    workflow_run_id TEXT NOT NULL,
    phase_template_id TEXT NOT NULL,

    -- Status
    status TEXT DEFAULT 'pending',
    iterations INTEGER DEFAULT 0,

    -- Timing
    started_at TEXT,
    completed_at TEXT,

    -- Git tracking
    commit_sha TEXT,

    -- Metrics
    input_tokens INTEGER DEFAULT 0,
    output_tokens INTEGER DEFAULT 0,
    cost_usd REAL DEFAULT 0,

    -- Output
    content TEXT,

    -- Error tracking
    error TEXT,

    -- Claude session link
    session_id TEXT,

    UNIQUE(workflow_run_id, phase_template_id),
    FOREIGN KEY (workflow_run_id) REFERENCES workflow_runs(id) ON DELETE CASCADE
);

INSERT INTO workflow_run_phases_new
    SELECT * FROM workflow_run_phases;

DROP TABLE workflow_run_phases;

ALTER TABLE workflow_run_phases_new RENAME TO workflow_run_phases;

CREATE INDEX IF NOT EXISTS idx_workflow_run_phases_run ON workflow_run_phases(workflow_run_id);
CREATE INDEX IF NOT EXISTS idx_workflow_run_phases_status ON workflow_run_phases(status);
