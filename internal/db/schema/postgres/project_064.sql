-- Migration 064: Add provider columns for multi-provider support
--
-- Mirrors global_012.sql for project-level workflow tables.
-- Provider determines which LLM executor handles a phase (claude, codex, ollama, etc.).

ALTER TABLE workflows ADD COLUMN default_provider TEXT DEFAULT '';
ALTER TABLE phase_templates ADD COLUMN provider TEXT DEFAULT '';
ALTER TABLE workflow_phases ADD COLUMN provider_override TEXT DEFAULT '';
ALTER TABLE _agents_storage ADD COLUMN provider TEXT DEFAULT '';

-- Recreate agents VIEW trigger functions to include the new provider column.
-- The VIEW itself (SELECT * FROM _agents_storage) automatically picks up the column,
-- but the trigger functions list columns explicitly and need updating.

CREATE OR REPLACE FUNCTION agents_insert_fn() RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO _agents_storage (id, name, description, prompt, tools, model, provider, system_prompt, claude_config, is_builtin, created_at, updated_at)
    VALUES (NEW.id, NEW.name, NEW.description, NEW.prompt, NEW.tools, NEW.model, NEW.provider, NEW.system_prompt, NEW.claude_config, NEW.is_builtin,
            COALESCE(NEW.created_at, NOW()), COALESCE(NEW.updated_at, NOW()))
    ON CONFLICT(id) DO UPDATE SET
        name = EXCLUDED.name,
        description = EXCLUDED.description,
        prompt = EXCLUDED.prompt,
        tools = EXCLUDED.tools,
        model = EXCLUDED.model,
        provider = EXCLUDED.provider,
        system_prompt = EXCLUDED.system_prompt,
        claude_config = EXCLUDED.claude_config,
        is_builtin = EXCLUDED.is_builtin,
        updated_at = EXCLUDED.updated_at;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION agents_update_fn() RETURNS TRIGGER AS $$
BEGIN
    UPDATE _agents_storage SET
        name = NEW.name,
        description = NEW.description,
        prompt = NEW.prompt,
        tools = NEW.tools,
        model = NEW.model,
        provider = NEW.provider,
        system_prompt = NEW.system_prompt,
        claude_config = NEW.claude_config,
        is_builtin = NEW.is_builtin,
        updated_at = COALESCE(NEW.updated_at, NOW())
    WHERE id = OLD.id;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
