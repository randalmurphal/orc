package executor

import (
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	llmkit "github.com/randalmurphal/llmkit/v2"
	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

func TestCheckResumeSession_SkipsRetryTargetPhase(t *testing.T) {
	t.Parallel()

	tsk := task.NewProtoTask("TASK-RETRY-SESSION", "retry target should start fresh")
	task.EnsurePhaseProto(tsk.Execution, "implement_codex")
	tsk.Execution.Phases["implement_codex"].Status = orcv1.PhaseStatus_PHASE_STATUS_PENDING
	task.SetPhaseSessionMetadataProto(tsk.Execution, "implement_codex", mustSessionMetadata(t, "codex", "codex-session-123"))
	task.SetRetryState(tsk, "review_cross", "implement_codex", "blocking review findings", "{}", 1)

	we := &WorkflowExecutor{
		task:       tsk,
		isResuming: true,
	}

	gotSessionID, shouldResume, err := checkResumeSession(we, "implement_codex")
	if err != nil {
		t.Fatalf("checkResumeSession() error = %v", err)
	}
	if shouldResume {
		t.Fatal("retry target phase should start fresh, not resume stale session")
	}
	if gotSessionID != "" {
		t.Fatalf("sessionID = %q, want empty", gotSessionID)
	}
}

func TestCheckResumeSession_ResumesNormalPendingPhase(t *testing.T) {
	t.Parallel()

	tsk := task.NewProtoTask("TASK-RESUME-SESSION", "normal pending phase should resume")
	task.EnsurePhaseProto(tsk.Execution, "implement_codex")
	tsk.Execution.Phases["implement_codex"].Status = orcv1.PhaseStatus_PHASE_STATUS_PENDING
	sessionID := "codex-session-123"
	task.SetPhaseSessionMetadataProto(tsk.Execution, "implement_codex", mustSessionMetadata(t, "codex", sessionID))

	we := &WorkflowExecutor{
		task:       tsk,
		isResuming: true,
	}

	gotSessionID, shouldResume, err := checkResumeSession(we, "implement_codex")
	if err != nil {
		t.Fatalf("checkResumeSession() error = %v", err)
	}
	if !shouldResume {
		t.Fatal("expected normal pending phase to resume existing session")
	}
	if gotSessionID != sessionID {
		t.Fatalf("sessionID = %q, want %q", gotSessionID, sessionID)
	}
}

func TestCheckResumeSession_SkipsReviewPhaseWhenBlocked(t *testing.T) {
	t.Parallel()

	tsk := task.NewProtoTask("TASK-BLOCKED-REVIEW", "blocked review should restart fresh")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_BLOCKED
	task.EnsurePhaseProto(tsk.Execution, "review_cross")
	tsk.Execution.Phases["review_cross"].Status = orcv1.PhaseStatus_PHASE_STATUS_PENDING
	task.SetPhaseSessionMetadataProto(tsk.Execution, "review_cross", mustSessionMetadata(t, "codex", "codex-review-session-123"))

	we := &WorkflowExecutor{
		task:       tsk,
		isResuming: true,
	}

	gotSessionID, shouldResume, err := checkResumeSession(we, "review_cross")
	if err != nil {
		t.Fatalf("checkResumeSession() error = %v", err)
	}
	if shouldResume {
		t.Fatal("review phase should restart fresh, not resume stale session")
	}
	if gotSessionID != "" {
		t.Fatalf("sessionID = %q, want empty", gotSessionID)
	}
}

func TestCheckResumeSession_SkipsReviewPhaseWhenInterrupted(t *testing.T) {
	t.Parallel()

	tsk := task.NewProtoTask("TASK-INTERRUPTED-REVIEW", "interrupted review should restart fresh")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	task.EnsurePhaseProto(tsk.Execution, "review_cross")
	tsk.Execution.Phases["review_cross"].Status = orcv1.PhaseStatus_PHASE_STATUS_PENDING
	task.SetPhaseSessionMetadataProto(tsk.Execution, "review_cross", mustSessionMetadata(t, "codex", "codex-review-session-456"))

	we := &WorkflowExecutor{
		task:       tsk,
		isResuming: true,
	}

	gotSessionID, shouldResume, err := checkResumeSession(we, "review_cross")
	if err != nil {
		t.Fatalf("checkResumeSession() error = %v", err)
	}
	if shouldResume {
		t.Fatal("interrupted review phase should restart fresh, not resume stale session")
	}
	if gotSessionID != "" {
		t.Fatalf("sessionID = %q, want empty", gotSessionID)
	}
}

func TestClearRetryStateForFreshPhaseStart_ClearsMetadataOnFreshStart(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	tsk := task.NewProtoTask("TASK-CLEAR-RETRY", "clear retry state when fresh retry starts")
	task.SetRetryState(tsk, "review_cross", "implement_codex", "blocking review findings", "{}", 2)
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	we := &WorkflowExecutor{
		backend: backend,
		task:    tsk,
		logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	if err := we.clearRetryStateForFreshPhaseStart("implement_codex", false); err != nil {
		t.Fatalf("clearRetryStateForFreshPhaseStart(): %v", err)
	}

	if rs := task.GetRetryState(tsk); rs != nil {
		t.Fatalf("retry state should be cleared after fresh retry start, got %+v", rs)
	}

	loaded, err := backend.LoadTask(tsk.Id)
	if err != nil {
		t.Fatalf("load task: %v", err)
	}
	if rs := task.GetRetryState(loaded); rs != nil {
		t.Fatalf("persisted retry state should be cleared, got %+v", rs)
	}
}

func TestClearRetryStateForFreshPhaseStart_PreservesMetadataOnResume(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	tsk := task.NewProtoTask("TASK-KEEP-RETRY", "keep retry state when resuming")
	task.SetRetryState(tsk, "review_cross", "implement_codex", "blocking review findings", "{}", 2)
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	we := &WorkflowExecutor{
		backend: backend,
		task:    tsk,
		logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	if err := we.clearRetryStateForFreshPhaseStart("implement_codex", true); err != nil {
		t.Fatalf("clearRetryStateForFreshPhaseStart(): %v", err)
	}

	if rs := task.GetRetryState(tsk); rs == nil {
		t.Fatal("retry state should remain while phase is resuming")
	}
}

func TestCheckResumeSession_ErrorsOnInvalidStoredMetadata(t *testing.T) {
	t.Parallel()

	tsk := task.NewProtoTask("TASK-BAD-SESSION", "invalid session metadata should fail fast")
	task.EnsurePhaseProto(tsk.Execution, "implement")
	tsk.Execution.Phases["implement"].Status = orcv1.PhaseStatus_PHASE_STATUS_PENDING
	raw := "not-json"
	tsk.Execution.Phases["implement"].SessionMetadata = &raw

	we := &WorkflowExecutor{
		task:       tsk,
		isResuming: true,
	}

	_, _, err := checkResumeSession(we, "implement")
	if err == nil {
		t.Fatal("checkResumeSession() error = nil, want parse failure")
	}
}

func TestClaudeAdapter_PrepareExecution_FailsWhenSessionMetadataPersistenceFails(t *testing.T) {
	t.Parallel()

	tsk := task.NewProtoTask("TASK-CLAUDE-SESSION", "claude session save failure")
	adapter := &claudeAdapter{}
	we := &WorkflowExecutor{
		task:    tsk,
		backend: &failingSessionMetadataBackend{saveErr: errors.New("save failed")},
		logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	_, err := adapter.PrepareExecution(&PhaseExecutionConfig{
		Prompt:   "do work",
		PhaseID:  "implement",
		Provider: ProviderClaude,
	}, we)
	if err == nil {
		t.Fatal("expected claude session metadata persistence failure")
	}
	if got := err.Error(); got == "" || !strings.Contains(got, "persist claude session metadata") {
		t.Fatalf("error = %v, want claude session persistence failure", err)
	}
}

func TestCodexAdapter_PostTurn_FailsWhenSessionMetadataPersistenceFails(t *testing.T) {
	t.Parallel()

	tsk := task.NewProtoTask("TASK-CODEX-SESSION", "codex session save failure")
	adapter := &codexAdapter{}
	we := &WorkflowExecutor{
		task:    tsk,
		backend: &failingSessionMetadataBackend{saveErr: errors.New("save failed")},
		logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	err := adapter.PostTurn(&TurnResult{SessionID: "codex-session-123"}, nil, &PhaseExecutionConfig{
		PhaseID:  "implement",
		Provider: ProviderCodex,
	}, we)
	if err == nil {
		t.Fatal("expected codex session metadata persistence failure")
	}
	if got := err.Error(); got == "" || !strings.Contains(got, "persist codex session metadata") {
		t.Fatalf("error = %v, want codex session persistence failure", err)
	}
}

func TestCodexAdapter_BuildTurnExecutorConfig_HonorsRuntimePolicyAndSharedSettings(t *testing.T) {
	t.Parallel()

	adapter := &codexAdapter{}
	runtimeCfg := &PhaseRuntimeConfig{
		Shared: llmkit.SharedRuntimeConfig{
			SystemPrompt: "Use the shared system prompt",
			Env:          map[string]string{"FOO": "bar"},
			AddDirs:      []string{"/repo/shared"},
		},
		Providers: PhaseRuntimeProviderConfig{
			Codex: &llmkit.CodexRuntimeConfig{
				ReasoningEffort: "high",
				WebSearchMode:   "cached",
				SandboxMode:     "workspace-write",
				ApprovalMode:    "on-request",
			},
		},
	}

	teCfg := adapter.BuildTurnExecutorConfig(&PhaseExecutionConfig{
		Provider:      ProviderCodex,
		Model:         "gpt-5",
		WorkingDir:    "/repo",
		PhaseID:       "implement_codex",
		TaskID:        "TASK-001",
		RunID:         "run-1",
		RuntimeConfig: runtimeCfg,
	}, &ProviderExecContext{}, &WorkflowExecutor{
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	if teCfg.RuntimeConfig != runtimeCfg {
		t.Fatal("runtime config pointer was not propagated to codex executor")
	}
	if teCfg.BypassApprovalsAndSandbox {
		t.Fatal("expected explicit sandbox/approval policy to disable bypass mode")
	}
	if teCfg.SandboxMode != "workspace-write" || teCfg.ApprovalMode != "on-request" {
		t.Fatalf("unexpected codex policy settings: %+v", teCfg)
	}
	if teCfg.ReasoningEffort != "high" || teCfg.WebSearchMode != "cached" {
		t.Fatalf("unexpected codex model settings: %+v", teCfg)
	}
	if teCfg.Env["FOO"] != "bar" || len(teCfg.AddDirs) != 1 || teCfg.AddDirs[0] != "/repo/shared" {
		t.Fatalf("shared runtime settings not propagated: %+v", teCfg)
	}
}

func mustSessionMetadata(t *testing.T, provider, sessionID string) string {
	t.Helper()
	metadata, err := llmkit.MarshalSessionMetadata(llmkit.SessionMetadataForID(provider, sessionID))
	if err != nil {
		t.Fatalf("marshal session metadata: %v", err)
	}
	return metadata
}

type failingSessionMetadataBackend struct {
	storage.Backend
	saveErr error
}

func (b *failingSessionMetadataBackend) SaveTask(*orcv1.Task) error {
	return b.saveErr
}
