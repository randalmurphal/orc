-- Migration 064: Add provider columns for multi-provider support
--
-- Mirrors global_012.sql for project-level workflow tables.
-- Provider determines which LLM executor handles a phase (claude or codex).

ALTER TABLE workflows ADD COLUMN default_provider TEXT DEFAULT '';
ALTER TABLE phase_templates ADD COLUMN provider TEXT DEFAULT '';
ALTER TABLE workflow_phases ADD COLUMN provider_override TEXT DEFAULT '';
ALTER TABLE _agents_storage ADD COLUMN provider TEXT DEFAULT '';

-- Recreate agents VIEW triggers to include the new provider column.
-- The VIEW itself (SELECT * FROM _agents_storage) automatically picks up the column,
-- but the INSTEAD OF triggers list columns explicitly and need updating.

DROP TRIGGER IF EXISTS agents_insert;
DROP TRIGGER IF EXISTS agents_update;

CREATE TRIGGER agents_insert
INSTEAD OF INSERT ON agents
BEGIN
    INSERT INTO _agents_storage (id, name, description, prompt, tools, model, provider, system_prompt, runtime_config, is_builtin, created_at, updated_at)
    VALUES (NEW.id, NEW.name, NEW.description, NEW.prompt, NEW.tools, NEW.model, NEW.provider, NEW.system_prompt, NEW.runtime_config, NEW.is_builtin,
            COALESCE(NEW.created_at, datetime('now')), COALESCE(NEW.updated_at, datetime('now')))
    ON CONFLICT(id) DO UPDATE SET
        name = excluded.name,
        description = excluded.description,
        prompt = excluded.prompt,
        tools = excluded.tools,
        model = excluded.model,
        provider = excluded.provider,
        system_prompt = excluded.system_prompt,
        runtime_config = excluded.runtime_config,
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
        provider = NEW.provider,
        system_prompt = NEW.system_prompt,
        runtime_config = NEW.runtime_config,
        is_builtin = NEW.is_builtin,
        updated_at = COALESCE(NEW.updated_at, datetime('now'))
    WHERE id = OLD.id;
END;
