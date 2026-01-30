-- Global database migration 004: Add workflow tables for cross-project sharing
-- Workflows, phase templates, and agents are shared across all projects.
-- Execution records (workflow_runs, workflow_run_phases) remain in project DBs.

--------------------------------------------------------------------------------
-- PHASE TEMPLATES: Reusable phase definitions (lego blocks)
--------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS phase_templates (
    id TEXT PRIMARY KEY,                    -- 'spec', 'implement', 'my-custom-review'
    name TEXT NOT NULL,
    description TEXT,

    -- Agent configuration (WHO runs this phase)
    agent_id TEXT,                          -- References agents.id - the executor agent
    sub_agents TEXT,                        -- JSON array of agent IDs to include as sub-agents

    -- Prompt configuration (WHAT to do)
    prompt_source TEXT DEFAULT 'embedded',  -- 'embedded', 'db', 'file'
    prompt_content TEXT,                    -- Inline prompt if source='db'
    prompt_path TEXT,                       -- Path for file-based prompts

    -- Contract: what variables this phase expects and produces
    input_variables TEXT,                   -- JSON: ["SPEC_CONTENT", "CUSTOM_VAR"]
    output_schema TEXT,                     -- JSON schema for validation
    produces_artifact BOOLEAN DEFAULT FALSE,
    artifact_type TEXT,                     -- 'spec', 'tests', 'docs', etc.
    output_var_name TEXT,                   -- Variable name for output (e.g., 'SPEC_CONTENT')

    -- Quality checks
    output_type TEXT DEFAULT 'none',        -- 'code', 'tests', 'document', 'data', 'research', 'none'
    quality_checks TEXT,                    -- JSON array of QualityCheck

    -- Execution config
    max_iterations INTEGER DEFAULT 20,
    model_override TEXT,                    -- NULL means use workflow default
    thinking_enabled BOOLEAN,               -- NULL means use workflow default
    gate_type TEXT DEFAULT 'auto',          -- 'auto', 'human', 'skip'
    checkpoint BOOLEAN DEFAULT TRUE,        -- Create git checkpoint after phase

    -- Retry configuration
    retry_from_phase TEXT,                  -- Which phase to retry from on failure
    retry_prompt_path TEXT,                 -- Optional retry-specific prompt

    -- Claude configuration
    system_prompt TEXT,                     -- Passed via --system-prompt to main phase executor
    claude_config TEXT,                     -- JSON: additional claude settings

    -- Metadata
    is_builtin BOOLEAN DEFAULT FALSE,       -- TRUE for built-in phases (immutable)
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now')),

    FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE SET NULL
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

    -- Agent overrides (WHO runs this phase)
    agent_override TEXT,                    -- Override executor agent
    sub_agents_override TEXT,               -- Override sub-agents (JSON array)

    -- Per-workflow phase overrides (NULL = use phase_template defaults)
    max_iterations_override INTEGER,
    model_override TEXT,
    thinking_override BOOLEAN,
    gate_type_override TEXT,
    condition TEXT,                         -- JSON: skip conditions (e.g., {"if_empty": "BREAKDOWN_CONTENT"})
    quality_checks_override TEXT,           -- JSON array, NULL=use template, []=disable all

    -- Loop configuration (JSON) - defines iterative loop behavior
    loop_config TEXT,

    -- Claude configuration override (JSON) - merged with template config
    claude_config_override TEXT,

    -- Visual editor position (NULL = auto-layout via dagre)
    position_x REAL,
    position_y REAL,

    UNIQUE(workflow_id, phase_template_id),
    FOREIGN KEY (workflow_id) REFERENCES workflows(id) ON DELETE CASCADE,
    FOREIGN KEY (phase_template_id) REFERENCES phase_templates(id) ON DELETE RESTRICT,
    FOREIGN KEY (agent_override) REFERENCES agents(id) ON DELETE SET NULL
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

    -- JSONPath extraction for API/phase_output sources
    extract TEXT,                           -- gjson path for extracting values

    UNIQUE(workflow_id, name),
    FOREIGN KEY (workflow_id) REFERENCES workflows(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_workflow_variables_workflow ON workflow_variables(workflow_id);

--------------------------------------------------------------------------------
-- AGENTS: Agent definitions for multi-agent phase execution
--------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS agents (
    id TEXT PRIMARY KEY,                        -- 'code-reviewer', 'silent-failure-hunter', etc.
    name TEXT NOT NULL,                         -- Display name
    description TEXT NOT NULL,                  -- When to use (required by Claude CLI)
    prompt TEXT NOT NULL,                       -- Context prompt for sub-agent role
    tools TEXT,                                 -- JSON array: ["Read", "Grep", "Edit"]
    model TEXT,                                 -- 'opus', 'sonnet', 'haiku' (optional override)

    -- Executor role fields (used when agent is main phase executor)
    system_prompt TEXT,                         -- Role framing for executor
    claude_config TEXT,                         -- JSON: additional claude settings

    is_builtin BOOLEAN DEFAULT FALSE,           -- True for built-in agents
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_agents_builtin ON agents(is_builtin);

--------------------------------------------------------------------------------
-- PHASE_AGENTS: Which agents run for which phases
--------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS phase_agents (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    phase_template_id TEXT NOT NULL,            -- References phase_templates.id
    agent_id TEXT NOT NULL,                     -- References agents.id
    sequence INTEGER NOT NULL DEFAULT 0,        -- Execution order (same sequence = parallel)
    role TEXT,                                  -- 'correctness', 'architecture', 'security', etc.
    weight_filter TEXT,                         -- JSON array: ["medium", "large"] or null for all
    is_builtin BOOLEAN DEFAULT FALSE,           -- True for built-in associations
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now')),
    UNIQUE(phase_template_id, agent_id),
    FOREIGN KEY (phase_template_id) REFERENCES phase_templates(id) ON DELETE CASCADE,
    FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_phase_agents_phase ON phase_agents(phase_template_id);
CREATE INDEX IF NOT EXISTS idx_phase_agents_agent ON phase_agents(agent_id);
