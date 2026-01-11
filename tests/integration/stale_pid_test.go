package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/randalmurphal/orc/internal/lock"
	"github.com/randalmurphal/orc/tests/testutil"
)

// TestStalePIDCleanup verifies that stale PID files (from processes that no
// longer exist) are automatically cleaned up.
func TestStalePIDCleanup(t *testing.T) {
	repo := testutil.SetupTestRepo(t)

	worktreePath := repo.CreateWorktree("TASK-001", "")
	guard := lock.NewPIDGuard(worktreePath)

	pidFile := filepath.Join(worktreePath, lock.PIDFileName)

	// Write a fake PID that doesn't exist (high PID unlikely to be running)
	// Use a PID that almost certainly doesn't exist
	if err := os.WriteFile(pidFile, []byte("999999"), 0644); err != nil {
		t.Fatalf("write fake PID file: %v", err)
	}

	// Verify PID file exists
	testutil.AssertFileExists(t, pidFile)

	// Check should succeed because the process doesn't exist (stale cleanup)
	if err := guard.Check(); err != nil {
		t.Errorf("check with stale PID failed: %v", err)
	}

	// Stale PID file should be removed
	testutil.AssertFileNotExists(t, pidFile)

	// Should be able to acquire now
	if err := guard.Acquire(); err != nil {
		t.Errorf("acquire after stale cleanup failed: %v", err)
	}

	// Verify new PID file contains our PID
	data, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatalf("read new PID file: %v", err)
	}
	if string(data) == "999999" {
		t.Error("PID file still contains stale PID")
	}

	guard.Release()
}

// TestStalePIDWithNegativePID verifies handling of invalid negative PIDs.
func TestStalePIDWithNegativePID(t *testing.T) {
	repo := testutil.SetupTestRepo(t)

	worktreePath := repo.CreateWorktree("TASK-002", "")
	guard := lock.NewPIDGuard(worktreePath)

	pidFile := filepath.Join(worktreePath, lock.PIDFileName)

	// Write negative PID (invalid)
	if err := os.WriteFile(pidFile, []byte("-1"), 0644); err != nil {
		t.Fatalf("write negative PID file: %v", err)
	}

	// Check should succeed (invalid PID cleaned up as stale)
	// Note: The current implementation will try to parse this as a valid int
	// A negative PID will fail processExists and be treated as stale
	if err := guard.Check(); err != nil {
		t.Errorf("check with negative PID failed: %v", err)
	}

	// File should be cleaned up or we should be able to proceed
	// Either way, acquire should succeed
	if err := guard.Acquire(); err != nil {
		t.Errorf("acquire after negative PID cleanup failed: %v", err)
	}

	guard.Release()
}

// TestStalePIDWithZeroPID verifies handling of PID 0.
func TestStalePIDWithZeroPID(t *testing.T) {
	repo := testutil.SetupTestRepo(t)

	worktreePath := repo.CreateWorktree("TASK-003", "")
	guard := lock.NewPIDGuard(worktreePath)

	pidFile := filepath.Join(worktreePath, lock.PIDFileName)

	// Write PID 0 (invalid - PID 0 is the kernel scheduler)
	if err := os.WriteFile(pidFile, []byte("0"), 0644); err != nil {
		t.Fatalf("write zero PID file: %v", err)
	}

	// Check should handle this case
	// Note: signal(0) to PID 0 may behave differently on different systems
	err := guard.Check()
	// We accept either success (cleaned up) or specific error handling
	if err != nil {
		t.Logf("check with zero PID returned: %v (may be expected)", err)
	}
}

// TestStalePIDWithEmptyFile verifies handling of empty PID files.
func TestStalePIDWithEmptyFile(t *testing.T) {
	repo := testutil.SetupTestRepo(t)

	worktreePath := repo.CreateWorktree("TASK-004", "")
	guard := lock.NewPIDGuard(worktreePath)

	pidFile := filepath.Join(worktreePath, lock.PIDFileName)

	// Write empty file
	if err := os.WriteFile(pidFile, []byte(""), 0644); err != nil {
		t.Fatalf("write empty PID file: %v", err)
	}

	// Check should succeed (empty file is invalid, cleaned up)
	if err := guard.Check(); err != nil {
		t.Errorf("check with empty PID file failed: %v", err)
	}

	// Should be cleaned up
	testutil.AssertFileNotExists(t, pidFile)
}

// TestStalePIDWithWhitespace verifies handling of PID files with whitespace.
func TestStalePIDWithWhitespace(t *testing.T) {
	repo := testutil.SetupTestRepo(t)

	worktreePath := repo.CreateWorktree("TASK-005", "")
	guard := lock.NewPIDGuard(worktreePath)

	pidFile := filepath.Join(worktreePath, lock.PIDFileName)

	// Write PID with leading/trailing whitespace
	if err := os.WriteFile(pidFile, []byte("  999999  \n"), 0644); err != nil {
		t.Fatalf("write whitespace PID file: %v", err)
	}

	// Check should succeed (whitespace trimmed, stale PID cleaned up)
	if err := guard.Check(); err != nil {
		t.Errorf("check with whitespace PID file failed: %v", err)
	}

	// Stale file should be cleaned up
	testutil.AssertFileNotExists(t, pidFile)
}

// TestStalePIDSequence verifies multiple stale PID scenarios in sequence.
func TestStalePIDSequence(t *testing.T) {
	repo := testutil.SetupTestRepo(t)

	worktreePath := repo.CreateWorktree("TASK-006", "")
	guard := lock.NewPIDGuard(worktreePath)

	pidFile := filepath.Join(worktreePath, lock.PIDFileName)

	// Scenario 1: Write stale PID, clean up, acquire
	if err := os.WriteFile(pidFile, []byte("888888"), 0644); err != nil {
		t.Fatalf("write first stale PID: %v", err)
	}

	if err := guard.Check(); err != nil {
		t.Fatalf("first check failed: %v", err)
	}
	testutil.AssertFileNotExists(t, pidFile)

	if err := guard.Acquire(); err != nil {
		t.Fatalf("first acquire failed: %v", err)
	}
	guard.Release()

	// Scenario 2: Write another stale PID, clean up, acquire
	if err := os.WriteFile(pidFile, []byte("777777"), 0644); err != nil {
		t.Fatalf("write second stale PID: %v", err)
	}

	if err := guard.Check(); err != nil {
		t.Fatalf("second check failed: %v", err)
	}
	testutil.AssertFileNotExists(t, pidFile)

	if err := guard.Acquire(); err != nil {
		t.Fatalf("second acquire failed: %v", err)
	}
	guard.Release()

	// Scenario 3: Write invalid content, clean up, acquire
	if err := os.WriteFile(pidFile, []byte("garbage"), 0644); err != nil {
		t.Fatalf("write garbage PID: %v", err)
	}

	if err := guard.Check(); err != nil {
		t.Fatalf("third check failed: %v", err)
	}
	testutil.AssertFileNotExists(t, pidFile)

	if err := guard.Acquire(); err != nil {
		t.Fatalf("third acquire failed: %v", err)
	}
	guard.Release()
}

// TestStalePIDConcurrentCleanup simulates concurrent access with stale PID.
func TestStalePIDConcurrentCleanup(t *testing.T) {
	repo := testutil.SetupTestRepo(t)

	worktreePath := repo.CreateWorktree("TASK-007", "")

	pidFile := filepath.Join(worktreePath, lock.PIDFileName)

	// Write stale PID
	if err := os.WriteFile(pidFile, []byte("666666"), 0644); err != nil {
		t.Fatalf("write stale PID: %v", err)
	}

	// Create two guards pointing to same worktree
	// This simulates race condition where two processes might try to clean up
	guard1 := lock.NewPIDGuard(worktreePath)
	guard2 := lock.NewPIDGuard(worktreePath)

	// Both should succeed (one cleans up, other finds no file)
	err1 := guard1.Check()
	err2 := guard2.Check()

	if err1 != nil {
		t.Errorf("guard1 check failed: %v", err1)
	}
	if err2 != nil {
		t.Errorf("guard2 check failed: %v", err2)
	}

	// PID file should be cleaned up
	testutil.AssertFileNotExists(t, pidFile)
}
