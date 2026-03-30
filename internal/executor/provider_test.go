package executor

import (
	"testing"

	llmkit "github.com/randalmurphal/llmkit/v2"
)

func TestParseProviderModel(t *testing.T) {
	tests := []struct {
		input        string
		wantProvider string
		wantModel    string
	}{
		{"codex:gpt-5", "codex", "gpt-5"},
		{"opus", "claude", "opus"},
		{"claude:sonnet", "claude", "sonnet"},
		{"", "", ""},
		{"  sonnet  ", "claude", "sonnet"},
		{"CODEX:gpt-5", "codex", "gpt-5"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			provider, model := ParseProviderModel(tt.input)
			if provider != tt.wantProvider || model != tt.wantModel {
				t.Fatalf("ParseProviderModel(%q) = (%q, %q), want (%q, %q)",
					tt.input, provider, model, tt.wantProvider, tt.wantModel)
			}
		})
	}
}

func TestFormatProviderModel(t *testing.T) {
	tests := []struct {
		provider string
		model    string
		want     string
	}{
		{"codex", "gpt-5", "codex:gpt-5"},
		{"claude", "sonnet", "sonnet"},
		{"", "opus", "opus"},
	}

	for _, tt := range tests {
		t.Run(tt.provider+":"+tt.model, func(t *testing.T) {
			if got := FormatProviderModel(tt.provider, tt.model); got != tt.want {
				t.Fatalf("FormatProviderModel(%q, %q) = %q, want %q", tt.provider, tt.model, got, tt.want)
			}
		})
	}
}

func TestNormalizeProvider(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"claude", "claude"},
		{"codex", "codex"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := normalizeProvider(tt.input); got != tt.want {
				t.Fatalf("normalizeProvider(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsCodexFamilyProvider(t *testing.T) {
	if !isCodexFamilyProvider("codex") {
		t.Fatal("codex should be a codex-family provider")
	}
	if isCodexFamilyProvider("claude") {
		t.Fatal("claude should not be a codex-family provider")
	}
}

func TestExplicitProviderFromModelTuple(t *testing.T) {
	provider, ok := explicitProviderFromModelTuple("codex:gpt-5")
	if provider != "codex" || !ok {
		t.Fatalf("explicitProviderFromModelTuple returned (%q, %v), want (codex, true)", provider, ok)
	}

	provider, ok = explicitProviderFromModelTuple("opus")
	if provider != "" || ok {
		t.Fatalf("explicitProviderFromModelTuple returned (%q, %v), want ('', false)", provider, ok)
	}
}

func TestValidateProvider(t *testing.T) {
	for _, provider := range []string{"claude", "codex"} {
		if err := validateProvider(provider); err != nil {
			t.Fatalf("validateProvider(%q) returned error: %v", provider, err)
		}
	}

	for _, provider := range []string{"ollama", "lmstudio", "foobar"} {
		if err := validateProvider(provider); err == nil {
			t.Fatalf("validateProvider(%q) should have failed", provider)
		}
	}
}

func TestValidateProvider_RejectsFutureLLMKitProvidersUntilOrcAdoptsThem(t *testing.T) {
	const provider = "future-test-provider"
	llmkit.RegisterProviderDefinition(llmkit.ProviderDefinition{
		Name:      provider,
		Supported: true,
	})

	if err := validateProvider(provider); err == nil {
		t.Fatalf("validateProvider(%q) should reject providers not explicitly adopted by orc", provider)
	}
}

func TestValidateProviderCapabilities(t *testing.T) {
	claudeCfg := llmkit.RuntimeConfig{
		Providers: llmkit.RuntimeProviderConfig{
			Claude: &llmkit.ClaudeRuntimeConfig{
				InlineAgents: map[string]llmkit.InlineAgentDef{
					"reviewer": {Description: "reviews code"},
				},
			},
		},
	}
	if err := llmkit.ValidateRuntimeConfig("claude", claudeCfg); err != nil {
		t.Fatalf("claude should allow inline agents: %v", err)
	}
	if err := llmkit.ValidateRuntimeConfig("codex", claudeCfg); err == nil {
		t.Fatal("codex should reject claude-specific runtime config")
	}
}
