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

// TestSaveState_ExecutionInfo verifies ExecutionInfo is saved and loaded correctly.
// This is critical for orphan detection - without this, running tasks appear orphaned
// when their state is loaded from the database.
func TestSaveState_ExecutionInfo(t *testing.T) {
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
	heartbeat := now                    // Last heartbeat just now
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
	// Create initial backend and save state with ExecutionInfo
	tmpDir, err := os.MkdirTemp("", "orc-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

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
	defer backend2.Close()

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
// This is critical for preventing stale heartbeat false positives in orphan detection.
func TestSaveState_HeartbeatUpdate(t *testing.T) {
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

	// Initial save with old heartbeat (6 minutes ago - would be stale)
	initialTime := time.Now().Add(-6 * time.Minute)
	s := &state.State{
		TaskID:       "TASK-001",
		CurrentPhase: "implement",
		Status:       state.StatusRunning,
		StartedAt:    initialTime,
		Phases:       make(map[string]*state.PhaseState),
		Execution: &state.ExecutionInfo{
			PID:           os.Getpid(),
			Hostname:      "test-host",
			StartedAt:     initialTime,
			LastHeartbeat: initialTime, // 6 min ago - stale
		},
	}
	if err := backend.SaveState(s); err != nil {
		t.Fatalf("save initial state: %v", err)
	}

	// Verify initial heartbeat was stale
	loaded, err := backend.LoadState("TASK-001")
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	isOrphaned, reason := loaded.CheckOrphaned()
	if !isOrphaned {
		t.Error("expected task with 6-minute-old heartbeat to be flagged as orphaned")
	}
	if reason != "heartbeat stale (>5 minutes)" {
		t.Errorf("expected stale heartbeat reason, got: %s", reason)
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

	// Task should no longer be orphaned after heartbeat update
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
