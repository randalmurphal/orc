-- Unified Phase Outputs
-- Replaces fragmented specs and phase_artifacts tables with a single,
-- flexible phase_outputs table. Supports custom workflows with declared output variables.

--------------------------------------------------------------------------------
-- PHASE OUTPUTS: Unified storage for all phase artifacts
--------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS phase_outputs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    workflow_run_id TEXT NOT NULL,
    phase_template_id TEXT NOT NULL,
    task_id TEXT,                           -- Nullable for non-task runs (branch, PR, standalone)
    content TEXT NOT NULL,
    content_hash TEXT,
    output_var_name TEXT NOT NULL,          -- Variable name (e.g., 'SPEC_CONTENT', 'TDD_TESTS_CONTENT')
    artifact_type TEXT,                     -- 'spec', 'tests', 'breakdown', 'research', 'docs'
    source TEXT,                            -- 'workflow', 'import', 'manual', 'migrated'
    iteration INTEGER DEFAULT 1,
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now')),
    UNIQUE(workflow_run_id, phase_template_id),
    FOREIGN KEY (workflow_run_id) REFERENCES workflow_runs(id) ON DELETE CASCADE,
    FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_phase_outputs_run ON phase_outputs(workflow_run_id);
CREATE INDEX IF NOT EXISTS idx_phase_outputs_task ON phase_outputs(task_id);
CREATE INDEX IF NOT EXISTS idx_phase_outputs_var ON phase_outputs(output_var_name);

--------------------------------------------------------------------------------
-- ADD output_var_name TO PHASE TEMPLATES
--------------------------------------------------------------------------------
ALTER TABLE phase_templates ADD COLUMN output_var_name TEXT;

--------------------------------------------------------------------------------
-- MIGRATE EXISTING DATA
--------------------------------------------------------------------------------

-- Migrate specs table data
INSERT OR IGNORE INTO phase_outputs (workflow_run_id, phase_template_id, task_id, content, content_hash, output_var_name, artifact_type, source, created_at, updated_at)
SELECT
    wr.id,
    CASE WHEN wr.workflow_id IN ('implement-trivial', 'implement-small') THEN 'tiny_spec' ELSE 'spec' END,
    s.task_id,
    s.content,
    s.content_hash,
    'SPEC_CONTENT',
    'spec',
    COALESCE(s.source, 'migrated'),
    s.created_at,
    s.updated_at
FROM specs s
JOIN workflow_runs wr ON wr.task_id = s.task_id;

-- Migrate phase_artifacts table data
INSERT OR IGNORE INTO phase_outputs (workflow_run_id, phase_template_id, task_id, content, content_hash, output_var_name, artifact_type, source, created_at, updated_at)
SELECT
    wr.id,
    pa.phase_id,
    pa.task_id,
    pa.content,
    pa.content_hash,
    CASE pa.phase_id
        WHEN 'tdd_write' THEN 'TDD_TESTS_CONTENT'
        WHEN 'breakdown' THEN 'BREAKDOWN_CONTENT'
        WHEN 'research' THEN 'RESEARCH_CONTENT'
        WHEN 'docs' THEN 'DOCS_CONTENT'
        ELSE 'OUTPUT_' || UPPER(REPLACE(pa.phase_id, '-', '_'))
    END,
    pa.phase_id,
    COALESCE(pa.source, 'migrated'),
    pa.created_at,
    pa.updated_at
FROM phase_artifacts pa
JOIN workflow_runs wr ON wr.task_id = pa.task_id;

--------------------------------------------------------------------------------
-- DROP OLD TABLES
--------------------------------------------------------------------------------
DROP TABLE IF EXISTS specs_fts;
DROP TABLE IF EXISTS specs;
DROP TABLE IF EXISTS phase_artifacts;
