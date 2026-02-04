-- Migration 055: Add sequences table for atomic ID generation
-- Fixes race condition in GetNextWorkflowRunID/GetNextTaskID/GetNextInitiativeID
-- where parallel processes could get the same ID due to non-atomic read-modify-write.

CREATE TABLE IF NOT EXISTS sequences (
    name TEXT PRIMARY KEY,
    current_value INTEGER NOT NULL DEFAULT 0
);

-- Seed initial values from existing data to avoid ID collisions
-- For workflow runs: get max numeric value from RUN-XXX pattern
INSERT OR IGNORE INTO sequences (name, current_value)
SELECT 'workflow_run', COALESCE(MAX(CAST(SUBSTR(id, 5) AS INTEGER)), 0)
FROM workflow_runs WHERE id LIKE 'RUN-%';

-- For tasks: get max numeric value from TASK-XXX pattern (solo mode, no prefix)
INSERT OR IGNORE INTO sequences (name, current_value)
SELECT 'task', COALESCE(MAX(CAST(SUBSTR(id, 6) AS INTEGER)), 0)
FROM tasks WHERE id LIKE 'TASK-%' AND id NOT LIKE 'TASK-%-%';

-- For initiatives: get max numeric value from INIT-XXX pattern
INSERT OR IGNORE INTO sequences (name, current_value)
SELECT 'initiative', COALESCE(MAX(CAST(SUBSTR(id, 6) AS INTEGER)), 0)
FROM initiatives WHERE id LIKE 'INIT-%';

-- For automation tasks: get max numeric value from AUTO-XXX pattern
INSERT OR IGNORE INTO sequences (name, current_value)
SELECT 'auto_task', COALESCE(MAX(CAST(SUBSTR(id, 6) AS INTEGER)), 0)
FROM tasks WHERE id LIKE 'AUTO-%';

-- Insert zero values for any sequences that don't exist yet (empty tables)
INSERT OR IGNORE INTO sequences (name, current_value) VALUES ('workflow_run', 0);
INSERT OR IGNORE INTO sequences (name, current_value) VALUES ('task', 0);
INSERT OR IGNORE INTO sequences (name, current_value) VALUES ('initiative', 0);
INSERT OR IGNORE INTO sequences (name, current_value) VALUES ('auto_task', 0);
