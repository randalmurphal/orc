package executor

import "testing"

func TestParseProviderModel(t *testing.T) {
	tests := []struct {
		input        string
		wantProvider string
		wantModel    string
	}{
		{"codex:gpt-5", "codex", "gpt-5"},
		{"ollama/qwen2.5-14b", "claude", "ollama/qwen2.5-14b"}, // slash not a delimiter
		{"codex:ollama/qwen2.5", "codex", "ollama/qwen2.5"},
		{"opus", "claude", "opus"},             // bare model defaults to claude
		{"claude:sonnet", "claude", "sonnet"},
		{"", "", ""},
		{"anthropic:opus", "claude", "opus"},   // alias normalized
		{"openai:gpt-5", "codex", "gpt-5"},    // alias normalized
		{"  sonnet  ", "claude", "sonnet"},     // whitespace trimmed
		{"CODEX:gpt-5", "codex", "gpt-5"},     // case insensitive provider
		{"lmstudio:llama3", "lmstudio", "llama3"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			provider, model := ParseProviderModel(tt.input)
			if provider != tt.wantProvider {
				t.Errorf("ParseProviderModel(%q) provider = %q, want %q", tt.input, provider, tt.wantProvider)
			}
			if model != tt.wantModel {
				t.Errorf("ParseProviderModel(%q) model = %q, want %q", tt.input, model, tt.wantModel)
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
		{"ollama", "qwen2.5-14b", "ollama:qwen2.5-14b"},
		{"claude", "sonnet", "sonnet"},   // claude provider omitted
		{"", "opus", "opus"},             // empty provider omitted
		{"codex", "ollama/qwen2.5", "codex:ollama/qwen2.5"},
		{"lmstudio", "llama3", "lmstudio:llama3"},
	}

	for _, tt := range tests {
		t.Run(tt.provider+":"+tt.model, func(t *testing.T) {
			got := FormatProviderModel(tt.provider, tt.model)
			if got != tt.want {
				t.Errorf("FormatProviderModel(%q, %q) = %q, want %q", tt.provider, tt.model, got, tt.want)
			}
		})
	}
}

func TestNormalizeProvider(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", "claude"},
		{"anthropic", "claude"},
		{"Anthropic", "claude"},
		{"ANTHROPIC", "claude"},
		{"openai", "codex"},
		{"OpenAI", "codex"},
		{"claude", "claude"},
		{"CLAUDE", "claude"},
		{"codex", "codex"},
		{"CODEX", "codex"},
		{"ollama", "ollama"},
		{"lmstudio", "lmstudio"},
		{"  codex  ", "codex"},  // whitespace trimmed
		{"unknown", "unknown"}, // unknown providers pass through
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeProvider(tt.input)
			if got != tt.want {
				t.Errorf("normalizeProvider(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsCodexFamilyProvider(t *testing.T) {
	tests := []struct {
		provider string
		want     bool
	}{
		{"codex", true},
		{"ollama", true},
		{"lmstudio", true},
		{"CODEX", true},     // case insensitive via normalizeProvider
		{"openai", true},    // openai normalizes to codex
		{"claude", false},
		{"anthropic", false}, // anthropic normalizes to claude
		{"", false},          // empty normalizes to claude
		{"unknown", false},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			got := isCodexFamilyProvider(tt.provider)
			if got != tt.want {
				t.Errorf("isCodexFamilyProvider(%q) = %v, want %v", tt.provider, got, tt.want)
			}
		})
	}
}

func TestExplicitProviderFromModelTuple(t *testing.T) {
	tests := []struct {
		model        string
		wantProvider string
		wantOK       bool
	}{
		{"codex:gpt-5", "codex", true},
		{"ollama:qwen2.5", "ollama", true},
		{"openai:gpt-5", "codex", true},     // alias normalized
		{"anthropic:opus", "claude", true},   // alias normalized
		{"opus", "", false},                  // bare model, no tuple
		{"", "", false},                      // empty
		{"ollama/qwen2.5", "", false},        // slash is not a provider delimiter
		{"  codex:gpt-5  ", "codex", true},   // whitespace trimmed
		{"lmstudio:llama3", "lmstudio", true},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			provider, ok := explicitProviderFromModelTuple(tt.model)
			if provider != tt.wantProvider || ok != tt.wantOK {
				t.Errorf("explicitProviderFromModelTuple(%q) = (%q, %v), want (%q, %v)",
					tt.model, provider, ok, tt.wantProvider, tt.wantOK)
			}
		})
	}
}

func TestLocalCodexProvider(t *testing.T) {
	tests := []struct {
		provider string
		want     string
	}{
		{"ollama", "ollama"},
		{"lmstudio", "lmstudio"},
		{"codex", ""},
		{"claude", ""},
		{"", ""},
		{"OLLAMA", "ollama"}, // case insensitive
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			got := localCodexProvider(tt.provider)
			if got != tt.want {
				t.Errorf("localCodexProvider(%q) = %q, want %q", tt.provider, got, tt.want)
			}
		})
	}
}

func TestValidateProviderCapabilities(t *testing.T) {
	we := &WorkflowExecutor{}

	t.Run("claude_always_passes", func(t *testing.T) {
		cfg := &PhaseClaudeConfig{
			InlineAgents: map[string]InlineAgentDef{
				"reviewer": {Description: "reviews code"},
			},
		}
		err := we.validateProviderCapabilities("claude", "review", cfg)
		if err != nil {
			t.Errorf("expected nil, got %v", err)
		}
	})

	t.Run("codex_with_agents_fails", func(t *testing.T) {
		cfg := &PhaseClaudeConfig{
			InlineAgents: map[string]InlineAgentDef{
				"reviewer": {Description: "reviews code"},
			},
		}
		err := we.validateProviderCapabilities("codex", "review", cfg)
		if err == nil {
			t.Error("expected error for codex with inline agents")
		}
	})

	t.Run("codex_with_agents_and_folding_passes", func(t *testing.T) {
		cfg := &PhaseClaudeConfig{
			InlineAgents: map[string]InlineAgentDef{
				"reviewer": {Description: "reviews code"},
			},
			AllowAgentFolding: true,
		}
		err := we.validateProviderCapabilities("codex", "review", cfg)
		if err != nil {
			t.Errorf("expected nil with AllowAgentFolding, got %v", err)
		}
	})

	t.Run("codex_without_agents_passes", func(t *testing.T) {
		cfg := &PhaseClaudeConfig{}
		err := we.validateProviderCapabilities("codex", "implement", cfg)
		if err != nil {
			t.Errorf("expected nil, got %v", err)
		}
	})

	t.Run("codex_nil_config_passes", func(t *testing.T) {
		err := we.validateProviderCapabilities("codex", "implement", nil)
		if err != nil {
			t.Errorf("expected nil, got %v", err)
		}
	})

	t.Run("ollama_with_agents_fails", func(t *testing.T) {
		cfg := &PhaseClaudeConfig{
			InlineAgents: map[string]InlineAgentDef{
				"reviewer": {Description: "reviews code"},
			},
		}
		err := we.validateProviderCapabilities("ollama", "review", cfg)
		if err == nil {
			t.Error("expected error for ollama (codex family) with inline agents")
		}
	})

	t.Run("lmstudio_with_agents_fails", func(t *testing.T) {
		cfg := &PhaseClaudeConfig{
			InlineAgents: map[string]InlineAgentDef{
				"reviewer": {Description: "reviews code"},
			},
		}
		err := we.validateProviderCapabilities("lmstudio", "review", cfg)
		if err == nil {
			t.Error("expected error for lmstudio (codex family) with inline agents")
		}
	})
}
