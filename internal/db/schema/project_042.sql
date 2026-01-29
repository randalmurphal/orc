-- Migration: Add position fields to workflow_phases, drop UNIQUE(workflow_id, sequence)
-- Supports visual workflow editor with draggable phase nodes and parallel phases

--------------------------------------------------------------------------------
-- WORKFLOW_PHASES: Recreate without UNIQUE(workflow_id, sequence), add position columns
-- Uses SQLite table recreation pattern (CREATE new → INSERT SELECT → DROP old → RENAME)
--------------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS workflow_phases_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    workflow_id TEXT NOT NULL,
    phase_template_id TEXT NOT NULL,
    sequence INTEGER NOT NULL,              -- Execution order (0, 1, 2, ...) - no longer unique per workflow
    depends_on TEXT,                        -- JSON: ["phase_id_1", "phase_id_2"]

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
    FOREIGN KEY (phase_template_id) REFERENCES phase_templates(id) ON DELETE RESTRICT
);

-- Copy existing data (position_x/position_y default to NULL = auto-layout)
INSERT INTO workflow_phases_new (id, workflow_id, phase_template_id, sequence, depends_on,
    max_iterations_override, model_override, thinking_override, gate_type_override, condition,
    quality_checks_override, loop_config, claude_config_override)
SELECT id, workflow_id, phase_template_id, sequence, depends_on,
    max_iterations_override, model_override, thinking_override, gate_type_override, condition,
    quality_checks_override, loop_config, claude_config_override
FROM workflow_phases;

-- Drop old table and rename new one
DROP TABLE workflow_phases;
ALTER TABLE workflow_phases_new RENAME TO workflow_phases;

-- Recreate indexes on new table
CREATE INDEX IF NOT EXISTS idx_workflow_phases_workflow ON workflow_phases(workflow_id);
CREATE INDEX IF NOT EXISTS idx_workflow_phases_sequence ON workflow_phases(workflow_id, sequence);
