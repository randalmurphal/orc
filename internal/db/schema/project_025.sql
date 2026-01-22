-- Add session_id column to phases table
-- Stores Claude CLI session UUID for --resume support per phase

ALTER TABLE phases ADD COLUMN session_id TEXT;
