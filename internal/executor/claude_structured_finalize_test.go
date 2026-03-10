package executor

import (
	"context"
	"fmt"
	"strings"
	"testing"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
)

type structuredFinalizeMockTurnExecutor struct {
	noSchemaResults []*TurnResult
	noSchemaErrors  []error
	schemaResults   []*TurnResult
	schemaErrors    []error

	noSchemaPrompts []string
	schemaPrompts   []string
	schemaSessions  []string
	updateCalls     []string
	sessionID       string
}

func (m *structuredFinalizeMockTurnExecutor) ExecuteTurn(ctx context.Context, prompt string) (*TurnResult, error) {
	m.schemaPrompts = append(m.schemaPrompts, prompt)
	m.schemaSessions = append(m.schemaSessions, m.sessionID)

	var result *TurnResult
	if len(m.schemaResults) > 0 {
		result = m.schemaResults[0]
		m.schemaResults = m.schemaResults[1:]
	}

	var err error
	if len(m.schemaErrors) > 0 {
		err = m.schemaErrors[0]
		m.schemaErrors = m.schemaErrors[1:]
	}

	if result != nil && result.SessionID == "" {
		result.SessionID = m.sessionID
	}
	return result, err
}

func (m *structuredFinalizeMockTurnExecutor) ExecuteTurnWithoutSchema(ctx context.Context, prompt string) (*TurnResult, error) {
	m.noSchemaPrompts = append(m.noSchemaPrompts, prompt)

	var result *TurnResult
	if len(m.noSchemaResults) > 0 {
		result = m.noSchemaResults[0]
		m.noSchemaResults = m.noSchemaResults[1:]
	}

	var err error
	if len(m.noSchemaErrors) > 0 {
		err = m.noSchemaErrors[0]
		m.noSchemaErrors = m.noSchemaErrors[1:]
	}

	if result != nil && result.SessionID == "" {
		result.SessionID = m.sessionID
	}
	return result, err
}

func (m *structuredFinalizeMockTurnExecutor) UpdateSessionID(id string) {
	m.updateCalls = append(m.updateCalls, id)
	m.sessionID = id
}

func (m *structuredFinalizeMockTurnExecutor) SessionID() string {
	return m.sessionID
}

func TestExecuteClaudeStructuredFinalize_UsesAnalysisThenStructuredFinalize(t *testing.T) {
	t.Parallel()

	mock := &structuredFinalizeMockTurnExecutor{
		noSchemaResults: []*TurnResult{
			{
				Content:   "Review complete. Ready to finalize.",
				SessionID: "review-session",
				Usage: &orcv1.TokenUsage{
					InputTokens:  100,
					OutputTokens: 40,
				},
				CostUSD: 0.25,
			},
		},
		schemaResults: []*TurnResult{
			{
				Content:   `{"needs_changes": false, "round": 1, "summary": "Looks good", "issues": []}`,
				SessionID: "review-session",
				Usage: &orcv1.TokenUsage{
					InputTokens:  20,
					OutputTokens: 10,
				},
				CostUSD: 0.10,
			},
		},
	}

	cfg := PhaseExecutionConfig{
		PhaseID:     "review",
		ReviewRound: 1,
	}

	finalTurn, turns, err := executeClaudeStructuredFinalize(context.Background(), mock, cfg, "review prompt")
	if err != nil {
		t.Fatalf("executeClaudeStructuredFinalize returned error: %v", err)
	}

	if got := len(mock.noSchemaPrompts); got != 1 {
		t.Fatalf("no-schema calls = %d, want 1", got)
	}
	if got := len(mock.schemaPrompts); got != 1 {
		t.Fatalf("schema calls = %d, want 1", got)
	}
	if got := len(turns); got != 2 {
		t.Fatalf("turn count = %d, want 2", got)
	}
	if turns[0].Content != "Review complete. Ready to finalize." {
		t.Fatalf("analysis content = %q", turns[0].Content)
	}
	if finalTurn == nil {
		t.Fatal("finalTurn is nil")
	}
	if finalTurn.Content != `{"needs_changes": false, "round": 1, "summary": "Looks good", "issues": []}` {
		t.Fatalf("final content = %q", finalTurn.Content)
	}
	if !strings.Contains(mock.schemaPrompts[0], "Return the final structured review result now") {
		t.Fatalf("finalize prompt missing instruction: %q", mock.schemaPrompts[0])
	}
	if got := len(mock.updateCalls); got != 1 {
		t.Fatalf("session update calls = %d, want 1", got)
	}
	if got := mock.updateCalls[0]; got != "review-session" {
		t.Fatalf("session update call = %q, want %q", got, "review-session")
	}
	if got := len(mock.schemaSessions); got != 1 {
		t.Fatalf("schema session count = %d, want 1", got)
	}
	if got := mock.schemaSessions[0]; got != "review-session" {
		t.Fatalf("schema call session = %q, want %q", got, "review-session")
	}
}

func TestExecuteClaudeStructuredFinalize_RetriesFinalizeWithoutRepeatingAnalysis(t *testing.T) {
	t.Parallel()

	mock := &structuredFinalizeMockTurnExecutor{
		noSchemaResults: []*TurnResult{
			{
				Content:   "Deep review completed. Final answer pending.",
				SessionID: "review-session",
				Usage: &orcv1.TokenUsage{
					InputTokens:  80,
					OutputTokens: 30,
				},
				CostUSD: 0.20,
			},
		},
		schemaResults: []*TurnResult{
			{
				SessionID: "review-session",
				Usage: &orcv1.TokenUsage{
					InputTokens:  5,
					OutputTokens: 2,
				},
				CostUSD: 0.01,
			},
			{
				Content:   `{"needs_changes": false, "round": 1, "summary": "Looks good", "issues": []}`,
				SessionID: "review-session",
				Usage: &orcv1.TokenUsage{
					InputTokens:  10,
					OutputTokens: 4,
				},
				CostUSD: 0.02,
			},
		},
		schemaErrors: []error{
			fmt.Errorf("claude complete: JSON schema was specified but no structured output received (num_turns=1, content=%q)", "PASS"),
			nil,
		},
	}

	cfg := PhaseExecutionConfig{
		PhaseID:     "review",
		ReviewRound: 1,
	}

	finalTurn, turns, err := executeClaudeStructuredFinalize(context.Background(), mock, cfg, "review prompt")
	if err != nil {
		t.Fatalf("executeClaudeStructuredFinalize returned error: %v", err)
	}

	if got := len(mock.noSchemaPrompts); got != 1 {
		t.Fatalf("no-schema calls = %d, want 1", got)
	}
	if got := len(mock.schemaPrompts); got != 2 {
		t.Fatalf("schema calls = %d, want 2", got)
	}
	if got := len(turns); got != 3 {
		t.Fatalf("turn count = %d, want 3", got)
	}
	if finalTurn == nil {
		t.Fatal("finalTurn is nil")
	}
	if finalTurn.Content != `{"needs_changes": false, "round": 1, "summary": "Looks good", "issues": []}` {
		t.Fatalf("final content = %q", finalTurn.Content)
	}
	if mock.schemaPrompts[0] == mock.schemaPrompts[1] {
		t.Fatal("expected retry finalize prompt to include prior error context")
	}
	if !strings.Contains(mock.schemaPrompts[1], "Previous finalize error:") {
		t.Fatalf("retry finalize prompt missing prior error: %q", mock.schemaPrompts[1])
	}
	if got := len(mock.updateCalls); got != 1 {
		t.Fatalf("session update calls = %d, want 1", got)
	}
	if got := len(mock.schemaSessions); got != 2 {
		t.Fatalf("schema session count = %d, want 2", got)
	}
	for i, sessionID := range mock.schemaSessions {
		if sessionID != "review-session" {
			t.Fatalf("schema session %d = %q, want %q", i, sessionID, "review-session")
		}
	}
}

func TestShouldUseClaudeStructuredFinalize(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		cfg     PhaseExecutionConfig
		adapter ProviderAdapter
		want    bool
	}{
		{
			name:    "review on claude",
			cfg:     PhaseExecutionConfig{PhaseID: "review"},
			adapter: &claudeAdapter{},
			want:    true,
		},
		{
			name:    "review cross on claude",
			cfg:     PhaseExecutionConfig{PhaseID: "review_cross"},
			adapter: &claudeAdapter{},
			want:    true,
		},
		{
			name:    "implement on claude",
			cfg:     PhaseExecutionConfig{PhaseID: "implement"},
			adapter: &claudeAdapter{},
			want:    false,
		},
		{
			name:    "review on codex",
			cfg:     PhaseExecutionConfig{PhaseID: "review"},
			adapter: &codexAdapter{},
			want:    false,
		},
		{
			name: "nil adapter",
			cfg:  PhaseExecutionConfig{PhaseID: "review"},
			want: false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := shouldUseClaudeStructuredFinalize(tc.cfg, tc.adapter)
			if got != tc.want {
				t.Fatalf("shouldUseClaudeStructuredFinalize() = %t, want %t", got, tc.want)
			}
		})
	}
}

func TestIsClaudeStructuredFinalizeRetryableError(t *testing.T) {
	t.Parallel()

	if isClaudeStructuredFinalizeRetryableError(nil) {
		t.Fatal("nil error should not be retryable")
	}

	retryable := []error{
		fmt.Errorf("claude complete: JSON schema was specified but no structured output received"),
		fmt.Errorf("structured_output is empty"),
		fmt.Errorf("phase completion JSON parse failed: unexpected end of JSON input"),
	}

	for _, err := range retryable {
		if !isClaudeStructuredFinalizeRetryableError(err) {
			t.Fatalf("expected retryable error: %v", err)
		}
	}

	nonRetryable := fmt.Errorf("context deadline exceeded")
	if isClaudeStructuredFinalizeRetryableError(nonRetryable) {
		t.Fatalf("expected non-retryable error: %v", nonRetryable)
	}
}
