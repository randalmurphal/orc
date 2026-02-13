package bench

import (
	"testing"
)

func TestDefaultRubric(t *testing.T) {
	tests := []struct {
		phaseID      string
		wantCriteria int
	}{
		{"spec", 4},
		{"tiny_spec", 4},
		{"tdd_write", 4},
		{"implement", 4},
		{"review", 4},
		{"docs", 4},
		{"unknown", 4}, // Falls through to default
	}

	for _, tt := range tests {
		rubric := DefaultRubric(tt.phaseID)
		if len(rubric.Criteria) != tt.wantCriteria {
			t.Errorf("DefaultRubric(%s) has %d criteria, want %d", tt.phaseID, len(rubric.Criteria), tt.wantCriteria)
		}
		if rubric.MaxScore != 5 {
			t.Errorf("DefaultRubric(%s) MaxScore = %d, want 5", tt.phaseID, rubric.MaxScore)
		}
	}
}

func TestParseJudgeResponse(t *testing.T) {
	rubric := DefaultRubric("implement")

	tests := []struct {
		name    string
		content string
		wantErr bool
	}{
		{
			name: "valid json",
			content: `Here's my evaluation:
{"scores":{"correctness":4,"completeness":5,"code_quality":3,"efficiency":4},"reasoning":"Good implementation"}`,
			wantErr: false,
		},
		{
			name: "json in code block",
			content: "```json\n{\"scores\":{\"correctness\":5,\"completeness\":4,\"code_quality\":5,\"efficiency\":4},\"reasoning\":\"Excellent\"}\n```",
			wantErr: false,
		},
		{
			name:    "no json",
			content: "This is just text with no JSON",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := parseJudgeResponse(tt.content, rubric)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseJudgeResponse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && resp != nil {
				if len(resp.Scores) == 0 {
					t.Error("expected scores in response")
				}
				// Check scores are within range
				for _, score := range resp.Scores {
					if score < 1 || score > 5 {
						t.Errorf("score %d out of range [1, 5]", score)
					}
				}
			}
		})
	}
}

func TestBuildJudgePrompt(t *testing.T) {
	req := JudgeRequest{
		PhaseID:       "implement",
		TaskTitle:     "Fix page split",
		TaskDesc:      "The page splitting algorithm fails on large keys",
		OutputContent: "func splitPage() { ... }",
		Rubric:        DefaultRubric("implement"),
		BlindedLabel:  "Output-42",
	}

	prompt := buildJudgePrompt(req)

	// Check key elements are present
	checks := []string{
		"expert code reviewer",
		"Fix page split",
		"Output-42",          // Blinded label, not variant name
		"correctness",        // Rubric criteria
		"completeness",
		"code_quality",
		"efficiency",
		"1-5",                // Score range
	}

	for _, check := range checks {
		if !contains(prompt, check) {
			t.Errorf("prompt missing expected content: %q", check)
		}
	}
}

func TestShouldJudge(t *testing.T) {
	jp := &JudgePanel{}

	// Opus judges codex outputs only
	opusJudge := JudgeConfig{
		Provider:        "claude",
		Model:           "opus",
		JudgesProviders: []string{"codex"},
	}

	if !jp.shouldJudge(opusJudge, "codex", "gpt-5.3-codex") {
		t.Error("opus should judge codex outputs")
	}
	if jp.shouldJudge(opusJudge, "claude", "sonnet") {
		t.Error("opus should NOT judge claude outputs")
	}

	// Sonnet judges everything except itself
	sonnetJudge := JudgeConfig{
		Provider:        "claude",
		Model:           "sonnet",
		JudgesProviders: nil,
	}

	if !jp.shouldJudge(sonnetJudge, "codex", "gpt-5.3-codex") {
		t.Error("sonnet should judge codex outputs")
	}
	if !jp.shouldJudge(sonnetJudge, "claude", "opus") {
		t.Error("sonnet should judge opus outputs")
	}
	// Self-evaluation guard: sonnet must NOT judge sonnet
	if jp.shouldJudge(sonnetJudge, "claude", "sonnet") {
		t.Error("sonnet should NOT judge its own outputs (self-evaluation bias)")
	}

	// GPT should not judge its own outputs
	gptJudge := JudgeConfig{
		Provider:        "codex",
		Model:           "gpt-5.3-codex",
		JudgesProviders: []string{"claude"},
	}
	if jp.shouldJudge(gptJudge, "codex", "gpt-5.3-codex") {
		t.Error("GPT should NOT judge its own outputs")
	}
	if !jp.shouldJudge(gptJudge, "claude", "opus") {
		t.Error("GPT should judge claude outputs")
	}
}

func TestAggregateJudgments(t *testing.T) {
	judgments := []*Judgment{
		{Scores: map[string]int{"correctness": 4, "completeness": 5}},
		{Scores: map[string]int{"correctness": 2, "completeness": 3}},
	}

	agg := AggregateJudgments(judgments)

	if agg["correctness"] != 3.0 {
		t.Errorf("expected avg correctness 3.0, got %f", agg["correctness"])
	}
	if agg["completeness"] != 4.0 {
		t.Errorf("expected avg completeness 4.0, got %f", agg["completeness"])
	}
}

func TestAggregateJudgments_Empty(t *testing.T) {
	agg := AggregateJudgments(nil)
	if agg != nil {
		t.Errorf("expected nil for empty judgments, got %v", agg)
	}
}

func TestSanitizeForBlinding(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		notWant  []string // These strings should NOT appear in output
	}{
		{
			name:    "commit co-author line",
			input:   "git commit -m 'fix bug'\n\nCo-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>",
			notWant: []string{"Claude", "Anthropic", "anthropic.com"},
		},
		{
			name:    "model name in code comment",
			input:   "// Generated by GPT-5.3-codex\nfunc main() {}",
			notWant: []string{"GPT-5.3", "codex"},
		},
		{
			name:    "mixed providers",
			input:   "Claude Sonnet generated this spec. Reviewed by GPT-5.2.",
			notWant: []string{"Claude Sonnet", "GPT-5.2"},
		},
		{
			name:    "orc commit prefix",
			input:   "[orc] TASK-001: implement - completed",
			notWant: []string{"[orc]"},
		},
		{
			name:    "clean content unchanged",
			input:   "func splitPage(data []byte) error {\n\treturn nil\n}",
			notWant: nil, // Nothing to redact
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeForBlinding(tt.input)
			for _, bad := range tt.notWant {
				if stringContains(result, bad) {
					t.Errorf("sanitized output still contains %q:\n%s", bad, result)
				}
			}
			// Should still have meaningful content (not empty)
			if len(tt.input) > 0 && len(result) == 0 {
				t.Error("sanitization produced empty output")
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && stringContains(s, substr)
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
