package cli

// NOTE: Tests in this file use the backend pattern with temporary directories.
// The edit command creates its own backend based on working directory.

import (
	"os"
	"path/filepath"
	"testing"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// createEditTestBackend creates a backend for testing edit operations.
func createEditTestBackend(t *testing.T) (storage.Backend, string) {
	t.Helper()
	tmpDir := t.TempDir()
	backend, err := storage.NewDatabaseBackend(tmpDir, nil)
	if err != nil {
		t.Fatalf("create backend: %v", err)
	}
	t.Cleanup(func() {
		_ = backend.Close()
	})
	return backend, tmpDir
}

func TestRegeneratePlanForWeight(t *testing.T) {
	backend, _ := createEditTestBackend(t)

	// Create and save a task with initial execution progress
	tk := task.NewProtoTask("TASK-001", "Test task")
	tk.Weight = orcv1.TaskWeight_TASK_WEIGHT_LARGE
	tk.Status = orcv1.TaskStatus_TASK_STATUS_PLANNED
	task.SetCurrentPhaseProto(tk, "implement")
	task.EnsureExecutionProto(tk)
	tk.Execution.CurrentIteration = 2
	tk.Execution.Phases["implement"] = &orcv1.PhaseState{
		Status: orcv1.PhaseStatus_PHASE_STATUS_COMPLETED,
	}
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Regenerate plan for weight change (resets execution state)
	if err := regeneratePlanForWeightProto(backend, tk); err != nil {
		t.Fatalf("regeneratePlanForWeightProto() error = %v", err)
	}

	// Verify execution state was reset
	reloadedTask, err := backend.LoadTask("TASK-001")
	if err != nil {
		t.Fatalf("failed to reload task: %v", err)
	}

	currentPhase := ""
	if reloadedTask.CurrentPhase != nil {
		currentPhase = *reloadedTask.CurrentPhase
	}
	if currentPhase != "" {
		t.Errorf("task current phase = %q, want empty", currentPhase)
	}

	if len(reloadedTask.Execution.Phases) != 0 {
		t.Errorf("execution phases = %d, want 0", len(reloadedTask.Execution.Phases))
	}

	if reloadedTask.Execution.CurrentIteration != 0 {
		t.Errorf("current iteration = %d, want 0", reloadedTask.Execution.CurrentIteration)
	}

	// Verify task status was set to planned
	if reloadedTask.Status != orcv1.TaskStatus_TASK_STATUS_PLANNED {
		t.Errorf("task status = %s, want %s", reloadedTask.Status, orcv1.TaskStatus_TASK_STATUS_PLANNED)
	}
}

func TestRegeneratePlanForWeight_NoExistingExecutionState(t *testing.T) {
	backend, _ := createEditTestBackend(t)

	// Create and save a task without execution state
	tk := task.NewProtoTask("TASK-001", "Test task")
	tk.Weight = orcv1.TaskWeight_TASK_WEIGHT_MEDIUM
	// Execution state is uninitialized (zero value)
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Regenerate plan - should initialize execution state
	if err := regeneratePlanForWeightProto(backend, tk); err != nil {
		t.Fatalf("regeneratePlanForWeightProto() error = %v", err)
	}

	// Verify task was updated with reset execution state
	reloadedTask, err := backend.LoadTask("TASK-001")
	if err != nil {
		t.Fatalf("failed to reload task: %v", err)
	}

	if reloadedTask.Id != "TASK-001" {
		t.Errorf("task ID = %q, want %q", reloadedTask.Id, "TASK-001")
	}

	// Execution state should be reset to defaults
	currentPhase := ""
	if reloadedTask.CurrentPhase != nil {
		currentPhase = *reloadedTask.CurrentPhase
	}
	if currentPhase != "" {
		t.Errorf("current phase = %q, want empty", currentPhase)
	}
}

func TestEditCommand_NoFlags(t *testing.T) {
	cmd := newEditCmd()

	// Verify command structure
	if cmd.Use != "edit <task-id>" {
		t.Errorf("command Use = %q, want %q", cmd.Use, "edit <task-id>")
	}

	// Verify flags exist
	if cmd.Flag("title") == nil {
		t.Error("missing --title flag")
	}
	if cmd.Flag("description") == nil {
		t.Error("missing --description flag")
	}
	if cmd.Flag("weight") == nil {
		t.Error("missing --weight flag")
	}

	// Verify shorthand flags
	if cmd.Flag("description").Shorthand != "d" {
		t.Errorf("description shorthand = %q, want 'd'", cmd.Flag("description").Shorthand)
	}
	if cmd.Flag("weight").Shorthand != "w" {
		t.Errorf("weight shorthand = %q, want 'w'", cmd.Flag("weight").Shorthand)
	}
	if cmd.Flag("title").Shorthand != "t" {
		t.Errorf("title shorthand = %q, want 't'", cmd.Flag("title").Shorthand)
	}
}

func TestEditCommand_RequiresArg(t *testing.T) {
	cmd := newEditCmd()

	// Should require exactly one argument
	if err := cmd.Args(cmd, []string{}); err == nil {
		t.Error("expected error for zero args")
	}
	if err := cmd.Args(cmd, []string{"TASK-001"}); err != nil {
		t.Errorf("unexpected error for one arg: %v", err)
	}
	if err := cmd.Args(cmd, []string{"TASK-001", "TASK-002"}); err == nil {
		t.Error("expected error for two args")
	}
}

func TestEditCommand_StatusFlag(t *testing.T) {
	cmd := newEditCmd()

	// Verify --status flag exists
	if cmd.Flag("status") == nil {
		t.Error("missing --status flag")
	}

	// Verify shorthand -s exists
	if cmd.Flag("status").Shorthand != "s" {
		t.Errorf("status shorthand = %q, want 's'", cmd.Flag("status").Shorthand)
	}
}

// TestEditCommand_StatusValidation tests that invalid status values are rejected.
// This test requires a working directory structure, so we use backend directly.
func TestEditCommand_StatusValidation(t *testing.T) {
	_, tmpDir := createEditTestBackend(t)

	// Set up working directory for command execution
	origDir := setupTestWorkDir(t, tmpDir)
	defer restoreWorkDir(t, origDir)

	// Create task via backend created by edit command
	backend, err := storage.NewDatabaseBackend(tmpDir, nil)
	if err != nil {
		t.Fatalf("create backend: %v", err)
	}
	defer func() { _ = backend.Close() }()

	tk := task.NewProtoTask("TASK-001", "Test task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_PLANNED
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Try to set invalid status
	cmd := newEditCmd()
	cmd.SetArgs([]string{"TASK-001", "--status", "invalid"})
	err = cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid status")
	}

	// Check error message contains valid options
	errMsg := err.Error()
	if !hasSubstring(errMsg, "invalid status") {
		t.Errorf("error message should mention 'invalid status', got: %s", errMsg)
	}
	if !hasSubstring(errMsg, "created") || !hasSubstring(errMsg, "completed") {
		t.Errorf("error message should list valid options, got: %s", errMsg)
	}
}

func TestEditCommand_StatusChange(t *testing.T) {
	_, tmpDir := createEditTestBackend(t)

	// Set up working directory for command execution
	origDir := setupTestWorkDir(t, tmpDir)
	defer restoreWorkDir(t, origDir)

	// Create task via backend
	backend, err := storage.NewDatabaseBackend(tmpDir, nil)
	if err != nil {
		t.Fatalf("create backend: %v", err)
	}
	defer func() { _ = backend.Close() }()

	tk := task.NewProtoTask("TASK-001", "Test task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_PLANNED
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Change status to completed
	cmd := newEditCmd()
	cmd.SetArgs([]string{"TASK-001", "--status", "completed"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("failed to execute edit command: %v", err)
	}

	// Verify status was updated
	updated, err := backend.LoadTask("TASK-001")
	if err != nil {
		t.Fatalf("failed to reload task: %v", err)
	}

	if updated.Status != orcv1.TaskStatus_TASK_STATUS_COMPLETED {
		t.Errorf("task status = %s, want %s", updated.Status, orcv1.TaskStatus_TASK_STATUS_COMPLETED)
	}
}

func TestEditCommand_StatusNoChangeIfSame(t *testing.T) {
	_, tmpDir := createEditTestBackend(t)

	// Set up working directory for command execution
	origDir := setupTestWorkDir(t, tmpDir)
	defer restoreWorkDir(t, origDir)

	// Create task via backend
	backend, err := storage.NewDatabaseBackend(tmpDir, nil)
	if err != nil {
		t.Fatalf("create backend: %v", err)
	}
	defer func() { _ = backend.Close() }()

	tk := task.NewProtoTask("TASK-001", "Test task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Try to set same status
	cmd := newEditCmd()
	cmd.SetArgs([]string{"TASK-001", "--status", "completed"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("failed to execute edit command: %v", err)
	}

	// Verify task is still completed
	updated, err := backend.LoadTask("TASK-001")
	if err != nil {
		t.Fatalf("failed to reload task: %v", err)
	}

	if updated.Status != orcv1.TaskStatus_TASK_STATUS_COMPLETED {
		t.Errorf("task status = %s, want %s", updated.Status, orcv1.TaskStatus_TASK_STATUS_COMPLETED)
	}
}

func TestEditCommand_CannotEditRunningTask(t *testing.T) {
	_, tmpDir := createEditTestBackend(t)

	// Set up working directory for command execution
	origDir := setupTestWorkDir(t, tmpDir)
	defer restoreWorkDir(t, origDir)

	// Create task via backend
	backend, err := storage.NewDatabaseBackend(tmpDir, nil)
	if err != nil {
		t.Fatalf("create backend: %v", err)
	}
	defer func() { _ = backend.Close() }()

	tk := task.NewProtoTask("TASK-001", "Test task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Try to edit status
	cmd := newEditCmd()
	cmd.SetArgs([]string{"TASK-001", "--status", "completed"})
	err = cmd.Execute()
	if err == nil {
		t.Fatal("expected error for editing running task")
	}

	if !hasSubstring(err.Error(), "cannot edit running task") {
		t.Errorf("error should mention cannot edit running task, got: %s", err.Error())
	}
}

// hasSubstring checks if substr is in s (helper for tests).
func hasSubstring(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// setupTestWorkDir changes to the given directory and returns the original directory.
func setupTestWorkDir(t *testing.T, dir string) string {
	t.Helper()
	// Create .orc directory to satisfy project root detection
	orcDir := filepath.Join(dir, ".orc")
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatalf("create .orc dir: %v", err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get current dir: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("change to test dir: %v", err)
	}
	return origDir
}

// restoreWorkDir restores the working directory.
func restoreWorkDir(t *testing.T, dir string) {
	t.Helper()
	if err := os.Chdir(dir); err != nil {
		t.Errorf("restore work dir: %v", err)
	}
}

func TestRegeneratePlanForWeight_UpdatesWeightBasedWorkflow(t *testing.T) {
	backend, _ := createEditTestBackend(t)

	// Create a task with weight=small and workflow_id="implement-small" (weight-based)
	tk := task.NewProtoTask("TASK-002", "Test weight-based workflow update")
	tk.Weight = orcv1.TaskWeight_TASK_WEIGHT_SMALL
	task.SetWorkflowIDProto(tk, "implement-small")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_PLANNED
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Change weight to medium
	tk.Weight = orcv1.TaskWeight_TASK_WEIGHT_MEDIUM

	// Regenerate plan
	if err := regeneratePlanForWeightProto(backend, tk); err != nil {
		t.Fatalf("regeneratePlanForWeightProto() error = %v", err)
	}

	// Reload and verify workflow_id was updated to match new weight
	reloaded, err := backend.LoadTask("TASK-002")
	if err != nil {
		t.Fatalf("failed to reload task: %v", err)
	}

	gotWorkflow := task.GetWorkflowIDProto(reloaded)
	if gotWorkflow != "implement-medium" {
		t.Errorf("workflow_id = %q, want %q", gotWorkflow, "implement-medium")
	}
}

func TestRegeneratePlanForWeight_PreservesExplicitWorkflow(t *testing.T) {
	backend, _ := createEditTestBackend(t)

	// Create a task with weight=small but explicit workflow_id="qa-e2e" (non-weight-based)
	tk := task.NewProtoTask("TASK-003", "Test explicit workflow preservation")
	tk.Weight = orcv1.TaskWeight_TASK_WEIGHT_SMALL
	task.SetWorkflowIDProto(tk, "qa-e2e")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_PLANNED
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Change weight to medium
	tk.Weight = orcv1.TaskWeight_TASK_WEIGHT_MEDIUM

	// Regenerate plan
	if err := regeneratePlanForWeightProto(backend, tk); err != nil {
		t.Fatalf("regeneratePlanForWeightProto() error = %v", err)
	}

	// Reload and verify workflow_id was preserved (not overwritten)
	reloaded, err := backend.LoadTask("TASK-003")
	if err != nil {
		t.Fatalf("failed to reload task: %v", err)
	}

	gotWorkflow := task.GetWorkflowIDProto(reloaded)
	if gotWorkflow != "qa-e2e" {
		t.Errorf("workflow_id = %q, want %q (explicit workflow should be preserved)", gotWorkflow, "qa-e2e")
	}
}
