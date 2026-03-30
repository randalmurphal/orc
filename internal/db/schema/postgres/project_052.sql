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

-- Step 3: Drop FK constraints referencing agents table
-- PostgreSQL can drop constraints directly (no table recreation needed)
ALTER TABLE phase_agents DROP CONSTRAINT IF EXISTS phase_agents_agent_id_fkey;
ALTER TABLE workflow_phases DROP CONSTRAINT IF EXISTS workflow_phases_agent_override_fkey;
ALTER TABLE phase_templates DROP CONSTRAINT IF EXISTS phase_templates_agent_id_fkey;

-- Step 4: Transform agents table into VIEW-based system
-- Rename real table to _agents_storage, create VIEW that filters executor-*
ALTER TABLE agents RENAME TO _agents_storage;

CREATE VIEW agents AS
SELECT * FROM _agents_storage WHERE id NOT LIKE 'executor-%';

-- Recreate index on phase_templates (idempotent, already exists from project_028)
CREATE INDEX IF NOT EXISTS idx_phase_templates_builtin ON phase_templates(is_builtin);

-- Step 5: INSTEAD OF triggers to make the view writeable (PostgreSQL syntax)

-- INSERT trigger function (handles upsert)
CREATE OR REPLACE FUNCTION agents_insert_fn() RETURNS trigger AS $$
BEGIN
    INSERT INTO _agents_storage (id, name, description, prompt, tools, model, system_prompt, runtime_config, is_builtin, created_at, updated_at)
    VALUES (NEW.id, NEW.name, NEW.description, NEW.prompt, NEW.tools, NEW.model, NEW.system_prompt, NEW.runtime_config, NEW.is_builtin,
            COALESCE(NEW.created_at, NOW()), COALESCE(NEW.updated_at, NOW()))
    ON CONFLICT(id) DO UPDATE SET
        name = EXCLUDED.name,
        description = EXCLUDED.description,
        prompt = EXCLUDED.prompt,
        tools = EXCLUDED.tools,
        model = EXCLUDED.model,
        system_prompt = EXCLUDED.system_prompt,
        runtime_config = EXCLUDED.runtime_config,
        is_builtin = EXCLUDED.is_builtin,
        updated_at = EXCLUDED.updated_at;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER agents_insert
INSTEAD OF INSERT ON agents
FOR EACH ROW EXECUTE FUNCTION agents_insert_fn();

-- UPDATE trigger function
CREATE OR REPLACE FUNCTION agents_update_fn() RETURNS trigger AS $$
BEGIN
    UPDATE _agents_storage SET
        name = NEW.name,
        description = NEW.description,
        prompt = NEW.prompt,
        tools = NEW.tools,
        model = NEW.model,
        system_prompt = NEW.system_prompt,
        runtime_config = NEW.runtime_config,
        is_builtin = NEW.is_builtin,
        updated_at = COALESCE(NEW.updated_at, NOW())
    WHERE id = OLD.id;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER agents_update
INSTEAD OF UPDATE ON agents
FOR EACH ROW EXECUTE FUNCTION agents_update_fn();

-- DELETE trigger function
CREATE OR REPLACE FUNCTION agents_delete_fn() RETURNS trigger AS $$
BEGIN
    DELETE FROM _agents_storage WHERE id = OLD.id;
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER agents_delete
INSTEAD OF DELETE ON agents
FOR EACH ROW EXECUTE FUNCTION agents_delete_fn();

-- Step 6: Triggers to nullify executor-* agent references in phase_templates
-- Uses BEFORE trigger in PostgreSQL (can modify NEW directly, more efficient than AFTER)
CREATE OR REPLACE FUNCTION nullify_executor_refs_fn() RETURNS trigger AS $$
BEGIN
    IF NEW.agent_id LIKE 'executor-%' THEN
        NEW.agent_id := NULL;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER nullify_executor_refs_insert
BEFORE INSERT ON phase_templates
FOR EACH ROW EXECUTE FUNCTION nullify_executor_refs_fn();

CREATE TRIGGER nullify_executor_refs_update
BEFORE UPDATE OF agent_id ON phase_templates
FOR EACH ROW EXECUTE FUNCTION nullify_executor_refs_fn();
