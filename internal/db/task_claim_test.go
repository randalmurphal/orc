package db

import (
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// ============================================================================
// SC-1: Atomic claim sets claimed_by and claimed_at on tasks table
// ============================================================================

// TestClaimTaskByUser_AtomicUpdate tests that ClaimTaskByUser sets claimed_by
// and claimed_at in a single atomic UPDATE operation on the tasks table.
// Covers: SC-1
func TestClaimTaskByUser_AtomicUpdate(t *testing.T) {
	t.Parallel()
	pdb := setupTestProjectDB(t)

	// Create a task
	task := &Task{
		ID:        "TASK-001",
		Title:     "Test Task",
		Status:    "created",
		CreatedAt: time.Now(),
	}
	if err := pdb.SaveTask(task); err != nil {
		t.Fatalf("save task: %v", err)
	}

	userID := "user-alice"
	beforeClaim := time.Now().Truncate(time.Second)

	// Claim the task - new atomic method on tasks table
	rowsAffected, err := pdb.ClaimTaskByUser("TASK-001", userID)
	if err != nil {
		t.Fatalf("ClaimTaskByUser failed: %v", err)
	}
	if rowsAffected != 1 {
		t.Errorf("expected 1 row affected, got %d", rowsAffected)
	}

	// Verify claimed_by and claimed_at are set on the task
	updatedTask, err := pdb.GetTask("TASK-001")
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if updatedTask.ClaimedBy != userID {
		t.Errorf("claimed_by = %q, want %q", updatedTask.ClaimedBy, userID)
	}
	if updatedTask.ClaimedAt == nil {
		t.Fatal("claimed_at should be set")
	}
	if updatedTask.ClaimedAt.Before(beforeClaim) {
		t.Errorf("claimed_at %v should be >= %v", updatedTask.ClaimedAt, beforeClaim)
	}
}

// TestClaimTaskByUser_AlreadyClaimed tests that claiming an already-claimed task
// returns 0 rows affected when claimed by another user.
// Covers: SC-1 error path
func TestClaimTaskByUser_AlreadyClaimed(t *testing.T) {
	t.Parallel()
	pdb := setupTestProjectDB(t)

	// Create and claim a task as alice
	task := &Task{ID: "TASK-001", Title: "Test", Status: "created", CreatedAt: time.Now()}
	if err := pdb.SaveTask(task); err != nil {
		t.Fatalf("save task: %v", err)
	}

	_, err := pdb.ClaimTaskByUser("TASK-001", "user-alice")
	if err != nil {
		t.Fatalf("first claim failed: %v", err)
	}

	// Bob tries to claim - should fail (0 rows affected)
	rowsAffected, err := pdb.ClaimTaskByUser("TASK-001", "user-bob")
	if err != nil {
		t.Fatalf("second claim should not error (just return 0 rows): %v", err)
	}
	if rowsAffected != 0 {
		t.Errorf("expected 0 rows affected when already claimed, got %d", rowsAffected)
	}
}

// TestClaimTaskByUser_Idempotent tests that claiming a task you already own is a no-op.
// Covers: SC-1 edge case (idempotent)
func TestClaimTaskByUser_Idempotent(t *testing.T) {
	t.Parallel()
	pdb := setupTestProjectDB(t)

	task := &Task{ID: "TASK-001", Title: "Test", Status: "created", CreatedAt: time.Now()}
	if err := pdb.SaveTask(task); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Claim as alice
	_, err := pdb.ClaimTaskByUser("TASK-001", "user-alice")
	if err != nil {
		t.Fatalf("first claim failed: %v", err)
	}

	// Claim again as alice - should succeed (idempotent)
	rowsAffected, err := pdb.ClaimTaskByUser("TASK-001", "user-alice")
	if err != nil {
		t.Fatalf("idempotent claim failed: %v", err)
	}
	if rowsAffected != 1 {
		t.Errorf("expected 1 row affected for idempotent claim, got %d", rowsAffected)
	}
}

// TestClaimTaskByUser_NonexistentTask tests claiming a task that doesn't exist.
// Covers: Failure mode - task not found
func TestClaimTaskByUser_NonexistentTask(t *testing.T) {
	t.Parallel()
	pdb := setupTestProjectDB(t)

	rowsAffected, err := pdb.ClaimTaskByUser("TASK-999", "user-alice")
	if err != nil {
		t.Fatalf("claim should not error for nonexistent task: %v", err)
	}
	if rowsAffected != 0 {
		t.Errorf("expected 0 rows affected for nonexistent task, got %d", rowsAffected)
	}
}

// ============================================================================
// SC-2: Concurrent claim attempts are serialized atomically
// ============================================================================

// TestConcurrentClaim_OnlyOneSucceeds tests that exactly one of many concurrent
// claim attempts succeeds, with no race conditions.
// Covers: SC-2
func TestConcurrentClaim_OnlyOneSucceeds(t *testing.T) {
	t.Parallel()
	pdb := setupTestProjectDB(t)

	task := &Task{ID: "TASK-001", Title: "Test", Status: "created", CreatedAt: time.Now()}
	if err := pdb.SaveTask(task); err != nil {
		t.Fatalf("save task: %v", err)
	}

	const numAttempts = 10
	var wg sync.WaitGroup
	results := make(chan int64, numAttempts)

	// Launch concurrent claim attempts from different "users"
	for i := 0; i < numAttempts; i++ {
		wg.Add(1)
		userID := "user-" + string(rune('a'+i))
		go func(userID string) {
			defer wg.Done()
			rowsAffected, err := pdb.ClaimTaskByUser("TASK-001", userID)
			if err != nil {
				t.Errorf("claim error: %v", err)
				results <- -1
				return
			}
			results <- rowsAffected
		}(userID)
	}

	wg.Wait()
	close(results)

	// Count successful claims
	var successCount int
	for rowsAffected := range results {
		if rowsAffected == 1 {
			successCount++
		}
	}

	// Exactly one should succeed
	if successCount != 1 {
		t.Errorf("expected exactly 1 successful claim, got %d", successCount)
	}
}

// ============================================================================
// SC-3: Successful claim inserts a row into task_claim_history table
// ============================================================================

// TestClaimHistory_InsertedOnClaim tests that claiming a task inserts
// a row into the task_claim_history table.
// Covers: SC-3
func TestClaimHistory_InsertedOnClaim(t *testing.T) {
	t.Parallel()
	pdb := setupTestProjectDB(t)

	task := &Task{ID: "TASK-001", Title: "Test", Status: "created", CreatedAt: time.Now()}
	if err := pdb.SaveTask(task); err != nil {
		t.Fatalf("save task: %v", err)
	}

	userID := "user-alice"
	beforeClaim := time.Now().Truncate(time.Second)

	// Claim the task
	_, err := pdb.ClaimTaskByUser("TASK-001", userID)
	if err != nil {
		t.Fatalf("ClaimTaskByUser failed: %v", err)
	}

	// Verify history entry was created
	history, err := pdb.GetUserClaimHistory("TASK-001")
	if err != nil {
		t.Fatalf("get claim history: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(history))
	}

	entry := history[0]
	if entry.TaskID != "TASK-001" {
		t.Errorf("history.TaskID = %q, want TASK-001", entry.TaskID)
	}
	if entry.UserID != userID {
		t.Errorf("history.UserID = %q, want %q", entry.UserID, userID)
	}
	if entry.ClaimedAt.Before(beforeClaim) {
		t.Errorf("history.ClaimedAt %v should be >= %v", entry.ClaimedAt, beforeClaim)
	}
	if entry.ReleasedAt != nil {
		t.Errorf("history.ReleasedAt should be nil for active claim")
	}
	if entry.StolenFrom != nil {
		t.Errorf("history.StolenFrom should be nil for normal claim")
	}
}

// ============================================================================
// SC-4: History table is append-only
// ============================================================================

// TestClaimHistoryAppendOnly tests that history rows are never deleted,
// only released_at is updated.
// Covers: SC-4
func TestClaimHistoryAppendOnly(t *testing.T) {
	t.Parallel()
	pdb := setupTestProjectDB(t)

	task := &Task{ID: "TASK-001", Title: "Test", Status: "created", CreatedAt: time.Now()}
	if err := pdb.SaveTask(task); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Claim, release, claim again, release again
	// First claim by alice
	_, _ = pdb.ClaimTaskByUser("TASK-001", "user-alice")
	_, _ = pdb.ReleaseUserClaim("TASK-001", "user-alice")

	// Second claim by bob
	_, _ = pdb.ClaimTaskByUser("TASK-001", "user-bob")
	_, _ = pdb.ReleaseUserClaim("TASK-001", "user-bob")

	// Check history - should have 2 entries
	history, err := pdb.GetUserClaimHistory("TASK-001")
	if err != nil {
		t.Fatalf("get claim history: %v", err)
	}

	if len(history) != 2 {
		t.Fatalf("expected 2 history entries (append-only), got %d", len(history))
	}

	// Both should have released_at set
	for i, entry := range history {
		if entry.ReleasedAt == nil {
			t.Errorf("entry[%d].ReleasedAt should be set", i)
		}
	}
}

// ============================================================================
// SC-5: Force steal updates claimed_by even when claimed by another user
// ============================================================================

// TestForceStealClaim tests that ForceClaimTaskByUser can steal a claim from another user.
// Covers: SC-5
func TestForceStealClaim(t *testing.T) {
	t.Parallel()
	pdb := setupTestProjectDB(t)

	task := &Task{ID: "TASK-001", Title: "Test", Status: "created", CreatedAt: time.Now()}
	if err := pdb.SaveTask(task); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Alice claims first
	_, err := pdb.ClaimTaskByUser("TASK-001", "user-alice")
	if err != nil {
		t.Fatalf("alice claim failed: %v", err)
	}

	// Verify alice owns it
	taskBefore, _ := pdb.GetTask("TASK-001")
	if taskBefore.ClaimedBy != "user-alice" {
		t.Fatalf("task should be claimed by alice, got %q", taskBefore.ClaimedBy)
	}

	// Bob force-steals
	stolenFrom, err := pdb.ForceClaimTaskByUser("TASK-001", "user-bob")
	if err != nil {
		t.Fatalf("force claim failed: %v", err)
	}
	if stolenFrom != "user-alice" {
		t.Errorf("stolenFrom = %q, want user-alice", stolenFrom)
	}

	// Verify bob now owns it
	taskAfter, _ := pdb.GetTask("TASK-001")
	if taskAfter.ClaimedBy != "user-bob" {
		t.Errorf("task should be claimed by bob, got %q", taskAfter.ClaimedBy)
	}
}

// TestForceStealClaim_Unclaimed tests that force steal on unclaimed task works normally.
// Covers: SC-5 edge case
func TestForceStealClaim_Unclaimed(t *testing.T) {
	t.Parallel()
	pdb := setupTestProjectDB(t)

	task := &Task{ID: "TASK-001", Title: "Test", Status: "created", CreatedAt: time.Now()}
	if err := pdb.SaveTask(task); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Force claim on unclaimed task - should work like normal claim
	stolenFrom, err := pdb.ForceClaimTaskByUser("TASK-001", "user-bob")
	if err != nil {
		t.Fatalf("force claim on unclaimed failed: %v", err)
	}
	if stolenFrom != "" {
		t.Errorf("stolenFrom should be empty for unclaimed task, got %q", stolenFrom)
	}

	// Verify bob owns it
	taskAfter, _ := pdb.GetTask("TASK-001")
	if taskAfter.ClaimedBy != "user-bob" {
		t.Errorf("task should be claimed by bob, got %q", taskAfter.ClaimedBy)
	}
}

// TestForceStealClaim_OwnClaim tests that force steal on your own claim is a no-op.
// Covers: SC-5 edge case
func TestForceStealClaim_OwnClaim(t *testing.T) {
	t.Parallel()
	pdb := setupTestProjectDB(t)

	task := &Task{ID: "TASK-001", Title: "Test", Status: "created", CreatedAt: time.Now()}
	if err := pdb.SaveTask(task); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Alice claims
	_, _ = pdb.ClaimTaskByUser("TASK-001", "user-alice")

	// Alice force-steals her own claim - should be no-op
	stolenFrom, err := pdb.ForceClaimTaskByUser("TASK-001", "user-alice")
	if err != nil {
		t.Fatalf("force claim own task failed: %v", err)
	}
	// stolenFrom should be empty since it's not actually stolen
	if stolenFrom != "" {
		t.Errorf("stolenFrom should be empty when claiming own task, got %q", stolenFrom)
	}
}

// ============================================================================
// SC-6: Force steal records stolen_from in history
// ============================================================================

// TestForceStealHistory tests that force steal records stolen_from in history.
// Covers: SC-6
func TestForceStealHistory(t *testing.T) {
	t.Parallel()
	pdb := setupTestProjectDB(t)

	task := &Task{ID: "TASK-001", Title: "Test", Status: "created", CreatedAt: time.Now()}
	if err := pdb.SaveTask(task); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Alice claims first
	_, _ = pdb.ClaimTaskByUser("TASK-001", "user-alice")

	// Bob force-steals
	_, _ = pdb.ForceClaimTaskByUser("TASK-001", "user-bob")

	// Check history
	history, err := pdb.GetUserClaimHistory("TASK-001")
	if err != nil {
		t.Fatalf("get claim history: %v", err)
	}

	// Should have 2 entries: alice's original claim and bob's steal
	if len(history) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(history))
	}

	// Find bob's entry (the steal)
	var bobEntry *UserClaimHistoryEntry
	for i := range history {
		if history[i].UserID == "user-bob" {
			bobEntry = &history[i]
			break
		}
	}
	if bobEntry == nil {
		t.Fatal("bob's claim entry not found in history")
	}

	// Bob's entry should have stolen_from set
	if bobEntry.StolenFrom == nil || *bobEntry.StolenFrom != "user-alice" {
		got := "nil"
		if bobEntry.StolenFrom != nil {
			got = *bobEntry.StolenFrom
		}
		t.Errorf("bob's stolen_from = %q, want user-alice", got)
	}
}

// ============================================================================
// SC-7: Release claim clears claimed_by and sets released_at in history
// ============================================================================

// TestReleaseUserClaim tests that releasing a claim clears claimed_by on the task
// and sets released_at in history.
// Covers: SC-7
func TestReleaseUserClaim(t *testing.T) {
	t.Parallel()
	pdb := setupTestProjectDB(t)

	task := &Task{ID: "TASK-001", Title: "Test", Status: "created", CreatedAt: time.Now()}
	if err := pdb.SaveTask(task); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Claim and then release
	_, _ = pdb.ClaimTaskByUser("TASK-001", "user-alice")
	beforeRelease := time.Now().Truncate(time.Second)

	rowsAffected, err := pdb.ReleaseUserClaim("TASK-001", "user-alice")
	if err != nil {
		t.Fatalf("release claim failed: %v", err)
	}
	if rowsAffected != 1 {
		t.Errorf("expected 1 row affected, got %d", rowsAffected)
	}

	// Verify task.claimed_by is cleared
	updatedTask, _ := pdb.GetTask("TASK-001")
	if updatedTask.ClaimedBy != "" {
		t.Errorf("claimed_by should be cleared, got %q", updatedTask.ClaimedBy)
	}
	if updatedTask.ClaimedAt != nil {
		t.Errorf("claimed_at should be cleared")
	}

	// Verify history.released_at is set
	history, _ := pdb.GetUserClaimHistory("TASK-001")
	if len(history) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(history))
	}
	if history[0].ReleasedAt == nil {
		t.Fatal("history.released_at should be set")
	}
	if history[0].ReleasedAt.Before(beforeRelease) {
		t.Errorf("released_at %v should be >= %v", history[0].ReleasedAt, beforeRelease)
	}
}

// TestReleaseUserClaim_NotOwner tests that releasing someone else's claim fails.
// Covers: SC-7 error path
func TestReleaseUserClaim_NotOwner(t *testing.T) {
	t.Parallel()
	pdb := setupTestProjectDB(t)

	task := &Task{ID: "TASK-001", Title: "Test", Status: "created", CreatedAt: time.Now()}
	if err := pdb.SaveTask(task); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Alice claims
	_, _ = pdb.ClaimTaskByUser("TASK-001", "user-alice")

	// Bob tries to release - should fail
	rowsAffected, err := pdb.ReleaseUserClaim("TASK-001", "user-bob")
	if err != nil {
		t.Fatalf("release should not error (just return 0 rows): %v", err)
	}
	if rowsAffected != 0 {
		t.Errorf("expected 0 rows affected when not owner, got %d", rowsAffected)
	}

	// Alice should still own it
	task2, _ := pdb.GetTask("TASK-001")
	if task2.ClaimedBy != "user-alice" {
		t.Errorf("alice should still own task, got %q", task2.ClaimedBy)
	}
}

// TestReleaseUserClaim_Idempotent tests that releasing an already-released claim is a no-op.
// Covers: SC-7 edge case
func TestReleaseUserClaim_Idempotent(t *testing.T) {
	t.Parallel()
	pdb := setupTestProjectDB(t)

	task := &Task{ID: "TASK-001", Title: "Test", Status: "created", CreatedAt: time.Now()}
	if err := pdb.SaveTask(task); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Claim and release
	_, _ = pdb.ClaimTaskByUser("TASK-001", "user-alice")
	_, _ = pdb.ReleaseUserClaim("TASK-001", "user-alice")

	// Release again - should be no-op
	rowsAffected, err := pdb.ReleaseUserClaim("TASK-001", "user-alice")
	if err != nil {
		t.Fatalf("idempotent release should not error: %v", err)
	}
	if rowsAffected != 0 {
		t.Errorf("expected 0 rows affected for already-released task, got %d", rowsAffected)
	}
}

// ============================================================================
// Additional edge cases
// ============================================================================

// TestClaimHistoryEmpty tests that querying history for a task with no claims
// returns an empty slice, not an error.
func TestClaimHistoryEmpty(t *testing.T) {
	t.Parallel()
	pdb := setupTestProjectDB(t)

	task := &Task{ID: "TASK-001", Title: "Test", Status: "created", CreatedAt: time.Now()}
	if err := pdb.SaveTask(task); err != nil {
		t.Fatalf("save task: %v", err)
	}

	history, err := pdb.GetUserClaimHistory("TASK-001")
	if err != nil {
		t.Fatalf("get empty history should not error: %v", err)
	}
	if history == nil {
		t.Fatal("history should be empty slice, not nil")
	}
	if len(history) != 0 {
		t.Errorf("expected 0 history entries, got %d", len(history))
	}
}

// ============================================================================
// Helper functions
// ============================================================================

// setupTestProjectDB creates a temporary ProjectDB for testing.
func setupTestProjectDB(t *testing.T) *ProjectDB {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	if err := db.Migrate("project"); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	t.Cleanup(func() { _ = db.Close() })

	return &ProjectDB{DB: db}
}
