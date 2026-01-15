package cli

// NOTE: Tests in this file use os.Chdir() which is process-wide and not goroutine-safe.
// These tests MUST NOT use t.Parallel() and run sequentially within this package.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/task"
)

// withEditTestDir creates a temp directory with task structure, changes to it,
// and restores the original working directory when the test completes.
func withEditTestDir(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, task.OrcDir, task.TasksDir)
	taskDir := filepath.Join(tasksDir, "TASK-001")
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		t.Fatalf("create task directory: %v", err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir to temp dir: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(origDir); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})
	return tmpDir
}

func TestRegeneratePlanForWeight(t *testing.T) {
	withEditTestDir(t)

	// Create and save a task
	tk := task.New("TASK-001", "Test task")
	tk.Weight = task.WeightLarge
	tk.Status = task.StatusPlanned
	if err := tk.Save(); err != nil {
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
	if err := initialPlan.Save("TASK-001"); err != nil {
		t.Fatalf("failed to save initial plan: %v", err)
	}

	// Create initial state with some progress
	initialState := state.New("TASK-001")
	initialState.CurrentPhase = "implement"
	initialState.Status = state.StatusRunning
	initialState.Phases["implement"] = &state.PhaseState{
		Status: state.StatusCompleted,
	}
	if err := initialState.Save(); err != nil {
		t.Fatalf("failed to save initial state: %v", err)
	}

	// Regenerate plan for weight change
	oldWeight := task.WeightSmall
	if err := regeneratePlanForWeight(tk, oldWeight); err != nil {
		t.Fatalf("regeneratePlanForWeight() error = %v", err)
	}

	// Verify plan was regenerated
	newPlan, err := plan.Load("TASK-001")
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
	newState, err := state.Load("TASK-001")
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
	reloadedTask, err := task.Load("TASK-001")
	if err != nil {
		t.Fatalf("failed to reload task: %v", err)
	}

	if reloadedTask.Status != task.StatusPlanned {
		t.Errorf("task status = %s, want %s", reloadedTask.Status, task.StatusPlanned)
	}
}

func TestRegeneratePlanForWeight_NoExistingState(t *testing.T) {
	withEditTestDir(t)

	// Create and save a task
	tk := task.New("TASK-001", "Test task")
	tk.Weight = task.WeightMedium
	if err := tk.Save(); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// No state.yaml exists yet

	// Regenerate plan
	oldWeight := task.WeightSmall
	if err := regeneratePlanForWeight(tk, oldWeight); err != nil {
		t.Fatalf("regeneratePlanForWeight() error = %v", err)
	}

	// Verify state was created
	newState, err := state.Load("TASK-001")
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

func TestEditCommand_StatusValidation(t *testing.T) {
	withEditTestDir(t)

	// Create and save a task
	tk := task.New("TASK-001", "Test task")
	tk.Status = task.StatusPlanned
	if err := tk.Save(); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Try to set invalid status
	cmd := newEditCmd()
	cmd.SetArgs([]string{"TASK-001", "--status", "invalid"})
	err := cmd.Execute()
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
	withEditTestDir(t)

	// Create and save a task
	tk := task.New("TASK-001", "Test task")
	tk.Status = task.StatusPlanned
	if err := tk.Save(); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Change status to completed
	cmd := newEditCmd()
	cmd.SetArgs([]string{"TASK-001", "--status", "completed"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("failed to execute edit command: %v", err)
	}

	// Verify status was updated
	updated, err := task.Load("TASK-001")
	if err != nil {
		t.Fatalf("failed to reload task: %v", err)
	}

	if updated.Status != task.StatusCompleted {
		t.Errorf("task status = %s, want %s", updated.Status, task.StatusCompleted)
	}
}

func TestEditCommand_StatusNoChangeIfSame(t *testing.T) {
	withEditTestDir(t)

	// Create and save a task already in completed status
	tk := task.New("TASK-001", "Test task")
	tk.Status = task.StatusCompleted
	if err := tk.Save(); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Try to set same status
	cmd := newEditCmd()
	cmd.SetArgs([]string{"TASK-001", "--status", "completed"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("failed to execute edit command: %v", err)
	}

	// Verify task is still completed
	updated, err := task.Load("TASK-001")
	if err != nil {
		t.Fatalf("failed to reload task: %v", err)
	}

	if updated.Status != task.StatusCompleted {
		t.Errorf("task status = %s, want %s", updated.Status, task.StatusCompleted)
	}
}

func TestEditCommand_CannotEditRunningTask(t *testing.T) {
	withEditTestDir(t)

	// Create and save a running task
	tk := task.New("TASK-001", "Test task")
	tk.Status = task.StatusRunning
	if err := tk.Save(); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Try to edit status
	cmd := newEditCmd()
	cmd.SetArgs([]string{"TASK-001", "--status", "completed"})
	err := cmd.Execute()
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
