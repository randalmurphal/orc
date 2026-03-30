-- Migration 032: Add runtime_config to phase templates and workflow phases
-- Enables per-phase runtime configuration (prompts, tools, hooks, MCP, etc.)

-- Add runtime_config to phase_templates (base config for the template)
ALTER TABLE phase_templates ADD COLUMN runtime_config TEXT;

-- Add runtime_config_override to workflow_phases (per-workflow override)
ALTER TABLE workflow_phases ADD COLUMN runtime_config_override TEXT;
