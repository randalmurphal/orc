package storage

import (
	"sync"
	"testing"
	"time"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/task"
)

// ============================================================================
// SC-2: Backend interface concurrent claim tests
// ============================================================================

// TestBackend_ClaimTaskByUser_ConcurrentAttempts tests that the Backend
// ClaimTaskByUser method properly serializes concurrent claim attempts.
// Covers: SC-2
func TestBackend_ClaimTaskByUser_ConcurrentAttempts(t *testing.T) {
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

	// Launch concurrent claim attempts
	for i := 0; i < numAttempts; i++ {
		wg.Add(1)
		userID := "user-" + string(rune('a'+i))
		go func(userID string) {
			defer wg.Done()
			success, err := backend.ClaimTaskByUser("TASK-001", userID)
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

	// Verify task has exactly one owner via the DB
	pdb := backend.DB()
	dbTask, err := pdb.GetTask("TASK-001")
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if dbTask.ClaimedBy == "" {
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

	userID := "user-alice"

	// Claim the task via backend
	success, err := backend.ClaimTaskByUser("TASK-001", userID)
	if err != nil {
		t.Fatalf("claim failed: %v", err)
	}
	if !success {
		t.Fatal("expected successful claim")
	}

	// Verify history entry exists via ProjectDB
	pdb := backend.DB()
	history, err := pdb.GetUserClaimHistory("TASK-001")
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

// TestBackend_ForceClaimTaskByUser tests that ForceClaimTaskByUser can steal claims
// and records stolen_from.
// Covers: SC-5, SC-6
func TestBackend_ForceClaimTaskByUser(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	tk := task.NewProtoTask("TASK-001", "Test Task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	tk.Execution = task.InitProtoExecutionState()
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Alice claims
	_, _ = backend.ClaimTaskByUser("TASK-001", "user-alice")

	// Bob force-steals
	stolenFrom, err := backend.ForceClaimTaskByUser("TASK-001", "user-bob")
	if err != nil {
		t.Fatalf("force claim failed: %v", err)
	}
	if stolenFrom != "user-alice" {
		t.Errorf("stolenFrom = %q, want user-alice", stolenFrom)
	}

	// Verify task ownership changed via DB
	pdb := backend.DB()
	dbTask, err := pdb.GetTask("TASK-001")
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if dbTask.ClaimedBy != "user-bob" {
		t.Errorf("task should be claimed by bob, got %q", dbTask.ClaimedBy)
	}

	// Verify history has stolen_from
	history, _ := pdb.GetUserClaimHistory("TASK-001")
	var bobEntry *UserClaimHistory
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

// TestBackend_ReleaseUserClaim tests that releasing a claim clears
// the task and updates history.
// Covers: SC-7
func TestBackend_ReleaseUserClaim(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTestDB(t)
	defer teardownTestDB(t, backend, tmpDir)

	tk := task.NewProtoTask("TASK-001", "Test Task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	tk.Execution = task.InitProtoExecutionState()
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	userID := "user-alice"

	// Claim then release
	_, _ = backend.ClaimTaskByUser("TASK-001", userID)
	beforeRelease := time.Now().Truncate(time.Second)

	released, err := backend.ReleaseUserClaim("TASK-001", userID)
	if err != nil {
		t.Fatalf("release failed: %v", err)
	}
	if !released {
		t.Error("expected successful release")
	}

	// Verify task is unclaimed via DB
	pdb := backend.DB()
	dbTask, err := pdb.GetTask("TASK-001")
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if dbTask.ClaimedBy != "" {
		t.Errorf("task should be unclaimed, got %q", dbTask.ClaimedBy)
	}

	// Verify history has released_at
	history, _ := pdb.GetUserClaimHistory("TASK-001")
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

// UserClaimHistory is a local alias for db.UserClaimHistoryEntry
// used in these tests for convenience.
type UserClaimHistory = db.UserClaimHistoryEntry
