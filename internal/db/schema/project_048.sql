-- Migration 048: Extended gate configuration
-- Adds gate config columns to match global_005 migration.
-- Phase template gate configuration
ALTER TABLE phase_templates ADD COLUMN gate_input_config TEXT;
ALTER TABLE phase_templates ADD COLUMN gate_output_config TEXT;
ALTER TABLE phase_templates ADD COLUMN gate_mode TEXT;
ALTER TABLE phase_templates ADD COLUMN gate_agent_id TEXT;

-- Workflow phase before-triggers
ALTER TABLE workflow_phases ADD COLUMN before_triggers TEXT;

-- Workflow lifecycle triggers
ALTER TABLE workflows ADD COLUMN triggers TEXT;
