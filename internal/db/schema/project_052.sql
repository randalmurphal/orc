-- orc:disable_fk
-- Migration 052: Remove auto-generated executor agents
-- Clean up legacy executor-* pattern agents created by migration 045.
-- These were a design mistake - phases should not auto-generate wrapper agents.
--
-- Implementation: Use a VIEW to hide executor-* agents from queries.
-- Inserts go to the underlying storage table, but SELECT queries filter them out.
-- This makes executor-* agents effectively invisible to the application.

-- Step 1: Clear references in all tables that point to executor-* agents
UPDATE phase_templates SET agent_id = NULL WHERE agent_id LIKE 'executor-%';
DELETE FROM phase_agents WHERE agent_id LIKE 'executor-%';
UPDATE workflow_phases SET agent_override = NULL WHERE agent_override LIKE 'executor-%';

-- Step 2: Delete existing executor-* agents
DELETE FROM agents WHERE id LIKE 'executor-%';

-- Step 3a: Recreate phase_agents table WITHOUT FK to agents
-- (VIEW-based agents table breaks FK references)
CREATE TABLE phase_agents_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    phase_template_id TEXT NOT NULL,
    agent_id TEXT NOT NULL,
    sequence INTEGER NOT NULL DEFAULT 0,
    role TEXT,
    weight_filter TEXT,
    is_builtin BOOLEAN DEFAULT FALSE,
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now')),
    UNIQUE(phase_template_id, agent_id),
    FOREIGN KEY (phase_template_id) REFERENCES phase_templates(id) ON DELETE CASCADE
    -- Note: FK to agents removed - agents is now a VIEW
);
INSERT INTO phase_agents_new SELECT * FROM phase_agents;
DROP TABLE phase_agents;
ALTER TABLE phase_agents_new RENAME TO phase_agents;

-- Step 3b: Recreate workflow_phases table WITHOUT FK to agents for agent_override
CREATE TABLE workflow_phases_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    workflow_id TEXT NOT NULL,
    phase_template_id TEXT NOT NULL,
    sequence INTEGER NOT NULL,
    depends_on TEXT,
    max_iterations_override INTEGER,
    model_override TEXT,
    thinking_override BOOLEAN,
    gate_type_override TEXT,
    condition TEXT,
    quality_checks_override TEXT,
    loop_config TEXT,
    claude_config_override TEXT,
    position_x REAL,
    position_y REAL,
    agent_override TEXT,  -- FK to agents removed - agents is now a VIEW
    sub_agents_override TEXT,
    before_triggers TEXT,
    UNIQUE(workflow_id, phase_template_id),
    FOREIGN KEY (workflow_id) REFERENCES workflows(id) ON DELETE CASCADE,
    FOREIGN KEY (phase_template_id) REFERENCES phase_templates(id) ON DELETE RESTRICT
);
INSERT INTO workflow_phases_new SELECT * FROM workflow_phases;
DROP TABLE workflow_phases;
ALTER TABLE workflow_phases_new RENAME TO workflow_phases;

-- Step 3c: Recreate phase_templates table with FK removed
-- This allows referencing agents that may be filtered by the view.
CREATE TABLE phase_templates_new (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    prompt_source TEXT DEFAULT 'embedded',
    prompt_content TEXT,
    prompt_path TEXT,
    input_variables TEXT,
    output_schema TEXT,
    produces_artifact BOOLEAN DEFAULT FALSE,
    artifact_type TEXT,
    max_iterations INTEGER DEFAULT 20,
    model_override TEXT,
    thinking_enabled BOOLEAN,
    gate_type TEXT DEFAULT 'auto',
    checkpoint BOOLEAN DEFAULT TRUE,
    retry_from_phase TEXT,
    retry_prompt_path TEXT,
    is_builtin BOOLEAN DEFAULT FALSE,
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now')),
    output_var_name TEXT,
    output_type TEXT DEFAULT 'none',
    quality_checks TEXT,
    claude_config TEXT,
    system_prompt TEXT,
    agent_id TEXT,
    sub_agents TEXT,
    gate_input_config TEXT,
    gate_output_config TEXT,
    gate_mode TEXT,
    gate_agent_id TEXT
);

INSERT INTO phase_templates_new SELECT * FROM phase_templates;
DROP TABLE phase_templates;
ALTER TABLE phase_templates_new RENAME TO phase_templates;
CREATE INDEX IF NOT EXISTS idx_phase_templates_builtin ON phase_templates(is_builtin);

-- Step 4: Transform agents table into VIEW-based system
-- Rename real table to _agents_storage, create VIEW that filters executor-*

ALTER TABLE agents RENAME TO _agents_storage;

CREATE VIEW agents AS
SELECT * FROM _agents_storage WHERE id NOT LIKE 'executor-%';

-- INSTEAD OF triggers to make the view writeable
-- The INSERT trigger handles UPSERT by checking if row exists first
CREATE TRIGGER agents_insert
INSTEAD OF INSERT ON agents
BEGIN
    INSERT INTO _agents_storage (id, name, description, prompt, tools, model, system_prompt, claude_config, is_builtin, created_at, updated_at)
    VALUES (NEW.id, NEW.name, NEW.description, NEW.prompt, NEW.tools, NEW.model, NEW.system_prompt, NEW.claude_config, NEW.is_builtin,
            COALESCE(NEW.created_at, datetime('now')), COALESCE(NEW.updated_at, datetime('now')))
    ON CONFLICT(id) DO UPDATE SET
        name = excluded.name,
        description = excluded.description,
        prompt = excluded.prompt,
        tools = excluded.tools,
        model = excluded.model,
        system_prompt = excluded.system_prompt,
        claude_config = excluded.claude_config,
        is_builtin = excluded.is_builtin,
        updated_at = excluded.updated_at;
END;

CREATE TRIGGER agents_update
INSTEAD OF UPDATE ON agents
BEGIN
    UPDATE _agents_storage SET
        name = NEW.name,
        description = NEW.description,
        prompt = NEW.prompt,
        tools = NEW.tools,
        model = NEW.model,
        system_prompt = NEW.system_prompt,
        claude_config = NEW.claude_config,
        is_builtin = NEW.is_builtin,
        updated_at = COALESCE(NEW.updated_at, datetime('now'))
    WHERE id = OLD.id;
END;

CREATE TRIGGER agents_delete
INSTEAD OF DELETE ON agents
BEGIN
    DELETE FROM _agents_storage WHERE id = OLD.id;
END;

-- Step 5: Trigger to nullify executor-* agent references in phase_templates
CREATE TRIGGER IF NOT EXISTS nullify_executor_refs_insert
AFTER INSERT ON phase_templates
FOR EACH ROW
WHEN NEW.agent_id LIKE 'executor-%'
BEGIN
    UPDATE phase_templates SET agent_id = NULL WHERE id = NEW.id;
END;

CREATE TRIGGER IF NOT EXISTS nullify_executor_refs_update
AFTER UPDATE OF agent_id ON phase_templates
FOR EACH ROW
WHEN NEW.agent_id LIKE 'executor-%'
BEGIN
    UPDATE phase_templates SET agent_id = NULL WHERE id = NEW.id;
END;
