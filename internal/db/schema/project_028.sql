-- Configurable Workflow System
-- Transforms orc from weight-based task execution to a fully configurable,
-- database-first workflow system with composable phases and custom variables.

--------------------------------------------------------------------------------
-- PHASE TEMPLATES: Reusable phase definitions (lego blocks)
--------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS phase_templates (
    id TEXT PRIMARY KEY,                    -- 'spec', 'implement', 'my-custom-review'
    name TEXT NOT NULL,
    description TEXT,

    -- Prompt configuration
    prompt_source TEXT DEFAULT 'embedded',  -- 'embedded', 'db', 'file'
    prompt_content TEXT,                    -- Inline prompt if source='db'
    prompt_path TEXT,                       -- Path for file-based prompts

    -- Contract: what variables this phase expects and produces
    input_variables TEXT,                   -- JSON: ["SPEC_CONTENT", "CUSTOM_VAR"]
    output_schema TEXT,                     -- JSON schema for validation
    produces_artifact BOOLEAN DEFAULT FALSE,
    artifact_type TEXT,                     -- 'spec', 'tests', 'docs', etc.

    -- Execution config
    max_iterations INTEGER DEFAULT 20,
    model_override TEXT,                    -- NULL means use workflow default
    thinking_enabled BOOLEAN,               -- NULL means use workflow default
    gate_type TEXT DEFAULT 'auto',          -- 'auto', 'human', 'skip'
    checkpoint BOOLEAN DEFAULT TRUE,        -- Create git checkpoint after phase

    -- Retry configuration
    retry_from_phase TEXT,                  -- Which phase to retry from on failure
    retry_prompt_path TEXT,                 -- Optional retry-specific prompt

    -- Metadata
    is_builtin BOOLEAN DEFAULT FALSE,       -- TRUE for built-in phases (immutable)
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_phase_templates_builtin ON phase_templates(is_builtin);

--------------------------------------------------------------------------------
-- WORKFLOWS: Compose phases into execution plans
--------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS workflows (
    id TEXT PRIMARY KEY,                    -- 'implement', 'review', 'my-custom-wf'
    name TEXT NOT NULL,
    description TEXT,
    workflow_type TEXT DEFAULT 'task',      -- 'task', 'branch', 'standalone'
    default_model TEXT,                     -- Default model for all phases
    default_thinking BOOLEAN DEFAULT FALSE, -- Default thinking mode
    is_builtin BOOLEAN DEFAULT FALSE,       -- TRUE for built-in workflows
    based_on TEXT,                          -- Cloned from which workflow (for lineage)
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now')),
    FOREIGN KEY (based_on) REFERENCES workflows(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_workflows_builtin ON workflows(is_builtin);
CREATE INDEX IF NOT EXISTS idx_workflows_type ON workflows(workflow_type);

--------------------------------------------------------------------------------
-- WORKFLOW_PHASES: Which phases in which order (junction table)
--------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS workflow_phases (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    workflow_id TEXT NOT NULL,
    phase_template_id TEXT NOT NULL,
    sequence INTEGER NOT NULL,              -- Execution order (0, 1, 2, ...)
    depends_on TEXT,                        -- JSON: ["phase_id_1", "phase_id_2"]

    -- Per-workflow phase overrides (NULL = use phase_template defaults)
    max_iterations_override INTEGER,
    model_override TEXT,
    thinking_override BOOLEAN,
    gate_type_override TEXT,
    condition TEXT,                         -- JSON: skip conditions (e.g., {"if_empty": "BREAKDOWN_CONTENT"})

    UNIQUE(workflow_id, phase_template_id),
    UNIQUE(workflow_id, sequence),
    FOREIGN KEY (workflow_id) REFERENCES workflows(id) ON DELETE CASCADE,
    FOREIGN KEY (phase_template_id) REFERENCES phase_templates(id) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_workflow_phases_workflow ON workflow_phases(workflow_id);
CREATE INDEX IF NOT EXISTS idx_workflow_phases_sequence ON workflow_phases(workflow_id, sequence);

--------------------------------------------------------------------------------
-- WORKFLOW_VARIABLES: Custom variable definitions per workflow
--------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS workflow_variables (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    workflow_id TEXT NOT NULL,
    name TEXT NOT NULL,                     -- 'JIRA_CONTEXT', 'STYLE_GUIDE'
    description TEXT,

    -- Source configuration
    source_type TEXT NOT NULL,              -- 'static', 'script', 'api', 'phase_output', 'env', 'prompt_fragment'
    source_config TEXT NOT NULL,            -- JSON: source-specific config

    required BOOLEAN DEFAULT FALSE,
    default_value TEXT,
    cache_ttl_seconds INTEGER DEFAULT 0,    -- 0 = no caching

    -- For script sources, store script content for cross-machine sync
    script_content TEXT,

    UNIQUE(workflow_id, name),
    FOREIGN KEY (workflow_id) REFERENCES workflows(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_workflow_variables_workflow ON workflow_variables(workflow_id);

--------------------------------------------------------------------------------
-- WORKFLOW_RUNS: Execution instances (universal anchor, replaces task-centric execution)
--------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS workflow_runs (
    id TEXT PRIMARY KEY,                    -- 'RUN-001' (auto-generated)
    workflow_id TEXT NOT NULL,

    -- Context (polymorphic - what we're running on)
    context_type TEXT NOT NULL,             -- 'task', 'branch', 'pr', 'standalone', 'tag'
    context_data TEXT NOT NULL,             -- JSON with context-specific fields
    task_id TEXT,                           -- Direct FK for task context (nullable)

    -- User inputs
    prompt TEXT NOT NULL,                   -- The main prompt/description
    instructions TEXT,                      -- Additional instructions for this run

    -- Status
    status TEXT DEFAULT 'pending',          -- 'pending', 'running', 'paused', 'completed', 'failed', 'cancelled'
    current_phase TEXT,
    started_at TEXT,
    completed_at TEXT,

    -- Runtime data
    variables_snapshot TEXT,                -- JSON: resolved variables at start (for audit)

    -- Metrics (aggregated from phases)
    total_cost_usd REAL DEFAULT 0,
    total_input_tokens INTEGER DEFAULT 0,
    total_output_tokens INTEGER DEFAULT 0,

    -- Error tracking
    error TEXT,

    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now')),

    FOREIGN KEY (workflow_id) REFERENCES workflows(id) ON DELETE RESTRICT,
    FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_workflow_runs_task ON workflow_runs(task_id);
CREATE INDEX IF NOT EXISTS idx_workflow_runs_status ON workflow_runs(status);
CREATE INDEX IF NOT EXISTS idx_workflow_runs_workflow ON workflow_runs(workflow_id);
CREATE INDEX IF NOT EXISTS idx_workflow_runs_created ON workflow_runs(created_at DESC);

--------------------------------------------------------------------------------
-- WORKFLOW_RUN_PHASES: Phase execution within a run
--------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS workflow_run_phases (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    workflow_run_id TEXT NOT NULL,
    phase_template_id TEXT NOT NULL,

    -- Status
    status TEXT DEFAULT 'pending',          -- 'pending', 'running', 'completed', 'failed', 'skipped'
    iterations INTEGER DEFAULT 0,

    -- Timing
    started_at TEXT,
    completed_at TEXT,

    -- Git tracking
    commit_sha TEXT,                        -- Checkpoint commit for this phase

    -- Metrics
    input_tokens INTEGER DEFAULT 0,
    output_tokens INTEGER DEFAULT 0,
    cost_usd REAL DEFAULT 0,

    -- Output
    artifact TEXT,                          -- Phase artifact content (spec, tests, etc.)

    -- Error tracking
    error TEXT,

    -- Claude session link
    session_id TEXT,

    UNIQUE(workflow_run_id, phase_template_id),
    FOREIGN KEY (workflow_run_id) REFERENCES workflow_runs(id) ON DELETE CASCADE,
    FOREIGN KEY (phase_template_id) REFERENCES phase_templates(id) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_workflow_run_phases_run ON workflow_run_phases(workflow_run_id);
CREATE INDEX IF NOT EXISTS idx_workflow_run_phases_status ON workflow_run_phases(status);

--------------------------------------------------------------------------------
-- ADD workflow_run_id TO EXISTING TABLES
-- Links transcripts and events to workflow runs (nullable for backward compat during migration)
--------------------------------------------------------------------------------
ALTER TABLE transcripts ADD COLUMN workflow_run_id TEXT REFERENCES workflow_runs(id) ON DELETE SET NULL;
ALTER TABLE event_log ADD COLUMN workflow_run_id TEXT REFERENCES workflow_runs(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_transcripts_run ON transcripts(workflow_run_id);
CREATE INDEX IF NOT EXISTS idx_event_log_run ON event_log(workflow_run_id);
