package task

import (
	"testing"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
)

func TestSetPhaseTokensProto_RecomputesExecutionTotals(t *testing.T) {
	exec := InitProtoExecutionState()

	SetPhaseTokensProto(exec, "plan", &orcv1.TokenUsage{
		InputTokens:              10,
		OutputTokens:             5,
		CacheCreationInputTokens: 3,
		CacheReadInputTokens:     2,
	})
	SetPhaseTokensProto(exec, "implement", &orcv1.TokenUsage{
		InputTokens:  20,
		OutputTokens: 15,
		TotalTokens:  35,
	})

	if exec.Tokens == nil {
		t.Fatal("expected aggregate execution tokens")
	}
	if exec.Tokens.InputTokens != 30 {
		t.Fatalf("input tokens = %d, want 30", exec.Tokens.InputTokens)
	}
	if exec.Tokens.OutputTokens != 20 {
		t.Fatalf("output tokens = %d, want 20", exec.Tokens.OutputTokens)
	}
	if exec.Tokens.CacheCreationInputTokens != 3 {
		t.Fatalf("cache creation tokens = %d, want 3", exec.Tokens.CacheCreationInputTokens)
	}
	if exec.Tokens.CacheReadInputTokens != 2 {
		t.Fatalf("cache read tokens = %d, want 2", exec.Tokens.CacheReadInputTokens)
	}
	if exec.Tokens.TotalTokens != 55 {
		t.Fatalf("total tokens = %d, want 55", exec.Tokens.TotalTokens)
	}
}

func TestResetPhaseProto_ClearsTokens(t *testing.T) {
	exec := InitProtoExecutionState()
	SetPhaseTokensProto(exec, "implement", &orcv1.TokenUsage{
		InputTokens:  12,
		OutputTokens: 8,
	})

	ResetPhaseProto(exec, "implement")

	phase := exec.Phases["implement"]
	if phase == nil || phase.Tokens == nil {
		t.Fatal("expected phase tokens to exist after reset")
	}
	if phase.Tokens.TotalTokens != 0 || phase.Tokens.InputTokens != 0 || phase.Tokens.OutputTokens != 0 {
		t.Fatalf("phase tokens were not cleared: %+v", phase.Tokens)
	}
	if exec.Tokens.TotalTokens != 0 {
		t.Fatalf("aggregate total tokens = %d, want 0", exec.Tokens.TotalTokens)
	}
}
