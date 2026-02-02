-- Project database migration 051: Add target_branch to workflows
-- Allows workflows to specify a default target branch for PRs: "" (inherit from config), or branch name.

ALTER TABLE workflows ADD COLUMN target_branch TEXT DEFAULT '';
