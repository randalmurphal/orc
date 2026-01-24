-- Migration 031: Add claude_config to phase templates and workflow phases
-- Enables per-phase Claude CLI configuration (system prompts, tool restrictions, etc.)

-- Add claude_config to phase_templates (base config for the template)
ALTER TABLE phase_templates ADD COLUMN claude_config TEXT;

-- Add claude_config_override to workflow_phases (per-workflow override)
ALTER TABLE workflow_phases ADD COLUMN claude_config_override TEXT;
