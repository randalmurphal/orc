// Package executor provides phase execution for workflows.
package executor

import (
	"fmt"

	"github.com/randalmurphal/llmkit/claudeconfig"
)

// AgentResolver resolves agent configurations from project settings.
type AgentResolver struct {
	projectRoot string
	claudeDir   string
}

// NewAgentResolver creates an agent resolver for the given project.
func NewAgentResolver(projectRoot, claudeDir string) *AgentResolver {
	return &AgentResolver{
		projectRoot: projectRoot,
		claudeDir:   claudeDir,
	}
}

// ResolveAgentConfig loads an agent by reference and merges its config into PhaseClaudeConfig.
// If agentRef is empty, no changes are made.
// The agent's config is merged with lower priority than explicit phase config values.
func (r *AgentResolver) ResolveAgentConfig(cfg *PhaseClaudeConfig) error {
	if cfg == nil || cfg.AgentRef == "" {
		return nil
	}

	svc := claudeconfig.NewAgentService(r.projectRoot)
	agent, err := svc.Get(cfg.AgentRef)
	if err != nil {
		return fmt.Errorf("get agent %q: %w", cfg.AgentRef, err)
	}

	// Merge agent config into PhaseClaudeConfig
	// Agent values only apply if not already set in the config (config takes precedence)

	// System prompt - agent provides if not set
	if cfg.SystemPrompt == "" && agent.Prompt != "" {
		cfg.SystemPrompt = agent.Prompt
	}

	// Tool restrictions - agent provides if not set
	if agent.Tools != nil {
		if len(cfg.AllowedTools) == 0 && len(agent.Tools.Allow) > 0 {
			cfg.AllowedTools = make([]string, len(agent.Tools.Allow))
			copy(cfg.AllowedTools, agent.Tools.Allow)
		}
		if len(cfg.DisallowedTools) == 0 && len(agent.Tools.Deny) > 0 {
			cfg.DisallowedTools = make([]string, len(agent.Tools.Deny))
			copy(cfg.DisallowedTools, agent.Tools.Deny)
		}
	}

	// Skills - append agent's skills (additive, not override)
	if len(agent.SkillRefs) > 0 {
		// Add agent skills, avoiding duplicates
		seen := make(map[string]bool)
		for _, s := range cfg.SkillRefs {
			seen[s] = true
		}
		for _, s := range agent.SkillRefs {
			if !seen[s] {
				cfg.SkillRefs = append(cfg.SkillRefs, s)
				seen[s] = true
			}
		}
	}

	return nil
}

// ResolveAgentForConfig is a convenience function that resolves an agent
// and loads any skills it references.
func (r *AgentResolver) ResolveAgentForConfig(cfg *PhaseClaudeConfig) error {
	if cfg == nil {
		return nil
	}

	// First resolve the agent reference
	if err := r.ResolveAgentConfig(cfg); err != nil {
		return err
	}

	// Then load skills (which may have been added by the agent)
	if len(cfg.SkillRefs) > 0 {
		loader := NewSkillLoader(r.claudeDir)
		if err := loader.LoadSkillsForConfig(cfg); err != nil {
			return fmt.Errorf("load skills for agent: %w", err)
		}
	}

	return nil
}

// ResolveAgentConfigSimple is a convenience function for resolving an agent without an AgentResolver instance.
func ResolveAgentConfigSimple(projectRoot, claudeDir string, cfg *PhaseClaudeConfig) error {
	resolver := NewAgentResolver(projectRoot, claudeDir)
	return resolver.ResolveAgentForConfig(cfg)
}
