package executor

import (
	"fmt"
	"strings"

	"github.com/randalmurphal/orc/internal/db"
)

// Known LLM providers supported by orc.
const (
	ProviderClaude  = "claude"
	ProviderCodex   = "codex"
	ProviderOllama  = "ollama"
	ProviderLMStudio = "lmstudio"
)

// normalizeProvider lowercases, trims, and maps aliases to canonical provider names.
func normalizeProvider(provider string) string {
	p := strings.ToLower(strings.TrimSpace(provider))
	switch p {
	case "", "anthropic":
		return "claude"
	case "openai":
		return "codex"
	default:
		return p
	}
}

// isCodexFamilyProvider returns true for providers that use the Codex CLI executor
// (codex itself, or local inference servers routed through codex).
func isCodexFamilyProvider(provider string) bool {
	switch normalizeProvider(provider) {
	case "codex", "ollama", "lmstudio":
		return true
	default:
		return false
	}
}

// localCodexProvider returns the local inference provider name ("ollama" or "lmstudio")
// if the provider is a local server, or empty string for cloud providers.
func localCodexProvider(provider string) string {
	switch normalizeProvider(provider) {
	case "ollama":
		return "ollama"
	case "lmstudio":
		return "lmstudio"
	default:
		return ""
	}
}

// explicitProviderFromModelTuple extracts a provider from a "provider:model" string.
// Returns the normalized provider and true if a provider prefix was found.
func explicitProviderFromModelTuple(model string) (string, bool) {
	model = strings.TrimSpace(model)
	idx := strings.Index(model, ":")
	if idx <= 0 {
		return "", false
	}
	p := normalizeProvider(model[:idx])
	if p == "" {
		return "", false
	}
	return p, true
}

// ParseProviderModel splits a "provider:model" string into its components.
// Only the ":" separator is recognized as a provider prefix delimiter.
// The "/" separator (e.g., "ollama/model") is NOT parsed -- it's treated as a bare model name.
//
// Bare model names default to provider "claude" for backward compatibility.
//
// Examples:
//
//	"codex:gpt-5"           -> ("codex", "gpt-5")
//	"ollama/qwen2.5-14b"    -> ("claude", "ollama/qwen2.5-14b")  // slash is not a provider delimiter
//	"codex:ollama/qwen2.5"  -> ("codex", "ollama/qwen2.5")
//	"opus"                  -> ("claude", "opus")                  // bare model, defaults to claude
//	"claude:sonnet"         -> ("claude", "sonnet")
//	"anthropic:opus"        -> ("claude", "opus")                  // alias normalized
//	"openai:gpt-5"          -> ("codex", "gpt-5")                 // alias normalized
func ParseProviderModel(s string) (provider, model string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", ""
	}

	idx := strings.Index(s, ":")
	if idx <= 0 {
		return "claude", s
	}

	provider = normalizeProvider(s[:idx])
	model = strings.TrimSpace(s[idx+1:])
	if provider == "" {
		provider = "claude"
	}
	return provider, model
}

// FormatProviderModel combines a provider and model into a "provider:model" string.
// If provider is empty or "claude", returns the model name alone for backward compatibility.
func FormatProviderModel(provider, model string) string {
	if provider == "" || provider == ProviderClaude {
		return model
	}
	return provider + ":" + model
}

// resolvePhaseProvider determines which LLM provider to use for a phase.
//
// Priority chain (first non-empty wins):
//  1. Run-level provider override (--provider flag)
//  2. Workflow phase provider override (per-workflow customization)
//  3. Workflow default_provider
//  4. Phase template provider
//  5. Executor agent provider
//  6. Config default provider
//  7. Provider extracted from model tuple fields (fallback):
//     a. phase.ModelOverride
//     b. workflow.DefaultModel
//     c. agent.Model
//     d. config.Model
//  8. "claude" (ultimate fallback)
func (we *WorkflowExecutor) resolvePhaseProvider(tmpl *db.PhaseTemplate, phase *db.WorkflowPhase) string {
	// Run-level provider override (--provider flag)
	if we.runProvider != "" {
		return normalizeProvider(we.runProvider)
	}

	// Workflow phase provider override (per-workflow customization)
	if phase.ProviderOverride != "" {
		return normalizeProvider(phase.ProviderOverride)
	}

	// Workflow default_provider
	if we.wf != nil && we.wf.DefaultProvider != "" {
		return normalizeProvider(we.wf.DefaultProvider)
	}

	// Phase template provider
	if tmpl != nil && tmpl.Provider != "" {
		return normalizeProvider(tmpl.Provider)
	}

	// Executor agent provider
	if agent := we.resolveExecutorAgent(tmpl, phase); agent != nil && agent.Provider != "" {
		return normalizeProvider(agent.Provider)
	}

	// Config default provider
	if we.orcConfig != nil && we.orcConfig.Provider != "" {
		return normalizeProvider(we.orcConfig.Provider)
	}

	// Fallback: extract provider from model tuple fields.
	// This handles cases like model="codex:gpt-5" where the provider is embedded in the model string.
	if phase != nil {
		if p, ok := explicitProviderFromModelTuple(phase.ModelOverride); ok {
			return p
		}
	}
	if we.wf != nil {
		if p, ok := explicitProviderFromModelTuple(we.wf.DefaultModel); ok {
			return p
		}
	}
	if agent := we.resolveExecutorAgent(tmpl, phase); agent != nil {
		if p, ok := explicitProviderFromModelTuple(agent.Model); ok {
			return p
		}
	}
	if we.orcConfig != nil {
		if p, ok := explicitProviderFromModelTuple(we.orcConfig.Model); ok {
			return p
		}
	}

	return "claude"
}

// validProviders is the set of known LLM providers. Used by validateProvider to reject unknowns.
// NOTE: Keep in sync with config.ValidLLMProviders (the canonical list for config validation).
var validProviders = map[string]bool{
	ProviderClaude:   true,
	ProviderCodex:    true,
	ProviderOllama:   true,
	ProviderLMStudio: true,
}

// validateProvider returns an error if the provider is not a known LLM provider.
// Must be called after normalizeProvider (which maps aliases like "anthropic" → "claude").
func validateProvider(provider string) error {
	if validProviders[provider] {
		return nil
	}
	return fmt.Errorf("unknown provider %q (supported: claude, codex, ollama, lmstudio)", provider)
}

// providerDefaultModel returns the sensible default model for a provider.
// Used as the final fallback in model resolution before the hard-coded "opus".
func providerDefaultModel(provider string) string {
	switch normalizeProvider(provider) {
	case ProviderClaude:
		return "opus"
	case ProviderCodex:
		return "gpt-5.3-codex"
	case ProviderOllama, ProviderLMStudio:
		// No universal default — caller should check config.Providers.Ollama.DefaultModel
		return ""
	default:
		return ""
	}
}

// validateProviderCapabilities checks that the resolved provider supports the
// features required by the phase configuration. Returns an error if the provider
// cannot handle the phase's requirements (e.g., codex-family providers don't
// support inline agents unless agent folding is enabled).
func (we *WorkflowExecutor) validateProviderCapabilities(provider string, phaseID string, cfg *PhaseClaudeConfig) error {
	if !isCodexFamilyProvider(provider) {
		return nil
	}
	if cfg != nil && len(cfg.InlineAgents) > 0 && !cfg.AllowAgentFolding {
		return fmt.Errorf("phase %q requires inline agents which codex does not support; set allow_agent_folding: true or use claude provider", phaseID)
	}
	return nil
}
