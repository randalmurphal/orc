package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/task"
)

func TestRegeneratePlanForWeight(t *testing.T) {
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, task.OrcDir, task.TasksDir)

	// Create task directory
	taskDir := filepath.Join(tasksDir, "TASK-001")
	err := os.MkdirAll(taskDir, 0755)
	if err != nil {
		t.Fatalf("failed to create task directory: %v", err)
	}

	// Change to temp directory to simulate project root
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current directory: %v", err)
	}
	defer os.Chdir(origDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}

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
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, task.OrcDir, task.TasksDir)

	// Create task directory
	taskDir := filepath.Join(tasksDir, "TASK-001")
	err := os.MkdirAll(taskDir, 0755)
	if err != nil {
		t.Fatalf("failed to create task directory: %v", err)
	}

	// Change to temp directory
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current directory: %v", err)
	}
	defer os.Chdir(origDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}

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
