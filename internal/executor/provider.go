package executor

import (
	"fmt"
	"strings"

	llmkit "github.com/randalmurphal/llmkit/v2"
	"github.com/randalmurphal/orc/internal/db"
)

// Known LLM providers supported by orc.
const (
	ProviderClaude = "claude"
	ProviderCodex  = "codex"
)

// normalizeProvider lowercases and trims provider names.
func normalizeProvider(provider string) string {
	return strings.ToLower(strings.TrimSpace(provider))
}

// isCodexFamilyProvider returns true for providers that use the Codex CLI executor
// (codex itself).
func isCodexFamilyProvider(provider string) bool {
	switch normalizeProvider(provider) {
	case ProviderCodex:
		return true
	default:
		return false
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
// The "/" separator (e.g., "vendor/model") is NOT parsed -- it's treated as a bare model name.
//
// Bare model names default to provider "claude".
//
// Examples:
//
//	"codex:gpt-5"           -> ("codex", "gpt-5")
//	"opus"                  -> ("claude", "opus")                  // bare model, defaults to claude
//	"claude:sonnet"         -> ("claude", "sonnet")
//	"codex:gpt-5"           -> ("codex", "gpt-5")
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

// validateProvider returns an error if the provider is not a known LLM provider.
func validateProvider(provider string) error {
	def, ok := llmkit.GetProviderDefinition(provider)
	if !ok {
		return fmt.Errorf("unknown provider %q (supported: claude, codex)", provider)
	}
	if !def.Supported {
		return fmt.Errorf("provider %q is not supported", provider)
	}
	return nil
}

// providerDefaultModel returns the sensible default model for a provider.
// Used as the final fallback in model resolution before the hard-coded "opus".
func providerDefaultModel(provider string) string {
	switch normalizeProvider(provider) {
	case ProviderClaude:
		return "opus"
	case ProviderCodex:
		return "gpt-5.3-codex"
	default:
		return ""
	}
}
