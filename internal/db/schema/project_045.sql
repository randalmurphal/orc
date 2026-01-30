-- Unified Agent-Phase System Migration
-- Agents can now be used as main executors, not just sub-agents.
-- Phase template config (system_prompt, model_override, claude_config) moves to agents.

-- Step 1: Add new columns to agents table
ALTER TABLE agents ADD COLUMN system_prompt TEXT;    -- For executor role
ALTER TABLE agents ADD COLUMN claude_config TEXT;    -- JSON: additional claude settings

-- Step 2: Add new columns to phase_templates
ALTER TABLE phase_templates ADD COLUMN agent_id TEXT REFERENCES agents(id);
ALTER TABLE phase_templates ADD COLUMN sub_agents TEXT;  -- JSON array of agent IDs

-- Step 3: Add new columns to workflow_phases
ALTER TABLE workflow_phases ADD COLUMN agent_override TEXT REFERENCES agents(id);
ALTER TABLE workflow_phases ADD COLUMN sub_agents_override TEXT;  -- JSON array of agent IDs

-- Step 4: Create executor agents from existing phase template config
-- Each phase template that has executor config becomes its own executor agent
INSERT INTO agents (id, name, description, prompt, tools, model, system_prompt, claude_config, is_builtin, created_at, updated_at)
SELECT
    'executor-' || id,
    name || '-executor',
    'Executor agent for ' || name || ' phase',
    '',  -- prompt is for sub-agent role, not used here
    '[]', -- tools will be inherited from phase requirements
    model_override,
    system_prompt,
    claude_config,
    1,  -- is_builtin = true for migrated agents
    datetime('now'),
    datetime('now')
FROM phase_templates
WHERE is_builtin = 1;  -- Only migrate builtin phase templates

-- Step 5: Link phase templates to their new executor agents
UPDATE phase_templates
SET agent_id = 'executor-' || id
WHERE is_builtin = 1;

-- Step 6: Drop old columns (moved to agents table)
-- SQLite doesn't support DROP COLUMN directly, so we recreate the table
-- This is handled by the Go migration code for complex cases

-- For now, just mark these columns as deprecated by leaving them
-- The Go code will stop reading them and only use agent_id
