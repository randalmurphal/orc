package config

import (
	"slices"
	"strings"

	llmkit "github.com/randalmurphal/llmkit/v2"
	_ "github.com/randalmurphal/llmkit/v2/claude"
	_ "github.com/randalmurphal/llmkit/v2/codex"
)

// SupportedLLMProviders returns the supported provider names from llmkit.
func SupportedLLMProviders() []string {
	defs := llmkit.ListProviders()
	providers := make([]string, 0, len(defs))
	for _, def := range defs {
		if def.Supported {
			providers = append(providers, def.Name)
		}
	}
	slices.Sort(providers)
	return providers
}

// IsValidLLMProvider returns true when the provider is empty or supported by llmkit.
func IsValidLLMProvider(provider string) bool {
	name := strings.ToLower(strings.TrimSpace(provider))
	if name == "" {
		return true
	}
	def, ok := llmkit.GetProviderDefinition(name)
	return ok && def.Supported
}
