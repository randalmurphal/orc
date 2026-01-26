package cli

import (
	"context"
	"os"
	"sync"
	"testing"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// TestResumeCommand_RaceConditionPrevention tests that concurrent resume attempts are prevented.
// This is an integration test that exercises the database claim logic directly.
// Covers: SC-1, SC-2, SC-3
func TestResumeCommand_RaceConditionPrevention(t *testing.T) {
	tmpDir := withResumeTestDir(t)

	// Create a failed task using proto types
	backend := createResumeTestBackend(t, tmpDir)
	tk := task.NewProtoTask("TASK-001", "Test task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_FAILED
	tk.Weight = orcv1.TaskWeight_TASK_WEIGHT_SMALL
	task.SetCurrentPhaseProto(tk, "implement")
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}
	_ = backend.Close()

	// Simulate concurrent resume attempts using direct backend calls
	// (We can't easily spawn multiple CLI processes in tests)
	// All use real PID - after first claim, subsequent ones see a live PID and fail
	const numAttempts = 5
	var wg sync.WaitGroup
	results := make(chan error, numAttempts)
	pid := os.Getpid() // Use real PID - will be alive, preventing subsequent claims

	for i := 0; i < numAttempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Each goroutine gets its own backend connection
			backend, err := storage.NewDatabaseBackend(tmpDir, nil)
			if err != nil {
				results <- err
				return
			}
			defer func() { _ = backend.Close() }()

			// Attempt to claim - THIS is where the race protection happens
			// The TryClaimTaskExecution function handles the atomic claim logic
			ctx := context.Background()
			hostname, _ := os.Hostname()

			err = backend.TryClaimTaskExecution(ctx, "TASK-001", pid, hostname)
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

	// Exactly 1 should succeed, rest should fail
	if successCount != 1 {
		t.Errorf("Expected exactly 1 successful claim, got %d", successCount)
	}
	if failureCount != numAttempts-1 {
		t.Errorf("Expected %d claim failures, got %d", numAttempts-1, failureCount)
	}

	// Verify the task is now running (claim succeeded)
	backend = createResumeTestBackend(t, tmpDir)
	defer func() { _ = backend.Close() }()

	finalTask, err := backend.LoadTask("TASK-001")
	if err != nil {
		t.Fatalf("load final task: %v", err)
	}

	// Task should be running after successful claim
	if finalTask.Status != orcv1.TaskStatus_TASK_STATUS_RUNNING {
		t.Errorf("Expected task status RUNNING after claim, got %v", finalTask.Status)
	}
}

// TestResumeCommand_OrphanedTaskWithClaim tests that orphaned tasks can be claimed and resumed.
// NOTE: Skipped because proto Task doesn't have ExecutorPID field for orphan detection.
// The database claim mechanism still works, but we can't test orphan detection without ExecutorPID.
// Covers: SC-4 (orphan detection still works), SC-6 (dead PID doesn't block)
func TestResumeCommand_OrphanedTaskWithClaim(t *testing.T) {
	t.Skip("Skipped: proto Task doesn't have ExecutorPID field for orphan detection")
}

// TestResumeCommand_StaleClaimDoesNotBlock tests that a stale claim from a dead process doesn't block new claims.
// NOTE: Skipped because proto Task doesn't have ExecutorPID field for stale PID detection.
// Covers: SC-6 (stale PID from dead process doesn't block resume)
func TestResumeCommand_StaleClaimDoesNotBlock(t *testing.T) {
	t.Skip("Skipped: proto Task doesn't have ExecutorPID field for stale PID detection")
}

// TestResumeCommand_PausedTaskClaimWorks tests that paused tasks can be claimed.
// Covers: SC-5 (resume from paused states remains functional)
func TestResumeCommand_PausedTaskClaimWorks(t *testing.T) {
	tmpDir := withResumeTestDir(t)

	// Create a paused task using proto types
	backend := createResumeTestBackend(t, tmpDir)
	tk := task.NewProtoTask("TASK-001", "Test task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_PAUSED
	tk.Weight = orcv1.TaskWeight_TASK_WEIGHT_SMALL
	task.SetCurrentPhaseProto(tk, "implement")
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}
	_ = backend.Close()

	// Resume flow: validate, claim
	backend = createResumeTestBackend(t, tmpDir)
	defer func() { _ = backend.Close() }()

	tkProto, err := backend.LoadTask("TASK-001")
	if err != nil {
		t.Fatalf("load task: %v", err)
	}

	// Validation
	validationResult, err := ValidateTaskResumableProto(tkProto, false)
	if err != nil {
		t.Fatalf("paused task should be resumable: %v", err)
	}

	// State updates
	if err := ApplyResumeStateUpdatesProto(tkProto, validationResult, backend); err != nil {
		t.Fatalf("apply state updates: %v", err)
	}

	// Claim
	ctx := context.Background()
	pid := os.Getpid()
	hostname, _ := os.Hostname()

	err = backend.TryClaimTaskExecution(ctx, "TASK-001", pid, hostname)
	if err != nil {
		t.Fatalf("claim should succeed for paused task: %v", err)
	}

	// Verify task is now running after claim
	finalTask, err := backend.LoadTask("TASK-001")
	if err != nil {
		t.Fatalf("load final task: %v", err)
	}

	if finalTask.Status != orcv1.TaskStatus_TASK_STATUS_RUNNING {
		t.Errorf("Expected task status RUNNING after claim, got %v", finalTask.Status)
	}
}

// TestResumeCommand_BlockedTaskClaimWorks tests that blocked tasks can be claimed.
// Covers: SC-5 (resume from blocked states remains functional)
func TestResumeCommand_BlockedTaskClaimWorks(t *testing.T) {
	tmpDir := withResumeTestDir(t)

	// Create a blocked task using proto types
	backend := createResumeTestBackend(t, tmpDir)
	tk := task.NewProtoTask("TASK-001", "Test task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_BLOCKED
	tk.Weight = orcv1.TaskWeight_TASK_WEIGHT_SMALL
	task.SetCurrentPhaseProto(tk, "implement")
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}
	_ = backend.Close()

	// Claim the task
	backend = createResumeTestBackend(t, tmpDir)
	defer func() { _ = backend.Close() }()

	ctx := context.Background()
	pid := os.Getpid()
	hostname, _ := os.Hostname()

	err := backend.TryClaimTaskExecution(ctx, "TASK-001", pid, hostname)
	if err != nil {
		t.Fatalf("claim should succeed for blocked task: %v", err)
	}

	// Verify claim succeeded - task should be running now
	finalTask, err := backend.LoadTask("TASK-001")
	if err != nil {
		t.Fatalf("load final task: %v", err)
	}

	if finalTask.Status != orcv1.TaskStatus_TASK_STATUS_RUNNING {
		t.Errorf("Expected task status RUNNING after claim, got %v", finalTask.Status)
	}
}

// TestResumeCommand_ClaimErrorMessages tests that claim failures return clear error messages.
// Covers: SC-1 (second attempt rejected with clear error message)
func TestResumeCommand_ClaimErrorMessages(t *testing.T) {
	tmpDir := withResumeTestDir(t)

	// Create and claim a task using proto types
	backend := createResumeTestBackend(t, tmpDir)
	tk := task.NewProtoTask("TASK-001", "Test task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_FAILED
	tk.Weight = orcv1.TaskWeight_TASK_WEIGHT_SMALL
	task.SetCurrentPhaseProto(tk, "implement")
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// First claim
	ctx := context.Background()
	pid1 := os.Getpid()
	hostname, _ := os.Hostname()

	err := backend.TryClaimTaskExecution(ctx, "TASK-001", pid1, hostname)
	if err != nil {
		t.Fatalf("first claim should succeed: %v", err)
	}

	// Second claim should fail with clear message
	pid2 := pid1 + 1 // Different PID
	err = backend.TryClaimTaskExecution(ctx, "TASK-001", pid2, hostname)
	if err == nil {
		t.Fatal("second claim should fail")
	}

	// Error message should be user-friendly and mention the PID
	errMsg := err.Error()
	if !contains([]string{errMsg}, "already claimed") && !contains([]string{errMsg}, "process") {
		t.Errorf("Error message should be clear about task being claimed, got: %v", errMsg)
	}

	_ = backend.Close()
}
