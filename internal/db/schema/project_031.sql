-- Add PR Info Storage
-- Stores pull request information (number, status, checks, etc.) in the tasks table.

ALTER TABLE tasks ADD COLUMN pr_info TEXT;
