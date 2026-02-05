-- Global database migration 005: Extended gate configuration
-- Adds AI gate support, gate input/output config, gate mode, before-phase triggers,
-- and workflow-level lifecycle triggers.

-- Phase template gate configuration
ALTER TABLE phase_templates ADD COLUMN gate_input_config TEXT;   -- JSON GateInputConfig
ALTER TABLE phase_templates ADD COLUMN gate_output_config TEXT;  -- JSON GateOutputConfig
ALTER TABLE phase_templates ADD COLUMN gate_mode TEXT;           -- 'gate' or 'reaction'
ALTER TABLE phase_templates ADD COLUMN gate_agent_id TEXT;       -- References agents.id

-- Workflow phase before-triggers
ALTER TABLE workflow_phases ADD COLUMN before_triggers TEXT;     -- JSON array of BeforePhaseTrigger

-- Workflow lifecycle triggers
ALTER TABLE workflows ADD COLUMN triggers TEXT;                  -- JSON array of WorkflowTrigger
