-- Migration 012: Add provider columns for multi-provider support
--
-- Adds provider fields to support routing phases to different LLM providers
-- (claude, codex, ollama, etc.) alongside existing model selection.
--
-- Provider resolution follows the same cascade as model:
--   workflow_phase.provider_override > workflow.default_provider > agent.provider > config > "claude"

ALTER TABLE workflows ADD COLUMN default_provider TEXT DEFAULT '';
ALTER TABLE phase_templates ADD COLUMN provider TEXT DEFAULT '';
ALTER TABLE workflow_phases ADD COLUMN provider_override TEXT DEFAULT '';
ALTER TABLE agents ADD COLUMN provider TEXT DEFAULT '';
ALTER TABLE cost_log ADD COLUMN provider TEXT DEFAULT 'claude';

CREATE INDEX IF NOT EXISTS idx_cost_provider ON cost_log(provider);
CREATE INDEX IF NOT EXISTS idx_cost_provider_timestamp ON cost_log(provider, timestamp);
