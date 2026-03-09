package cli

import (
	"testing"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestResetCommand_ClearsFreshRunState(t *testing.T) {
	backendIface, tmpDir := createEditTestBackend(t)
	backend := backendIface.(*storage.DatabaseBackend)

	origDir := setupTestWorkDir(t, tmpDir)
	defer restoreWorkDir(t, origDir)

	tk := task.NewProtoTask("TASK-001", "Reset me")
	task.SetWorkflowIDProto(tk, "implement-medium")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_CLOSED
	tk.StartedAt = timestamppb.Now()
	tk.CompletedAt = timestamppb.Now()
	tk.Metadata = map[string]string{
		"closed":             "true",
		"closed_at":          "2026-03-01T00:00:00Z",
		"phase:review:model": "opus",
		"completion_skipped": "no_changes",
		"completion_note":    "stale",
		"worktree_was_dirty": "true",
		"_retry_state":       `{"from_phase":"review"}`,
	}
	tk.Execution.Phases["review"] = &orcv1.PhaseState{
		Status:    orcv1.PhaseStatus_PHASE_STATUS_COMPLETED,
		StartedAt: timestamppb.Now(),
		SessionId: testStringPtr("session-1"),
		Tokens:    &orcv1.TokenUsage{TotalTokens: 99},
	}
	tk.Execution.Gates = []*orcv1.GateDecision{{Phase: "review"}}
	tk.Execution.Tokens = &orcv1.TokenUsage{TotalTokens: 99}
	tk.Execution.Cost = &orcv1.CostTracking{TotalCostUsd: 4.2}
	tk.Quality = &orcv1.QualityMetrics{
		PhaseRetries:       map[string]int32{"review": 2},
		ManualIntervention: true,
	}
	tk.Pr = &orcv1.PRInfo{
		Url:    testStringPtr("https://example.com/pr/1"),
		Number: testCLIInt32Ptr(1),
		Status: orcv1.PRStatus_PR_STATUS_PENDING_REVIEW,
	}
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}
	if err := backend.SetTaskExecutor("TASK-001", 99999, "test-host"); err != nil {
		t.Fatalf("set task executor: %v", err)
	}

	cmd := newResetCmd()
	cmd.SetArgs([]string{"TASK-001", "--force"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute reset command: %v", err)
	}

	reloaded, err := backend.LoadTask("TASK-001")
	if err != nil {
		t.Fatalf("reload task: %v", err)
	}

	if reloaded.Status != orcv1.TaskStatus_TASK_STATUS_PLANNED {
		t.Fatalf("status = %v, want planned", reloaded.Status)
	}
	if reloaded.StartedAt != nil || reloaded.CompletedAt != nil {
		t.Fatalf("timestamps should be cleared, got started=%v completed=%v", reloaded.StartedAt, reloaded.CompletedAt)
	}
	if reloaded.ExecutorPid != 0 || reloaded.ExecutorHostname != nil || reloaded.LastHeartbeat != nil {
		t.Fatalf("executor state should be cleared, got pid=%d host=%v heartbeat=%v", reloaded.ExecutorPid, reloaded.ExecutorHostname, reloaded.LastHeartbeat)
	}
	if len(reloaded.Execution.Phases) != 0 {
		t.Fatalf("execution phases should be empty after reset, got %d", len(reloaded.Execution.Phases))
	}
	if len(reloaded.Execution.Gates) != 0 {
		t.Fatalf("execution gates should be empty after reset, got %d", len(reloaded.Execution.Gates))
	}
	if reloaded.Execution.Tokens == nil || reloaded.Execution.Tokens.TotalTokens != 0 {
		t.Fatalf("execution tokens should be cleared, got %+v", reloaded.Execution.Tokens)
	}
	if reloaded.Execution.Cost == nil || reloaded.Execution.Cost.TotalCostUsd != 0 {
		t.Fatalf("execution cost should be cleared, got %+v", reloaded.Execution.Cost)
	}
	if reloaded.Pr != nil {
		t.Fatalf("PR info should be cleared, got %+v", reloaded.Pr)
	}
	if reloaded.Quality != nil {
		t.Fatalf("quality metrics should be cleared, got %+v", reloaded.Quality)
	}
	if !task.HasFreshResetMarkerProto(reloaded) || len(reloaded.Metadata) != 1 {
		t.Fatalf("expected only fresh reset marker after reset, got %+v", reloaded.Metadata)
	}
	if !task.IsFreshRunProto(reloaded) {
		t.Fatal("task should be recognized as a fresh run after reset")
	}
}
