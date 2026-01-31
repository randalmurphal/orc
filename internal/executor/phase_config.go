// Package executor provides phase execution for workflows.
package executor

import (
	"encoding/json"
	"fmt"
	"maps"

	"github.com/randalmurphal/llmkit/claude"
)

// PhaseClaudeConfig contains all Claude CLI configuration options that can be
// set per-phase. This enables fine-grained control over Claude's behavior
// for different workflow phases.
//
// Configuration is resolved in priority order:
//  1. workflow_phases.claude_config_override (per-workflow override)
//  2. phase_templates.claude_config (template default)
//  3. Global config fallback
type PhaseClaudeConfig struct {
	// System prompts (inline)
	SystemPrompt       string `json:"system_prompt,omitempty"`        // Replace entire system prompt
	AppendSystemPrompt string `json:"append_system_prompt,omitempty"` // Append to default prompt

	// System prompts (file-based, print mode only - which orc uses)
	SystemPromptFile       string `json:"system_prompt_file,omitempty"`        // Replace from file path
	AppendSystemPromptFile string `json:"append_system_prompt_file,omitempty"` // Append from file path

	// Tool control
	AllowedTools    []string `json:"allowed_tools,omitempty"`    // Tools that execute without prompting
	DisallowedTools []string `json:"disallowed_tools,omitempty"` // Tools removed from context entirely
	Tools           []string `json:"tools,omitempty"`            // Restrict available tools (empty = none, "default" = all)

	// MCP servers
	MCPServers      map[string]claude.MCPServerConfig `json:"mcp_servers,omitempty"`
	StrictMCPConfig bool                              `json:"strict_mcp_config,omitempty"` // Only use these MCPs

	// Budget & limits (print mode only - which orc uses)
	MaxBudgetUSD float64 `json:"max_budget_usd,omitempty"` // Maximum spend in USD
	MaxTurns     int     `json:"max_turns,omitempty"`      // Maximum conversation turns (0 = no limit)

	// Environment
	Env     map[string]string `json:"env,omitempty"`      // Environment variables
	AddDirs []string          `json:"add_dirs,omitempty"` // Additional directories Claude can access

	// Skills
	SkillRefs []string `json:"skill_refs,omitempty"` // Skill names to load and inject

	// Agent assignment
	AgentRef     string                      `json:"agent_ref,omitempty"`     // --agent: Use existing agent by name
	InlineAgents map[string]InlineAgentDef `json:"inline_agents,omitempty"` // --agents: Define subagents inline

	// Hook handling — maps event type (e.g. "PreToolUse") to matchers.
	// Matches Claude Code's settings.json hooks format.
	Hooks map[string][]HookMatcher `json:"hooks,omitempty"`
}

// HookMatcher defines a single hook matcher entry in Claude Code's settings.json.
// Each matcher specifies a tool pattern and one or more hook commands to run.
type HookMatcher struct {
	Matcher string      `json:"matcher,omitempty"`
	Hooks   []HookEntry `json:"hooks"`
}

// HookEntry defines a single hook command within a HookMatcher.
type HookEntry struct {
	Type    string `json:"type"`              // "command" or "prompt"
	Command string `json:"command,omitempty"` // For type: "command"
	Prompt  string `json:"prompt,omitempty"`  // For type: "prompt"
	Timeout int    `json:"timeout,omitempty"` // Optional timeout in seconds
	Once    bool   `json:"once,omitempty"`    // Fire once then remove
}

// WorktreeBaseConfig contains base configuration for worktree setup.
// Replaces the old ClaudeCodeHookConfig. Used by ApplyPhaseSettings to
// configure the worktree's .claude/ directory with safety-critical settings.
type WorktreeBaseConfig struct {
	// WorktreePath is the absolute path to the worktree.
	WorktreePath string

	// MainRepoPath is the absolute path to the main repository.
	MainRepoPath string

	// TaskID is the task identifier.
	TaskID string

	// AdditionalEnv are extra environment variables to inject.
	AdditionalEnv map[string]string
}

// InlineAgentDef matches Claude CLI's --agents JSON format for defining
// subagents inline rather than referencing existing ones.
type InlineAgentDef struct {
	Description string   `json:"description"`       // Required: when to use this agent
	Prompt      string   `json:"prompt"`            // Required: system prompt for the agent
	Tools       []string `json:"tools,omitempty"`   // Optional: tool restrictions (inherits if omitted)
	Model       string   `json:"model,omitempty"`   // Optional: sonnet, opus, haiku, or inherit
}

// ParsePhaseClaudeConfig parses a JSON string into PhaseClaudeConfig.
// Returns nil for empty input.
func ParsePhaseClaudeConfig(raw string) (*PhaseClaudeConfig, error) {
	if raw == "" {
		return nil, nil
	}
	var cfg PhaseClaudeConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return nil, fmt.Errorf("parse phase claude config: %w", err)
	}
	return &cfg, nil
}

// Merge combines two configs, with override taking precedence for non-empty values.
// Returns a new config without modifying the original.
func (p *PhaseClaudeConfig) Merge(override *PhaseClaudeConfig) *PhaseClaudeConfig {
	if override == nil {
		return p
	}
	if p == nil {
		return override
	}

	result := *p // Copy base

	// System prompts - override takes precedence
	if override.SystemPrompt != "" {
		result.SystemPrompt = override.SystemPrompt
	}
	if override.AppendSystemPrompt != "" {
		result.AppendSystemPrompt = override.AppendSystemPrompt
	}
	if override.SystemPromptFile != "" {
		result.SystemPromptFile = override.SystemPromptFile
	}
	if override.AppendSystemPromptFile != "" {
		result.AppendSystemPromptFile = override.AppendSystemPromptFile
	}

	// Tool control - override replaces (not merges)
	if len(override.AllowedTools) > 0 {
		result.AllowedTools = override.AllowedTools
	}
	if len(override.DisallowedTools) > 0 {
		result.DisallowedTools = override.DisallowedTools
	}
	if len(override.Tools) > 0 {
		result.Tools = override.Tools
	}

	// MCP servers - merge maps
	if len(override.MCPServers) > 0 {
		if result.MCPServers == nil {
			result.MCPServers = make(map[string]claude.MCPServerConfig)
		}
		maps.Copy(result.MCPServers, override.MCPServers)
	}
	if override.StrictMCPConfig {
		result.StrictMCPConfig = true
	}

	// Budget & limits
	if override.MaxBudgetUSD > 0 {
		result.MaxBudgetUSD = override.MaxBudgetUSD
	}
	if override.MaxTurns > 0 {
		result.MaxTurns = override.MaxTurns
	}

	// Environment - merge maps
	if len(override.Env) > 0 {
		if result.Env == nil {
			result.Env = make(map[string]string)
		}
		maps.Copy(result.Env, override.Env)
	}
	if len(override.AddDirs) > 0 {
		result.AddDirs = override.AddDirs
	}

	// Skills - append
	if len(override.SkillRefs) > 0 {
		result.SkillRefs = append(result.SkillRefs, override.SkillRefs...)
	}

	// Agent - override takes precedence
	if override.AgentRef != "" {
		result.AgentRef = override.AgentRef
	}
	if len(override.InlineAgents) > 0 {
		if result.InlineAgents == nil {
			result.InlineAgents = make(map[string]InlineAgentDef)
		}
		maps.Copy(result.InlineAgents, override.InlineAgents)
	}

	// Hooks - per event key, append matchers (not replace).
	// Deep-copy base hooks first to avoid mutating originals.
	if len(p.Hooks) > 0 || len(override.Hooks) > 0 {
		merged := make(map[string][]HookMatcher)
		for event, matchers := range p.Hooks {
			cp := make([]HookMatcher, len(matchers))
			copy(cp, matchers)
			merged[event] = cp
		}
		for event, matchers := range override.Hooks {
			merged[event] = append(merged[event], matchers...)
		}
		result.Hooks = merged
	}

	return &result
}

// IsEmpty returns true if the config has no meaningful values set.
func (p *PhaseClaudeConfig) IsEmpty() bool {
	if p == nil {
		return true
	}
	return p.SystemPrompt == "" &&
		p.AppendSystemPrompt == "" &&
		p.SystemPromptFile == "" &&
		p.AppendSystemPromptFile == "" &&
		len(p.AllowedTools) == 0 &&
		len(p.DisallowedTools) == 0 &&
		len(p.Tools) == 0 &&
		len(p.MCPServers) == 0 &&
		!p.StrictMCPConfig &&
		p.MaxBudgetUSD == 0 &&
		p.MaxTurns == 0 &&
		len(p.Env) == 0 &&
		len(p.AddDirs) == 0 &&
		len(p.SkillRefs) == 0 &&
		p.AgentRef == "" &&
		len(p.InlineAgents) == 0 &&
		len(p.Hooks) == 0
}

// JSON returns the config as a JSON string for database storage.
func (p *PhaseClaudeConfig) JSON() string {
	if p == nil || p.IsEmpty() {
		return ""
	}
	b, _ := json.Marshal(p)
	return string(b)
}

// InlineAgentsJSON returns the inline agents as a JSON string for --agents flag.
func (p *PhaseClaudeConfig) InlineAgentsJSON() string {
	if p == nil || len(p.InlineAgents) == 0 {
		return ""
	}
	b, _ := json.Marshal(p.InlineAgents)
	return string(b)
}
