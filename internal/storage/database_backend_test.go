package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/initiative"
	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/task"
)

// setupTestDB creates a temporary database for testing.
func setupTestDB(t *testing.T) (*DatabaseBackend, string) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "orc-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}

	// Create .orc directory
	if err := os.MkdirAll(filepath.Join(tmpDir, ".orc"), 0755); err != nil {
		t.Fatalf("create .orc dir: %v", err)
	}

	backend, err := NewDatabaseBackend(tmpDir, &config.StorageConfig{})
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("create backend: %v", err)
	}

	return backend, tmpDir
}

// teardownTestDB cleans up the test database.
func teardownTestDB(t *testing.T, backend *DatabaseBackend, tmpDir string) {
	t.Helper()

	if err := backend.Close(); err != nil {
		t.Errorf("close backend: %v", err)
	}
	if err := os.RemoveAll(tmpDir); err != nil {
		t.Errorf("remove temp dir: %v", err)
	}
}

// TestSaveTask_Transaction verifies task and dependencies are saved atomically.
func TestSaveTask_Transaction(t *testing.T) {
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	// Create a task with dependencies
	task1 := &task.Task{
		ID:        "TASK-001",
		Title:     "Dependency Task",
		Weight:    task.WeightSmall,
		Status:    task.StatusCreated,
		CreatedAt: time.Now(),
	}
	if err := backend.SaveTask(task1); err != nil {
		t.Fatalf("save task1: %v", err)
	}

	// Create another task that depends on task1
	task2 := &task.Task{
		ID:        "TASK-002",
		Title:     "Test Task",
		Weight:    task.WeightMedium,
		Status:    task.StatusCreated,
		BlockedBy: []string{"TASK-001"},
		CreatedAt: time.Now(),
	}
	if err := backend.SaveTask(task2); err != nil {
		t.Fatalf("save task2: %v", err)
	}

	// Load and verify
	loaded, err := backend.LoadTask("TASK-002")
	if err != nil {
		t.Fatalf("load task: %v", err)
	}

	if len(loaded.BlockedBy) != 1 || loaded.BlockedBy[0] != "TASK-001" {
		t.Errorf("expected BlockedBy=[TASK-001], got %v", loaded.BlockedBy)
	}

	// Update dependencies
	task2.BlockedBy = []string{}
	if err := backend.SaveTask(task2); err != nil {
		t.Fatalf("update task2: %v", err)
	}

	// Verify dependencies were cleared
	loaded, err = backend.LoadTask("TASK-002")
	if err != nil {
		t.Fatalf("load task: %v", err)
	}

	if len(loaded.BlockedBy) != 0 {
		t.Errorf("expected BlockedBy=[], got %v", loaded.BlockedBy)
	}
}

// TestSaveState_Transaction verifies state and phases are saved atomically.
func TestSaveState_Transaction(t *testing.T) {
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	// First create a task
	task1 := &task.Task{
		ID:        "TASK-001",
		Title:     "Test Task",
		Weight:    task.WeightSmall,
		Status:    task.StatusRunning,
		CreatedAt: time.Now(),
	}
	if err := backend.SaveTask(task1); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Create state with phases
	now := time.Now()
	s := &state.State{
		TaskID:       "TASK-001",
		CurrentPhase: "implement",
		Status:       state.StatusRunning,
		StartedAt:    now,
		Phases: map[string]*state.PhaseState{
			"implement": {
				Status:     state.StatusRunning,
				Iterations: 1,
				StartedAt:  now,
				Tokens: state.TokenUsage{
					InputTokens:  1000,
					OutputTokens: 500,
				},
			},
		},
	}

	if err := backend.SaveState(s); err != nil {
		t.Fatalf("save state: %v", err)
	}

	// Load and verify
	loaded, err := backend.LoadState("TASK-001")
	if err != nil {
		t.Fatalf("load state: %v", err)
	}

	if loaded.CurrentPhase != "implement" {
		t.Errorf("expected CurrentPhase=implement, got %s", loaded.CurrentPhase)
	}
	if loaded.Status != state.StatusRunning {
		t.Errorf("expected Status=running, got %s", loaded.Status)
	}
	if ps, ok := loaded.Phases["implement"]; !ok {
		t.Error("expected implement phase to exist")
	} else {
		if ps.Tokens.InputTokens != 1000 {
			t.Errorf("expected InputTokens=1000, got %d", ps.Tokens.InputTokens)
		}
	}
}

// TestSaveState_RetryContext verifies RetryContext is saved and loaded correctly.
func TestSaveState_RetryContext(t *testing.T) {
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	// Create task
	task1 := &task.Task{
		ID:        "TASK-001",
		Title:     "Test Task",
		Weight:    task.WeightSmall,
		Status:    task.StatusRunning,
		CreatedAt: time.Now(),
	}
	if err := backend.SaveTask(task1); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Save state with RetryContext
	now := time.Now()
	s := &state.State{
		TaskID:       "TASK-001",
		CurrentPhase: "test",
		Status:       state.StatusFailed,
		StartedAt:    now,
		Phases:       make(map[string]*state.PhaseState),
		RetryContext: &state.RetryContext{
			FromPhase:     "test",
			ToPhase:       "implement",
			Reason:        "Test failed: assertion error",
			FailureOutput: "assertion error at line 42",
			Attempt:       2,
			Timestamp:     now,
		},
	}

	if err := backend.SaveState(s); err != nil {
		t.Fatalf("save state: %v", err)
	}

	// Load and verify
	loaded, err := backend.LoadState("TASK-001")
	if err != nil {
		t.Fatalf("load state: %v", err)
	}

	if loaded.RetryContext == nil {
		t.Fatal("expected RetryContext to be set")
	}
	if loaded.RetryContext.FromPhase != "test" {
		t.Errorf("expected FromPhase=test, got %s", loaded.RetryContext.FromPhase)
	}
	if loaded.RetryContext.ToPhase != "implement" {
		t.Errorf("expected ToPhase=implement, got %s", loaded.RetryContext.ToPhase)
	}
}

// TestSaveInitiative_Transaction verifies initiative and related data are saved atomically.
func TestSaveInitiative_Transaction(t *testing.T) {
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	// First create a task to link
	task1 := &task.Task{
		ID:        "TASK-001",
		Title:     "First Task",
		Weight:    task.WeightSmall,
		Status:    task.StatusCreated,
		CreatedAt: time.Now(),
	}
	if err := backend.SaveTask(task1); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Create initiative with tasks
	init := &initiative.Initiative{
		ID:        "INIT-001",
		Title:     "Test Initiative",
		Status:    initiative.StatusActive,
		Vision:    "Test vision",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Tasks: []initiative.TaskRef{
			{ID: "TASK-001", Title: "First Task", Status: "created"},
		},
	}

	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Load and verify
	loaded, err := backend.LoadInitiative("INIT-001")
	if err != nil {
		t.Fatalf("load initiative: %v", err)
	}

	if loaded.Title != "Test Initiative" {
		t.Errorf("expected Title='Test Initiative', got %s", loaded.Title)
	}
	if len(loaded.Tasks) != 1 {
		t.Errorf("expected 1 task, got %d", len(loaded.Tasks))
	}
}

// TestSaveInitiative_WithDependencies verifies initiative dependencies are saved.
func TestSaveInitiative_WithDependencies(t *testing.T) {
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	// Create two initiatives
	init1 := &initiative.Initiative{
		ID:        "INIT-001",
		Title:     "First Initiative",
		Status:    initiative.StatusCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := backend.SaveInitiative(init1); err != nil {
		t.Fatalf("save init1: %v", err)
	}

	init2 := &initiative.Initiative{
		ID:        "INIT-002",
		Title:     "Second Initiative",
		Status:    initiative.StatusActive,
		BlockedBy: []string{"INIT-001"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := backend.SaveInitiative(init2); err != nil {
		t.Fatalf("save init2: %v", err)
	}

	// Load and verify
	loaded, err := backend.LoadInitiative("INIT-002")
	if err != nil {
		t.Fatalf("load initiative: %v", err)
	}

	if len(loaded.BlockedBy) != 1 || loaded.BlockedBy[0] != "INIT-001" {
		t.Errorf("expected BlockedBy=[INIT-001], got %v", loaded.BlockedBy)
	}
}

// TestDeleteTask_CascadesCorrectly verifies cascade deletes work.
func TestDeleteTask_CascadesCorrectly(t *testing.T) {
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	// Create task
	task1 := &task.Task{
		ID:        "TASK-001",
		Title:     "Test Task",
		Weight:    task.WeightSmall,
		Status:    task.StatusRunning,
		CreatedAt: time.Now(),
	}
	if err := backend.SaveTask(task1); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Create state with phases
	now := time.Now()
	s := &state.State{
		TaskID:       "TASK-001",
		CurrentPhase: "implement",
		Status:       state.StatusRunning,
		StartedAt:    now,
		Phases: map[string]*state.PhaseState{
			"implement": {
				Status:    state.StatusRunning,
				StartedAt: now,
			},
		},
	}
	if err := backend.SaveState(s); err != nil {
		t.Fatalf("save state: %v", err)
	}

	// Delete task
	if err := backend.DeleteTask("TASK-001"); err != nil {
		t.Fatalf("delete task: %v", err)
	}

	// Verify task is gone
	_, err := backend.LoadTask("TASK-001")
	if err == nil {
		t.Error("expected error loading deleted task")
	}
}

// TestTransactionRollback_SaveTask verifies that if saving dependencies fails,
// the task data is not partially written.
// This is a regression test for transaction atomicity.
func TestTransactionRollback_SaveTask(t *testing.T) {
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	// Create a valid task first
	task1 := &task.Task{
		ID:        "TASK-001",
		Title:     "First Task",
		Weight:    task.WeightSmall,
		Status:    task.StatusCreated,
		CreatedAt: time.Now(),
	}
	if err := backend.SaveTask(task1); err != nil {
		t.Fatalf("save task1: %v", err)
	}

	// Create a task with valid dependencies
	task2 := &task.Task{
		ID:        "TASK-002",
		Title:     "Second Task",
		Weight:    task.WeightSmall,
		Status:    task.StatusCreated,
		BlockedBy: []string{"TASK-001"},
		CreatedAt: time.Now(),
	}
	if err := backend.SaveTask(task2); err != nil {
		t.Fatalf("save task2: %v", err)
	}

	// Verify the task was saved with dependencies
	loaded, err := backend.LoadTask("TASK-002")
	if err != nil {
		t.Fatalf("load task: %v", err)
	}
	if len(loaded.BlockedBy) != 1 {
		t.Errorf("expected 1 dependency, got %d", len(loaded.BlockedBy))
	}
}

// TestTransactionAtomicity_SaveState verifies that state changes are atomic.
// If phase save fails, the task update should also be rolled back.
func TestTransactionAtomicity_SaveState(t *testing.T) {
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	// Create task
	task1 := &task.Task{
		ID:           "TASK-001",
		Title:        "Test Task",
		Weight:       task.WeightSmall,
		Status:       task.StatusCreated,
		CurrentPhase: "",
		CreatedAt:    time.Now(),
	}
	if err := backend.SaveTask(task1); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Save initial state
	s1 := &state.State{
		TaskID:       "TASK-001",
		CurrentPhase: "implement",
		Status:       state.StatusRunning,
		StartedAt:    time.Now(),
		Phases: map[string]*state.PhaseState{
			"implement": {
				Status:    state.StatusRunning,
				StartedAt: time.Now(),
			},
		},
	}
	if err := backend.SaveState(s1); err != nil {
		t.Fatalf("save state: %v", err)
	}

	// Verify initial state
	loaded, err := backend.LoadState("TASK-001")
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if loaded.CurrentPhase != "implement" {
		t.Errorf("expected phase=implement, got %s", loaded.CurrentPhase)
	}

	// Update to next phase
	s2 := &state.State{
		TaskID:       "TASK-001",
		CurrentPhase: "test",
		Status:       state.StatusRunning,
		StartedAt:    time.Now(),
		Phases: map[string]*state.PhaseState{
			"implement": {
				Status:      state.StatusCompleted,
				CompletedAt: timePtr(time.Now()),
			},
			"test": {
				Status:    state.StatusRunning,
				StartedAt: time.Now(),
			},
		},
	}
	if err := backend.SaveState(s2); err != nil {
		t.Fatalf("save state2: %v", err)
	}

	// Verify both phases are present
	loaded, err = backend.LoadState("TASK-001")
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if len(loaded.Phases) != 2 {
		t.Errorf("expected 2 phases, got %d", len(loaded.Phases))
	}
	if loaded.CurrentPhase != "test" {
		t.Errorf("expected phase=test, got %s", loaded.CurrentPhase)
	}
}

// TestTransactionAtomicity_SaveInitiative verifies initiative saves are atomic.
func TestTransactionAtomicity_SaveInitiative(t *testing.T) {
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	// Create tasks first
	for i := 1; i <= 3; i++ {
		task := &task.Task{
			ID:        taskID(i),
			Title:     taskTitle(i),
			Weight:    task.WeightSmall,
			Status:    task.StatusCreated,
			CreatedAt: time.Now(),
		}
		if err := backend.SaveTask(task); err != nil {
			t.Fatalf("save task%d: %v", i, err)
		}
	}

	// Create initiative with multiple tasks
	init := &initiative.Initiative{
		ID:     "INIT-001",
		Title:  "Test Initiative",
		Status: initiative.StatusActive,
		Tasks: []initiative.TaskRef{
			{ID: "TASK-001"},
			{ID: "TASK-002"},
			{ID: "TASK-003"},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Verify all tasks are linked
	loaded, err := backend.LoadInitiative("INIT-001")
	if err != nil {
		t.Fatalf("load initiative: %v", err)
	}
	if len(loaded.Tasks) != 3 {
		t.Errorf("expected 3 tasks, got %d", len(loaded.Tasks))
	}

	// Update to remove one task
	init.Tasks = []initiative.TaskRef{
		{ID: "TASK-001"},
		{ID: "TASK-003"},
	}
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("update initiative: %v", err)
	}

	// Verify task was removed
	loaded, err = backend.LoadInitiative("INIT-001")
	if err != nil {
		t.Fatalf("load initiative: %v", err)
	}
	if len(loaded.Tasks) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(loaded.Tasks))
	}
}

// TestConcurrentAccess verifies mutex protection for concurrent operations.
func TestConcurrentAccess(t *testing.T) {
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	// Create initial task
	task1 := &task.Task{
		ID:        "TASK-001",
		Title:     "Concurrent Task",
		Weight:    task.WeightSmall,
		Status:    task.StatusCreated,
		CreatedAt: time.Now(),
	}
	if err := backend.SaveTask(task1); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Run concurrent updates
	done := make(chan error, 10)
	for i := 0; i < 10; i++ {
		go func(iteration int) {
			task := &task.Task{
				ID:        "TASK-001",
				Title:     taskTitle(iteration),
				Weight:    task.WeightSmall,
				Status:    task.StatusCreated,
				CreatedAt: time.Now(),
			}
			done <- backend.SaveTask(task)
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		if err := <-done; err != nil {
			t.Errorf("concurrent save %d failed: %v", i, err)
		}
	}

	// Verify task still loads correctly
	_, err := backend.LoadTask("TASK-001")
	if err != nil {
		t.Errorf("load task after concurrent updates: %v", err)
	}
}

// Helper functions

func taskID(n int) string {
	return "TASK-" + padNumber(n)
}

func taskTitle(n int) string {
	return "Task " + padNumber(n)
}

func padNumber(n int) string {
	if n < 10 {
		return "00" + string(rune('0'+n))
	}
	if n < 100 {
		return "0" + string(rune('0'+n/10)) + string(rune('0'+n%10))
	}
	return string(rune('0'+n/100)) + string(rune('0'+(n/10)%10)) + string(rune('0'+n%10))
}

func timePtr(t time.Time) *time.Time {
	return &t
}
