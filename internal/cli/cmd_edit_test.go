package cli

// NOTE: Tests in this file use the backend pattern with temporary directories.
// The edit command creates its own backend based on working directory.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/state"
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
		backend.Close()
	})
	return backend, tmpDir
}

func TestRegeneratePlanForWeight(t *testing.T) {
	backend, _ := createEditTestBackend(t)

	// Create and save a task
	tk := task.New("TASK-001", "Test task")
	tk.Weight = task.WeightLarge
	tk.Status = task.StatusPlanned
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Create initial plan
	initialPlan := &plan.Plan{
		Version:     1,
		TaskID:      "TASK-001",
		Weight:      task.WeightSmall,
		Description: "Initial plan",
		Phases: []plan.Phase{
			{ID: "implement", Name: "implement", Status: plan.PhasePending},
			{ID: "test", Name: "test", Status: plan.PhasePending},
		},
	}
	if err := backend.SavePlan(initialPlan, "TASK-001"); err != nil {
		t.Fatalf("failed to save initial plan: %v", err)
	}

	// Create initial state with some progress
	initialState := state.New("TASK-001")
	initialState.CurrentPhase = "implement"
	initialState.Status = state.StatusRunning
	initialState.Phases["implement"] = &state.PhaseState{
		Status: state.StatusCompleted,
	}
	if err := backend.SaveState(initialState); err != nil {
		t.Fatalf("failed to save initial state: %v", err)
	}

	// Regenerate plan for weight change
	oldWeight := task.WeightSmall
	if err := regeneratePlanForWeight(backend, tk, oldWeight); err != nil {
		t.Fatalf("regeneratePlanForWeight() error = %v", err)
	}

	// Verify plan was regenerated
	newPlan, err := backend.LoadPlan("TASK-001")
	if err != nil {
		t.Fatalf("failed to load new plan: %v", err)
	}

	if newPlan.Weight != task.WeightLarge {
		t.Errorf("plan weight = %s, want %s", newPlan.Weight, task.WeightLarge)
	}

	// Large weight should have more phases than small
	if len(newPlan.Phases) <= 2 {
		t.Errorf("plan phases = %d, expected more than 2 for large weight", len(newPlan.Phases))
	}

	// Verify state was reset
	newState, err := backend.LoadState("TASK-001")
	if err != nil {
		t.Fatalf("failed to load new state: %v", err)
	}

	if newState.Status != state.StatusPending {
		t.Errorf("state status = %s, want %s", newState.Status, state.StatusPending)
	}

	if newState.CurrentPhase != "" {
		t.Errorf("state current phase = %q, want empty", newState.CurrentPhase)
	}

	if len(newState.Phases) != 0 {
		t.Errorf("state phases = %d, want 0", len(newState.Phases))
	}

	// Verify task status was set to planned
	reloadedTask, err := backend.LoadTask("TASK-001")
	if err != nil {
		t.Fatalf("failed to reload task: %v", err)
	}

	if reloadedTask.Status != task.StatusPlanned {
		t.Errorf("task status = %s, want %s", reloadedTask.Status, task.StatusPlanned)
	}
}

func TestRegeneratePlanForWeight_NoExistingState(t *testing.T) {
	backend, _ := createEditTestBackend(t)

	// Create and save a task
	tk := task.New("TASK-001", "Test task")
	tk.Weight = task.WeightMedium
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// No state exists yet

	// Regenerate plan
	oldWeight := task.WeightSmall
	if err := regeneratePlanForWeight(backend, tk, oldWeight); err != nil {
		t.Fatalf("regeneratePlanForWeight() error = %v", err)
	}

	// Verify state was created
	newState, err := backend.LoadState("TASK-001")
	if err != nil {
		t.Fatalf("failed to load new state: %v", err)
	}

	if newState.Status != state.StatusPending {
		t.Errorf("state status = %s, want %s", newState.Status, state.StatusPending)
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
	defer backend.Close()

	tk := task.New("TASK-001", "Test task")
	tk.Status = task.StatusPlanned
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
	defer backend.Close()

	tk := task.New("TASK-001", "Test task")
	tk.Status = task.StatusPlanned
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

	if updated.Status != task.StatusCompleted {
		t.Errorf("task status = %s, want %s", updated.Status, task.StatusCompleted)
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
	defer backend.Close()

	tk := task.New("TASK-001", "Test task")
	tk.Status = task.StatusCompleted
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

	if updated.Status != task.StatusCompleted {
		t.Errorf("task status = %s, want %s", updated.Status, task.StatusCompleted)
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
	defer backend.Close()

	tk := task.New("TASK-001", "Test task")
	tk.Status = task.StatusRunning
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
