package task

import (
	"testing"

	llmkit "github.com/randalmurphal/llmkit/v2"
	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
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

func TestResetExecutionStateProto_ClearsAllExecutionData(t *testing.T) {
	exec := InitProtoExecutionState()
	EnsurePhaseProto(exec, "implement")
	exec.Phases["implement"].StartedAt = timestamppb.Now()
	exec.Phases["implement"].Status = orcv1.PhaseStatus_PHASE_STATUS_COMPLETED
	exec.Gates = []*orcv1.GateDecision{{Phase: "implement"}}
	exec.Tokens = &orcv1.TokenUsage{TotalTokens: 42}
	exec.Cost = &orcv1.CostTracking{TotalCostUsd: 1.5}
	exec.Error = testStrPtr("boom")

	ResetExecutionStateProto(exec)

	if len(exec.Phases) != 0 {
		t.Fatalf("phase state should be empty after reset, got %d phases", len(exec.Phases))
	}
	if len(exec.Gates) != 0 {
		t.Fatalf("gate decisions should be cleared, got %d", len(exec.Gates))
	}
	if exec.Tokens == nil || exec.Tokens.TotalTokens != 0 {
		t.Fatalf("execution tokens should be reset, got %+v", exec.Tokens)
	}
	if exec.Cost == nil || exec.Cost.TotalCostUsd != 0 {
		t.Fatalf("execution cost should be reset, got %+v", exec.Cost)
	}
	if exec.Error != nil {
		t.Fatalf("execution error should be cleared, got %q", *exec.Error)
	}
}

func TestResetTaskForFreshRunProto_ClearsRuntimeState(t *testing.T) {
	now := timestamppb.Now()
	task := NewProtoTask("TASK-001", "Reset me")
	task.Status = orcv1.TaskStatus_TASK_STATUS_FAILED
	task.StartedAt = now
	task.CompletedAt = now
	task.ExecutorPid = 1234
	host := "executor-host"
	task.ExecutorHostname = &host
	task.LastHeartbeat = now
	task.Pr = &orcv1.PRInfo{
		Url:    testStrPtr("https://example.com/pr/1"),
		Number: testInt32Ptr(1),
		Status: orcv1.PRStatus_PR_STATUS_PENDING_REVIEW,
	}
	task.Metadata = map[string]string{
		"closed":                   "true",
		"closed_at":                "2026-03-01T00:00:00Z",
		"completion_skipped":       "no_changes",
		"phase:implement:provider": "codex",
		"_retry_state":             `{"from_phase":"review"}`,
	}
	task.Quality = &orcv1.QualityMetrics{
		PhaseRetries:             map[string]int32{"review": 2},
		ReviewRejections:         1,
		ManualIntervention:       true,
		ManualInterventionReason: testStrPtr("manual"),
		TotalRetries:             2,
	}
	task.Execution = &orcv1.ExecutionState{
		Phases: map[string]*orcv1.PhaseState{
			"review": {
				Status:          orcv1.PhaseStatus_PHASE_STATUS_COMPLETED,
				StartedAt:       now,
				SessionMetadata: strPtr(mustSessionMetadata(t, "claude", "session-1")),
				Tokens:          &orcv1.TokenUsage{TotalTokens: 10},
			},
		},
		Gates:  []*orcv1.GateDecision{{Phase: "review"}},
		Tokens: &orcv1.TokenUsage{TotalTokens: 10},
		Cost:   &orcv1.CostTracking{TotalCostUsd: 2.3},
	}

	ResetTaskForFreshRunProto(task)

	if task.Status != orcv1.TaskStatus_TASK_STATUS_PLANNED {
		t.Fatalf("status = %v, want planned", task.Status)
	}
	if task.StartedAt != nil || task.CompletedAt != nil {
		t.Fatalf("timestamps should be cleared, got started=%v completed=%v", task.StartedAt, task.CompletedAt)
	}
	if task.ExecutorPid != 0 || task.ExecutorHostname != nil || task.LastHeartbeat != nil {
		t.Fatalf("executor fields not cleared: pid=%d host=%v heartbeat=%v", task.ExecutorPid, task.ExecutorHostname, task.LastHeartbeat)
	}
	if task.Pr != nil {
		t.Fatalf("PR info should be cleared, got %+v", task.Pr)
	}
	if task.Quality != nil {
		t.Fatalf("quality metrics should be cleared, got %+v", task.Quality)
	}
	if len(task.Metadata) != 1 || task.Metadata[freshResetMarkerKey] != "true" {
		t.Fatalf("expected only fresh reset marker in metadata, got %+v", task.Metadata)
	}
	if !IsFreshRunProto(task) {
		t.Fatal("task should be recognized as a fresh run after reset")
	}
}

func testStrPtr(v string) *string { return &v }

func testInt32Ptr(v int32) *int32 { return &v }

func mustSessionMetadata(t *testing.T, provider, sessionID string) string {
	t.Helper()
	metadata, err := llmkit.MarshalSessionMetadata(llmkit.SessionMetadataForID(provider, sessionID))
	if err != nil {
		t.Fatalf("marshal session metadata: %v", err)
	}
	return metadata
}
