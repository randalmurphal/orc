// Package integration contains integration tests for the P2P workflow.
package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/randalmurphal/orc/internal/git"
	"github.com/randalmurphal/orc/internal/lock"
	"github.com/randalmurphal/orc/tests/testutil"
)

// TestMultiUserExecution verifies that two users can work on the same task
// concurrently with separate branches and worktrees.
func TestMultiUserExecution(t *testing.T) {
	repo := testutil.SetupTestRepo(t)
	repo.InitSharedDir()

	// Set P2P mode in shared config
	repo.SetSharedConfig("task_id.mode", "p2p")

	taskID := "TASK-001"

	// Alice's setup
	alicePrefix := "am"
	aliceBranch := git.BranchName(taskID, alicePrefix)
	aliceWorktreePath := git.WorktreePath(
		filepath.Join(repo.OrcDir, "worktrees"),
		taskID,
		alicePrefix,
	)

	// Bob's setup
	bobPrefix := "bj"
	bobBranch := git.BranchName(taskID, bobPrefix)
	bobWorktreePath := git.WorktreePath(
		filepath.Join(repo.OrcDir, "worktrees"),
		taskID,
		bobPrefix,
	)

	// Verify branch names are distinct
	if aliceBranch == bobBranch {
		t.Fatalf("Alice and Bob should have different branch names: %s vs %s", aliceBranch, bobBranch)
	}

	// Verify expected branch names
	expectedAliceBranch := "orc/TASK-001-am"
	expectedBobBranch := "orc/TASK-001-bj"
	if aliceBranch != expectedAliceBranch {
		t.Errorf("Alice branch = %q, want %q", aliceBranch, expectedAliceBranch)
	}
	if bobBranch != expectedBobBranch {
		t.Errorf("Bob branch = %q, want %q", bobBranch, expectedBobBranch)
	}

	// Verify worktree paths are distinct
	if aliceWorktreePath == bobWorktreePath {
		t.Fatalf("Alice and Bob should have different worktree paths: %s vs %s", aliceWorktreePath, bobWorktreePath)
	}

	// Create worktrees for both users
	if err := os.MkdirAll(aliceWorktreePath, 0755); err != nil {
		t.Fatalf("create Alice worktree: %v", err)
	}
	if err := os.MkdirAll(bobWorktreePath, 0755); err != nil {
		t.Fatalf("create Bob worktree: %v", err)
	}

	// Each user acquires their own PID guard (should not conflict)
	aliceGuard := lock.NewPIDGuard(aliceWorktreePath)
	bobGuard := lock.NewPIDGuard(bobWorktreePath)

	// Both should be able to check without conflict
	if err := aliceGuard.Check(); err != nil {
		t.Errorf("Alice PID check failed: %v", err)
	}
	if err := bobGuard.Check(); err != nil {
		t.Errorf("Bob PID check failed: %v", err)
	}

	// Both should be able to acquire without conflict
	if err := aliceGuard.Acquire(); err != nil {
		t.Errorf("Alice PID acquire failed: %v", err)
	}
	if err := bobGuard.Acquire(); err != nil {
		t.Errorf("Bob PID acquire failed: %v", err)
	}

	// Verify separate PID files exist
	alicePIDFile := filepath.Join(aliceWorktreePath, lock.PIDFileName)
	bobPIDFile := filepath.Join(bobWorktreePath, lock.PIDFileName)

	testutil.AssertFileExists(t, alicePIDFile)
	testutil.AssertFileExists(t, bobPIDFile)

	// Verify PID files are in separate locations
	if alicePIDFile == bobPIDFile {
		t.Error("Alice and Bob PID files should be in different locations")
	}

	// Create separate state files (simulating independent execution)
	aliceStateDir := filepath.Join(aliceWorktreePath, ".orc", "state")
	bobStateDir := filepath.Join(bobWorktreePath, ".orc", "state")

	if err := os.MkdirAll(aliceStateDir, 0755); err != nil {
		t.Fatalf("create Alice state dir: %v", err)
	}
	if err := os.MkdirAll(bobStateDir, 0755); err != nil {
		t.Fatalf("create Bob state dir: %v", err)
	}

	// Write state files
	aliceState := map[string]any{
		"current_phase": "implement",
		"iteration":     1,
		"executor":      "am",
	}
	bobState := map[string]any{
		"current_phase": "test",
		"iteration":     2,
		"executor":      "bj",
	}

	testutil.WriteYAML(t, filepath.Join(aliceStateDir, "state.yaml"), aliceState)
	testutil.WriteYAML(t, filepath.Join(bobStateDir, "state.yaml"), bobState)

	// Verify states are independent
	aliceStateRead := testutil.ReadYAML(t, filepath.Join(aliceStateDir, "state.yaml"))
	bobStateRead := testutil.ReadYAML(t, filepath.Join(bobStateDir, "state.yaml"))

	if aliceStateRead["executor"] == bobStateRead["executor"] {
		t.Error("Alice and Bob should have different executor values in state")
	}
	if aliceStateRead["current_phase"] == bobStateRead["current_phase"] {
		t.Error("Alice and Bob can be in different phases")
	}

	// Cleanup
	aliceGuard.Release()
	bobGuard.Release()

	testutil.AssertFileNotExists(t, alicePIDFile)
	testutil.AssertFileNotExists(t, bobPIDFile)
}

// TestMultiUserWorkingOnDifferentTasks verifies users working on different tasks.
func TestMultiUserWorkingOnDifferentTasks(t *testing.T) {
	repo := testutil.SetupTestRepo(t)
	repo.InitSharedDir()

	// Alice working on TASK-AM-001
	aliceTaskID := "TASK-AM-001"
	aliceBranch := git.BranchName(aliceTaskID, "")
	aliceWorktreeDir := git.WorktreeDirName(aliceTaskID, "")

	// Bob working on TASK-BJ-001
	bobTaskID := "TASK-BJ-001"
	bobBranch := git.BranchName(bobTaskID, "")
	bobWorktreeDir := git.WorktreeDirName(bobTaskID, "")

	// Verify branches are distinct
	if aliceBranch == bobBranch {
		t.Errorf("Branches should be different: %s vs %s", aliceBranch, bobBranch)
	}

	// Verify worktree dirs are distinct
	if aliceWorktreeDir == bobWorktreeDir {
		t.Errorf("Worktree dirs should be different: %s vs %s", aliceWorktreeDir, bobWorktreeDir)
	}

	// Expected values
	if aliceBranch != "orc/TASK-AM-001" {
		t.Errorf("Alice branch = %q, want orc/TASK-AM-001", aliceBranch)
	}
	if bobBranch != "orc/TASK-BJ-001" {
		t.Errorf("Bob branch = %q, want orc/TASK-BJ-001", bobBranch)
	}
	if aliceWorktreeDir != "orc-TASK-AM-001" {
		t.Errorf("Alice worktree dir = %q, want orc-TASK-AM-001", aliceWorktreeDir)
	}
	if bobWorktreeDir != "orc-TASK-BJ-001" {
		t.Errorf("Bob worktree dir = %q, want orc-TASK-BJ-001", bobWorktreeDir)
	}
}

// TestBranchNameParsing verifies that branch names can be correctly parsed
// to extract task ID and executor prefix.
func TestBranchNameParsing(t *testing.T) {
	tests := []struct {
		branch         string
		wantTaskID     string
		wantExecutor   string
		wantOK         bool
	}{
		// Solo mode
		{"orc/TASK-001", "TASK-001", "", true},
		{"orc/TASK-999", "TASK-999", "", true},

		// P2P mode with executor suffix
		{"orc/TASK-001-am", "TASK-001", "am", true},
		{"orc/TASK-001-bj", "TASK-001", "bj", true},
		{"orc/TASK-999-xyz", "TASK-999", "xyz", true},

		// Prefixed task IDs (no executor suffix)
		{"orc/TASK-AM-001", "TASK-AM-001", "", true},
		{"orc/TASK-BJ-001", "TASK-BJ-001", "", true},

		// Prefixed task IDs with executor suffix
		{"orc/TASK-AM-001-bj", "TASK-AM-001", "bj", true},
		{"orc/TASK-BJ-002-am", "TASK-BJ-002", "am", true},

		// Invalid branches
		{"main", "", "", false},
		{"feature/foo", "", "", false},
		{"refs/heads/orc/TASK-001", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.branch, func(t *testing.T) {
			taskID, executor, ok := git.ParseBranchName(tt.branch)
			if ok != tt.wantOK {
				t.Errorf("ParseBranchName(%q) ok = %v, want %v", tt.branch, ok, tt.wantOK)
				return
			}
			if !tt.wantOK {
				return
			}
			if taskID != tt.wantTaskID {
				t.Errorf("ParseBranchName(%q) taskID = %q, want %q", tt.branch, taskID, tt.wantTaskID)
			}
			if executor != tt.wantExecutor {
				t.Errorf("ParseBranchName(%q) executor = %q, want %q", tt.branch, executor, tt.wantExecutor)
			}
		})
	}
}
