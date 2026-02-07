package executor

import "strings"

// Known LLM providers supported by orc.
const (
	ProviderClaude = "claude"
	ProviderCodex  = "codex"
	ProviderOllama = "ollama"
)

// ParseProviderModel splits a "provider:model" string into its components.
// Only the ":" separator is recognized as a provider prefix delimiter.
// The "/" separator (e.g., "ollama/model") is NOT parsed — it's treated as a bare model name.
// Examples:
//
//	"codex:gpt-5"           → ("codex", "gpt-5")
//	"ollama/qwen2.5-14b"    → ("", "ollama/qwen2.5-14b")  // slash is not a provider delimiter
//	"codex:ollama/qwen2.5"  → ("codex", "ollama/qwen2.5")
//	"opus"                  → ("", "opus")                  // bare model, no provider
//	"claude:sonnet"         → ("claude", "sonnet")
//
// Returns empty provider string when no provider prefix is found.
func ParseProviderModel(s string) (provider, model string) {
	if s == "" {
		return "", ""
	}

	// Check for "provider:model" format
	if idx := strings.Index(s, ":"); idx > 0 {
		return s[:idx], s[idx+1:]
	}

	// No provider prefix — bare model name
	return "", s
}

// FormatProviderModel combines a provider and model into a "provider:model" string.
// If provider is empty or "claude", returns the model name alone for backward compatibility.
func FormatProviderModel(provider, model string) string {
	if provider == "" || provider == ProviderClaude {
		return model
	}
	return provider + ":" + model
}
