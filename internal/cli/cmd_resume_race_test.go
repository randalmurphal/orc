package cli

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// TestResumeCommand_RaceConditionPrevention tests that concurrent resume attempts are prevented.
// This is an integration test that exercises the full resume flow with claim logic.
// Covers: SC-1, SC-2, SC-3
func TestResumeCommand_RaceConditionPrevention(t *testing.T) {
	tmpDir := withResumeTestDir(t)

	// Create a failed task
	backend := createResumeTestBackend(t, tmpDir)
	tk := task.New("TASK-001", "Test task")
	tk.Status = task.StatusFailed
	tk.Weight = task.WeightSmall
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	s := state.New("TASK-001")
	s.Status = state.StatusFailed
	s.CurrentPhase = "implement"
	if err := backend.SaveState(s); err != nil {
		t.Fatalf("failed to save state: %v", err)
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

			// Load task and state
			tk, err := backend.LoadTask("TASK-001")
			if err != nil {
				results <- err
				return
			}

			s, err := backend.LoadState("TASK-001")
			if err != nil {
				results <- err
				return
			}

			// Validate resumable
			validationResult, err := ValidateTaskResumable(tk, s, false)
			if err != nil {
				results <- err
				return
			}

			// Apply state updates if needed
			if err := ApplyResumeStateUpdates(tk, s, validationResult, false, backend); err != nil {
				results <- err
				return
			}

			// Attempt to claim - THIS is where the race protection happens
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

	// Verify only one PID was written to the database
	backend = createResumeTestBackend(t, tmpDir)
	defer func() { _ = backend.Close() }()

	finalState, err := backend.LoadState("TASK-001")
	if err != nil {
		t.Fatalf("load final state: %v", err)
	}

	if finalState.Execution == nil {
		t.Fatal("Execution info should be set by successful claim")
	}

	// PID should be our process PID
	if finalState.Execution.PID != pid {
		t.Errorf("Expected PID %d, got %d", pid, finalState.Execution.PID)
	}
}

// TestResumeCommand_OrphanedTaskWithClaim tests that orphaned tasks can be claimed and resumed.
// Covers: SC-4 (orphan detection still works), SC-6 (dead PID doesn't block)
func TestResumeCommand_OrphanedTaskWithClaim(t *testing.T) {
	tmpDir := withResumeTestDir(t)

	// Create a running task with a dead executor
	backend := createResumeTestBackend(t, tmpDir)
	tk := task.New("TASK-001", "Test task")
	tk.Status = task.StatusRunning
	tk.Weight = task.WeightSmall
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	s := state.New("TASK-001")
	s.Status = state.StatusRunning
	s.CurrentPhase = "implement"
	s.StartExecution(999999, "old-host") // Dead PID
	if err := backend.SaveState(s); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}
	_ = backend.Close()

	// Validation should detect orphan
	backend = createResumeTestBackend(t, tmpDir)
	defer func() { _ = backend.Close() }()

	tk, err := backend.LoadTask("TASK-001")
	if err != nil {
		t.Fatalf("load task: %v", err)
	}

	s, err = backend.LoadState("TASK-001")
	if err != nil {
		t.Fatalf("load state: %v", err)
	}

	validationResult, err := ValidateTaskResumable(tk, s, false)
	if err != nil {
		t.Fatalf("orphaned task should be resumable: %v", err)
	}

	if !validationResult.IsOrphaned {
		t.Error("Expected IsOrphaned=true for dead PID")
	}

	// Apply state updates
	if err := ApplyResumeStateUpdates(tk, s, validationResult, false, backend); err != nil {
		t.Fatalf("apply state updates: %v", err)
	}

	// Claim should succeed (overwrites dead PID)
	ctx := context.Background()
	newPID := os.Getpid()
	hostname, _ := os.Hostname()

	err = backend.TryClaimTaskExecution(ctx, "TASK-001", newPID, hostname)
	if err != nil {
		t.Fatalf("claim should succeed for orphaned task: %v", err)
	}

	// Verify new PID was set
	finalState, err := backend.LoadState("TASK-001")
	if err != nil {
		t.Fatalf("load final state: %v", err)
	}

	if finalState.Execution == nil {
		t.Fatal("Execution info should be set")
	}
	if finalState.Execution.PID != newPID {
		t.Errorf("Expected new PID %d, got %d", newPID, finalState.Execution.PID)
	}
}

// TestResumeCommand_StaleClaimDoesNotBlock tests that a stale claim from a dead process doesn't block new claims.
// Covers: SC-6 (stale PID from dead process doesn't block resume)
func TestResumeCommand_StaleClaimDoesNotBlock(t *testing.T) {
	tmpDir := withResumeTestDir(t)

	// Create a task with a claim from a process that's now dead
	backend := createResumeTestBackend(t, tmpDir)
	tk := task.New("TASK-001", "Test task")
	tk.Status = task.StatusRunning
	tk.Weight = task.WeightSmall
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	s := state.New("TASK-001")
	s.Status = state.StatusRunning
	s.CurrentPhase = "implement"
	// Simulate a claim from a dead process (very high PID unlikely to exist)
	deadPID := 888888
	s.StartExecution(deadPID, "stale-host")
	if err := backend.SaveState(s); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}
	_ = backend.Close()

	// Wait a moment to ensure PID is definitely stale
	time.Sleep(100 * time.Millisecond)

	// New claim should succeed
	backend = createResumeTestBackend(t, tmpDir)
	defer func() { _ = backend.Close() }()

	ctx := context.Background()
	newPID := os.Getpid()
	hostname, _ := os.Hostname()

	err := backend.TryClaimTaskExecution(ctx, "TASK-001", newPID, hostname)
	if err != nil {
		t.Fatalf("claim should succeed despite stale PID: %v", err)
	}

	// Verify old PID was replaced
	finalState, err := backend.LoadState("TASK-001")
	if err != nil {
		t.Fatalf("load final state: %v", err)
	}

	if finalState.Execution == nil {
		t.Fatal("Execution info should be set")
	}
	if finalState.Execution.PID == deadPID {
		t.Error("Old PID should have been replaced")
	}
	if finalState.Execution.PID != newPID {
		t.Errorf("Expected new PID %d, got %d", newPID, finalState.Execution.PID)
	}
}

// TestResumeCommand_PausedTaskClaimWorks tests that paused tasks can be claimed.
// Covers: SC-5 (resume from paused states remains functional)
func TestResumeCommand_PausedTaskClaimWorks(t *testing.T) {
	tmpDir := withResumeTestDir(t)

	// Create a paused task
	backend := createResumeTestBackend(t, tmpDir)
	tk := task.New("TASK-001", "Test task")
	tk.Status = task.StatusPaused
	tk.Weight = task.WeightSmall
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	s := state.New("TASK-001")
	s.Status = state.StatusPaused
	s.CurrentPhase = "implement"
	if err := backend.SaveState(s); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}
	_ = backend.Close()

	// Resume flow: validate, claim, execute
	backend = createResumeTestBackend(t, tmpDir)
	defer func() { _ = backend.Close() }()

	tk, err := backend.LoadTask("TASK-001")
	if err != nil {
		t.Fatalf("load task: %v", err)
	}

	s, err = backend.LoadState("TASK-001")
	if err != nil {
		t.Fatalf("load state: %v", err)
	}

	// Validation
	validationResult, err := ValidateTaskResumable(tk, s, false)
	if err != nil {
		t.Fatalf("paused task should be resumable: %v", err)
	}

	// State updates
	if err := ApplyResumeStateUpdates(tk, s, validationResult, false, backend); err != nil {
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

	// Verify task is now claimed and running
	finalState, err := backend.LoadState("TASK-001")
	if err != nil {
		t.Fatalf("load final state: %v", err)
	}

	if finalState.Status != state.StatusRunning {
		t.Errorf("Expected status running after claim, got %s", finalState.Status)
	}
	if finalState.Execution == nil || finalState.Execution.PID != pid {
		t.Error("Execution info should be set with correct PID")
	}
}

// TestResumeCommand_BlockedTaskClaimWorks tests that blocked tasks can be claimed.
// Covers: SC-5 (resume from blocked states remains functional)
func TestResumeCommand_BlockedTaskClaimWorks(t *testing.T) {
	tmpDir := withResumeTestDir(t)

	// Create a blocked task
	backend := createResumeTestBackend(t, tmpDir)
	tk := task.New("TASK-001", "Test task")
	tk.Status = task.StatusBlocked
	tk.Weight = task.WeightSmall
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	s := state.New("TASK-001")
	s.Status = state.StatusPaused // State uses paused, task uses blocked
	s.CurrentPhase = "implement"
	if err := backend.SaveState(s); err != nil {
		t.Fatalf("failed to save state: %v", err)
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

	// Verify claim succeeded
	finalState, err := backend.LoadState("TASK-001")
	if err != nil {
		t.Fatalf("load final state: %v", err)
	}

	if finalState.Execution == nil || finalState.Execution.PID != pid {
		t.Error("Execution info should be set for blocked task")
	}
}

// TestResumeCommand_ClaimErrorMessages tests that claim failures return clear error messages.
// Covers: SC-1 (second attempt rejected with clear error message)
func TestResumeCommand_ClaimErrorMessages(t *testing.T) {
	tmpDir := withResumeTestDir(t)

	// Create and claim a task
	backend := createResumeTestBackend(t, tmpDir)
	tk := task.New("TASK-001", "Test task")
	tk.Status = task.StatusFailed
	tk.Weight = task.WeightSmall
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	s := state.New("TASK-001")
	s.Status = state.StatusFailed
	s.CurrentPhase = "implement"
	if err := backend.SaveState(s); err != nil {
		t.Fatalf("failed to save state: %v", err)
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
