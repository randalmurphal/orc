-- Project database migration 049: Add default_max_iterations to workflows
-- Allows workflows to specify a default max iterations for all phases.

ALTER TABLE workflows ADD COLUMN default_max_iterations INTEGER DEFAULT 0;
