-- Project database migration 057: Add user attribution columns to tasks, initiatives, phases, workflow_runs
-- User columns store user IDs that reference users.id in GlobalDB.
-- Note: SQLite doesn't enforce FK constraints across databases, so these are just TEXT columns.
-- Note: created_by already exists on tasks table (added in project_012.sql), so we only add assigned_to here.

-- Add assigned_to to tasks (created_by already exists from project_012)
ALTER TABLE tasks ADD COLUMN assigned_to TEXT;

-- Add created_by and owned_by to initiatives
ALTER TABLE initiatives ADD COLUMN created_by TEXT;
ALTER TABLE initiatives ADD COLUMN owned_by TEXT;

-- Add executed_by to phases
ALTER TABLE phases ADD COLUMN executed_by TEXT;

-- Add started_by to workflow_runs
ALTER TABLE workflow_runs ADD COLUMN started_by TEXT;
