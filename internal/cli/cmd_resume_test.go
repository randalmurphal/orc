package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	llmkit "github.com/randalmurphal/llmkit/v2"
	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/internal/workflow"
)

// withResumeTestDir creates a temp directory with orc initialized and changes to it
func withResumeTestDir(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()
	if err := config.InitAt(tmpDir, false); err != nil {
		t.Fatalf("failed to init orc: %v", err)
	}

	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working dir: %v", err)
	}

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	t.Cleanup(func() {
		_ = os.Chdir(origWd)
	})

	return tmpDir
}

// createResumeTestBackend creates a backend in the given directory.
func createResumeTestBackend(t *testing.T, dir string) storage.Backend {
	t.Helper()
	backend, err := storage.NewDatabaseBackend(dir, nil)
	if err != nil {
		t.Fatalf("create backend: %v", err)
	}
	return backend
}

func TestResumeCommand_TaskNotFound(t *testing.T) {
	withResumeTestDir(t)

	// Run resume command for non-existent task
	cmd := newResumeCmd()
	cmd.SetArgs([]string{"TASK-999"})

	var stderr bytes.Buffer
	cmd.SetErr(&stderr)

	err := cmd.Execute()
	if err == nil {
		t.Error("Resume should fail for non-existent task")
	}

	if err != nil && !contains([]string{err.Error()}, "task TASK-999 not found") {
		t.Errorf("Expected 'task not found' error, got: %v", err)
	}
}

// TestValidateTaskResumableProto tests the validation logic directly without running the executor.
func TestValidateTaskResumableProto(t *testing.T) {
	tests := []struct {
		name        string
		status      orcv1.TaskStatus
		forceResume bool
		wantErr     bool
		errContains string
	}{
		// Non-resumable statuses
		{
			name:        "completed task not resumable",
			status:      orcv1.TaskStatus_TASK_STATUS_COMPLETED,
			wantErr:     true,
			errContains: "cannot be resumed",
		},
		{
			name:        "created task not resumable",
			status:      orcv1.TaskStatus_TASK_STATUS_CREATED,
			wantErr:     true,
			errContains: "cannot be resumed",
		},
		// Resumable statuses
		{
			name:    "paused task is resumable",
			status:  orcv1.TaskStatus_TASK_STATUS_PAUSED,
			wantErr: false,
		},
		{
			name:    "blocked task is resumable",
			status:  orcv1.TaskStatus_TASK_STATUS_BLOCKED,
			wantErr: false,
		},
		{
			name:    "failed task is resumable",
			status:  orcv1.TaskStatus_TASK_STATUS_FAILED,
			wantErr: false,
		},
		// Running task cases - with executor tracking fields:
		// - ExecutorPid=0 means orphaned (resumable without force)
		// - ExecutorPid=alive PID means truly running (not resumable without force)
		{
			name:        "running task not resumable without force",
			status:      orcv1.TaskStatus_TASK_STATUS_RUNNING,
			forceResume: false,
			wantErr:     true,
			errContains: "currently running",
		},
		{
			name:        "running task resumable with force",
			status:      orcv1.TaskStatus_TASK_STATUS_RUNNING,
			forceResume: true,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tk := task.NewProtoTask("TASK-001", "Test task")
			tk.Status = tt.status
			task.SetCurrentPhaseProto(tk, "implement")

			// For running task tests, set a valid PID so the task appears truly running
			// (not orphaned). Otherwise CheckOrphanedProto would detect it as orphaned.
			if tt.status == orcv1.TaskStatus_TASK_STATUS_RUNNING {
				tk.ExecutorPid = int32(os.Getpid())
			}

			result, err := ValidateTaskResumableProto(tk, tt.forceResume)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error containing %q, got nil", tt.errContains)
					return
				}
				if !contains([]string{err.Error()}, tt.errContains) {
					t.Errorf("Expected error containing %q, got: %v", tt.errContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
				if result == nil {
					t.Error("Expected validation result, got nil")
				}
			}
		})
	}
}

// TestValidateTaskResumable_OrphanedTask tests orphan detection in validation.
func TestValidateTaskResumable_OrphanedTask(t *testing.T) {
	// Create a running task with no executor info (ExecutorPid=0)
	// This should be detected as orphaned and be resumable without force
	tk := task.NewProtoTask("TASK-001", "Test task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	task.SetCurrentPhaseProto(tk, "implement")
	// ExecutorPid defaults to 0, which means orphaned

	// Without force - should succeed because task is orphaned
	result, err := ValidateTaskResumableProto(tk, false)
	if err != nil {
		t.Errorf("Expected orphaned running task to be resumable without force, got: %v", err)
	}
	if result == nil {
		t.Error("Expected validation result, got nil")
	}
}

// TestValidateTaskResumable_ForceRunning tests force flag with running task.
// Uses proto types. Set ExecutorPid to a live PID so the task appears truly running.
func TestValidateTaskResumable_ForceRunning(t *testing.T) {
	tk := task.NewProtoTask("TASK-001", "Test task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	task.SetCurrentPhaseProto(tk, "implement")
	// Set to current process PID so the task appears to be running (not orphaned)
	tk.ExecutorPid = int32(os.Getpid())

	// Without force - should fail (task appears to still be running)
	_, err := ValidateTaskResumableProto(tk, false)
	if err == nil {
		t.Error("Expected error for running task without force")
	}

	// With force - should succeed
	result, err := ValidateTaskResumableProto(tk, true)
	if err != nil {
		t.Errorf("Expected no error with force flag, got: %v", err)
	}
	if result == nil {
		t.Fatal("Expected validation result, got nil")
	}
	if !result.RequiresStateUpdate {
		t.Error("Expected RequiresStateUpdate=true for force-resumed task")
	}
}

func TestApplyResumeStateUpdatesProto_OrphanedTaskSetsRetryState(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	tk := task.NewProtoTask("TASK-ORPHAN-RESUME", "orphaned task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	task.SetCurrentPhaseProto(tk, "implement_codex")
	task.EnsurePhaseProto(tk.Execution, "implement_codex")
	sessionID := "codex-session-123"
	sessionMetadata, err := llmkit.MarshalSessionMetadata(llmkit.SessionMetadataForID("codex", sessionID))
	if err != nil {
		t.Fatalf("marshal session metadata: %v", err)
	}
	tk.Execution.Phases["implement_codex"].SessionMetadata = &sessionMetadata
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	if err := backend.AddTranscript(&storage.Transcript{
		TaskID:    tk.Id,
		Phase:     "implement_codex",
		SessionID: sessionID,
		Type:      "tool_result",
		Role:      "tool",
		Content:   "golangci-lint run\nstatus: failed, exit_code: 1\ninternal/api/thread_server.go:42 missing event publish",
	}); err != nil {
		t.Fatalf("add transcript: %v", err)
	}

	result := &ResumeValidationResult{
		IsOrphaned:          true,
		OrphanReason:        "executor process not running",
		RequiresStateUpdate: true,
	}

	if err := ApplyResumeStateUpdatesProto(tk, result, backend); err != nil {
		t.Fatalf("apply resume state updates: %v", err)
	}

	phase := tk.Execution.Phases["implement_codex"]
	if phase == nil || phase.InterruptedAt == nil {
		t.Fatal("expected interrupted timestamp on current phase in memory")
	}

	loaded, err := backend.LoadTask(tk.Id)
	if err != nil {
		t.Fatalf("load task: %v", err)
	}

	if loaded.Status != orcv1.TaskStatus_TASK_STATUS_BLOCKED {
		t.Fatalf("status = %v, want blocked", loaded.Status)
	}

	rs := task.GetRetryState(loaded)
	if rs == nil {
		t.Fatal("expected retry state for orphaned resume")
	}
	if rs.FromPhase != "implement_codex" || rs.ToPhase != "implement_codex" {
		t.Fatalf("unexpected retry phases: from=%q to=%q", rs.FromPhase, rs.ToPhase)
	}
	if rs.Attempt != 1 {
		t.Fatalf("retry attempt = %d, want 1", rs.Attempt)
	}
	if !contains([]string{rs.Reason}, "Unexpected executor/provider interruption") {
		t.Fatalf("retry reason missing interruption context: %q", rs.Reason)
	}
	if !contains([]string{rs.Reason}, "git status") {
		t.Fatalf("retry reason missing worktree guidance: %q", rs.Reason)
	}
	if !contains([]string{rs.Reason}, "Last tool result") {
		t.Fatalf("retry reason missing transcript summary: %q", rs.Reason)
	}
	if !contains([]string{rs.FailureOutput}, "thread_server.go") {
		t.Fatalf("failure output missing transcript evidence: %q", rs.FailureOutput)
	}

	diagnostic := task.GetExecutorDiagnosticProto(loaded)
	if diagnostic == nil {
		t.Fatal("expected executor diagnostic for orphaned resume")
	}
	if diagnostic.Kind != "orphaned_executor" {
		t.Fatalf("diagnostic kind = %q, want orphaned_executor", diagnostic.Kind)
	}
	if diagnostic.Phase != "implement_codex" {
		t.Fatalf("diagnostic phase = %q, want implement_codex", diagnostic.Phase)
	}
	if diagnostic.ExecutorPID != tk.ExecutorPid {
		t.Fatalf("diagnostic pid = %d, want %d", diagnostic.ExecutorPID, tk.ExecutorPid)
	}
}

func TestApplyResumeStateUpdatesProto_OrphanedTaskCancelsRunningWorkflowRuns(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	tk := task.NewProtoTask("TASK-ORPHAN-RUN", "orphaned task with running workflow")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	task.SetCurrentPhaseProto(tk, "implement")
	task.EnsurePhaseProto(tk.Execution, "implement")
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	if err := backend.SaveWorkflow(&db.Workflow{
		ID:        "small",
		Name:      "small",
		CreatedAt: time.Now(),
	}); err != nil {
		t.Fatalf("save workflow: %v", err)
	}

	taskID := tk.Id
	startedAt := time.Now().Add(-5 * time.Minute)
	runningRun := &db.WorkflowRun{
		ID:           "RUN-ORPHAN-001",
		WorkflowID:   "small",
		ContextType:  "task",
		TaskID:       &taskID,
		Status:       string(workflow.RunStatusRunning),
		CurrentPhase: "implement",
		StartedAt:    &startedAt,
		CreatedAt:    startedAt,
		UpdatedAt:    startedAt,
	}
	if err := backend.SaveWorkflowRun(runningRun); err != nil {
		t.Fatalf("save running workflow run: %v", err)
	}

	completedAt := time.Now().Add(-time.Minute)
	completedRun := &db.WorkflowRun{
		ID:           "RUN-COMPLETE-001",
		WorkflowID:   "small",
		ContextType:  "task",
		TaskID:       &taskID,
		Status:       string(workflow.RunStatusCompleted),
		CurrentPhase: "docs",
		StartedAt:    &startedAt,
		CompletedAt:  &completedAt,
		CreatedAt:    startedAt,
		UpdatedAt:    completedAt,
	}
	if err := backend.SaveWorkflowRun(completedRun); err != nil {
		t.Fatalf("save completed workflow run: %v", err)
	}

	runPhase := &db.WorkflowRunPhase{
		WorkflowRunID:   runningRun.ID,
		PhaseTemplateID: "implement",
		Status:          "running",
		Iterations:      1,
		StartedAt:       &startedAt,
	}
	if err := backend.SaveWorkflowRunPhase(runPhase); err != nil {
		t.Fatalf("save workflow run phase: %v", err)
	}

	result := &ResumeValidationResult{
		IsOrphaned:          true,
		OrphanReason:        "executor process not running",
		RequiresStateUpdate: true,
	}

	if err := ApplyResumeStateUpdatesProto(tk, result, backend); err != nil {
		t.Fatalf("apply resume state updates: %v", err)
	}

	updatedRun, err := backend.GetWorkflowRun(runningRun.ID)
	if err != nil {
		t.Fatalf("load running workflow run: %v", err)
	}
	if updatedRun.Status != string(workflow.RunStatusCancelled) {
		t.Fatalf("running run status = %q, want %q", updatedRun.Status, workflow.RunStatusCancelled)
	}
	if updatedRun.CompletedAt == nil {
		t.Fatal("expected cancelled run to have completed_at")
	}
	if !strings.Contains(updatedRun.Error, "executor interrupted unexpectedly") {
		t.Fatalf("running run error = %q, want interruption context", updatedRun.Error)
	}

	updatedCompletedRun, err := backend.GetWorkflowRun(completedRun.ID)
	if err != nil {
		t.Fatalf("load completed workflow run: %v", err)
	}
	if updatedCompletedRun.Status != string(workflow.RunStatusCompleted) {
		t.Fatalf("completed run status = %q, want %q", updatedCompletedRun.Status, workflow.RunStatusCompleted)
	}

	phases, err := backend.GetWorkflowRunPhases(runningRun.ID)
	if err != nil {
		t.Fatalf("load workflow run phases: %v", err)
	}
	if len(phases) != 1 {
		t.Fatalf("phase count = %d, want 1", len(phases))
	}
	if phases[0].Status != "failed" {
		t.Fatalf("phase status = %q, want failed", phases[0].Status)
	}
	if phases[0].CompletedAt == nil {
		t.Fatal("expected phase completed_at to be set")
	}
	if !strings.Contains(phases[0].Error, "executor interrupted unexpectedly during implement") {
		t.Fatalf("phase error = %q, want interruption context", phases[0].Error)
	}
}

func TestApplyResumeStateUpdatesProto_ForceResumeKeepsNormalResumeSemantics(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	tk := task.NewProtoTask("TASK-FORCE-RESUME", "force resume task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	task.SetCurrentPhaseProto(tk, "implement_codex")
	task.EnsurePhaseProto(tk.Execution, "implement_codex")
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	result := &ResumeValidationResult{
		IsOrphaned:          false,
		RequiresStateUpdate: true,
	}

	if err := ApplyResumeStateUpdatesProto(tk, result, backend); err != nil {
		t.Fatalf("apply resume state updates: %v", err)
	}

	loaded, err := backend.LoadTask(tk.Id)
	if err != nil {
		t.Fatalf("load task: %v", err)
	}

	if rs := task.GetRetryState(loaded); rs != nil {
		t.Fatalf("force resume should not create retry state, got %+v", rs)
	}
}

func TestResumeCommand_FromWorktreeDirectory(t *testing.T) {
	// Create main project structure
	tmpDir := t.TempDir()
	if err := config.InitAt(tmpDir, false); err != nil {
		t.Fatalf("failed to init orc: %v", err)
	}

	// Create a task in the main repo via backend
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working dir: %v", err)
	}
	defer func() { _ = os.Chdir(origWd) }()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir to tmpDir: %v", err)
	}

	// Create test data via backend - use completed status so it fails validation
	// early (before trying to run executor)
	backend := createResumeTestBackend(t, tmpDir)

	tk := task.NewProtoTask("TASK-001", "Test task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED // Will fail with "cannot be resumed"
	task.SetCurrentPhaseProto(tk, "implement")
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	_ = backend.Close()

	// Create a "worktree-like" subdirectory (simulating worktree context)
	worktreeDir := filepath.Join(tmpDir, ".orc", "worktrees", "orc-TASK-001")
	if err := os.MkdirAll(worktreeDir, 0755); err != nil {
		t.Fatalf("failed to create worktree dir: %v", err)
	}

	// Create minimal .orc in worktree (like a real worktree would have)
	worktreeOrcDir := filepath.Join(worktreeDir, ".orc")
	if err := os.MkdirAll(worktreeOrcDir, 0755); err != nil {
		t.Fatalf("failed to create worktree .orc dir: %v", err)
	}

	// Change to worktree directory (no tasks here!)
	if err := os.Chdir(worktreeDir); err != nil {
		t.Fatalf("failed to chdir to worktree: %v", err)
	}

	// The resume command should find the task in the main repo via FindProjectRoot
	cmd := newResumeCmd()
	cmd.SetArgs([]string{"TASK-001"})

	err = cmd.Execute()

	// Should get "cannot be resumed" (because completed), NOT "task not found"
	// This verifies the task was found from the worktree directory
	if err == nil {
		t.Error("Expected error for completed task")
	}
	if err != nil && contains([]string{err.Error()}, "task TASK-001 not found") {
		t.Errorf("Resume from worktree should find task in main repo, got: %v", err)
	}
}

// contains helper to check if any string in the slice contains the substring
func contains(strs []string, substr string) bool {
	for _, s := range strs {
		if s != "" && len(s) > 0 {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
		}
	}
	return false
}
