package storage

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/task"
)

// TestTryClaimTaskExecution_SuccessfulClaim tests basic claim operation.
// Covers: SC-2 (only one PID written)
func TestTryClaimTaskExecution_SuccessfulClaim(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	// Create a failed task (resumable)
	tk := &task.Task{
		ID:        "TASK-001",
		Title:     "Test Task",
		Weight:    task.WeightSmall,
		Status:    task.StatusFailed,
		CreatedAt: time.Now(),
	}
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Create state with failed status
	s := state.New("TASK-001")
	s.Status = state.StatusFailed
	s.CurrentPhase = "implement"
	if err := backend.SaveState(s); err != nil {
		t.Fatalf("save state: %v", err)
	}

	// Claim the task
	pid := os.Getpid()
	hostname, _ := os.Hostname()
	ctx := context.Background()

	err := backend.TryClaimTaskExecution(ctx, "TASK-001", pid, hostname)
	if err != nil {
		t.Fatalf("claim should succeed: %v", err)
	}

	// Verify PID was written
	updatedState, err := backend.LoadState("TASK-001")
	if err != nil {
		t.Fatalf("load state: %v", err)
	}

	if updatedState.Execution == nil {
		t.Fatal("Execution info should be set")
	}
	if updatedState.Execution.PID != pid {
		t.Errorf("Expected PID %d, got %d", pid, updatedState.Execution.PID)
	}
	if updatedState.Execution.Hostname != hostname {
		t.Errorf("Expected hostname %q, got %q", hostname, updatedState.Execution.Hostname)
	}
	if updatedState.Status != state.StatusRunning {
		t.Errorf("Expected status running, got %s", updatedState.Status)
	}
}

// TestTryClaimTaskExecution_ConcurrentAttempts tests race condition protection.
// Covers: SC-1 (second attempt rejected), SC-3 (atomic claim operation)
func TestTryClaimTaskExecution_ConcurrentAttempts(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	// Create a paused task
	tk := &task.Task{
		ID:        "TASK-001",
		Title:     "Test Task",
		Weight:    task.WeightSmall,
		Status:    task.StatusPaused,
		CreatedAt: time.Now(),
	}
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	s := state.New("TASK-001")
	s.Status = state.StatusPaused
	s.CurrentPhase = "implement"
	if err := backend.SaveState(s); err != nil {
		t.Fatalf("save state: %v", err)
	}

	// Simulate concurrent resume attempts (10 goroutines trying to claim)
	// All use the same real PID - this simulates the realistic case where
	// after one claim succeeds, the PID is alive and subsequent claims fail
	const numAttempts = 10
	var wg sync.WaitGroup
	results := make(chan error, numAttempts)

	hostname, _ := os.Hostname()
	ctx := context.Background()
	pid := os.Getpid() // Use real PID - will be alive, preventing subsequent claims

	for i := 0; i < numAttempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := backend.TryClaimTaskExecution(ctx, "TASK-001", pid, hostname)
			results <- err
		}()
	}

	wg.Wait()
	close(results)

	// Count successes and failures
	var successCount, failureCount int
	for err := range results {
		if err == nil {
			successCount++
		} else {
			failureCount++
		}
	}

	// Exactly 1 should succeed
	if successCount != 1 {
		t.Errorf("Expected exactly 1 successful claim, got %d", successCount)
	}
	if failureCount != numAttempts-1 {
		t.Errorf("Expected %d failures, got %d", numAttempts-1, failureCount)
	}

	// Verify PID was written
	finalState, err := backend.LoadState("TASK-001")
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if finalState.Execution == nil {
		t.Fatal("Execution info should be set")
	}
	// PID should be our process PID
	if finalState.Execution.PID != pid {
		t.Errorf("Expected PID %d, got %d", pid, finalState.Execution.PID)
	}
}

// TestTryClaimTaskExecution_AlreadyClaimed tests rejection when task is already claimed.
// Covers: SC-1 (second attempt rejected with clear error)
func TestTryClaimTaskExecution_AlreadyClaimed(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	// Create a blocked task
	tk := &task.Task{
		ID:        "TASK-001",
		Title:     "Test Task",
		Weight:    task.WeightSmall,
		Status:    task.StatusBlocked,
		CreatedAt: time.Now(),
	}
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	s := state.New("TASK-001")
	s.Status = state.StatusPaused // Use paused state for blocked task
	s.CurrentPhase = "implement"
	if err := backend.SaveState(s); err != nil {
		t.Fatalf("save state: %v", err)
	}

	hostname, _ := os.Hostname()
	ctx := context.Background()

	// First claim succeeds
	pid1 := os.Getpid()
	err := backend.TryClaimTaskExecution(ctx, "TASK-001", pid1, hostname)
	if err != nil {
		t.Fatalf("first claim should succeed: %v", err)
	}

	// Second claim should fail
	pid2 := pid1 + 1 // Different PID (but same process family, still alive)
	err = backend.TryClaimTaskExecution(ctx, "TASK-001", pid2, hostname)
	if err == nil {
		t.Fatal("second claim should fail")
	}

	// Error should mention PID
	expectedSubstr := "already claimed"
	if !containsSubstring(err.Error(), expectedSubstr) {
		t.Errorf("Error should contain %q, got: %v", expectedSubstr, err)
	}
}

// TestTryClaimTaskExecution_StalePID tests claiming when previous executor died.
// Covers: SC-4 (orphan detection), SC-6 (stale PID doesn't block resume)
func TestTryClaimTaskExecution_StalePID(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	// Create a running task with a dead PID
	tk := &task.Task{
		ID:        "TASK-001",
		Title:     "Test Task",
		Weight:    task.WeightSmall,
		Status:    task.StatusRunning,
		CreatedAt: time.Now(),
	}
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	s := state.New("TASK-001")
	s.Status = state.StatusRunning
	s.CurrentPhase = "implement"
	// Set a PID that's very unlikely to exist (high number)
	s.StartExecution(999999, "old-host")
	if err := backend.SaveState(s); err != nil {
		t.Fatalf("save state: %v", err)
	}

	// New claim should succeed (detects dead PID)
	newPID := os.Getpid()
	hostname, _ := os.Hostname()
	ctx := context.Background()

	err := backend.TryClaimTaskExecution(ctx, "TASK-001", newPID, hostname)
	if err != nil {
		t.Fatalf("claim should succeed with dead PID: %v", err)
	}

	// Verify new PID overwrote old one
	updatedState, err := backend.LoadState("TASK-001")
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if updatedState.Execution == nil {
		t.Fatal("Execution info should be set")
	}
	if updatedState.Execution.PID != newPID {
		t.Errorf("Expected new PID %d, got %d", newPID, updatedState.Execution.PID)
	}
	if updatedState.Execution.Hostname != hostname {
		t.Errorf("Expected new hostname %q, got %q", hostname, updatedState.Execution.Hostname)
	}
}

// TestTryClaimTaskExecution_NonResumableStatus tests claim rejection for non-resumable tasks.
// Covers: Edge case from spec (completed task)
func TestTryClaimTaskExecution_NonResumableStatus(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	// Create a completed task
	tk := &task.Task{
		ID:        "TASK-001",
		Title:     "Test Task",
		Weight:    task.WeightSmall,
		Status:    task.StatusCompleted,
		CreatedAt: time.Now(),
	}
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	s := state.New("TASK-001")
	s.Status = state.StatusCompleted
	if err := backend.SaveState(s); err != nil {
		t.Fatalf("save state: %v", err)
	}

	ctx := context.Background()
	pid := os.Getpid()
	hostname, _ := os.Hostname()

	err := backend.TryClaimTaskExecution(ctx, "TASK-001", pid, hostname)
	if err == nil {
		t.Fatal("claim should fail for completed task")
	}

	expectedSubstr := "cannot be claimed"
	if !containsSubstring(err.Error(), expectedSubstr) {
		t.Errorf("Error should contain %q, got: %v", expectedSubstr, err)
	}
}

// TestTryClaimTaskExecution_PausedTask tests claiming paused task.
// Covers: SC-5 (paused state remains functional)
func TestTryClaimTaskExecution_PausedTask(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	// Create a paused task
	tk := &task.Task{
		ID:        "TASK-001",
		Title:     "Test Task",
		Weight:    task.WeightSmall,
		Status:    task.StatusPaused,
		CreatedAt: time.Now(),
	}
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	s := state.New("TASK-001")
	s.Status = state.StatusPaused
	s.CurrentPhase = "implement"
	if err := backend.SaveState(s); err != nil {
		t.Fatalf("save state: %v", err)
	}

	ctx := context.Background()
	pid := os.Getpid()
	hostname, _ := os.Hostname()

	err := backend.TryClaimTaskExecution(ctx, "TASK-001", pid, hostname)
	if err != nil {
		t.Fatalf("claim paused task should succeed: %v", err)
	}

	// Verify task is now running
	updatedState, err := backend.LoadState("TASK-001")
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if updatedState.Status != state.StatusRunning {
		t.Errorf("Expected status running, got %s", updatedState.Status)
	}
}

// TestTryClaimTaskExecution_TaskNotFound tests error handling for non-existent task.
// Covers: Error path - resource not found
func TestTryClaimTaskExecution_TaskNotFound(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	ctx := context.Background()
	pid := os.Getpid()
	hostname, _ := os.Hostname()

	err := backend.TryClaimTaskExecution(ctx, "TASK-999", pid, hostname)
	if err == nil {
		t.Fatal("claim should fail for non-existent task")
	}

	expectedSubstr := "not found"
	if !containsSubstring(err.Error(), expectedSubstr) {
		t.Errorf("Error should contain %q, got: %v", expectedSubstr, err)
	}
}

// TestTryClaimTaskExecution_HeartbeatUpdated tests that heartbeat is set on claim.
// Covers: Additional behavior verification
func TestTryClaimTaskExecution_HeartbeatUpdated(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	// Create a failed task
	tk := &task.Task{
		ID:        "TASK-001",
		Title:     "Test Task",
		Weight:    task.WeightSmall,
		Status:    task.StatusFailed,
		CreatedAt: time.Now(),
	}
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	s := state.New("TASK-001")
	s.Status = state.StatusFailed
	if err := backend.SaveState(s); err != nil {
		t.Fatalf("save state: %v", err)
	}

	// Truncate to second precision since RFC3339 storage only has second granularity
	beforeClaim := time.Now().Truncate(time.Second)

	ctx := context.Background()
	pid := os.Getpid()
	hostname, _ := os.Hostname()

	err := backend.TryClaimTaskExecution(ctx, "TASK-001", pid, hostname)
	if err != nil {
		t.Fatalf("claim should succeed: %v", err)
	}

	// Verify heartbeat was set
	updatedState, err := backend.LoadState("TASK-001")
	if err != nil {
		t.Fatalf("load state: %v", err)
	}

	if updatedState.Execution == nil {
		t.Fatal("Execution info should be set")
	}
	// Heartbeat should be at or after beforeClaim (comparing at second precision)
	heartbeatTrunc := updatedState.Execution.LastHeartbeat.Truncate(time.Second)
	if heartbeatTrunc.Before(beforeClaim) {
		t.Errorf("Heartbeat should be updated, got %v before claim at %v", heartbeatTrunc, beforeClaim)
	}
}

// containsSubstring is a helper for error message checking
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
