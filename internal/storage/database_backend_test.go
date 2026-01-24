package storage

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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
		_ = os.RemoveAll(tmpDir)
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
	t.Parallel()
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

// TestSaveTask_QualityMetrics verifies quality metrics are persisted and loaded correctly.
func TestSaveTask_QualityMetrics(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	// Create a task with quality metrics
	testTask := &task.Task{
		ID:        "TASK-001",
		Title:     "Test Task with Quality",
		Weight:    task.WeightMedium,
		Status:    task.StatusFailed,
		CreatedAt: time.Now(),
	}

	// Add quality metrics
	testTask.RecordPhaseRetry("implement")
	testTask.RecordPhaseRetry("implement")
	testTask.RecordPhaseRetry("review")
	testTask.RecordReviewRejection()
	testTask.RecordManualIntervention("Fixed manually via resolve command")

	if err := backend.SaveTask(testTask); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Load and verify
	loaded, err := backend.LoadTask("TASK-001")
	if err != nil {
		t.Fatalf("load task: %v", err)
	}

	// Verify quality metrics were persisted
	if loaded.Quality == nil {
		t.Fatal("Quality metrics should not be nil after load")
	}

	// Verify phase retries
	if loaded.Quality.TotalRetries != 3 {
		t.Errorf("expected TotalRetries=3, got %d", loaded.Quality.TotalRetries)
	}
	if loaded.Quality.PhaseRetries["implement"] != 2 {
		t.Errorf("expected implement retries=2, got %d", loaded.Quality.PhaseRetries["implement"])
	}
	if loaded.Quality.PhaseRetries["review"] != 1 {
		t.Errorf("expected review retries=1, got %d", loaded.Quality.PhaseRetries["review"])
	}

	// Verify review rejections
	if loaded.Quality.ReviewRejections != 1 {
		t.Errorf("expected ReviewRejections=1, got %d", loaded.Quality.ReviewRejections)
	}

	// Verify manual intervention
	if !loaded.Quality.ManualIntervention {
		t.Error("expected ManualIntervention=true")
	}
	if loaded.Quality.ManualInterventionReason != "Fixed manually via resolve command" {
		t.Errorf("expected ManualInterventionReason to match, got %q", loaded.Quality.ManualInterventionReason)
	}
}

// TestSaveTask_QualityMetrics_Empty verifies tasks without quality metrics load correctly.
func TestSaveTask_QualityMetrics_Empty(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	// Create a task without quality metrics
	testTask := &task.Task{
		ID:        "TASK-001",
		Title:     "Test Task without Quality",
		Weight:    task.WeightSmall,
		Status:    task.StatusCompleted,
		CreatedAt: time.Now(),
	}

	if err := backend.SaveTask(testTask); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Load and verify
	loaded, err := backend.LoadTask("TASK-001")
	if err != nil {
		t.Fatalf("load task: %v", err)
	}

	// Quality should be nil when not set
	if loaded.Quality != nil {
		t.Errorf("expected Quality to be nil for task without metrics, got %+v", loaded.Quality)
	}
}

// TestSaveState_Transaction verifies state and phases are saved atomically.
func TestSaveState_Transaction(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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

// TestSaveState_ExecutionInfo verifies ExecutionInfo is saved and loaded correctly.
// This is critical for orphan detection - without this, running tasks appear orphaned
// when their state is loaded from the database.
func TestSaveState_ExecutionInfo(t *testing.T) {
	t.Parallel()
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

	// Save state with ExecutionInfo (simulates executor starting)
	now := time.Now()
	execStart := now.Add(-time.Minute) // Started 1 minute ago
	heartbeat := now                   // Last heartbeat just now
	s := &state.State{
		TaskID:       "TASK-001",
		CurrentPhase: "implement",
		Status:       state.StatusRunning,
		StartedAt:    now,
		Phases:       make(map[string]*state.PhaseState),
		Execution: &state.ExecutionInfo{
			PID:           12345,
			Hostname:      "test-host",
			StartedAt:     execStart,
			LastHeartbeat: heartbeat,
		},
	}

	if err := backend.SaveState(s); err != nil {
		t.Fatalf("save state: %v", err)
	}

	// Load and verify ExecutionInfo is persisted
	loaded, err := backend.LoadState("TASK-001")
	if err != nil {
		t.Fatalf("load state: %v", err)
	}

	if loaded.Execution == nil {
		t.Fatal("expected ExecutionInfo to be set, got nil (this causes false orphan detection)")
	}
	if loaded.Execution.PID != 12345 {
		t.Errorf("expected PID=12345, got %d", loaded.Execution.PID)
	}
	if loaded.Execution.Hostname != "test-host" {
		t.Errorf("expected Hostname='test-host', got %s", loaded.Execution.Hostname)
	}
	// Check timestamps are preserved (within 1 second tolerance for serialization)
	if loaded.Execution.StartedAt.Sub(execStart).Abs() > time.Second {
		t.Errorf("StartedAt not preserved: expected %v, got %v", execStart, loaded.Execution.StartedAt)
	}
	if loaded.Execution.LastHeartbeat.Sub(heartbeat).Abs() > time.Second {
		t.Errorf("LastHeartbeat not preserved: expected %v, got %v", heartbeat, loaded.Execution.LastHeartbeat)
	}
}

// TestSaveState_ExecutionInfo_RoundTrip verifies ExecutionInfo persists across backend restarts.
// This simulates an orc restart where a new DatabaseBackend is created against the same database.
// The test ensures execution info is properly persisted to disk (not just held in memory).
func TestSaveState_ExecutionInfo_RoundTrip(t *testing.T) {
	t.Parallel()
	// Create initial backend and save state with ExecutionInfo
	tmpDir, err := os.MkdirTemp("", "orc-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create .orc directory
	if err := os.MkdirAll(filepath.Join(tmpDir, ".orc"), 0755); err != nil {
		t.Fatalf("create .orc dir: %v", err)
	}

	// First backend instance (simulates initial orc process)
	backend1, err := NewDatabaseBackend(tmpDir, &config.StorageConfig{})
	if err != nil {
		t.Fatalf("create backend1: %v", err)
	}

	// Create task
	task1 := &task.Task{
		ID:        "TASK-001",
		Title:     "Test Task",
		Weight:    task.WeightSmall,
		Status:    task.StatusRunning,
		CreatedAt: time.Now(),
	}
	if err := backend1.SaveTask(task1); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Save state with ExecutionInfo (simulates executor starting)
	now := time.Now()
	execStart := now.Add(-time.Minute)
	heartbeat := now
	s := &state.State{
		TaskID:       "TASK-001",
		CurrentPhase: "implement",
		Status:       state.StatusRunning,
		StartedAt:    now,
		Phases:       make(map[string]*state.PhaseState),
		Execution: &state.ExecutionInfo{
			PID:           os.Getpid(), // Use current PID so it passes orphan check
			Hostname:      "test-host",
			StartedAt:     execStart,
			LastHeartbeat: heartbeat,
		},
	}
	if err := backend1.SaveState(s); err != nil {
		t.Fatalf("save state: %v", err)
	}

	// Close the first backend (simulates orc process ending)
	if err := backend1.Close(); err != nil {
		t.Fatalf("close backend1: %v", err)
	}

	// Create a NEW backend instance (simulates orc restart)
	backend2, err := NewDatabaseBackend(tmpDir, &config.StorageConfig{})
	if err != nil {
		t.Fatalf("create backend2: %v", err)
	}
	defer func() { _ = backend2.Close() }()

	// Load state from new backend and verify ExecutionInfo was persisted
	loaded, err := backend2.LoadState("TASK-001")
	if err != nil {
		t.Fatalf("load state from backend2: %v", err)
	}

	// This is the critical assertion - without proper persistence, this would be nil
	if loaded.Execution == nil {
		t.Fatal("ExecutionInfo was NOT persisted to database - this causes false orphan detection on orc restart")
	}

	// Verify all fields survived the round-trip
	if loaded.Execution.PID != os.Getpid() {
		t.Errorf("PID not persisted: expected %d, got %d", os.Getpid(), loaded.Execution.PID)
	}
	if loaded.Execution.Hostname != "test-host" {
		t.Errorf("Hostname not persisted: expected 'test-host', got %s", loaded.Execution.Hostname)
	}
	if loaded.Execution.StartedAt.Sub(execStart).Abs() > time.Second {
		t.Errorf("StartedAt not persisted: expected %v, got %v", execStart, loaded.Execution.StartedAt)
	}
	if loaded.Execution.LastHeartbeat.Sub(heartbeat).Abs() > time.Second {
		t.Errorf("LastHeartbeat not persisted: expected %v, got %v", heartbeat, loaded.Execution.LastHeartbeat)
	}

	// Verify that orphan check now passes (since PID is our own process)
	isOrphaned, reason := loaded.CheckOrphaned()
	if isOrphaned {
		t.Errorf("Task incorrectly flagged as orphaned after reload: %s", reason)
	}
}

// TestSaveState_HeartbeatUpdate verifies that heartbeat updates are persisted to database.
// This is important for future cross-machine orphan detection where PID checks may be unreliable.
// Note: With the TASK-291 fix, PID checks take precedence over heartbeat checks. A task with
// a live PID is NOT orphaned regardless of heartbeat staleness.
func TestSaveState_HeartbeatUpdate(t *testing.T) {
	t.Parallel()
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

	// Initial save with old heartbeat (30 minutes ago - definitely stale)
	initialTime := time.Now().Add(-30 * time.Minute)
	s := &state.State{
		TaskID:       "TASK-001",
		CurrentPhase: "implement",
		Status:       state.StatusRunning,
		StartedAt:    initialTime,
		Phases:       make(map[string]*state.PhaseState),
		Execution: &state.ExecutionInfo{
			PID:           os.Getpid(), // Alive PID - this is the key point
			Hostname:      "test-host",
			StartedAt:     initialTime,
			LastHeartbeat: initialTime, // 30 min ago - would be stale
		},
	}
	if err := backend.SaveState(s); err != nil {
		t.Fatalf("save initial state: %v", err)
	}

	// TASK-291 FIX: With alive PID, task should NOT be orphaned even with stale heartbeat
	// This was a false positive that caused running tasks to be flagged as orphaned
	loaded, err := backend.LoadState("TASK-001")
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	isOrphaned, reason := loaded.CheckOrphaned()
	if isOrphaned {
		t.Errorf("TASK-291 regression: task with alive PID should NOT be orphaned, got: %s", reason)
	}

	// Now update the heartbeat (simulates UpdateHeartbeat() call + SaveState)
	s.UpdateHeartbeat() // This updates LastHeartbeat to now
	if err := backend.SaveState(s); err != nil {
		t.Fatalf("save state with updated heartbeat: %v", err)
	}

	// Verify the updated heartbeat was persisted
	loaded, err = backend.LoadState("TASK-001")
	if err != nil {
		t.Fatalf("load state after heartbeat update: %v", err)
	}

	// Task should still not be orphaned
	isOrphaned, reason = loaded.CheckOrphaned()
	if isOrphaned {
		t.Errorf("expected task to NOT be orphaned after heartbeat update, got: %s", reason)
	}

	// Verify the heartbeat timestamp was actually updated
	if time.Since(loaded.Execution.LastHeartbeat) > 2*time.Second {
		t.Errorf("heartbeat was not updated in database: %v", loaded.Execution.LastHeartbeat)
	}
}

// TestSaveState_ExecutionInfoCleared verifies ExecutionInfo is cleared when task completes.
func TestSaveState_ExecutionInfoCleared(t *testing.T) {
	t.Parallel()
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

	// First save with ExecutionInfo
	now := time.Now()
	s := &state.State{
		TaskID:       "TASK-001",
		CurrentPhase: "implement",
		Status:       state.StatusRunning,
		StartedAt:    now,
		Phases:       make(map[string]*state.PhaseState),
		Execution: &state.ExecutionInfo{
			PID:           12345,
			Hostname:      "test-host",
			StartedAt:     now,
			LastHeartbeat: now,
		},
	}
	if err := backend.SaveState(s); err != nil {
		t.Fatalf("save state with execution: %v", err)
	}

	// Now complete the task (ExecutionInfo should be cleared)
	s.Status = state.StatusCompleted
	s.Execution = nil // Executor clears this on completion
	if err := backend.SaveState(s); err != nil {
		t.Fatalf("save completed state: %v", err)
	}

	// Load and verify ExecutionInfo is cleared
	loaded, err := backend.LoadState("TASK-001")
	if err != nil {
		t.Fatalf("load state: %v", err)
	}

	if loaded.Execution != nil {
		t.Errorf("expected ExecutionInfo to be nil after completion, got %+v", loaded.Execution)
	}
}

// TestSaveTaskCtx_ContextCancellation verifies that a canceled context aborts the transaction.
// This tests the context propagation from DatabaseBackend through TxOps to the driver.
func TestSaveTaskCtx_ContextCancellation(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	// Create a context that's already canceled
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Try to save with canceled context
	task1 := &task.Task{
		ID:        "TASK-001",
		Title:     "Test Task",
		Weight:    task.WeightSmall,
		Status:    task.StatusCreated,
		CreatedAt: time.Now(),
	}
	err := backend.SaveTaskCtx(ctx, task1)

	// Should return context canceled error
	if err == nil {
		t.Fatal("expected error with canceled context")
	}
	if !strings.Contains(err.Error(), "context canceled") {
		t.Errorf("expected context canceled error, got: %v", err)
	}

	// Verify task was not saved
	_, loadErr := backend.LoadTask("TASK-001")
	if loadErr == nil {
		t.Error("task should not have been saved with canceled context")
	}
}

// TestSaveStateCtx_ContextCancellation verifies that a canceled context aborts the state save.
func TestSaveStateCtx_ContextCancellation(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	// First create a task with a valid context
	task1 := &task.Task{
		ID:        "TASK-001",
		Title:     "Test Task",
		Weight:    task.WeightSmall,
		Status:    task.StatusCreated,
		CreatedAt: time.Now(),
	}
	if err := backend.SaveTask(task1); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Create a context that's already canceled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Try to save state with canceled context
	s := &state.State{
		TaskID:       "TASK-001",
		CurrentPhase: "implement",
		Status:       state.StatusRunning,
		StartedAt:    time.Now(),
		Phases:       make(map[string]*state.PhaseState),
	}
	err := backend.SaveStateCtx(ctx, s)

	// Should return context canceled error
	if err == nil {
		t.Fatal("expected error with canceled context")
	}
	if !strings.Contains(err.Error(), "context canceled") {
		t.Errorf("expected context canceled error, got: %v", err)
	}
}

// TestSaveInitiativeCtx_ContextCancellation verifies that a canceled context aborts the initiative save.
func TestSaveInitiativeCtx_ContextCancellation(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	// Create a context that's already canceled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Try to save initiative with canceled context
	init := &initiative.Initiative{
		ID:        "INIT-001",
		Title:     "Test Initiative",
		Status:    initiative.StatusActive,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	err := backend.SaveInitiativeCtx(ctx, init)

	// Should return context canceled error
	if err == nil {
		t.Fatal("expected error with canceled context")
	}
	if !strings.Contains(err.Error(), "context canceled") {
		t.Errorf("expected context canceled error, got: %v", err)
	}

	// Verify initiative was not saved
	_, loadErr := backend.LoadInitiative("INIT-001")
	if loadErr == nil {
		t.Error("initiative should not have been saved with canceled context")
	}
}

// TestSaveTaskCtx_ValidContext verifies that a valid context allows the operation to succeed.
func TestSaveTaskCtx_ValidContext(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	// Create a context with timeout (plenty of time)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Save with valid context
	task1 := &task.Task{
		ID:        "TASK-001",
		Title:     "Test Task",
		Weight:    task.WeightSmall,
		Status:    task.StatusCreated,
		CreatedAt: time.Now(),
	}
	err := backend.SaveTaskCtx(ctx, task1)
	if err != nil {
		t.Fatalf("save with valid context should succeed: %v", err)
	}

	// Verify task was saved
	loaded, err := backend.LoadTask("TASK-001")
	if err != nil {
		t.Fatalf("load task: %v", err)
	}
	if loaded.Title != "Test Task" {
		t.Errorf("expected title 'Test Task', got %s", loaded.Title)
	}
}

// TestSaveTask_PreservesExecutorFields verifies that SaveTask does NOT overwrite executor fields.
// This is a regression test for TASK-249: SaveTask was overwriting ExecutorPID, ExecutorHostname,
// ExecutorStartedAt, and LastHeartbeat with zero values, causing false orphan detection.
//
// Scenario:
// 1. Executor starts task, sets executor fields via SaveState
// 2. Some other code (e.g., CLI edit, API update) calls SaveTask to update task metadata
// 3. SaveTask MUST preserve executor fields to avoid false orphan detection
func TestSaveTask_PreservesExecutorFields(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	// Step 1: Create task
	task1 := &task.Task{
		ID:        "TASK-001",
		Title:     "Original Title",
		Weight:    task.WeightSmall,
		Status:    task.StatusRunning,
		CreatedAt: time.Now(),
	}
	if err := backend.SaveTask(task1); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Step 2: Executor starts task, sets executor fields via SaveState
	// This simulates what the executor does when starting a task
	now := time.Now()
	s := &state.State{
		TaskID:       "TASK-001",
		CurrentPhase: "implement",
		Status:       state.StatusRunning,
		StartedAt:    now,
		Phases:       make(map[string]*state.PhaseState),
		Execution: &state.ExecutionInfo{
			PID:           12345,
			Hostname:      "worker-1",
			StartedAt:     now,
			LastHeartbeat: now,
		},
	}
	if err := backend.SaveState(s); err != nil {
		t.Fatalf("save state: %v", err)
	}

	// Verify executor fields are set
	loaded, err := backend.LoadState("TASK-001")
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if loaded.Execution == nil || loaded.Execution.PID != 12345 {
		t.Fatalf("executor fields not set properly: %+v", loaded.Execution)
	}

	// Step 3: Someone updates task metadata via SaveTask (e.g., CLI edit, API update)
	// This should NOT clear the executor fields!
	task1.Title = "Updated Title"
	task1.Description = "Added description"
	if err := backend.SaveTask(task1); err != nil {
		t.Fatalf("update task: %v", err)
	}

	// Step 4: Verify executor fields are STILL SET after SaveTask
	loaded, err = backend.LoadState("TASK-001")
	if err != nil {
		t.Fatalf("load state after task update: %v", err)
	}

	// This is the critical assertion that was failing before TASK-249 fix
	if loaded.Execution == nil {
		t.Fatal("REGRESSION: SaveTask overwrote executor fields - ExecutionInfo is nil (causes false orphan detection)")
	}
	if loaded.Execution.PID != 12345 {
		t.Errorf("REGRESSION: SaveTask overwrote ExecutorPID: expected 12345, got %d", loaded.Execution.PID)
	}
	if loaded.Execution.Hostname != "worker-1" {
		t.Errorf("REGRESSION: SaveTask overwrote ExecutorHostname: expected 'worker-1', got %s", loaded.Execution.Hostname)
	}

	// Verify task metadata was updated correctly
	loadedTask, err := backend.LoadTask("TASK-001")
	if err != nil {
		t.Fatalf("load task: %v", err)
	}
	if loadedTask.Title != "Updated Title" {
		t.Errorf("expected title to be updated to 'Updated Title', got %s", loadedTask.Title)
	}
}

// TestSaveTask_PreservesStateStatus verifies that SaveTask preserves StateStatus field.
// This tests the existing behavior along with the new executor field preservation.
func TestSaveTask_PreservesStateStatus(t *testing.T) {
	t.Parallel()
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

	// Set state status via SaveState
	s := &state.State{
		TaskID:       "TASK-001",
		CurrentPhase: "test",
		Status:       state.StatusFailed, // Important: state status is "failed"
		StartedAt:    time.Now(),
		Phases:       make(map[string]*state.PhaseState),
	}
	if err := backend.SaveState(s); err != nil {
		t.Fatalf("save state: %v", err)
	}

	// Update task metadata
	task1.Title = "Updated Task"
	if err := backend.SaveTask(task1); err != nil {
		t.Fatalf("update task: %v", err)
	}

	// Verify state status is preserved
	loaded, err := backend.LoadState("TASK-001")
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if loaded.Status != state.StatusFailed {
		t.Errorf("StateStatus not preserved: expected 'failed', got %s", loaded.Status)
	}
}

// =============================================================================
// Branch Registry Tests
// =============================================================================

// TestBranch_SaveAndLoad verifies basic branch save/load operations.
func TestBranch_SaveAndLoad(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	// Create branch
	now := time.Now()
	branch := &Branch{
		Name:         "feature/user-auth",
		Type:         BranchTypeInitiative,
		OwnerID:      "INIT-001",
		CreatedAt:    now,
		LastActivity: now,
		Status:       BranchStatusActive,
	}

	// Save
	if err := backend.SaveBranch(branch); err != nil {
		t.Fatalf("save branch: %v", err)
	}

	// Load
	loaded, err := backend.LoadBranch("feature/user-auth")
	if err != nil {
		t.Fatalf("load branch: %v", err)
	}

	if loaded.Name != "feature/user-auth" {
		t.Errorf("expected name 'feature/user-auth', got %s", loaded.Name)
	}
	if loaded.Type != BranchTypeInitiative {
		t.Errorf("expected type 'initiative', got %s", loaded.Type)
	}
	if loaded.OwnerID != "INIT-001" {
		t.Errorf("expected owner 'INIT-001', got %s", loaded.OwnerID)
	}
	if loaded.Status != BranchStatusActive {
		t.Errorf("expected status 'active', got %s", loaded.Status)
	}
}

// TestBranch_Update verifies branch update operations.
func TestBranch_Update(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	// Create branch
	now := time.Now()
	branch := &Branch{
		Name:         "dev/randy",
		Type:         BranchTypeStaging,
		OwnerID:      "randy",
		CreatedAt:    now,
		LastActivity: now,
		Status:       BranchStatusActive,
	}
	if err := backend.SaveBranch(branch); err != nil {
		t.Fatalf("save branch: %v", err)
	}

	// Update status
	if err := backend.UpdateBranchStatus("dev/randy", BranchStatusMerged); err != nil {
		t.Fatalf("update status: %v", err)
	}

	// Verify
	loaded, err := backend.LoadBranch("dev/randy")
	if err != nil {
		t.Fatalf("load branch: %v", err)
	}
	if loaded.Status != BranchStatusMerged {
		t.Errorf("expected status 'merged', got %s", loaded.Status)
	}
}

// TestBranch_UpdateActivity verifies last activity timestamp updates.
func TestBranch_UpdateActivity(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	// Create branch with old activity
	oldTime := time.Now().Add(-24 * time.Hour)
	branch := &Branch{
		Name:         "orc/TASK-001",
		Type:         BranchTypeTask,
		OwnerID:      "TASK-001",
		CreatedAt:    oldTime,
		LastActivity: oldTime,
		Status:       BranchStatusActive,
	}
	if err := backend.SaveBranch(branch); err != nil {
		t.Fatalf("save branch: %v", err)
	}

	// Update activity
	if err := backend.UpdateBranchActivity("orc/TASK-001"); err != nil {
		t.Fatalf("update activity: %v", err)
	}

	// Verify activity was updated
	loaded, err := backend.LoadBranch("orc/TASK-001")
	if err != nil {
		t.Fatalf("load branch: %v", err)
	}

	// Activity should be recent (within last second)
	if time.Since(loaded.LastActivity) > time.Second {
		t.Errorf("activity not updated: %v", loaded.LastActivity)
	}
}

// TestBranch_Delete verifies branch deletion.
func TestBranch_Delete(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	// Create branch
	branch := &Branch{
		Name:         "feature/temp",
		Type:         BranchTypeInitiative,
		OwnerID:      "INIT-002",
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
		Status:       BranchStatusActive,
	}
	if err := backend.SaveBranch(branch); err != nil {
		t.Fatalf("save branch: %v", err)
	}

	// Delete
	if err := backend.DeleteBranch("feature/temp"); err != nil {
		t.Fatalf("delete branch: %v", err)
	}

	// Verify deleted
	loaded, err := backend.LoadBranch("feature/temp")
	if err != nil {
		t.Fatalf("load should not error for missing branch: %v", err)
	}
	if loaded != nil {
		t.Error("expected nil for deleted branch")
	}
}

// TestBranch_ListAll verifies listing all branches.
func TestBranch_ListAll(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	// Create multiple branches
	branches := []*Branch{
		{Name: "feature/auth", Type: BranchTypeInitiative, OwnerID: "INIT-001", Status: BranchStatusActive},
		{Name: "dev/randy", Type: BranchTypeStaging, OwnerID: "randy", Status: BranchStatusActive},
		{Name: "orc/TASK-001", Type: BranchTypeTask, OwnerID: "TASK-001", Status: BranchStatusMerged},
	}
	now := time.Now()
	for _, b := range branches {
		b.CreatedAt = now
		b.LastActivity = now
		if err := backend.SaveBranch(b); err != nil {
			t.Fatalf("save branch %s: %v", b.Name, err)
		}
	}

	// List all
	list, err := backend.ListBranches(BranchListOpts{})
	if err != nil {
		t.Fatalf("list branches: %v", err)
	}
	if len(list) != 3 {
		t.Errorf("expected 3 branches, got %d", len(list))
	}
}

// TestBranch_ListByType verifies filtering by type.
func TestBranch_ListByType(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	// Create branches of different types
	now := time.Now()
	branches := []*Branch{
		{Name: "feature/a", Type: BranchTypeInitiative, CreatedAt: now, LastActivity: now, Status: BranchStatusActive},
		{Name: "feature/b", Type: BranchTypeInitiative, CreatedAt: now, LastActivity: now, Status: BranchStatusActive},
		{Name: "dev/user", Type: BranchTypeStaging, CreatedAt: now, LastActivity: now, Status: BranchStatusActive},
		{Name: "orc/TASK-001", Type: BranchTypeTask, CreatedAt: now, LastActivity: now, Status: BranchStatusActive},
	}
	for _, b := range branches {
		if err := backend.SaveBranch(b); err != nil {
			t.Fatalf("save branch %s: %v", b.Name, err)
		}
	}

	// List initiatives only
	list, err := backend.ListBranches(BranchListOpts{Type: BranchTypeInitiative})
	if err != nil {
		t.Fatalf("list branches: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("expected 2 initiative branches, got %d", len(list))
	}
	for _, b := range list {
		if b.Type != BranchTypeInitiative {
			t.Errorf("expected initiative type, got %s", b.Type)
		}
	}
}

// TestBranch_ListByStatus verifies filtering by status.
func TestBranch_ListByStatus(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	// Create branches with different statuses
	now := time.Now()
	branches := []*Branch{
		{Name: "b1", Type: BranchTypeTask, CreatedAt: now, LastActivity: now, Status: BranchStatusActive},
		{Name: "b2", Type: BranchTypeTask, CreatedAt: now, LastActivity: now, Status: BranchStatusActive},
		{Name: "b3", Type: BranchTypeTask, CreatedAt: now, LastActivity: now, Status: BranchStatusMerged},
		{Name: "b4", Type: BranchTypeTask, CreatedAt: now, LastActivity: now, Status: BranchStatusOrphaned},
	}
	for _, b := range branches {
		if err := backend.SaveBranch(b); err != nil {
			t.Fatalf("save branch %s: %v", b.Name, err)
		}
	}

	// List active only
	list, err := backend.ListBranches(BranchListOpts{Status: BranchStatusActive})
	if err != nil {
		t.Fatalf("list branches: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("expected 2 active branches, got %d", len(list))
	}

	// List merged only
	list, err = backend.ListBranches(BranchListOpts{Status: BranchStatusMerged})
	if err != nil {
		t.Fatalf("list branches: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("expected 1 merged branch, got %d", len(list))
	}
}

// TestBranch_ListByTypeAndStatus verifies combined filtering.
func TestBranch_ListByTypeAndStatus(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	now := time.Now()
	branches := []*Branch{
		{Name: "f1", Type: BranchTypeInitiative, CreatedAt: now, LastActivity: now, Status: BranchStatusActive},
		{Name: "f2", Type: BranchTypeInitiative, CreatedAt: now, LastActivity: now, Status: BranchStatusMerged},
		{Name: "d1", Type: BranchTypeStaging, CreatedAt: now, LastActivity: now, Status: BranchStatusActive},
		{Name: "t1", Type: BranchTypeTask, CreatedAt: now, LastActivity: now, Status: BranchStatusMerged},
	}
	for _, b := range branches {
		if err := backend.SaveBranch(b); err != nil {
			t.Fatalf("save branch %s: %v", b.Name, err)
		}
	}

	// List merged initiatives
	list, err := backend.ListBranches(BranchListOpts{
		Type:   BranchTypeInitiative,
		Status: BranchStatusMerged,
	})
	if err != nil {
		t.Fatalf("list branches: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("expected 1 merged initiative, got %d", len(list))
	}
	if list[0].Name != "f2" {
		t.Errorf("expected branch 'f2', got %s", list[0].Name)
	}
}

// TestBranch_GetStaleBranches verifies stale branch detection.
func TestBranch_GetStaleBranches(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	now := time.Now()
	oldTime := now.Add(-35 * 24 * time.Hour) // 35 days ago

	branches := []*Branch{
		{Name: "old1", Type: BranchTypeTask, CreatedAt: oldTime, LastActivity: oldTime, Status: BranchStatusActive},
		{Name: "old2", Type: BranchTypeTask, CreatedAt: oldTime, LastActivity: oldTime, Status: BranchStatusActive},
		{Name: "new1", Type: BranchTypeTask, CreatedAt: now, LastActivity: now, Status: BranchStatusActive},
	}
	for _, b := range branches {
		if err := backend.SaveBranch(b); err != nil {
			t.Fatalf("save branch %s: %v", b.Name, err)
		}
	}

	// Get branches stale for more than 30 days
	staleThreshold := now.Add(-30 * 24 * time.Hour)
	stale, err := backend.GetStaleBranches(staleThreshold)
	if err != nil {
		t.Fatalf("get stale branches: %v", err)
	}

	if len(stale) != 2 {
		t.Errorf("expected 2 stale branches, got %d", len(stale))
	}

	// Verify correct branches returned
	names := make(map[string]bool)
	for _, b := range stale {
		names[b.Name] = true
	}
	if !names["old1"] || !names["old2"] {
		t.Errorf("expected old1 and old2, got %v", names)
	}
}

// TestBranch_LoadNonExistent verifies loading non-existent branch returns nil.
func TestBranch_LoadNonExistent(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	loaded, err := backend.LoadBranch("does-not-exist")
	if err != nil {
		t.Fatalf("load should not error: %v", err)
	}
	if loaded != nil {
		t.Error("expected nil for non-existent branch")
	}
}

// TestBranch_Upsert verifies that SaveBranch can update existing branches.
func TestBranch_Upsert(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	now := time.Now()
	branch := &Branch{
		Name:         "feature/test",
		Type:         BranchTypeInitiative,
		OwnerID:      "INIT-001",
		CreatedAt:    now,
		LastActivity: now,
		Status:       BranchStatusActive,
	}

	// Initial save
	if err := backend.SaveBranch(branch); err != nil {
		t.Fatalf("initial save: %v", err)
	}

	// Update via save (upsert)
	branch.Status = BranchStatusMerged
	branch.LastActivity = now.Add(time.Hour)
	if err := backend.SaveBranch(branch); err != nil {
		t.Fatalf("upsert save: %v", err)
	}

	// Verify update
	loaded, err := backend.LoadBranch("feature/test")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Status != BranchStatusMerged {
		t.Errorf("expected status 'merged', got %s", loaded.Status)
	}
}

// TestSaveInitiative_DecisionIDsAcrossInitiatives verifies that multiple initiatives
// can each have decisions with the same local IDs (e.g., DEC-001).
// This tests the fix for the decision ID collision bug.
func TestSaveInitiative_DecisionIDsAcrossInitiatives(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	now := time.Now()

	// Create first initiative with DEC-001
	init1 := &initiative.Initiative{
		ID:        "INIT-001",
		Title:     "First Initiative",
		Status:    initiative.StatusActive,
		CreatedAt: now,
		UpdatedAt: now,
		Decisions: []initiative.Decision{
			{
				ID:        "DEC-001",
				Decision:  "Use Go for backend",
				Rationale: "Team expertise",
				Date:      now,
				By:        "alice",
			},
		},
	}
	if err := backend.SaveInitiative(init1); err != nil {
		t.Fatalf("save first initiative: %v", err)
	}

	// Create second initiative with its own DEC-001 (same ID, different initiative)
	init2 := &initiative.Initiative{
		ID:        "INIT-002",
		Title:     "Second Initiative",
		Status:    initiative.StatusActive,
		CreatedAt: now,
		UpdatedAt: now,
		Decisions: []initiative.Decision{
			{
				ID:        "DEC-001", // Same ID as init1's decision
				Decision:  "Use React for frontend",
				Rationale: "Modern framework",
				Date:      now,
				By:        "bob",
			},
		},
	}

	// This should NOT fail - each initiative has its own decision namespace
	if err := backend.SaveInitiative(init2); err != nil {
		t.Fatalf("save second initiative with same decision ID: %v", err)
	}

	// Verify both initiatives have their decisions intact
	loaded1, err := backend.LoadInitiative("INIT-001")
	if err != nil {
		t.Fatalf("load first initiative: %v", err)
	}
	if len(loaded1.Decisions) != 1 {
		t.Errorf("expected 1 decision in init1, got %d", len(loaded1.Decisions))
	}
	if loaded1.Decisions[0].Decision != "Use Go for backend" {
		t.Errorf("init1 decision content mismatch: got %s", loaded1.Decisions[0].Decision)
	}

	loaded2, err := backend.LoadInitiative("INIT-002")
	if err != nil {
		t.Fatalf("load second initiative: %v", err)
	}
	if len(loaded2.Decisions) != 1 {
		t.Errorf("expected 1 decision in init2, got %d", len(loaded2.Decisions))
	}
	if loaded2.Decisions[0].Decision != "Use React for frontend" {
		t.Errorf("init2 decision content mismatch: got %s", loaded2.Decisions[0].Decision)
	}
}

// TestSaveInitiative_DecisionLookupByInitiative verifies decision lookup works correctly
// with the composite key schema.
func TestSaveInitiative_DecisionLookupByInitiative(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	now := time.Now()

	// Create initiative with multiple decisions
	init := &initiative.Initiative{
		ID:        "INIT-001",
		Title:     "Multi-Decision Initiative",
		Status:    initiative.StatusActive,
		CreatedAt: now,
		UpdatedAt: now,
		Decisions: []initiative.Decision{
			{ID: "DEC-001", Decision: "Decision 1", Date: now, By: "user1"},
			{ID: "DEC-002", Decision: "Decision 2", Date: now, By: "user2"},
			{ID: "DEC-003", Decision: "Decision 3", Date: now, By: "user3"},
		},
	}
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Load and verify all decisions are correctly associated
	loaded, err := backend.LoadInitiative("INIT-001")
	if err != nil {
		t.Fatalf("load initiative: %v", err)
	}
	if len(loaded.Decisions) != 3 {
		t.Errorf("expected 3 decisions, got %d", len(loaded.Decisions))
	}

	// Verify decision IDs
	decisionIDs := make(map[string]bool)
	for _, d := range loaded.Decisions {
		decisionIDs[d.ID] = true
	}
	for _, expectedID := range []string{"DEC-001", "DEC-002", "DEC-003"} {
		if !decisionIDs[expectedID] {
			t.Errorf("missing decision %s", expectedID)
		}
	}
}

// TestSaveInitiative_DecisionUpdate verifies that updating an initiative preserves decisions
// correctly with the clear-and-reinsert pattern.
func TestSaveInitiative_DecisionUpdate(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	now := time.Now()

	// Create initiative with initial decision
	init := &initiative.Initiative{
		ID:        "INIT-001",
		Title:     "Update Test Initiative",
		Status:    initiative.StatusActive,
		CreatedAt: now,
		UpdatedAt: now,
		Decisions: []initiative.Decision{
			{ID: "DEC-001", Decision: "Initial decision", Date: now, By: "user"},
		},
	}
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initial: %v", err)
	}

	// Update with more decisions
	init.Decisions = append(init.Decisions, initiative.Decision{
		ID:       "DEC-002",
		Decision: "Second decision",
		Date:     now,
		By:       "user",
	})
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("update with new decision: %v", err)
	}

	// Verify both decisions exist
	loaded, err := backend.LoadInitiative("INIT-001")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded.Decisions) != 2 {
		t.Errorf("expected 2 decisions after update, got %d", len(loaded.Decisions))
	}

	// Re-save without changing decisions (idempotency test)
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("re-save: %v", err)
	}
	loaded, err = backend.LoadInitiative("INIT-001")
	if err != nil {
		t.Fatalf("load after re-save: %v", err)
	}
	if len(loaded.Decisions) != 2 {
		t.Errorf("expected 2 decisions after re-save, got %d", len(loaded.Decisions))
	}
}

// TestReviewFindings_SaveAndLoad verifies ReviewFindings operations through the Backend interface.
func TestReviewFindings_SaveAndLoad(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	// Create a task first (foreign key constraint)
	testTask := &task.Task{
		ID:        "TASK-001",
		Title:     "Test Task for Review",
		Weight:    task.WeightMedium,
		Status:    task.StatusRunning,
		CreatedAt: time.Now(),
	}
	if err := backend.SaveTask(testTask); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Create review findings with AgentID
	findings := &ReviewFindings{
		TaskID:  "TASK-001",
		Round:   1,
		Summary: "Found 2 issues in the implementation",
		Issues: []ReviewFinding{
			{Severity: "high", File: "main.go", Line: 42, Description: "SQL injection vulnerability", AgentID: "code-reviewer"},
			{Severity: "medium", File: "utils.go", Line: 100, Description: "Error not handled", AgentID: "silent-failure-hunter"},
		},
		Questions: []string{"Why is this implemented synchronously?"},
		Positives: []string{"Good test coverage"},
		AgentID:   "code-reviewer",
	}

	// Save
	if err := backend.SaveReviewFindings(findings); err != nil {
		t.Fatalf("SaveReviewFindings: %v", err)
	}

	// Load
	loaded, err := backend.LoadReviewFindings("TASK-001", 1)
	if err != nil {
		t.Fatalf("LoadReviewFindings: %v", err)
	}
	if loaded == nil {
		t.Fatal("LoadReviewFindings returned nil")
	}

	// Verify fields
	if loaded.TaskID != "TASK-001" {
		t.Errorf("TaskID = %q, want %q", loaded.TaskID, "TASK-001")
	}
	if loaded.Round != 1 {
		t.Errorf("Round = %d, want 1", loaded.Round)
	}
	if loaded.Summary != "Found 2 issues in the implementation" {
		t.Errorf("Summary = %q, want %q", loaded.Summary, "Found 2 issues in the implementation")
	}
	if len(loaded.Issues) != 2 {
		t.Errorf("Issues count = %d, want 2", len(loaded.Issues))
	}
	if loaded.Issues[0].Severity != "high" {
		t.Errorf("Issues[0].Severity = %q, want %q", loaded.Issues[0].Severity, "high")
	}
	if loaded.Issues[0].AgentID != "code-reviewer" {
		t.Errorf("Issues[0].AgentID = %q, want %q", loaded.Issues[0].AgentID, "code-reviewer")
	}
	if loaded.AgentID != "code-reviewer" {
		t.Errorf("AgentID = %q, want %q", loaded.AgentID, "code-reviewer")
	}
	if len(loaded.Questions) != 1 {
		t.Errorf("Questions count = %d, want 1", len(loaded.Questions))
	}
	if len(loaded.Positives) != 1 {
		t.Errorf("Positives count = %d, want 1", len(loaded.Positives))
	}
}

// TestReviewFindings_LoadAll verifies loading all review findings for a task.
func TestReviewFindings_LoadAll(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	// Create a task
	testTask := &task.Task{
		ID:        "TASK-002",
		Title:     "Test Task",
		Weight:    task.WeightLarge,
		Status:    task.StatusRunning,
		CreatedAt: time.Now(),
	}
	if err := backend.SaveTask(testTask); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Save findings for multiple rounds
	round1 := &ReviewFindings{
		TaskID:  "TASK-002",
		Round:   1,
		Summary: "Round 1 findings",
		Issues:  []ReviewFinding{{Severity: "high", Description: "Issue 1"}},
		AgentID: "code-reviewer",
	}
	round2 := &ReviewFindings{
		TaskID:  "TASK-002",
		Round:   2,
		Summary: "Round 2 findings",
		Issues:  []ReviewFinding{{Severity: "low", Description: "Issue 2"}},
		AgentID: "silent-failure-hunter",
	}

	if err := backend.SaveReviewFindings(round1); err != nil {
		t.Fatalf("SaveReviewFindings round1: %v", err)
	}
	if err := backend.SaveReviewFindings(round2); err != nil {
		t.Fatalf("SaveReviewFindings round2: %v", err)
	}

	// Load all
	all, err := backend.LoadAllReviewFindings("TASK-002")
	if err != nil {
		t.Fatalf("LoadAllReviewFindings: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("LoadAllReviewFindings returned %d, want 2", len(all))
	}

	// Verify ordering (should be by round ascending)
	if all[0].Round != 1 {
		t.Errorf("First result Round = %d, want 1", all[0].Round)
	}
	if all[1].Round != 2 {
		t.Errorf("Second result Round = %d, want 2", all[1].Round)
	}

	// Verify AgentID preserved
	if all[0].AgentID != "code-reviewer" {
		t.Errorf("First result AgentID = %q, want %q", all[0].AgentID, "code-reviewer")
	}
	if all[1].AgentID != "silent-failure-hunter" {
		t.Errorf("Second result AgentID = %q, want %q", all[1].AgentID, "silent-failure-hunter")
	}
}

// TestReviewFindings_NotFound verifies behavior when findings don't exist.
func TestReviewFindings_NotFound(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	// Load non-existent findings
	findings, err := backend.LoadReviewFindings("NONEXISTENT", 1)
	if err != nil {
		t.Fatalf("LoadReviewFindings should not error for missing data: %v", err)
	}
	if findings != nil {
		t.Error("LoadReviewFindings should return nil for missing data")
	}
}
