-- Phase-Level Quality Checks System
-- Replaces hardcoded backpressure with database-driven, phase-level quality checks.
-- Quality checks are configured per phase template, with optional workflow-level overrides.

--------------------------------------------------------------------------------
-- PROJECT COMMANDS: Language-specific commands for quality checks
--------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS project_commands (
    name TEXT PRIMARY KEY,                  -- 'test', 'lint', 'build', 'typecheck', or custom
    domain TEXT NOT NULL DEFAULT 'code',    -- 'code', 'custom'
    command TEXT NOT NULL,                  -- Full command: 'go test ./...'
    short_command TEXT,                     -- Optional short variant: 'go test -short ./...'
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    description TEXT,
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_project_commands_domain ON project_commands(domain);
CREATE INDEX IF NOT EXISTS idx_project_commands_enabled ON project_commands(enabled);

--------------------------------------------------------------------------------
-- PHASE TEMPLATES: Add output_type and quality_checks columns
--------------------------------------------------------------------------------
-- output_type: What kind of output this phase produces (affects which checks make sense)
-- Values: 'code', 'tests', 'document', 'data', 'research', 'none'
ALTER TABLE phase_templates ADD COLUMN output_type TEXT DEFAULT 'none';

-- quality_checks: JSON array of QualityCheck objects to run after phase completion
-- Example: [{"type":"code","name":"tests","enabled":true,"on_failure":"block"}]
ALTER TABLE phase_templates ADD COLUMN quality_checks TEXT;

--------------------------------------------------------------------------------
-- WORKFLOW PHASES: Add quality_checks_override column
--------------------------------------------------------------------------------
-- quality_checks_override: JSON array to override phase template's quality_checks
-- NULL means use phase template defaults
-- Empty array [] means disable all checks for this workflow
ALTER TABLE workflow_phases ADD COLUMN quality_checks_override TEXT;
