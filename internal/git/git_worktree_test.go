package git

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBranchName(t *testing.T) {
	tmpDir := setupTestRepo(t)
	g, _ := New(tmpDir, DefaultConfig())

	name := g.BranchName("TASK-001")
	if name != "orc/TASK-001" {
		t.Errorf("BranchName() = %s, want orc/TASK-001", name)
	}
}

func TestWorktreePath(t *testing.T) {
	tmpDir := setupTestRepo(t)
	cfg := DefaultConfig()
	cfg.WorktreeDir = ".orc/worktrees"
	g, _ := New(tmpDir, cfg)

	path := g.WorktreePath("TASK-001")
	if !strings.Contains(path, ".orc/worktrees") {
		t.Errorf("WorktreePath() = %s, should contain .orc/worktrees", path)
	}
	// New naming: orc-TASK-001 (preserves case, no branch sanitization)
	if !strings.Contains(path, "orc-TASK-001") {
		t.Errorf("WorktreePath() = %s, should contain orc-TASK-001", path)
	}
}

func TestCreateAndCleanupWorktree(t *testing.T) {
	tmpDir := setupTestRepo(t)
	g, _ := New(tmpDir, DefaultConfig())

	// Get current branch to use as base
	baseBranch, _ := g.GetCurrentBranch()

	// Create worktree
	worktreePath, err := g.CreateWorktree("TASK-001", baseBranch)
	if err != nil {
		t.Fatalf("CreateWorktree() failed: %v", err)
	}

	// Verify worktree exists
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		t.Errorf("worktree not created at %s", worktreePath)
	}

	// Cleanup
	err = g.CleanupWorktree("TASK-001")
	if err != nil {
		t.Fatalf("CleanupWorktree() failed: %v", err)
	}

	// Verify worktree removed
	if _, err := os.Stat(worktreePath); !os.IsNotExist(err) {
		t.Error("worktree should be removed after cleanup")
	}
}

func TestInWorktree(t *testing.T) {
	tmpDir := setupTestRepo(t)
	g, _ := New(tmpDir, DefaultConfig())

	baseBranch, _ := g.GetCurrentBranch()
	worktreePath, err := g.CreateWorktree("TASK-001", baseBranch)
	if err != nil {
		t.Fatalf("CreateWorktree() failed: %v", err)
	}
	defer func() { _ = g.CleanupWorktree("TASK-001") }()

	// Get git instance for worktree
	wtGit := g.InWorktree(worktreePath)

	// Verify it operates in worktree
	branch, err := wtGit.GetCurrentBranch()
	if err != nil {
		t.Fatalf("GetCurrentBranch() in worktree failed: %v", err)
	}

	if branch != "orc/TASK-001" {
		t.Errorf("worktree branch = %s, want orc/TASK-001", branch)
	}

	// Create a file in worktree
	testFile := filepath.Join(worktreePath, "worktree-test.txt")
	_ = os.WriteFile(testFile, []byte("test"), 0644)

	clean, _ := wtGit.IsClean()
	if clean {
		t.Error("worktree should have uncommitted changes")
	}
}

// TestCleanupWorktree_NotExists tests cleanup of non-existent worktree
func TestCleanupWorktree_NotExists(t *testing.T) {
	tmpDir := setupTestRepo(t)
	g, _ := New(tmpDir, DefaultConfig())

	// Try to cleanup a worktree that doesn't exist - should error
	err := g.CleanupWorktree("NONEXISTENT")
	if err == nil {
		t.Error("CleanupWorktree() should fail for non-existent worktree")
	}
}

// TestIsInWorktreeContext tests worktree context tracking
func TestIsInWorktreeContext(t *testing.T) {
	tmpDir := setupTestRepo(t)
	g, _ := New(tmpDir, DefaultConfig())

	// Main git instance is not in worktree context
	if g.IsInWorktreeContext() {
		t.Error("main Git instance should not be in worktree context")
	}

	// Create worktree
	baseBranch, _ := g.GetCurrentBranch()
	worktreePath, err := g.CreateWorktree("TASK-001", baseBranch)
	if err != nil {
		t.Fatalf("CreateWorktree() failed: %v", err)
	}
	defer func() { _ = g.CleanupWorktree("TASK-001") }()

	// Worktree git instance should be in worktree context
	wtGit := g.InWorktree(worktreePath)
	if !wtGit.IsInWorktreeContext() {
		t.Error("worktree Git instance should be in worktree context")
	}
}

// TestInjectWorktreeHooks_Integration tests hook injection into a real worktree
func TestInjectWorktreeHooks_Integration(t *testing.T) {
	tmpDir := setupTestRepo(t)
	g, _ := New(tmpDir, DefaultConfig())

	baseBranch, _ := g.GetCurrentBranch()
	worktreePath, err := g.CreateWorktree("TASK-002", baseBranch)
	if err != nil {
		t.Fatalf("CreateWorktree() failed: %v", err)
	}
	defer func() { _ = g.CleanupWorktree("TASK-002") }()

	// Check that hooks were created in worktree's git directory
	// The .git file in worktree points to the actual git directory
	gitFile := filepath.Join(worktreePath, ".git")
	content, err := os.ReadFile(gitFile)
	if err != nil {
		t.Fatalf("failed to read .git file: %v", err)
	}

	// Parse gitdir path
	line := strings.TrimSpace(string(content))
	if !strings.HasPrefix(line, "gitdir: ") {
		t.Fatalf("unexpected .git file format: %s", line)
	}
	gitDir := strings.TrimPrefix(line, "gitdir: ")

	// Check hooks directory exists
	hooksDir := filepath.Join(gitDir, "hooks")
	info, err := os.Stat(hooksDir)
	if err != nil {
		t.Fatalf("hooks directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("hooks path should be a directory")
	}

	// Check pre-push hook exists and is executable
	prePushPath := filepath.Join(hooksDir, "pre-push")
	info, err = os.Stat(prePushPath)
	if err != nil {
		t.Fatalf("pre-push hook not created: %v", err)
	}
	if info.Mode()&0111 == 0 {
		t.Error("pre-push hook should be executable")
	}

	// Check pre-commit hook exists and is executable
	preCommitPath := filepath.Join(hooksDir, "pre-commit")
	info, err = os.Stat(preCommitPath)
	if err != nil {
		t.Fatalf("pre-commit hook not created: %v", err)
	}
	if info.Mode()&0111 == 0 {
		t.Error("pre-commit hook should be executable")
	}

	// Verify hook content contains expected values
	prePushContent, _ := os.ReadFile(prePushPath)
	if !strings.Contains(string(prePushContent), "TASK-002") {
		t.Error("pre-push hook should contain task ID")
	}
	if !strings.Contains(string(prePushContent), "orc/TASK-002") {
		t.Error("pre-push hook should contain task branch")
	}

	// Verify core.hooksPath is set in worktree config file
	// (we write directly to .git/worktrees/<name>/config instead of using git config
	// to avoid polluting the main repo's config)
	worktreeConfigPath := filepath.Join(gitDir, "config")
	configContent, err := os.ReadFile(worktreeConfigPath)
	if err != nil {
		t.Fatalf("failed to read worktree config: %v", err)
	}
	if !strings.Contains(string(configContent), "hooksPath") {
		t.Error("worktree config should contain hooksPath")
	}
	if !strings.Contains(string(configContent), hooksDir) {
		t.Errorf("worktree config should point to hooks dir, got: %s", string(configContent))
	}
}

// TestRemoveWorktreeHooks_Integration tests hook removal during cleanup
func TestRemoveWorktreeHooks_Integration(t *testing.T) {
	tmpDir := setupTestRepo(t)
	g, _ := New(tmpDir, DefaultConfig())

	baseBranch, _ := g.GetCurrentBranch()
	worktreePath, err := g.CreateWorktree("TASK-003", baseBranch)
	if err != nil {
		t.Fatalf("CreateWorktree() failed: %v", err)
	}

	// Get hooks directory path before cleanup
	gitFile := filepath.Join(worktreePath, ".git")
	content, _ := os.ReadFile(gitFile)
	line := strings.TrimSpace(string(content))
	gitDir := strings.TrimPrefix(line, "gitdir: ")
	hooksDir := filepath.Join(gitDir, "hooks")

	// Verify hooks exist
	if _, err := os.Stat(hooksDir); os.IsNotExist(err) {
		t.Fatal("hooks directory should exist before cleanup")
	}

	// Remove hooks explicitly
	err = g.RemoveWorktreeHooks(worktreePath)
	if err != nil {
		t.Fatalf("RemoveWorktreeHooks() failed: %v", err)
	}

	// Verify hooks directory is removed
	if _, err := os.Stat(hooksDir); !os.IsNotExist(err) {
		t.Error("hooks directory should be removed after RemoveWorktreeHooks()")
	}

	// Cleanup worktree
	_ = g.CleanupWorktree("TASK-003")
}

// TestCreateWorktree_HooksContainProtectedBranches tests hooks contain all protected branches
func TestCreateWorktree_HooksContainProtectedBranches(t *testing.T) {
	tmpDir := setupTestRepo(t)
	g, _ := New(tmpDir, DefaultConfig())

	baseBranch, _ := g.GetCurrentBranch()
	worktreePath, err := g.CreateWorktree("TASK-004", baseBranch)
	if err != nil {
		t.Fatalf("CreateWorktree() failed: %v", err)
	}
	defer func() { _ = g.CleanupWorktree("TASK-004") }()

	// Read pre-push hook content
	gitFile := filepath.Join(worktreePath, ".git")
	content, _ := os.ReadFile(gitFile)
	line := strings.TrimSpace(string(content))
	gitDir := strings.TrimPrefix(line, "gitdir: ")
	prePushPath := filepath.Join(gitDir, "hooks", "pre-push")

	hookContent, err := os.ReadFile(prePushPath)
	if err != nil {
		t.Fatalf("failed to read pre-push hook: %v", err)
	}

	// Verify all default protected branches are in the hook
	for _, branch := range DefaultProtectedBranches {
		if !strings.Contains(string(hookContent), branch) {
			t.Errorf("pre-push hook should contain protected branch %q", branch)
		}
	}
}

// TestPruneWorktrees tests the PruneWorktrees method
func TestPruneWorktrees(t *testing.T) {
	tmpDir := setupTestRepo(t)
	g, _ := New(tmpDir, DefaultConfig())

	// PruneWorktrees should succeed even when there's nothing to prune
	err := g.PruneWorktrees()
	if err != nil {
		t.Fatalf("PruneWorktrees() failed: %v", err)
	}
}

// TestCreateWorktree_StaleWorktree tests that CreateWorktree handles stale registrations
func TestCreateWorktree_StaleWorktree(t *testing.T) {
	tmpDir := setupTestRepo(t)
	g, _ := New(tmpDir, DefaultConfig())

	baseBranch, _ := g.GetCurrentBranch()

	// Create a worktree first
	worktreePath, err := g.CreateWorktree("TASK-STALE", baseBranch)
	if err != nil {
		t.Fatalf("CreateWorktree() failed: %v", err)
	}

	// Manually delete the worktree directory (simulating stale registration)
	if err := os.RemoveAll(worktreePath); err != nil {
		t.Fatalf("failed to remove worktree directory: %v", err)
	}

	// Verify directory is gone but git still thinks worktree exists
	if _, err := os.Stat(worktreePath); !os.IsNotExist(err) {
		t.Fatal("worktree directory should not exist")
	}

	// git worktree list should still show the stale worktree
	output, err := g.ctx.RunGit("worktree", "list")
	if err != nil {
		t.Fatalf("git worktree list failed: %v", err)
	}
	if !strings.Contains(output, "orc-TASK-STALE") {
		t.Skip("git may have auto-pruned the stale worktree")
	}

	// Now try to create the same worktree again - should succeed due to auto-prune
	worktreePath2, err := g.CreateWorktree("TASK-STALE", baseBranch)
	if err != nil {
		t.Fatalf("CreateWorktree() should auto-prune stale worktree: %v", err)
	}

	// Verify worktree was created
	if _, err := os.Stat(worktreePath2); os.IsNotExist(err) {
		t.Error("worktree should exist after re-creation")
	}

	// Cleanup
	_ = g.CleanupWorktree("TASK-STALE")
}

// TestCreateWorktree_StaleWorktree_ExistingBranch tests stale worktree with existing branch
func TestCreateWorktree_StaleWorktree_ExistingBranch(t *testing.T) {
	tmpDir := setupTestRepo(t)
	g, _ := New(tmpDir, DefaultConfig())

	baseBranch, _ := g.GetCurrentBranch()

	// Create a worktree first
	worktreePath, err := g.CreateWorktree("TASK-STALE2", baseBranch)
	if err != nil {
		t.Fatalf("CreateWorktree() failed: %v", err)
	}

	// Make a commit in the worktree so the branch has content
	wtGit := g.InWorktree(worktreePath)
	testFile := filepath.Join(worktreePath, "stale-test.txt")
	_ = os.WriteFile(testFile, []byte("test"), 0644)
	_, _ = wtGit.CreateCheckpoint("TASK-STALE2", "implement", "test commit")

	// Manually delete the worktree directory (simulating stale registration)
	if err := os.RemoveAll(worktreePath); err != nil {
		t.Fatalf("failed to remove worktree directory: %v", err)
	}

	// Now the branch exists but worktree registration is stale
	// Try to create the worktree again - should succeed
	worktreePath2, err := g.CreateWorktree("TASK-STALE2", baseBranch)
	if err != nil {
		t.Fatalf("CreateWorktree() should handle stale worktree with existing branch: %v", err)
	}

	// Verify worktree was created with the existing branch
	wtGit2 := g.InWorktree(worktreePath2)
	branch, _ := wtGit2.GetCurrentBranch()
	if branch != "orc/TASK-STALE2" {
		t.Errorf("worktree should be on orc/TASK-STALE2, got %s", branch)
	}

	// Verify the file from the previous commit exists
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Error("stale-test.txt should exist from previous commit")
	}

	// Cleanup
	_ = g.CleanupWorktree("TASK-STALE2")
}

// TestInWorktree_IndependentMutex verifies that InWorktree returns
// a Git instance with its own independent mutex.
func TestInWorktree_IndependentMutex(t *testing.T) {
	tmpDir := setupTestRepo(t)
	g, _ := New(tmpDir, DefaultConfig())

	baseBranch, _ := g.GetCurrentBranch()
	worktreePath, err := g.CreateWorktree("TASK-MUTEX", baseBranch)
	if err != nil {
		t.Fatalf("CreateWorktree() failed: %v", err)
	}
	defer func() { _ = g.CleanupWorktree("TASK-MUTEX") }()

	// Get worktree Git instance
	wtGit := g.InWorktree(worktreePath)

	// Create file in worktree
	testFile := filepath.Join(worktreePath, "mutex-test.txt")
	_ = os.WriteFile(testFile, []byte("test"), 0644)

	// Both instances should be able to work independently
	// (they have separate mutexes)
	done := make(chan error, 2)

	// Parent Git instance
	go func() {
		// Create a file in main repo
		mainFile := filepath.Join(tmpDir, "main-test.txt")
		_ = os.WriteFile(mainFile, []byte("main content"), 0644)
		_, err := g.CreateCheckpoint("TASK-MUTEX-MAIN", "implement", "main change")
		done <- err
	}()

	// Worktree Git instance
	go func() {
		_, err := wtGit.CreateCheckpoint("TASK-MUTEX", "implement", "worktree change")
		done <- err
	}()

	// Both should complete without deadlock
	for i := 0; i < 2; i++ {
		err := <-done
		if err != nil {
			// Errors are expected (different directories), just verify no deadlock
			t.Logf("Expected error (different contexts): %v", err)
		}
	}
}

// TestCleanupWorktreeAtPath tests path-based worktree cleanup
func TestCleanupWorktreeAtPath(t *testing.T) {
	tmpDir := setupTestRepo(t)
	g, _ := New(tmpDir, DefaultConfig())

	// Get current branch to use as base
	baseBranch, _ := g.GetCurrentBranch()

	// Create worktree
	worktreePath, err := g.CreateWorktree("TASK-PATH-001", baseBranch)
	if err != nil {
		t.Fatalf("CreateWorktree() failed: %v", err)
	}

	// Verify worktree exists
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		t.Errorf("worktree not created at %s", worktreePath)
	}

	// Cleanup using path-based method
	err = g.CleanupWorktreeAtPath(worktreePath)
	if err != nil {
		t.Fatalf("CleanupWorktreeAtPath() failed: %v", err)
	}

	// Verify worktree removed
	if _, err := os.Stat(worktreePath); !os.IsNotExist(err) {
		t.Error("worktree should be removed after CleanupWorktreeAtPath")
	}
}

// TestCleanupWorktreeAtPath_EmptyPath tests CleanupWorktreeAtPath with empty path
func TestCleanupWorktreeAtPath_EmptyPath(t *testing.T) {
	tmpDir := setupTestRepo(t)
	g, _ := New(tmpDir, DefaultConfig())

	// Empty path should return nil (nothing to clean up)
	err := g.CleanupWorktreeAtPath("")
	if err != nil {
		t.Errorf("CleanupWorktreeAtPath('') should return nil, got: %v", err)
	}
}

// TestCleanupWorktreeAtPath_InitiativePrefix tests cleanup of initiative-prefixed worktrees
func TestCleanupWorktreeAtPath_InitiativePrefix(t *testing.T) {
	tmpDir := setupTestRepo(t)
	g, _ := New(tmpDir, DefaultConfig())

	baseBranch, _ := g.GetCurrentBranch()

	// Create worktree with initiative prefix
	// Initiative prefix "feature/auth-" becomes "feature-auth-" in directory name
	worktreePath, err := g.CreateWorktreeWithInitiativePrefix("TASK-INIT-001", baseBranch, "feature/auth-")
	if err != nil {
		t.Fatalf("CreateWorktreeWithInitiativePrefix() failed: %v", err)
	}

	// Verify path contains initiative prefix (slashes replaced with dashes)
	if !strings.Contains(worktreePath, "feature-auth-TASK-INIT-001") {
		t.Errorf("worktree path should contain initiative prefix, got: %s", worktreePath)
	}

	// Verify worktree exists
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		t.Fatalf("worktree not created at %s", worktreePath)
	}

	// Cleanup using path-based method (this is what the fix enables)
	err = g.CleanupWorktreeAtPath(worktreePath)
	if err != nil {
		t.Fatalf("CleanupWorktreeAtPath() failed for initiative-prefixed worktree: %v", err)
	}

	// Verify worktree removed
	if _, err := os.Stat(worktreePath); !os.IsNotExist(err) {
		t.Error("initiative-prefixed worktree should be removed after CleanupWorktreeAtPath")
	}
}

// TestCleanupWorktreeAtPath_VsCleanupWorktree verifies path-based cleanup
// works correctly for initiative-prefixed worktrees where ID-based cleanup would fail
func TestCleanupWorktreeAtPath_VsCleanupWorktree(t *testing.T) {
	tmpDir := setupTestRepo(t)
	g, _ := New(tmpDir, DefaultConfig())

	baseBranch, _ := g.GetCurrentBranch()

	// Create worktree with initiative prefix
	worktreePath, err := g.CreateWorktreeWithInitiativePrefix("TASK-VS-001", baseBranch, "feature/test-")
	if err != nil {
		t.Fatalf("CreateWorktreeWithInitiativePrefix() failed: %v", err)
	}

	// Verify the paths differ - ID-based would look for orc-TASK-VS-001
	idBasedPath := g.WorktreePath("TASK-VS-001")
	if idBasedPath == worktreePath {
		t.Skip("paths are the same - this test is for verifying initiative prefix behavior")
	}

	// The ID-based path should NOT match the actual worktree path
	if !strings.Contains(idBasedPath, "orc-TASK-VS-001") {
		t.Errorf("ID-based path should use default prefix, got: %s", idBasedPath)
	}
	if !strings.Contains(worktreePath, "feature-test-TASK-VS-001") {
		t.Errorf("initiative path should use initiative prefix, got: %s", worktreePath)
	}

	// Path-based cleanup should work
	err = g.CleanupWorktreeAtPath(worktreePath)
	if err != nil {
		t.Fatalf("CleanupWorktreeAtPath() should work with actual path: %v", err)
	}

	// Verify cleanup succeeded
	if _, err := os.Stat(worktreePath); !os.IsNotExist(err) {
		t.Error("worktree should be removed after CleanupWorktreeAtPath")
	}
}

func TestCreateCheckpoint(t *testing.T) {
	tmpDir := setupTestRepo(t)
	baseGit, _ := New(tmpDir, DefaultConfig())
	// Use InWorktree to mark as worktree context (CreateCheckpoint requires this)
	g := baseGit.InWorktree(tmpDir)

	if err := g.CreateBranch("TASK-001"); err != nil {
		t.Fatalf("CreateBranch failed: %v", err)
	}

	// Add a change
	testFile := filepath.Join(tmpDir, "test.go")
	_ = os.WriteFile(testFile, []byte("package main\n"), 0644)

	checkpoint, err := g.CreateCheckpoint("TASK-001", "implement", "completed")
	if err != nil {
		t.Fatalf("CreateCheckpoint() failed: %v", err)
	}

	if checkpoint.TaskID != "TASK-001" {
		t.Errorf("TaskID = %s, want TASK-001", checkpoint.TaskID)
	}

	if checkpoint.Phase != "implement" {
		t.Errorf("Phase = %s, want implement", checkpoint.Phase)
	}

	if checkpoint.CommitSHA == "" {
		t.Error("CommitSHA is empty")
	}

	if checkpoint.Message != "completed" {
		t.Errorf("Message = %s, want completed", checkpoint.Message)
	}
}

func TestCreateCheckpointEmpty(t *testing.T) {
	tmpDir := setupTestRepo(t)
	baseGit, _ := New(tmpDir, DefaultConfig())
	// Use InWorktree to mark as worktree context (CreateCheckpoint requires this)
	g := baseGit.InWorktree(tmpDir)

	if err := g.CreateBranch("TASK-001"); err != nil {
		t.Fatalf("CreateBranch failed: %v", err)
	}

	// Create checkpoint without changes (should use --allow-empty)
	checkpoint, err := g.CreateCheckpoint("TASK-001", "implement", "checkpoint")
	if err != nil {
		t.Fatalf("CreateCheckpoint() with empty changes failed: %v", err)
	}

	if checkpoint.CommitSHA == "" {
		t.Error("CommitSHA is empty for empty commit")
	}
}

// TestConcurrentCheckpoints tests that CreateCheckpoint is protected by mutex
// when called concurrently from multiple goroutines.
func TestConcurrentCheckpoints(t *testing.T) {
	tmpDir := setupTestRepo(t)
	baseGit, _ := New(tmpDir, DefaultConfig())
	// Use InWorktree to mark as worktree context (CreateCheckpoint requires this)
	g := baseGit.InWorktree(tmpDir)

	_ = g.CreateBranch("TASK-CONCURRENT")

	// Create multiple files for concurrent commits
	const numGoroutines = 5
	done := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			// Create a unique file for this goroutine
			testFile := filepath.Join(tmpDir, fmt.Sprintf("concurrent-%d.txt", idx))
			_ = os.WriteFile(testFile, []byte(fmt.Sprintf("content %d", idx)), 0644)

			// Create checkpoint - mutex should ensure atomicity
			_, err := g.CreateCheckpoint("TASK-CONCURRENT", "implement", fmt.Sprintf("change %d", idx))
			done <- err
		}(i)
	}

	// Wait for all goroutines to complete
	var errors []error
	for i := 0; i < numGoroutines; i++ {
		if err := <-done; err != nil {
			errors = append(errors, err)
		}
	}

	// With mutex protection, all checkpoints should succeed
	// (they serialize access to the git operations)
	if len(errors) > 0 {
		t.Errorf("CreateCheckpoint() concurrent calls failed: %v", errors)
	}
}
