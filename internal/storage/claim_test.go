package storage

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

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
		ID:           "TASK-001",
		Title:        "Test Task",
		Weight:       task.WeightSmall,
		Status:       task.StatusFailed,
		CurrentPhase: "implement",
		CreatedAt:    time.Now(),
		Execution:    task.InitExecutionState(),
	}
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Claim the task
	pid := os.Getpid()
	hostname, _ := os.Hostname()
	ctx := context.Background()

	err := backend.TryClaimTaskExecution(ctx, "TASK-001", pid, hostname)
	if err != nil {
		t.Fatalf("claim should succeed: %v", err)
	}

	// Verify PID was written to task
	updatedTask, err := backend.LoadTask("TASK-001")
	if err != nil {
		t.Fatalf("load task: %v", err)
	}

	if updatedTask.ExecutorPID != pid {
		t.Errorf("Expected PID %d, got %d", pid, updatedTask.ExecutorPID)
	}
	if updatedTask.Status != task.StatusRunning {
		t.Errorf("Expected status running, got %s", updatedTask.Status)
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
		ID:           "TASK-001",
		Title:        "Test Task",
		Weight:       task.WeightSmall,
		Status:       task.StatusPaused,
		CurrentPhase: "implement",
		CreatedAt:    time.Now(),
		Execution:    task.InitExecutionState(),
	}
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
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

	// Verify PID was written to task
	finalTask, err := backend.LoadTask("TASK-001")
	if err != nil {
		t.Fatalf("load task: %v", err)
	}
	// PID should be our process PID
	if finalTask.ExecutorPID != pid {
		t.Errorf("Expected PID %d, got %d", pid, finalTask.ExecutorPID)
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
		ID:           "TASK-001",
		Title:        "Test Task",
		Weight:       task.WeightSmall,
		Status:       task.StatusBlocked,
		CurrentPhase: "implement",
		CreatedAt:    time.Now(),
		Execution:    task.InitExecutionState(),
	}
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
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
		ID:           "TASK-001",
		Title:        "Test Task",
		Weight:       task.WeightSmall,
		Status:       task.StatusRunning,
		CurrentPhase: "implement",
		ExecutorPID:  999999, // Dead PID - very high number unlikely to exist
		CreatedAt:    time.Now(),
		Execution:    task.InitExecutionState(),
	}
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// New claim should succeed (detects dead PID)
	newPID := os.Getpid()
	hostname, _ := os.Hostname()
	ctx := context.Background()

	err := backend.TryClaimTaskExecution(ctx, "TASK-001", newPID, hostname)
	if err != nil {
		t.Fatalf("claim should succeed with dead PID: %v", err)
	}

	// Verify new PID overwrote old one on task
	updatedTask, err := backend.LoadTask("TASK-001")
	if err != nil {
		t.Fatalf("load task: %v", err)
	}
	if updatedTask.ExecutorPID != newPID {
		t.Errorf("Expected new PID %d, got %d", newPID, updatedTask.ExecutorPID)
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
		Execution: task.InitExecutionState(),
	}
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	ctx := context.Background()
	pid := os.Getpid()
	hostname, _ := os.Hostname()

	err := backend.TryClaimTaskExecution(ctx, "TASK-001", pid, hostname)
	if err == nil {
		t.Fatal("claim should fail for completed task")
	}

	// Error should mention cannot be resumed
	expectedSubstr := "cannot be resumed"
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
		ID:           "TASK-001",
		Title:        "Test Task",
		Weight:       task.WeightSmall,
		Status:       task.StatusPaused,
		CurrentPhase: "implement",
		CreatedAt:    time.Now(),
		Execution:    task.InitExecutionState(),
	}
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	ctx := context.Background()
	pid := os.Getpid()
	hostname, _ := os.Hostname()

	err := backend.TryClaimTaskExecution(ctx, "TASK-001", pid, hostname)
	if err != nil {
		t.Fatalf("claim paused task should succeed: %v", err)
	}

	// Verify task is now running
	updatedTask, err := backend.LoadTask("TASK-001")
	if err != nil {
		t.Fatalf("load task: %v", err)
	}
	if updatedTask.Status != task.StatusRunning {
		t.Errorf("Expected status running, got %s", updatedTask.Status)
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
		Execution: task.InitExecutionState(),
	}
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
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

	// Verify heartbeat was set on task
	updatedTask, err := backend.LoadTask("TASK-001")
	if err != nil {
		t.Fatalf("load task: %v", err)
	}

	// Heartbeat should be at or after beforeClaim (comparing at second precision)
	heartbeatTrunc := updatedTask.LastHeartbeat.Truncate(time.Second)
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
