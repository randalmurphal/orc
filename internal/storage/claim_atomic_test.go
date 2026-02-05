package storage

import (
	"context"
	"sync"
	"testing"
	"time"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/task"
)

// ============================================================================
// SC-2: Backend interface concurrent claim tests
// ============================================================================

// TestBackend_AtomicClaimTask_ConcurrentAttempts tests that the Backend.ClaimTask
// method properly serializes concurrent claim attempts.
// Covers: SC-2
func TestBackend_AtomicClaimTask_ConcurrentAttempts(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	// Create a task
	tk := task.NewProtoTask("TASK-001", "Test Task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	tk.Execution = task.InitProtoExecutionState()
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	const numAttempts = 10
	var wg sync.WaitGroup
	results := make(chan bool, numAttempts)

	ctx := context.Background()

	// Launch concurrent claim attempts
	for i := 0; i < numAttempts; i++ {
		wg.Add(1)
		userID := "user-" + string(rune('a'+i))
		go func(userID string) {
			defer wg.Done()
			success, err := backend.ClaimTaskByUser(ctx, "TASK-001", userID)
			if err != nil {
				t.Errorf("claim error for %s: %v", userID, err)
				results <- false
				return
			}
			results <- success
		}(userID)
	}

	wg.Wait()
	close(results)

	// Count successful claims
	var successCount int
	for success := range results {
		if success {
			successCount++
		}
	}

	// Exactly one should succeed
	if successCount != 1 {
		t.Errorf("expected exactly 1 successful claim, got %d", successCount)
	}

	// Verify task has exactly one owner
	updatedTask, err := backend.LoadTask("TASK-001")
	if err != nil {
		t.Fatalf("load task: %v", err)
	}
	if updatedTask.ClaimedBy == "" {
		t.Error("task should have a claimant")
	}
}

// ============================================================================
// SC-3: Backend claim creates history entry
// ============================================================================

// TestBackend_ClaimTaskByUser_CreatesHistory tests that claiming via Backend
// creates a history entry.
// Covers: SC-3
func TestBackend_ClaimTaskByUser_CreatesHistory(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	tk := task.NewProtoTask("TASK-001", "Test Task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	tk.Execution = task.InitProtoExecutionState()
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	ctx := context.Background()
	userID := "user-alice"

	// Claim the task
	success, err := backend.ClaimTaskByUser(ctx, "TASK-001", userID)
	if err != nil {
		t.Fatalf("claim failed: %v", err)
	}
	if !success {
		t.Fatal("expected successful claim")
	}

	// Verify history entry exists
	history, err := backend.GetTaskClaimHistory("TASK-001")
	if err != nil {
		t.Fatalf("get history failed: %v", err)
	}
	if len(history) != 1 {
		t.Errorf("expected 1 history entry, got %d", len(history))
	}
	if history[0].UserID != userID {
		t.Errorf("history user = %q, want %q", history[0].UserID, userID)
	}
}

// ============================================================================
// SC-5, SC-6: Backend force claim
// ============================================================================

// TestBackend_ForceClaimTask tests that ForceClaimTask can steal claims
// and records stolen_from.
// Covers: SC-5, SC-6
func TestBackend_ForceClaimTask(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	tk := task.NewProtoTask("TASK-001", "Test Task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	tk.Execution = task.InitProtoExecutionState()
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	ctx := context.Background()

	// Alice claims
	_, _ = backend.ClaimTaskByUser(ctx, "TASK-001", "user-alice")

	// Bob force-steals
	stolenFrom, err := backend.ForceClaimTask(ctx, "TASK-001", "user-bob")
	if err != nil {
		t.Fatalf("force claim failed: %v", err)
	}
	if stolenFrom != "user-alice" {
		t.Errorf("stolenFrom = %q, want user-alice", stolenFrom)
	}

	// Verify task ownership changed
	updatedTask, _ := backend.LoadTask("TASK-001")
	if updatedTask.ClaimedBy != "user-bob" {
		t.Errorf("task should be claimed by bob, got %q", updatedTask.ClaimedBy)
	}

	// Verify history has stolen_from
	history, _ := backend.GetTaskClaimHistory("TASK-001")
	var bobEntry *TaskClaimHistory
	for i := range history {
		if history[i].UserID == "user-bob" {
			bobEntry = &history[i]
			break
		}
	}
	if bobEntry == nil {
		t.Fatal("bob's history entry not found")
	}
	if bobEntry.StolenFrom == nil || *bobEntry.StolenFrom != "user-alice" {
		t.Errorf("bob's stolen_from should be user-alice")
	}
}

// ============================================================================
// SC-7: Backend release claim
// ============================================================================

// TestBackend_ReleaseClaimByUser tests that releasing a claim clears
// the task and updates history.
// Covers: SC-7
func TestBackend_ReleaseClaimByUser(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	tk := task.NewProtoTask("TASK-001", "Test Task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	tk.Execution = task.InitProtoExecutionState()
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	ctx := context.Background()
	userID := "user-alice"

	// Claim then release
	_, _ = backend.ClaimTaskByUser(ctx, "TASK-001", userID)
	beforeRelease := time.Now().Truncate(time.Second)

	released, err := backend.ReleaseClaimByUser(ctx, "TASK-001", userID)
	if err != nil {
		t.Fatalf("release failed: %v", err)
	}
	if !released {
		t.Error("expected successful release")
	}

	// Verify task is unclaimed
	updatedTask, _ := backend.LoadTask("TASK-001")
	if updatedTask.ClaimedBy != "" {
		t.Errorf("task should be unclaimed, got %q", updatedTask.ClaimedBy)
	}

	// Verify history has released_at
	history, _ := backend.GetTaskClaimHistory("TASK-001")
	if len(history) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(history))
	}
	if history[0].ReleasedAt == nil {
		t.Error("released_at should be set")
	}
	if history[0].ReleasedAt.Before(beforeRelease) {
		t.Errorf("released_at %v should be >= %v", history[0].ReleasedAt, beforeRelease)
	}
}

// ============================================================================
// Types for claim history (expected to be added to Backend interface)
// ============================================================================

// TaskClaimHistory represents a claim history entry.
// This will be defined in the storage package when implemented.
type TaskClaimHistory struct {
	ID         int64
	TaskID     string
	UserID     string
	ClaimedAt  time.Time
	ReleasedAt *time.Time
	StolenFrom *string
}
