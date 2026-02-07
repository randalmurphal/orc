// Regression test for token accumulation in the unified executeWithProvider() loop.
// The old Codex path was missing cache token accumulation. The unified loop now
// accumulates all 4 token fields (InputTokens, OutputTokens, CacheCreationInputTokens,
// CacheReadInputTokens) plus CostUSD uniformly across all providers and iterations.
package executor

import (
	"context"
	"log/slog"
	"math"
	"testing"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/storage"
)

// TestTokenAccumulation_CacheTokensAcrossIterations verifies that the shared
// executeWithProvider loop accumulates all 4 token fields and CostUSD correctly
// across multiple iterations (continue + complete). This is a regression test:
// the old Codex-specific path silently dropped CacheCreationInputTokens and
// CacheReadInputTokens.
func TestTokenAccumulation_CacheTokensAcrossIterations(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	mock := &MockTurnExecutor{
		Responses: []string{
			`{"status": "continue", "reason": "still working"}`,
			`{"status": "complete", "summary": "done"}`,
		},
		UsageOverrides: []*orcv1.TokenUsage{
			{
				InputTokens:              100,
				OutputTokens:             50,
				CacheCreationInputTokens: 30,
				CacheReadInputTokens:     20,
			},
			{
				InputTokens:              200,
				OutputTokens:             80,
				CacheCreationInputTokens: 40,
				CacheReadInputTokens:     60,
			},
		},
		CostOverrides:  []float64{0.25, 0.50},
		SessionIDValue: "test-session",
	}

	we := NewWorkflowExecutor(
		backend, nil, nil, &config.Config{}, t.TempDir(),
		WithWorkflowTurnExecutor(mock),
		WithWorkflowLogger(slog.Default()),
	)

	cfg := PhaseExecutionConfig{
		PhaseID:       "implement",
		PhaseTemplate: &db.PhaseTemplate{ID: "implement"},
	}

	adapter := &claudeAdapter{}
	result, err := we.executeWithProvider(context.Background(), cfg, adapter)
	if err != nil {
		t.Fatalf("executeWithProvider returned error: %v", err)
	}

	// Verify iteration count
	if result.Iterations != 2 {
		t.Errorf("Iterations = %d, want 2", result.Iterations)
	}

	// Verify standard token accumulation
	if result.InputTokens != 300 { // 100 + 200
		t.Errorf("InputTokens = %d, want 300", result.InputTokens)
	}
	if result.OutputTokens != 130 { // 50 + 80
		t.Errorf("OutputTokens = %d, want 130", result.OutputTokens)
	}

	// Verify cache token accumulation (the regression target)
	if result.CacheCreationTokens != 70 { // 30 + 40
		t.Errorf("CacheCreationTokens = %d, want 70", result.CacheCreationTokens)
	}
	if result.CacheReadTokens != 80 { // 20 + 60
		t.Errorf("CacheReadTokens = %d, want 80", result.CacheReadTokens)
	}

	// Verify cost accumulation (0.25 + 0.50 = 0.75, exact in IEEE 754)
	if result.CostUSD != 0.75 {
		t.Errorf("CostUSD = %f, want 0.75", result.CostUSD)
	}
}

// TestTokenAccumulation_NilUsage verifies that iterations with nil Usage
// don't corrupt the running totals (no panic, no incorrect accumulation).
func TestTokenAccumulation_NilUsage(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	mock := &MockTurnExecutor{
		Responses: []string{
			`{"status": "continue", "reason": "working"}`,
			`{"status": "complete", "summary": "done"}`,
		},
		UsageOverrides: []*orcv1.TokenUsage{
			{
				InputTokens:              100,
				OutputTokens:             50,
				CacheCreationInputTokens: 25,
				CacheReadInputTokens:     15,
			},
			nil, // Second iteration has no usage data
		},
		CostOverrides:  []float64{0.25, 0.50},
		SessionIDValue: "test-session",
	}

	we := NewWorkflowExecutor(
		backend, nil, nil, &config.Config{}, t.TempDir(),
		WithWorkflowTurnExecutor(mock),
		WithWorkflowLogger(slog.Default()),
	)

	cfg := PhaseExecutionConfig{
		PhaseID:       "implement",
		PhaseTemplate: &db.PhaseTemplate{ID: "implement"},
	}

	adapter := &claudeAdapter{}
	result, err := we.executeWithProvider(context.Background(), cfg, adapter)
	if err != nil {
		t.Fatalf("executeWithProvider returned error: %v", err)
	}

	// Only iteration 1 tokens should be present
	if result.InputTokens != 100 {
		t.Errorf("InputTokens = %d, want 100", result.InputTokens)
	}
	if result.OutputTokens != 50 {
		t.Errorf("OutputTokens = %d, want 50", result.OutputTokens)
	}
	if result.CacheCreationTokens != 25 {
		t.Errorf("CacheCreationTokens = %d, want 25", result.CacheCreationTokens)
	}
	if result.CacheReadTokens != 15 {
		t.Errorf("CacheReadTokens = %d, want 15", result.CacheReadTokens)
	}

	// Cost still accumulates from both iterations
	if math.Abs(result.CostUSD-0.75) > 1e-9 {
		t.Errorf("CostUSD = %f, want 0.75", result.CostUSD)
	}
}

// TestTokenAccumulation_SingleIteration verifies accumulation works for
// a single-iteration (immediate complete) case with cache tokens.
func TestTokenAccumulation_SingleIteration(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	mock := &MockTurnExecutor{
		Responses: []string{
			`{"status": "complete", "summary": "done immediately"}`,
		},
		UsageOverrides: []*orcv1.TokenUsage{
			{
				InputTokens:              500,
				OutputTokens:             200,
				CacheCreationInputTokens: 100,
				CacheReadInputTokens:     300,
			},
		},
		CostOverrides:  []float64{1.25},
		SessionIDValue: "test-session",
	}

	we := NewWorkflowExecutor(
		backend, nil, nil, &config.Config{}, t.TempDir(),
		WithWorkflowTurnExecutor(mock),
		WithWorkflowLogger(slog.Default()),
	)

	cfg := PhaseExecutionConfig{
		PhaseID:       "implement",
		PhaseTemplate: &db.PhaseTemplate{ID: "implement"},
	}

	adapter := &claudeAdapter{}
	result, err := we.executeWithProvider(context.Background(), cfg, adapter)
	if err != nil {
		t.Fatalf("executeWithProvider returned error: %v", err)
	}

	if result.Iterations != 1 {
		t.Errorf("Iterations = %d, want 1", result.Iterations)
	}
	if result.InputTokens != 500 {
		t.Errorf("InputTokens = %d, want 500", result.InputTokens)
	}
	if result.OutputTokens != 200 {
		t.Errorf("OutputTokens = %d, want 200", result.OutputTokens)
	}
	if result.CacheCreationTokens != 100 {
		t.Errorf("CacheCreationTokens = %d, want 100", result.CacheCreationTokens)
	}
	if result.CacheReadTokens != 300 {
		t.Errorf("CacheReadTokens = %d, want 300", result.CacheReadTokens)
	}
	if result.CostUSD != 1.25 {
		t.Errorf("CostUSD = %f, want 1.25", result.CostUSD)
	}
}
