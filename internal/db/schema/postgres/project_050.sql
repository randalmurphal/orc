-- Project database migration 050: Add completion_action to workflows
-- Allows workflows to specify a completion action: "pr", "commit", "none", or "" (inherit from config).

ALTER TABLE workflows ADD COLUMN completion_action TEXT DEFAULT '';
