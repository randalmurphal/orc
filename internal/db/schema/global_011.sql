-- Migration 011: Add phase type columns for phase type dispatch (TASK-004)
--
-- Adds:
--   phase_templates.type - executor type ("llm", "knowledge", etc.) with "llm" default
--   workflow_phases.type_override - per-workflow type override (nullable)

ALTER TABLE phase_templates ADD COLUMN type TEXT NOT NULL DEFAULT 'llm';
ALTER TABLE workflow_phases ADD COLUMN type_override TEXT DEFAULT '';
