package cli

import (
	"testing"
	"time"

	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/storage"
)

// TestReleaseCommand_Structure verifies command structure matches spec.
func TestReleaseCommand_Structure(t *testing.T) {
	t.Parallel()
	cmd := newReleaseCmd()

	if cmd.Use != "release <task-id>" {
		t.Errorf("command Use = %q, want %q", cmd.Use, "release <task-id>")
	}
	if cmd.Short == "" {
		t.Error("missing Short description")
	}
	if cmd.Long == "" {
		t.Error("missing Long help text")
	}
}

// TestReleaseCommand_RequiresArg verifies the command requires exactly one argument.
func TestReleaseCommand_RequiresArg(t *testing.T) {
	t.Parallel()
	cmd := newReleaseCmd()

	if err := cmd.Args(cmd, []string{}); err == nil {
		t.Error("expected error for zero args")
	}
	if err := cmd.Args(cmd, []string{"TASK-001"}); err != nil {
		t.Errorf("unexpected error for one arg: %v", err)
	}
	if err := cmd.Args(cmd, []string{"TASK-001", "TASK-002"}); err == nil {
		t.Error("expected error for two args")
	}
}

// TestReleaseCommand_ReleasesOwnClaim verifies that releasing your own claim succeeds.
// Tests the release logic via backend APIs. The CLI command execution is tested
// via E2E tests which have proper GlobalDB setup.
// Covers: SC-1
func TestReleaseCommand_ReleasesOwnClaim(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	globalDB := storage.NewTestGlobalDB(t)

	// Create a user for the current system user
	currentUserID, err := globalDB.GetOrCreateUser("testuser")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	dbTask := &db.Task{
		ID:        "TASK-001",
		Title:     "Test Task",
		Status:    "created",
		CreatedAt: time.Now(),
	}
	if err := backend.DB().SaveTask(dbTask); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Claim the task
	rowsAffected, err := backend.DB().ClaimTaskByUser("TASK-001", currentUserID)
	if err != nil {
		t.Fatalf("claim task: %v", err)
	}
	if rowsAffected != 1 {
		t.Fatalf("expected 1 row affected on claim, got %d", rowsAffected)
	}

	// Verify task is claimed
	task, err := backend.DB().GetTask("TASK-001")
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if task.ClaimedBy != currentUserID {
		t.Fatalf("task should be claimed by %s, got %s", currentUserID, task.ClaimedBy)
	}

	// Release the claim (this is what the release command does internally)
	released, err := backend.ReleaseUserClaim("TASK-001", currentUserID)
	if err != nil {
		t.Fatalf("release claim: %v", err)
	}
	if !released {
		t.Fatal("expected release to succeed")
	}

	// Verify claim is released
	task, err = backend.DB().GetTask("TASK-001")
	if err != nil {
		t.Fatalf("get task: %v", err)
	}

	if task.ClaimedBy != "" {
		t.Errorf("claimed_by should be empty, got %q", task.ClaimedBy)
	}
	if task.ClaimedAt != nil {
		t.Errorf("claimed_at should be nil, got %v", task.ClaimedAt)
	}

	// Verify history has released_at set
	history, err := backend.DB().GetUserClaimHistory("TASK-001")
	if err != nil {
		t.Fatalf("get claim history: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(history))
	}
	if history[0].ReleasedAt == nil {
		t.Error("history.released_at should be set")
	}
}

// TestReleaseCommand_ErrorNotClaimed verifies error when task has no claim.
// Tests the release logic via backend APIs.
// Covers: SC-2
func TestReleaseCommand_ErrorNotClaimed(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	globalDB := storage.NewTestGlobalDB(t)

	currentUserID, err := globalDB.GetOrCreateUser("testuser")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	dbTask := &db.Task{
		ID:        "TASK-002",
		Title:     "Unclaimed Task",
		Status:    "created",
		CreatedAt: time.Now(),
	}
	if err := backend.DB().SaveTask(dbTask); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Check claim status
	task, err := backend.DB().GetTask("TASK-002")
	if err != nil {
		t.Fatalf("get task: %v", err)
	}

	// Task is not claimed - should fail
	if task.ClaimedBy != "" {
		t.Fatal("task should not be claimed")
	}

	// ReleaseUserClaim returns false if not owned (which includes unclaimed)
	released, err := backend.ReleaseUserClaim("TASK-002", currentUserID)
	if err != nil {
		t.Fatalf("release claim error: %v", err)
	}
	if released {
		t.Error("expected release to fail for unclaimed task")
	}
}

// TestReleaseCommand_ErrorClaimedByAnother verifies error when claimed by different user.
// Tests the release logic via backend APIs.
// Covers: SC-3
func TestReleaseCommand_ErrorClaimedByAnother(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	globalDB := storage.NewTestGlobalDB(t)

	aliceID, err := globalDB.GetOrCreateUser("alice")
	if err != nil {
		t.Fatalf("create alice: %v", err)
	}

	bobID, err := globalDB.GetOrCreateUser("bob")
	if err != nil {
		t.Fatalf("create bob: %v", err)
	}

	dbTask := &db.Task{
		ID:        "TASK-003",
		Title:     "Alice's Task",
		Status:    "created",
		CreatedAt: time.Now(),
	}
	if err := backend.DB().SaveTask(dbTask); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Alice claims the task
	rowsAffected, err := backend.DB().ClaimTaskByUser("TASK-003", aliceID)
	if err != nil {
		t.Fatalf("alice claim: %v", err)
	}
	if rowsAffected != 1 {
		t.Fatalf("alice claim should succeed")
	}

	// Check claim status - verify alice owns it
	task, err := backend.DB().GetTask("TASK-003")
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if task.ClaimedBy != aliceID {
		t.Fatalf("task should be claimed by alice (%s), got %s", aliceID, task.ClaimedBy)
	}

	// Get alice's name for error message verification
	alice, err := globalDB.GetUser(task.ClaimedBy)
	if err != nil {
		t.Fatalf("get alice: %v", err)
	}
	if alice == nil || alice.Name != "alice" {
		t.Fatal("alice user should exist")
	}

	// Bob tries to release - should fail
	released, err := backend.ReleaseUserClaim("TASK-003", bobID)
	if err != nil {
		t.Fatalf("release claim error: %v", err)
	}
	if released {
		t.Error("expected release to fail when claimed by another user")
	}

	// Verify task is still claimed by alice
	task, err = backend.DB().GetTask("TASK-003")
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if task.ClaimedBy != aliceID {
		t.Errorf("task should still be claimed by alice, got %s", task.ClaimedBy)
	}
}

// TestReleaseCommand_TaskNotFound verifies error when task doesn't exist.
func TestReleaseCommand_TaskNotFound(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	globalDB := storage.NewTestGlobalDB(t)

	userID, err := globalDB.GetOrCreateUser("testuser")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	// Try to release claim on nonexistent task
	// ReleaseUserClaim returns false if task doesn't exist (0 rows affected)
	released, err := backend.ReleaseUserClaim("TASK-999", userID)
	if err != nil {
		t.Fatalf("release claim error: %v", err)
	}
	if released {
		t.Error("expected release to fail for nonexistent task")
	}
}
