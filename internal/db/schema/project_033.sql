-- Migration 033: Add agents and phase_agents tables for multi-agent phase execution
-- Enables database-backed agent definitions that get passed to Claude CLI as sub-agents

-- Agent definitions (the actual agent content)
-- These get passed to Claude CLI via --agents JSON as sub-agents
CREATE TABLE IF NOT EXISTS agents (
    id TEXT PRIMARY KEY,                        -- 'code-reviewer', 'silent-failure-hunter', etc.
    name TEXT NOT NULL,                         -- Display name
    description TEXT NOT NULL,                  -- When to use (required by Claude CLI)
    prompt TEXT NOT NULL,                       -- System prompt (required by Claude CLI)
    tools TEXT,                                 -- JSON array: ["Read", "Grep", "Edit"]
    model TEXT,                                 -- 'opus', 'sonnet', 'haiku' (optional override)
    is_builtin BOOLEAN DEFAULT FALSE,           -- True for built-in agents
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);

-- Phase-agent associations (which agents run for which phases)
-- Sequence 0 = parallel execution, different sequences = sequential
CREATE TABLE IF NOT EXISTS phase_agents (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    phase_template_id TEXT NOT NULL,            -- References phase_templates.id
    agent_id TEXT NOT NULL,                     -- References agents.id
    sequence INTEGER NOT NULL DEFAULT 0,        -- Execution order (same sequence = parallel)
    role TEXT,                                  -- 'correctness', 'architecture', 'security', etc.
    weight_filter TEXT,                         -- JSON array: ["medium", "large"] or null for all
    is_builtin BOOLEAN DEFAULT FALSE,           -- True for built-in associations
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now')),
    UNIQUE(phase_template_id, agent_id),
    FOREIGN KEY (phase_template_id) REFERENCES phase_templates(id) ON DELETE CASCADE,
    FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_phase_agents_phase ON phase_agents(phase_template_id);
CREATE INDEX IF NOT EXISTS idx_phase_agents_agent ON phase_agents(agent_id);

-- Add system_prompt to phase_templates for orchestration prompts
-- This is passed via --system-prompt to the main phase executor
ALTER TABLE phase_templates ADD COLUMN system_prompt TEXT;
