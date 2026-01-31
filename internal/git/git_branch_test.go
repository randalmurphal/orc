package git

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateBranch(t *testing.T) {
	tmpDir := setupTestRepo(t)
	g, _ := New(tmpDir, DefaultConfig())

	err := g.CreateBranch("TASK-001")
	if err != nil {
		t.Fatalf("CreateBranch() failed: %v", err)
	}

	// Verify branch was created and checked out
	branch, err := g.GetCurrentBranch()
	if err != nil {
		t.Fatalf("GetCurrentBranch() failed: %v", err)
	}

	if branch != "orc/TASK-001" {
		t.Errorf("current branch = %s, want orc/TASK-001", branch)
	}
}

func TestSwitchBranch(t *testing.T) {
	tmpDir := setupTestRepo(t)
	g, _ := New(tmpDir, DefaultConfig())

	// Create the branch first
	err := g.CreateBranch("TASK-001")
	if err != nil {
		t.Fatalf("CreateBranch() failed: %v", err)
	}

	// Switch back to main/master
	cmd := exec.Command("git", "checkout", "-")
	cmd.Dir = tmpDir
	_ = cmd.Run()

	// Now switch to the task branch
	err = g.SwitchBranch("TASK-001")
	if err != nil {
		t.Fatalf("SwitchBranch() failed: %v", err)
	}

	branch, _ := g.GetCurrentBranch()
	if branch != "orc/TASK-001" {
		t.Errorf("current branch = %s, want orc/TASK-001", branch)
	}
}

func TestRewind(t *testing.T) {
	tmpDir := setupTestRepo(t)
	baseGit, _ := New(tmpDir, DefaultConfig())
	// Use InWorktree to mark as worktree context (Rewind requires this)
	g := baseGit.InWorktree(tmpDir)

	if err := g.CreateBranch("TASK-001"); err != nil {
		t.Fatalf("CreateBranch failed: %v", err)
	}

	// Create first checkpoint
	testFile := filepath.Join(tmpDir, "first.txt")
	if err := os.WriteFile(testFile, []byte("first"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	checkpoint1, _ := g.CreateCheckpoint("TASK-001", "spec", "first")

	// Create second checkpoint
	testFile2 := filepath.Join(tmpDir, "second.txt")
	if err := os.WriteFile(testFile2, []byte("second"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	_, _ = g.CreateCheckpoint("TASK-001", "implement", "second")

	// second.txt should exist
	if _, err := os.Stat(testFile2); os.IsNotExist(err) {
		t.Error("second.txt should exist before rewind")
	}

	// Rewind to first checkpoint
	err := g.Rewind(checkpoint1.CommitSHA)
	if err != nil {
		t.Fatalf("Rewind() failed: %v", err)
	}

	// second.txt should not exist
	if _, err := os.Stat(testFile2); !os.IsNotExist(err) {
		t.Error("second.txt should not exist after rewind")
	}
}

// TestPush_ProtectedBranch tests that Push() blocks protected branches
func TestPush_ProtectedBranch(t *testing.T) {
	tmpDir := setupTestRepo(t)
	baseGit, _ := New(tmpDir, DefaultConfig())
	// Use InWorktree to mark as worktree context (Push requires this)
	g := baseGit.InWorktree(tmpDir)

	tests := []struct {
		branch    string
		wantError bool
	}{
		{"main", true},
		{"master", true},
		{"develop", true},
		{"release", true},
		{"orc/TASK-001", false},
		{"feature/foo", false},
	}

	for _, tt := range tests {
		err := g.Push("origin", tt.branch, false)
		// Note: Push will fail because there's no remote, but for protected
		// branches it should fail with ErrProtectedBranch BEFORE trying
		if tt.wantError {
			if err == nil {
				t.Errorf("Push(%q) should return error for protected branch", tt.branch)
			}
			if !strings.Contains(err.Error(), "protected branch") {
				t.Errorf("Push(%q) error should mention protected branch, got: %v", tt.branch, err)
			}
		} else {
			// For non-protected branches, it will still fail (no remote)
			// but NOT with protected branch error
			if err != nil && strings.Contains(err.Error(), "protected branch") {
				t.Errorf("Push(%q) should not fail with protected branch error", tt.branch)
			}
		}
	}
}

// TestProtectedBranches_CustomConfig tests custom protected branches
func TestProtectedBranches_CustomConfig(t *testing.T) {
	tmpDir := setupTestRepo(t)
	cfg := Config{
		BranchPrefix:      "orc/",
		CommitPrefix:      "[orc]",
		WorktreeDir:       ".orc/worktrees",
		ProtectedBranches: []string{"prod", "staging"},
	}

	baseGit, err := New(tmpDir, cfg)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	// Use InWorktree to mark as worktree context (Push requires this)
	g := baseGit.InWorktree(tmpDir)

	// Check custom protected branches
	protected := g.ProtectedBranches()
	if len(protected) != 2 {
		t.Errorf("ProtectedBranches() = %v, want 2 items", protected)
	}

	// main should NOT be protected in this config
	err = g.Push("origin", "main", false)
	if err != nil && strings.Contains(err.Error(), "protected branch") {
		t.Error("main should not be protected with custom config")
	}

	// prod should be protected
	err = g.Push("origin", "prod", false)
	if err == nil || !strings.Contains(err.Error(), "protected branch") {
		t.Error("prod should be protected with custom config")
	}
}

// TestRemoteBranchExists tests the RemoteBranchExists method
func TestRemoteBranchExists(t *testing.T) {
	tmpDir := setupTestRepo(t)
	g, _ := New(tmpDir, DefaultConfig())

	// For a local-only repo without a configured remote, ls-remote will fail
	// This is expected behavior - we're testing the method exists and works correctly
	_, err := g.RemoteBranchExists("origin", "main")
	// The error should be about ls-remote failing (no remote), not a panic
	if err != nil {
		if !strings.Contains(err.Error(), "ls-remote failed") {
			t.Errorf("RemoteBranchExists() unexpected error: %v", err)
		}
		// This is expected - no remote configured
	}
}

// TestPushForce_TaskBranch tests that PushForce works for task branches
func TestPushForce_TaskBranch(t *testing.T) {
	tmpDir := setupTestRepo(t)
	g, _ := New(tmpDir, DefaultConfig())

	// PushForce should NOT fail with protected branch error for task branches
	// (it will fail because there's no remote, but that's a different error)
	err := g.PushForce("origin", "orc/TASK-001", false)
	if err != nil && strings.Contains(err.Error(), "protected branch") {
		t.Error("PushForce() should not fail with protected branch error for task branches")
	}
}

// TestPushForce_RequiresWorktree verifies PushForce requires worktree context
func TestPushForce_RequiresWorktree(t *testing.T) {
	tmpDir := setupTestRepo(t)
	g, _ := New(tmpDir, DefaultConfig())

	// PushForce should fail with worktree check first
	err := g.PushForce("origin", "main", false)
	if err == nil {
		t.Fatal("PushForce() should fail outside of worktree context")
	}
	if !strings.Contains(err.Error(), "worktree context") {
		t.Errorf("PushForce() error should mention worktree context, got: %v", err)
	}
}

// TestHasRemote_NoRemote tests HasRemote when no remote is configured
func TestHasRemote_NoRemote(t *testing.T) {
	tmpDir := setupTestRepo(t)
	g, _ := New(tmpDir, DefaultConfig())

	// A freshly created local repo has no remotes
	hasRemote := g.HasRemote("origin")
	if hasRemote {
		t.Error("HasRemote('origin') = true, want false for repo with no remotes")
	}
}

// TestHasRemote_WithRemote tests HasRemote when a remote is configured
func TestHasRemote_WithRemote(t *testing.T) {
	tmpDir := setupTestRepo(t)
	g, _ := New(tmpDir, DefaultConfig())

	// Add a remote (use file:// to avoid HTTPS auth prompts in CI/tests)
	cmd := exec.Command("git", "remote", "add", "origin", "file:///tmp/fake-remote.git")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to add remote: %v", err)
	}

	// Now HasRemote should return true
	hasRemote := g.HasRemote("origin")
	if !hasRemote {
		t.Error("HasRemote('origin') = false, want true for repo with origin remote")
	}

	// Non-existent remote should return false
	hasRemote = g.HasRemote("nonexistent")
	if hasRemote {
		t.Error("HasRemote('nonexistent') = true, want false")
	}
}

// TestHasRemote_InWorktree tests HasRemote in worktree context
func TestHasRemote_InWorktree(t *testing.T) {
	tmpDir := setupTestRepo(t)
	g, _ := New(tmpDir, testConfigWithWorktreeDir(tmpDir))

	// Add a remote to main repo (use file:// to avoid HTTPS auth prompts)
	cmd := exec.Command("git", "remote", "add", "origin", "file:///tmp/fake-remote.git")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to add remote: %v", err)
	}

	baseBranch, _ := g.GetCurrentBranch()
	worktreePath, err := g.CreateWorktree("TASK-REMOTE", baseBranch)
	if err != nil {
		t.Fatalf("CreateWorktree() failed: %v", err)
	}
	defer func() { _ = g.CleanupWorktree("TASK-REMOTE") }()

	wtGit := g.InWorktree(worktreePath)

	// Worktree should inherit remote configuration from main repo
	hasRemote := wtGit.HasRemote("origin")
	if !hasRemote {
		t.Error("HasRemote('origin') in worktree = false, want true")
	}
}

// TestMainRepoProtection_RequireWorktreeContext tests the worktree context validation
func TestMainRepoProtection_RequireWorktreeContext(t *testing.T) {
	tmpDir := setupTestRepo(t)
	g, err := New(tmpDir, DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create Git: %v", err)
	}

	// RequireWorktreeContext should fail for main repo
	err = g.RequireWorktreeContext("test operation")
	if err == nil {
		t.Error("RequireWorktreeContext() should return error for main repo")
	}
	if !errors.Is(err, ErrMainRepoModification) {
		t.Errorf("error should be ErrMainRepoModification, got: %v", err)
	}
}

// TestMainRepoProtection_RewindBlocked tests that Rewind is blocked on main repo
func TestMainRepoProtection_RewindBlocked(t *testing.T) {
	tmpDir := setupTestRepo(t)
	g, err := New(tmpDir, DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create Git: %v", err)
	}

	// Get HEAD for reset target
	head, _ := g.ctx.HeadCommit()

	// Rewind should fail on main repo
	err = g.Rewind(head)
	if err == nil {
		t.Fatal("Rewind() should fail on main repo")
	}
	if !errors.Is(err, ErrMainRepoModification) {
		t.Errorf("error should be ErrMainRepoModification, got: %v", err)
	}
}

// TestMainRepoProtection_CheckoutSafeBlocked verifies CheckoutSafe is blocked on main repo
func TestMainRepoProtection_CheckoutSafeBlocked(t *testing.T) {
	tmpDir := setupTestRepo(t)
	g, _ := New(tmpDir, DefaultConfig())

	// CheckoutSafe should fail on main repo
	err := g.CheckoutSafe("feature")
	if err == nil {
		t.Fatal("CheckoutSafe() should fail on main repo")
	}
	if !errors.Is(err, ErrMainRepoModification) {
		t.Errorf("error should be ErrMainRepoModification, got: %v", err)
	}
}

// TestMainRepoProtection_WorktreeContextAllowed tests that worktree context allows operations
func TestMainRepoProtection_WorktreeContextAllowed(t *testing.T) {
	tmpDir := setupTestRepo(t)
	g, err := New(tmpDir, testConfigWithWorktreeDir(tmpDir))
	if err != nil {
		t.Fatalf("failed to create Git: %v", err)
	}

	// Create a worktree
	worktreePath, err := g.CreateWorktree("TASK-TEST", "main")
	if err != nil {
		t.Fatalf("CreateWorktree() failed: %v", err)
	}
	defer func() { _ = g.CleanupWorktree("TASK-TEST") }()

	// Get worktree Git instance
	wg := g.InWorktree(worktreePath)

	// RequireWorktreeContext should succeed for worktree
	err = wg.RequireWorktreeContext("test operation")
	if err != nil {
		t.Errorf("RequireWorktreeContext() should succeed for worktree, got: %v", err)
	}

	// CheckoutSafe should work in worktree
	err = wg.CheckoutSafe("orc/TASK-TEST")
	if err != nil {
		t.Errorf("CheckoutSafe() should succeed for worktree, got: %v", err)
	}
}

// TestMainRepoProtection_MainBranchUnchangedAfterWorktreeOperations verifies main repo stays untouched
func TestMainRepoProtection_MainBranchUnchangedAfterWorktreeOperations(t *testing.T) {
	tmpDir := setupTestRepo(t)
	g, err := New(tmpDir, testConfigWithWorktreeDir(tmpDir))
	if err != nil {
		t.Fatalf("failed to create Git: %v", err)
	}

	// Record initial main branch state
	initialHead, _ := g.ctx.HeadCommit()
	initialBranch, _ := g.GetCurrentBranch()

	// Create worktree
	worktreePath, err := g.CreateWorktree("TASK-INTEGRITY", "main")
	if err != nil {
		t.Fatalf("CreateWorktree() failed: %v", err)
	}
	defer func() { _ = g.CleanupWorktree("TASK-INTEGRITY") }()

	// Get worktree Git instance
	wg := g.InWorktree(worktreePath)

	// Make changes in worktree
	testFile := filepath.Join(worktreePath, "test-file.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	_, _ = wg.CreateCheckpoint("TASK-INTEGRITY", "implement", "test change")

	// Verify main repo is unchanged
	finalHead, _ := g.ctx.HeadCommit()
	finalBranch, _ := g.GetCurrentBranch()

	if initialHead != finalHead {
		t.Errorf("main repo HEAD changed from %s to %s", initialHead, finalHead)
	}
	if initialBranch != finalBranch {
		t.Errorf("main repo branch changed from %s to %s", initialBranch, finalBranch)
	}

	// Verify no unexpected files in main repo
	_, err = os.Stat(filepath.Join(tmpDir, "test-file.txt"))
	if !os.IsNotExist(err) {
		t.Error("test-file.txt should NOT exist in main repo")
	}
}

// TestMainRepoProtection_ProtectedBranchRewindBlocked tests that Rewind is blocked on protected branches
func TestMainRepoProtection_ProtectedBranchRewindBlocked(t *testing.T) {
	tmpDir := setupTestRepo(t)
	g, err := New(tmpDir, testConfigWithWorktreeDir(tmpDir))
	if err != nil {
		t.Fatalf("failed to create Git: %v", err)
	}

	// Create a worktree on main branch
	worktreePath, err := g.CreateWorktree("TASK-MAIN", "main")
	if err != nil {
		t.Fatalf("CreateWorktree() failed: %v", err)
	}
	defer func() { _ = g.CleanupWorktree("TASK-MAIN") }()

	// Get worktree Git instance
	wg := g.InWorktree(worktreePath)

	// Switch to main branch (protected)
	// This shouldn't happen normally but we test the safety anyway
	_ = wg.ctx.Checkout("main")

	// Verify current branch is main
	branch, _ := wg.GetCurrentBranch()
	if branch != "main" {
		t.Skipf("Could not switch to main in worktree, got: %s", branch)
	}

	// Rewind should fail on protected branch even in worktree context
	head, _ := wg.ctx.HeadCommit()
	err = wg.Rewind(head)
	if err == nil {
		t.Fatal("Rewind() should fail on protected branch")
	}
	if !errors.Is(err, ErrMainRepoModification) {
		t.Errorf("error should be ErrMainRepoModification, got: %v", err)
	}
}
