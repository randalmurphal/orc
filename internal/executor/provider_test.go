package executor

import "testing"

func TestParseProviderModel(t *testing.T) {
	tests := []struct {
		input        string
		wantProvider string
		wantModel    string
	}{
		{"codex:gpt-5", "codex", "gpt-5"},
		{"ollama/qwen2.5-14b", "", "ollama/qwen2.5-14b"},
		{"codex:ollama/qwen2.5", "codex", "ollama/qwen2.5"},
		{"opus", "", "opus"},
		{"claude:sonnet", "claude", "sonnet"},
		{"", "", ""},
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
