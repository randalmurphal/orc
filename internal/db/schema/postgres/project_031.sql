-- Add current_iteration column to tasks table
-- Stores the iteration number for the current phase (1-based)

ALTER TABLE tasks ADD COLUMN current_iteration INTEGER DEFAULT 0;
