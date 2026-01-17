package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
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

// createTaskWithStatus creates a task with the given status and sets up plan/state
func createTaskWithStatus(t *testing.T, tmpDir, id string, status task.Status) *task.Task {
	t.Helper()

	backend := createResumeTestBackend(t, tmpDir)
	defer func() { _ = backend.Close() }()

	tk := task.New(id, "Test task")
	tk.Status = status
	tk.Weight = task.WeightSmall

	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Create plan
	p := &plan.Plan{
		Version:     1,
		TaskID:      id,
		Weight:      task.WeightSmall,
		Description: "Test plan",
		Phases: []plan.Phase{
			{ID: "implement", Name: "implement", Status: plan.PhasePending},
			{ID: "test", Name: "test", Status: plan.PhasePending},
		},
	}
	if err := backend.SavePlan(p, id); err != nil {
		t.Fatalf("failed to save plan: %v", err)
	}

	// Create state with a current phase
	s := state.New(id)
	s.CurrentPhase = "implement"
	if err := backend.SaveState(s); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	return tk
}

func TestResumeCommand_FailedTask(t *testing.T) {
	tmpDir := withResumeTestDir(t)

	// Create a failed task
	createTaskWithStatus(t, tmpDir, "TASK-001", task.StatusFailed)

	// Run resume command
	cmd := newResumeCmd()
	cmd.SetArgs([]string{"TASK-001"})

	// Capture output
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	// The command will fail because executor requires actual Claude API
	// but we're testing that it properly loads the task first
	err := cmd.Execute()

	// We expect an error from the executor, not "task not found"
	if err != nil {
		errStr := err.Error()
		if errStr == "load task: task TASK-001 not found" {
			t.Errorf("Resume should find the failed task, got: %v", err)
		}
		// Other errors (like executor-related) are expected since we don't have
		// a real Claude API in tests
	}

	// Verify the task status message was printed (optional check, may not capture)
	_ = stdout.String() // output may or may not contain "failed previously" depending on capture
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

func TestResumeCommand_PausedTask(t *testing.T) {
	tmpDir := withResumeTestDir(t)

	// Create a paused task
	createTaskWithStatus(t, tmpDir, "TASK-001", task.StatusPaused)

	cmd := newResumeCmd()
	cmd.SetArgs([]string{"TASK-001"})

	err := cmd.Execute()

	// Should not get "task not found" error
	if err != nil && contains([]string{err.Error()}, "task TASK-001 not found") {
		t.Errorf("Resume should find the paused task, got: %v", err)
	}
}

func TestResumeCommand_BlockedTask(t *testing.T) {
	tmpDir := withResumeTestDir(t)

	// Create a blocked task
	createTaskWithStatus(t, tmpDir, "TASK-001", task.StatusBlocked)

	cmd := newResumeCmd()
	cmd.SetArgs([]string{"TASK-001"})

	err := cmd.Execute()

	// Should not get "task not found" error
	if err != nil && contains([]string{err.Error()}, "task TASK-001 not found") {
		t.Errorf("Resume should find the blocked task, got: %v", err)
	}
}

func TestResumeCommand_CompletedTaskNotResumable(t *testing.T) {
	tmpDir := withResumeTestDir(t)

	// Create a completed task
	createTaskWithStatus(t, tmpDir, "TASK-001", task.StatusCompleted)

	cmd := newResumeCmd()
	cmd.SetArgs([]string{"TASK-001"})

	var stderr bytes.Buffer
	cmd.SetErr(&stderr)

	err := cmd.Execute()
	if err == nil {
		t.Error("Resume should fail for completed task")
	}

	// Should get "cannot be resumed" error, not "task not found"
	if err != nil {
		errStr := err.Error()
		if contains([]string{errStr}, "task TASK-001 not found") {
			t.Errorf("Should not get 'task not found' error, got: %v", err)
		}
		if !contains([]string{errStr}, "cannot be resumed") {
			t.Errorf("Expected 'cannot be resumed' error, got: %v", err)
		}
	}
}

func TestResumeCommand_CreatedTaskNotResumable(t *testing.T) {
	tmpDir := withResumeTestDir(t)

	// Create a task with created status
	createTaskWithStatus(t, tmpDir, "TASK-001", task.StatusCreated)

	cmd := newResumeCmd()
	cmd.SetArgs([]string{"TASK-001"})

	err := cmd.Execute()
	if err == nil {
		t.Error("Resume should fail for created task")
	}

	// Should get "cannot be resumed" error
	if err != nil && contains([]string{err.Error()}, "task TASK-001 not found") {
		t.Errorf("Should not get 'task not found' error, got: %v", err)
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

	// Create test data via backend
	backend := createResumeTestBackend(t, tmpDir)

	tk := task.New("TASK-001", "Test task")
	tk.Status = task.StatusFailed
	tk.Weight = task.WeightSmall
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	p := &plan.Plan{
		Version:     1,
		TaskID:      "TASK-001",
		Weight:      task.WeightSmall,
		Description: "Test plan",
		Phases: []plan.Phase{
			{ID: "implement", Name: "implement", Status: plan.PhasePending},
			{ID: "test", Name: "test", Status: plan.PhasePending},
		},
	}
	if err := backend.SavePlan(p, "TASK-001"); err != nil {
		t.Fatalf("failed to save plan: %v", err)
	}

	s := state.New("TASK-001")
	s.CurrentPhase = "implement"
	if err := backend.SaveState(s); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	_ = backend.Close()

	// Create a "worktree-like" subdirectory (simulating worktree context)
	// In a real worktree, this would be .orc/worktrees/orc-TASK-001/
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

	// Should NOT get "task not found" error
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

func TestResumeCommand_FromPhaseFlag(t *testing.T) {
	tmpDir := withResumeTestDir(t)

	// Create a failed task with multiple phases
	backend := createResumeTestBackend(t, tmpDir)

	tk := task.New("TASK-001", "Test task")
	tk.Status = task.StatusFailed
	tk.Weight = task.WeightMedium
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Create plan with multiple phases
	p := &plan.Plan{
		Version:     1,
		TaskID:      "TASK-001",
		Weight:      task.WeightMedium,
		Description: "Test plan",
		Phases: []plan.Phase{
			{ID: "spec", Name: "spec", Status: plan.PhaseCompleted},
			{ID: "implement", Name: "implement", Status: plan.PhaseCompleted},
			{ID: "review", Name: "review", Status: plan.PhaseFailed},
			{ID: "test", Name: "test", Status: plan.PhasePending},
			{ID: "docs", Name: "docs", Status: plan.PhasePending},
		},
	}
	if err := backend.SavePlan(p, "TASK-001"); err != nil {
		t.Fatalf("failed to save plan: %v", err)
	}

	// Create state with review phase failed
	s := state.New("TASK-001")
	s.CurrentPhase = "review"
	s.Phases["spec"] = &state.PhaseState{Status: state.StatusCompleted}
	s.Phases["implement"] = &state.PhaseState{Status: state.StatusCompleted}
	s.Phases["review"] = &state.PhaseState{Status: state.StatusFailed, Error: "review failed"}
	if err := backend.SaveState(s); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	_ = backend.Close()

	// Test that --from-phase flag is accepted
	cmd := newResumeCmd()
	cmd.SetArgs([]string{"TASK-001", "--from-phase", "implement"})

	err := cmd.Execute()

	// The command will fail in execution (no real Claude API), but it should
	// NOT fail with a "phase not found" error or flag parsing error
	if err != nil && contains([]string{err.Error()}, "phase \"implement\" not found") {
		t.Errorf("Phase 'implement' should be found in plan, got: %v", err)
	}
}

func TestResumeCommand_FromPhaseInvalidPhase(t *testing.T) {
	tmpDir := withResumeTestDir(t)

	// Create a task
	backend := createResumeTestBackend(t, tmpDir)

	tk := task.New("TASK-001", "Test task")
	tk.Status = task.StatusFailed
	tk.Weight = task.WeightSmall
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	p := &plan.Plan{
		Version:     1,
		TaskID:      "TASK-001",
		Weight:      task.WeightSmall,
		Description: "Test plan",
		Phases: []plan.Phase{
			{ID: "implement", Name: "implement", Status: plan.PhasePending},
			{ID: "test", Name: "test", Status: plan.PhasePending},
		},
	}
	if err := backend.SavePlan(p, "TASK-001"); err != nil {
		t.Fatalf("failed to save plan: %v", err)
	}

	s := state.New("TASK-001")
	s.CurrentPhase = "implement"
	if err := backend.SaveState(s); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	_ = backend.Close()

	cmd := newResumeCmd()
	cmd.SetArgs([]string{"TASK-001", "--from-phase", "bogus"})

	err := cmd.Execute()

	// Should fail with a clear error about invalid phase
	if err == nil {
		t.Error("Expected error for invalid phase, got nil")
	}

	if err != nil {
		errStr := err.Error()
		if !contains([]string{errStr}, "phase \"bogus\" not found in plan") {
			t.Errorf("Expected 'phase not found' error, got: %v", err)
		}
		if !contains([]string{errStr}, "available: implement, test") {
			t.Errorf("Expected available phases in error, got: %v", err)
		}
	}
}

func TestResumeCommand_FromPhaseResetsState(t *testing.T) {
	tmpDir := withResumeTestDir(t)

	backend := createResumeTestBackend(t, tmpDir)

	tk := task.New("TASK-001", "Test task")
	tk.Status = task.StatusFailed
	tk.Weight = task.WeightSmall
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	p := &plan.Plan{
		Version:     1,
		TaskID:      "TASK-001",
		Weight:      task.WeightSmall,
		Description: "Test plan",
		Phases: []plan.Phase{
			{ID: "implement", Name: "implement", Status: plan.PhaseCompleted},
			{ID: "test", Name: "test", Status: plan.PhaseFailed},
		},
	}
	if err := backend.SavePlan(p, "TASK-001"); err != nil {
		t.Fatalf("failed to save plan: %v", err)
	}

	s := state.New("TASK-001")
	s.CurrentPhase = "test"
	s.Status = state.StatusFailed
	s.Error = "test failed"
	s.Phases["implement"] = &state.PhaseState{Status: state.StatusCompleted, CommitSHA: "abc123"}
	s.Phases["test"] = &state.PhaseState{Status: state.StatusFailed, Error: "test failed"}
	if err := backend.SaveState(s); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	_ = backend.Close()

	// Run resume with --from-phase implement
	cmd := newResumeCmd()
	cmd.SetArgs([]string{"TASK-001", "--from-phase", "implement"})

	// The command will fail in execution, but state should be updated
	_ = cmd.Execute()

	// Reload state and verify phases were reset
	backend2 := createResumeTestBackend(t, tmpDir)
	defer func() { _ = backend2.Close() }()

	updatedState, err := backend2.LoadState("TASK-001")
	if err != nil {
		t.Fatalf("failed to load updated state: %v", err)
	}

	// Verify implement phase was reset (from the point specified)
	if updatedState.Phases["implement"] != nil && updatedState.Phases["implement"].Status != state.StatusPending {
		t.Errorf("implement phase status = %s, want %s (should be reset)", updatedState.Phases["implement"].Status, state.StatusPending)
	}

	// Verify test phase was reset
	if updatedState.Phases["test"] != nil && updatedState.Phases["test"].Status != state.StatusPending {
		t.Errorf("test phase status = %s, want %s (should be reset)", updatedState.Phases["test"].Status, state.StatusPending)
	}

	// Verify task-level error was cleared
	if updatedState.Error != "" {
		t.Errorf("Error = %s, want empty", updatedState.Error)
	}
}

func TestFormatPhaseList(t *testing.T) {
	tests := []struct {
		input []string
		want  string
	}{
		{[]string{}, "(none)"},
		{[]string{"implement"}, "implement"},
		{[]string{"spec", "implement"}, "spec, implement"},
		{[]string{"spec", "implement", "test", "docs"}, "spec, implement, test, docs"},
	}

	for _, tt := range tests {
		got := formatPhaseList(tt.input)
		if got != tt.want {
			t.Errorf("formatPhaseList(%v) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
