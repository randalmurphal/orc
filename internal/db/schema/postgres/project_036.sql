-- Migration 036: Rename artifact column to content in workflow_run_phases
-- Part of the artifact -> content terminology cleanup.
-- The "artifact" terminology is being replaced with "content" throughout the codebase.

ALTER TABLE workflow_run_phases RENAME COLUMN artifact TO content;
