-- Migration 036: Rename artifact column to content in workflow_run_phases
-- Part of the artifact -> content terminology cleanup.
-- The "artifact" terminology is being replaced with "content" throughout the codebase.
--
-- SQLite 3.25.0+ supports ALTER TABLE RENAME COLUMN.
-- The migration system tracks applied migrations, so this only runs once.

ALTER TABLE workflow_run_phases RENAME COLUMN artifact TO content;
