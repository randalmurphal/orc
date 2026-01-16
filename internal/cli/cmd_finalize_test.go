package cli

import (
	"os"
	"testing"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// withFinalizeTestDir creates a temp directory with orc initialized and changes to it
func withFinalizeTestDir(t *testing.T) string {
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

// createFinalizeTestTask creates a task suitable for finalize testing
func createFinalizeTestTask(t *testing.T, tmpDir, id string, status task.Status) *task.Task {
	t.Helper()

	backend, err := storage.NewDatabaseBackend(tmpDir, nil)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}
	defer func() { _ = backend.Close() }()

	tk := task.New(id, "Test task for finalize")
	tk.Status = status
	tk.Weight = task.WeightLarge

	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Create plan with finalize phase
	p := &plan.Plan{
		Version:     1,
		TaskID:      id,
		Weight:      task.WeightLarge,
		Description: "Test plan with finalize",
		Phases: []plan.Phase{
			{ID: "spec", Name: "spec", Status: plan.PhaseCompleted},
			{ID: "implement", Name: "implement", Status: plan.PhaseCompleted},
			{ID: "test", Name: "test", Status: plan.PhaseCompleted},
			{ID: "docs", Name: "docs", Status: plan.PhaseCompleted},
			{ID: "validate", Name: "validate", Status: plan.PhaseCompleted},
			{ID: "finalize", Name: "finalize", Status: plan.PhasePending},
		},
	}
	if err := backend.SavePlan(p, id); err != nil {
		t.Fatalf("failed to save plan: %v", err)
	}

	// Create state
	s := state.New(id)
	s.CurrentPhase = "finalize"
	if err := backend.SaveState(s); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	return tk
}

func TestFinalizeCommand_CompletedTaskNotAllowed(t *testing.T) {
	tmpDir := withFinalizeTestDir(t)

	// Create a completed task
	createFinalizeTestTask(t, tmpDir, "TASK-001", task.StatusCompleted)

	cmd := newFinalizeCmd()
	cmd.SetArgs([]string{"TASK-001"})

	err := cmd.Execute()
	if err == nil {
		t.Error("Finalize should fail for completed task")
	}

	if err != nil && !containsSubstr(err.Error(), "already completed") {
		t.Errorf("Expected 'already completed' error, got: %v", err)
	}
}

func TestFinalizeCommand_RunningTaskNotAllowed(t *testing.T) {
	tmpDir := withFinalizeTestDir(t)

	// Create a running task
	createFinalizeTestTask(t, tmpDir, "TASK-001", task.StatusRunning)

	cmd := newFinalizeCmd()
	cmd.SetArgs([]string{"TASK-001"})

	err := cmd.Execute()
	if err == nil {
		t.Error("Finalize should fail for running task")
	}

	if err != nil && !containsSubstr(err.Error(), "currently running") {
		t.Errorf("Expected 'currently running' error, got: %v", err)
	}
}

func TestFinalizeCommand_TaskNotFound(t *testing.T) {
	_ = withFinalizeTestDir(t)

	cmd := newFinalizeCmd()
	cmd.SetArgs([]string{"TASK-999"})

	err := cmd.Execute()
	if err == nil {
		t.Error("Finalize should fail for non-existent task")
	}

	if err != nil && !containsSubstr(err.Error(), "TASK-999 not found") {
		t.Errorf("Expected 'task not found' error, got: %v", err)
	}
}

func TestFinalizeCommand_PausedTaskAllowed(t *testing.T) {
	tmpDir := withFinalizeTestDir(t)

	// Create a paused task
	createFinalizeTestTask(t, tmpDir, "TASK-001", task.StatusPaused)

	cmd := newFinalizeCmd()
	cmd.SetArgs([]string{"TASK-001"})

	err := cmd.Execute()

	// Should not get "task not found" error
	if err != nil && containsSubstr(err.Error(), "TASK-001 not found") {
		t.Errorf("Finalize should find the paused task, got: %v", err)
	}

	// Other errors (like executor-related) are expected since we don't have
	// a real git repo or Claude API in tests
}

func TestFinalizeCommand_FailedTaskAllowed(t *testing.T) {
	tmpDir := withFinalizeTestDir(t)

	// Create a failed task
	createFinalizeTestTask(t, tmpDir, "TASK-001", task.StatusFailed)

	cmd := newFinalizeCmd()
	cmd.SetArgs([]string{"TASK-001"})

	err := cmd.Execute()

	// Should not get state validation error
	if err != nil && containsSubstr(err.Error(), "already completed") {
		t.Errorf("Finalize should allow failed task, got: %v", err)
	}
}

func TestFinalizeCommand_BlockedTaskAllowed(t *testing.T) {
	tmpDir := withFinalizeTestDir(t)

	// Create a blocked task
	createFinalizeTestTask(t, tmpDir, "TASK-001", task.StatusBlocked)

	cmd := newFinalizeCmd()
	cmd.SetArgs([]string{"TASK-001"})

	err := cmd.Execute()

	// Should not get state validation error
	if err != nil && containsSubstr(err.Error(), "already completed") {
		t.Errorf("Finalize should allow blocked task, got: %v", err)
	}
}

func TestFinalizeCommand_InvalidGateType(t *testing.T) {
	tmpDir := withFinalizeTestDir(t)

	// Create a task
	createFinalizeTestTask(t, tmpDir, "TASK-001", task.StatusPaused)

	cmd := newFinalizeCmd()
	cmd.SetArgs([]string{"TASK-001", "--gate", "invalid"})

	err := cmd.Execute()
	if err == nil {
		t.Error("Finalize should fail for invalid gate type")
	}

	if err != nil && !containsSubstr(err.Error(), "invalid gate type") {
		t.Errorf("Expected 'invalid gate type' error, got: %v", err)
	}
}

func TestFinalizeCommand_ValidGateTypes(t *testing.T) {
	validGates := []string{"human", "ai", "none", "auto"}

	for _, gate := range validGates {
		t.Run("gate_"+gate, func(t *testing.T) {
			if !isValidGateType(gate) {
				t.Errorf("Gate type %q should be valid", gate)
			}
		})
	}
}

func TestFinalizeCommand_ForceFlag(t *testing.T) {
	tmpDir := withFinalizeTestDir(t)

	// Create a task
	createFinalizeTestTask(t, tmpDir, "TASK-001", task.StatusPaused)

	cmd := newFinalizeCmd()
	cmd.SetArgs([]string{"TASK-001", "--force"})

	err := cmd.Execute()

	// Should not get validation error about --force flag
	if err != nil && containsSubstr(err.Error(), "unknown flag") {
		t.Errorf("--force flag should be recognized, got: %v", err)
	}
}

func TestFinalizeCommand_StreamFlag(t *testing.T) {
	tmpDir := withFinalizeTestDir(t)

	// Create a task
	createFinalizeTestTask(t, tmpDir, "TASK-001", task.StatusPaused)

	cmd := newFinalizeCmd()
	cmd.SetArgs([]string{"TASK-001", "--stream"})

	err := cmd.Execute()

	// Should not get validation error about --stream flag
	if err != nil && containsSubstr(err.Error(), "unknown flag") {
		t.Errorf("--stream flag should be recognized, got: %v", err)
	}
}

func TestValidateFinalizeState(t *testing.T) {
	tests := []struct {
		name      string
		status    task.Status
		wantErr   bool
		errSubstr string
	}{
		{
			name:      "completed task not allowed",
			status:    task.StatusCompleted,
			wantErr:   true,
			errSubstr: "already completed",
		},
		{
			name:      "finished task not allowed",
			status:    task.StatusFinished,
			wantErr:   true,
			errSubstr: "already completed",
		},
		{
			name:      "running task not allowed",
			status:    task.StatusRunning,
			wantErr:   true,
			errSubstr: "currently running",
		},
		{
			name:    "planned task allowed",
			status:  task.StatusPlanned,
			wantErr: false,
		},
		{
			name:    "paused task allowed",
			status:  task.StatusPaused,
			wantErr: false,
		},
		{
			name:    "blocked task allowed",
			status:  task.StatusBlocked,
			wantErr: false,
		},
		{
			name:    "failed task allowed",
			status:  task.StatusFailed,
			wantErr: false,
		},
		{
			name:    "created task allowed",
			status:  task.StatusCreated,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tk := &task.Task{
				ID:     "TASK-001",
				Status: tt.status,
			}

			err := validateFinalizeState(tk)

			if tt.wantErr && err == nil {
				t.Errorf("validateFinalizeState() should return error for status %s", tt.status)
			}

			if !tt.wantErr && err != nil {
				t.Errorf("validateFinalizeState() should not return error for status %s, got: %v", tt.status, err)
			}

			if tt.wantErr && err != nil && tt.errSubstr != "" {
				if !containsSubstr(err.Error(), tt.errSubstr) {
					t.Errorf("Error should contain %q, got: %v", tt.errSubstr, err)
				}
			}
		})
	}
}

func TestGetFinalizePhase(t *testing.T) {
	t.Run("returns existing phase", func(t *testing.T) {
		p := &plan.Plan{
			Phases: []plan.Phase{
				{ID: "implement", Name: "Implement"},
				{ID: "finalize", Name: "Finalize", Status: plan.PhasePending},
			},
		}

		phase := getFinalizePhase(p)

		if phase.ID != "finalize" {
			t.Errorf("Expected finalize phase, got: %s", phase.ID)
		}
		if phase.Status != plan.PhasePending {
			t.Errorf("Expected pending status, got: %s", phase.Status)
		}
	})

	t.Run("creates new phase if not present", func(t *testing.T) {
		p := &plan.Plan{
			Phases: []plan.Phase{
				{ID: "implement", Name: "Implement"},
				{ID: "test", Name: "Test"},
			},
		}

		phase := getFinalizePhase(p)

		if phase.ID != "finalize" {
			t.Errorf("Expected new finalize phase, got: %s", phase.ID)
		}
		if phase.Status != plan.PhasePending {
			t.Errorf("Expected pending status for new phase, got: %s", phase.Status)
		}
	})
}

// containsSubstr checks if a string contains a substring
func containsSubstr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstrHelper(s, substr))
}

func containsSubstrHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
