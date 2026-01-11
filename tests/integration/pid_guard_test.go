package integration

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/randalmurphal/orc/internal/lock"
	"github.com/randalmurphal/orc/tests/testutil"
)

// TestSameUserProtection verifies that the same user cannot run the same
// task twice (PID guard blocks).
func TestSameUserProtection(t *testing.T) {
	repo := testutil.SetupTestRepo(t)

	worktreePath := repo.CreateWorktree("TASK-001", "")
	guard := lock.NewPIDGuard(worktreePath)

	// First check should succeed (no PID file)
	if err := guard.Check(); err != nil {
		t.Fatalf("initial check failed: %v", err)
	}

	// Acquire PID
	if err := guard.Acquire(); err != nil {
		t.Fatalf("acquire failed: %v", err)
	}

	// Verify PID file was created
	pidFile := filepath.Join(worktreePath, lock.PIDFileName)
	testutil.AssertFileExists(t, pidFile)

	// Read PID file to verify content
	data, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatalf("read PID file: %v", err)
	}
	pid, err := strconv.Atoi(string(data))
	if err != nil {
		t.Fatalf("parse PID: %v", err)
	}
	if pid != os.Getpid() {
		t.Errorf("PID = %d, want %d", pid, os.Getpid())
	}

	// Second check from same process should detect already running
	// Note: Since we're in the same process, the PID check will find our own PID
	// and consider it "running" which is correct behavior
	err = guard.Check()
	var alreadyRunning *lock.AlreadyRunningError
	if !errors.As(err, &alreadyRunning) {
		t.Errorf("expected AlreadyRunningError, got %v", err)
	} else if alreadyRunning.PID != os.Getpid() {
		t.Errorf("AlreadyRunningError.PID = %d, want %d", alreadyRunning.PID, os.Getpid())
	}

	// Release PID
	guard.Release()

	// Verify PID file was removed
	testutil.AssertFileNotExists(t, pidFile)

	// Check should succeed again after release
	if err := guard.Check(); err != nil {
		t.Errorf("check after release failed: %v", err)
	}
}

// TestPIDGuardAcquireReleaseCycle tests the full acquire/release cycle.
func TestPIDGuardAcquireReleaseCycle(t *testing.T) {
	repo := testutil.SetupTestRepo(t)

	worktreePath := repo.CreateWorktree("TASK-002", "am")
	guard := lock.NewPIDGuard(worktreePath)

	pidFile := filepath.Join(worktreePath, lock.PIDFileName)

	// Multiple acquire/release cycles
	for i := 0; i < 3; i++ {
		// Check should succeed
		if err := guard.Check(); err != nil {
			t.Fatalf("cycle %d: check failed: %v", i, err)
		}

		// Acquire
		if err := guard.Acquire(); err != nil {
			t.Fatalf("cycle %d: acquire failed: %v", i, err)
		}
		testutil.AssertFileExists(t, pidFile)

		// Release
		guard.Release()
		testutil.AssertFileNotExists(t, pidFile)
	}
}

// TestPIDGuardCreatesWorktreeDir verifies that Acquire creates the worktree
// directory if it doesn't exist.
func TestPIDGuardCreatesWorktreeDir(t *testing.T) {
	tmpDir := t.TempDir()

	// Use a non-existent worktree path
	worktreePath := filepath.Join(tmpDir, "nonexistent", "worktree")

	guard := lock.NewPIDGuard(worktreePath)

	// Acquire should create the directory
	if err := guard.Acquire(); err != nil {
		t.Fatalf("acquire failed: %v", err)
	}

	// Verify directory was created
	testutil.AssertFileExists(t, worktreePath)
	testutil.AssertFileExists(t, filepath.Join(worktreePath, lock.PIDFileName))

	guard.Release()
}

// TestPIDGuardSafeToReleaseMultipleTimes verifies Release is idempotent.
func TestPIDGuardSafeToReleaseMultipleTimes(t *testing.T) {
	repo := testutil.SetupTestRepo(t)

	worktreePath := repo.CreateWorktree("TASK-003", "")
	guard := lock.NewPIDGuard(worktreePath)

	if err := guard.Acquire(); err != nil {
		t.Fatalf("acquire failed: %v", err)
	}

	// Release multiple times should not panic or error
	guard.Release()
	guard.Release()
	guard.Release()

	// PID file should not exist
	pidFile := filepath.Join(worktreePath, lock.PIDFileName)
	testutil.AssertFileNotExists(t, pidFile)
}

// TestDifferentUsersCanRunSameTask verifies that different users (different
// worktree paths with executor prefixes) can run the same task simultaneously.
func TestDifferentUsersCanRunSameTask(t *testing.T) {
	repo := testutil.SetupTestRepo(t)
	repo.InitSharedDir()

	taskID := "TASK-001"

	// Create worktrees for Alice and Bob
	aliceWorktree := repo.CreateWorktree(taskID, "am")
	bobWorktree := repo.CreateWorktree(taskID, "bj")

	aliceGuard := lock.NewPIDGuard(aliceWorktree)
	bobGuard := lock.NewPIDGuard(bobWorktree)

	// Both should be able to acquire without blocking each other
	if err := aliceGuard.Check(); err != nil {
		t.Errorf("Alice check failed: %v", err)
	}
	if err := aliceGuard.Acquire(); err != nil {
		t.Errorf("Alice acquire failed: %v", err)
	}

	if err := bobGuard.Check(); err != nil {
		t.Errorf("Bob check failed: %v", err)
	}
	if err := bobGuard.Acquire(); err != nil {
		t.Errorf("Bob acquire failed: %v", err)
	}

	// Both PID files should exist
	testutil.AssertFileExists(t, filepath.Join(aliceWorktree, lock.PIDFileName))
	testutil.AssertFileExists(t, filepath.Join(bobWorktree, lock.PIDFileName))

	// Cleanup
	aliceGuard.Release()
	bobGuard.Release()
}

// TestPIDGuardWithInvalidPIDFile verifies handling of corrupted PID files.
func TestPIDGuardWithInvalidPIDFile(t *testing.T) {
	repo := testutil.SetupTestRepo(t)

	worktreePath := repo.CreateWorktree("TASK-004", "")
	guard := lock.NewPIDGuard(worktreePath)

	pidFile := filepath.Join(worktreePath, lock.PIDFileName)

	// Write invalid content to PID file
	if err := os.WriteFile(pidFile, []byte("not-a-number"), 0644); err != nil {
		t.Fatalf("write invalid PID file: %v", err)
	}

	// Check should succeed (invalid PID file is cleaned up)
	if err := guard.Check(); err != nil {
		t.Errorf("check with invalid PID file failed: %v", err)
	}

	// Invalid PID file should be removed
	testutil.AssertFileNotExists(t, pidFile)
}
