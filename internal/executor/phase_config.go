// Package executor provides phase execution for workflows.
package executor

import (
	"encoding/json"
	"fmt"
	"maps"

	llmkit "github.com/randalmurphal/llmkit/v2"
)

// PhaseRuntimeConfig contains provider-neutral runtime configuration for a phase.
//
// Configuration is resolved in priority order:
//  1. workflow_phases.runtime_config_override (per-workflow override)
//  2. phase_templates.runtime_config (template default)
//  3. executor agent runtime_config (base)
type PhaseRuntimeConfig struct {
	Shared    llmkit.SharedRuntimeConfig `json:"shared,omitempty"`
	Providers PhaseRuntimeProviderConfig `json:"providers,omitempty"`
}

type PhaseRuntimeProviderConfig struct {
	Claude *llmkit.ClaudeRuntimeConfig `json:"claude,omitempty"`
	Codex  *llmkit.CodexRuntimeConfig  `json:"codex,omitempty"`
}

type HookMatcher = llmkit.HookMatcher
type HookEntry = llmkit.HookEntry
type InlineAgentDef = llmkit.InlineAgentDef

// WorktreeBaseConfig contains base configuration for worktree setup.
type WorktreeBaseConfig struct {
	WorktreePath  string
	MainRepoPath  string
	TaskID        string
	AdditionalEnv map[string]string
}

// ParsePhaseRuntimeConfig parses a JSON string into PhaseRuntimeConfig.
// Returns nil for empty input.
func ParsePhaseRuntimeConfig(raw string) (*PhaseRuntimeConfig, error) {
	if raw == "" {
		return nil, nil
	}
	var cfg PhaseRuntimeConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return nil, fmt.Errorf("parse phase runtime config: %w", err)
	}
	return &cfg, nil
}

// Merge combines two configs, with override taking precedence for non-empty values.
func (p *PhaseRuntimeConfig) Merge(override *PhaseRuntimeConfig) *PhaseRuntimeConfig {
	if override == nil {
		return p
	}
	if p == nil {
		return override
	}

	result := *p
	result.Shared = mergeSharedRuntimeConfig(p.Shared, override.Shared)
	result.Providers = mergeProviderRuntimeConfig(p.Providers, override.Providers)
	return &result
}

func mergeSharedRuntimeConfig(base, override llmkit.SharedRuntimeConfig) llmkit.SharedRuntimeConfig {
	result := base

	if override.SystemPrompt != "" {
		result.SystemPrompt = override.SystemPrompt
	}
	if override.AppendSystemPrompt != "" {
		result.AppendSystemPrompt = override.AppendSystemPrompt
	}
	if len(override.AllowedTools) > 0 {
		result.AllowedTools = append([]string(nil), override.AllowedTools...)
	}
	if len(override.DisallowedTools) > 0 {
		result.DisallowedTools = append([]string(nil), override.DisallowedTools...)
	}
	if len(override.Tools) > 0 {
		result.Tools = append([]string(nil), override.Tools...)
	}
	if len(override.MCPServers) > 0 {
		if result.MCPServers == nil {
			result.MCPServers = make(map[string]llmkit.MCPServerConfig)
		}
		maps.Copy(result.MCPServers, override.MCPServers)
	}
	if override.StrictMCPConfig {
		result.StrictMCPConfig = true
	}
	if override.MaxBudgetUSD > 0 {
		result.MaxBudgetUSD = override.MaxBudgetUSD
	}
	if override.MaxTurns > 0 {
		result.MaxTurns = override.MaxTurns
	}
	if len(override.Env) > 0 {
		if result.Env == nil {
			result.Env = make(map[string]string)
		}
		maps.Copy(result.Env, override.Env)
	}
	if len(override.AddDirs) > 0 {
		result.AddDirs = append([]string(nil), override.AddDirs...)
	}

	return result
}

func mergeProviderRuntimeConfig(base, override PhaseRuntimeProviderConfig) PhaseRuntimeProviderConfig {
	result := base
	result.Claude = mergeClaudeRuntimeConfig(base.Claude, override.Claude)
	result.Codex = mergeCodexRuntimeConfig(base.Codex, override.Codex)
	return result
}

func mergeClaudeRuntimeConfig(base, override *llmkit.ClaudeRuntimeConfig) *llmkit.ClaudeRuntimeConfig {
	if override == nil {
		return base
	}
	if base == nil {
		cp := *override
		return &cp
	}

	result := *base
	if override.SystemPromptFile != "" {
		result.SystemPromptFile = override.SystemPromptFile
	}
	if override.AppendSystemPromptFile != "" {
		result.AppendSystemPromptFile = override.AppendSystemPromptFile
	}
	if len(override.SkillRefs) > 0 {
		result.SkillRefs = append(result.SkillRefs, override.SkillRefs...)
	}
	if override.AgentRef != "" {
		result.AgentRef = override.AgentRef
	}
	if len(override.InlineAgents) > 0 {
		if result.InlineAgents == nil {
			result.InlineAgents = make(map[string]llmkit.InlineAgentDef)
		}
		maps.Copy(result.InlineAgents, override.InlineAgents)
	}
	if len(base.Hooks) > 0 || len(override.Hooks) > 0 {
		merged := make(map[string][]llmkit.HookMatcher)
		for event, matchers := range base.Hooks {
			cp := make([]llmkit.HookMatcher, len(matchers))
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

func mergeCodexRuntimeConfig(base, override *llmkit.CodexRuntimeConfig) *llmkit.CodexRuntimeConfig {
	if override == nil {
		return base
	}
	if base == nil {
		cp := *override
		return &cp
	}
	result := *base
	if override.ReasoningEffort != "" {
		result.ReasoningEffort = override.ReasoningEffort
	}
	if override.WebSearchMode != "" {
		result.WebSearchMode = override.WebSearchMode
	}
	return &result
}

// ToLLMKit returns the llmkit runtime contract for execution and preparation.
func (p *PhaseRuntimeConfig) ToLLMKit() llmkit.RuntimeConfig {
	if p == nil {
		return llmkit.RuntimeConfig{}
	}
	return llmkit.RuntimeConfig{
		Shared: p.Shared,
		Providers: llmkit.RuntimeProviderConfig{
			Claude: p.Providers.Claude,
			Codex:  p.Providers.Codex,
		},
	}
}

// IsEmpty returns true if the config has no meaningful values set.
func (p *PhaseRuntimeConfig) IsEmpty() bool {
	if p == nil {
		return true
	}
	return p.Shared.SystemPrompt == "" &&
		p.Shared.AppendSystemPrompt == "" &&
		len(p.Shared.AllowedTools) == 0 &&
		len(p.Shared.DisallowedTools) == 0 &&
		len(p.Shared.Tools) == 0 &&
		len(p.Shared.MCPServers) == 0 &&
		!p.Shared.StrictMCPConfig &&
		p.Shared.MaxBudgetUSD == 0 &&
		p.Shared.MaxTurns == 0 &&
		len(p.Shared.Env) == 0 &&
		len(p.Shared.AddDirs) == 0 &&
		claudeRuntimeConfigEmpty(p.Providers.Claude) &&
		codexRuntimeConfigEmpty(p.Providers.Codex)
}

func claudeRuntimeConfigEmpty(cfg *llmkit.ClaudeRuntimeConfig) bool {
	if cfg == nil {
		return true
	}
	return cfg.SystemPromptFile == "" &&
		cfg.AppendSystemPromptFile == "" &&
		len(cfg.SkillRefs) == 0 &&
		cfg.AgentRef == "" &&
		len(cfg.InlineAgents) == 0 &&
		len(cfg.Hooks) == 0
}

func codexRuntimeConfigEmpty(cfg *llmkit.CodexRuntimeConfig) bool {
	if cfg == nil {
		return true
	}
	return cfg.ReasoningEffort == "" &&
		cfg.WebSearchMode == ""
}

// JSON returns the config as a JSON string for database storage.
func (p *PhaseRuntimeConfig) JSON() string {
	if p == nil || p.IsEmpty() {
		return ""
	}
	b, _ := json.Marshal(p)
	return string(b)
}

// InlineAgentsJSON returns the Claude inline agents as a JSON string for llmkit consumers.
func (p *PhaseRuntimeConfig) InlineAgentsJSON() string {
	if p == nil || p.Providers.Claude == nil || len(p.Providers.Claude.InlineAgents) == 0 {
		return ""
	}
	b, _ := json.Marshal(p.Providers.Claude.InlineAgents)
	return string(b)
}
