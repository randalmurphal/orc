package git

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func setupTestRepo(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}

	// Configure git user for commits
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = tmpDir
	cmd.Run()

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = tmpDir
	cmd.Run()

	// Create initial commit
	testFile := filepath.Join(tmpDir, "README.md")
	if err := os.WriteFile(testFile, []byte("# Test\n"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = tmpDir
	cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to create initial commit: %v", err)
	}

	return tmpDir
}

func TestNew(t *testing.T) {
	tmpDir := setupTestRepo(t)
	cfg := DefaultConfig()

	g, err := New(tmpDir, cfg)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	if g.branchPrefix != "orc/" {
		t.Errorf("branchPrefix = %s, want orc/", g.branchPrefix)
	}

	if g.commitPrefix != "[orc]" {
		t.Errorf("commitPrefix = %s, want [orc]", g.commitPrefix)
	}
}

func TestNewInvalidPath(t *testing.T) {
	_, err := New("/nonexistent/path", DefaultConfig())
	if err == nil {
		t.Error("New() should fail for non-git directory")
	}
}

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
	cmd.Run()

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

func TestCreateCheckpoint(t *testing.T) {
	tmpDir := setupTestRepo(t)
	g, _ := New(tmpDir, DefaultConfig())

	g.CreateBranch("TASK-001")

	// Add a change
	testFile := filepath.Join(tmpDir, "test.go")
	os.WriteFile(testFile, []byte("package main\n"), 0644)

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
	g, _ := New(tmpDir, DefaultConfig())

	g.CreateBranch("TASK-001")

	// Create checkpoint without changes (should use --allow-empty)
	checkpoint, err := g.CreateCheckpoint("TASK-001", "implement", "checkpoint")
	if err != nil {
		t.Fatalf("CreateCheckpoint() with empty changes failed: %v", err)
	}

	if checkpoint.CommitSHA == "" {
		t.Error("CommitSHA is empty for empty commit")
	}
}

func TestIsClean(t *testing.T) {
	tmpDir := setupTestRepo(t)
	g, _ := New(tmpDir, DefaultConfig())

	// Should be clean after initial commit
	clean, err := g.IsClean()
	if err != nil {
		t.Fatalf("IsClean() failed: %v", err)
	}

	if !clean {
		t.Error("IsClean() = false, want true")
	}

	// Add uncommitted change
	testFile := filepath.Join(tmpDir, "dirty.txt")
	os.WriteFile(testFile, []byte("dirty"), 0644)

	clean, err = g.IsClean()
	if err != nil {
		t.Fatalf("IsClean() failed: %v", err)
	}

	if clean {
		t.Error("IsClean() = true, want false")
	}
}

func TestGetCurrentBranch(t *testing.T) {
	tmpDir := setupTestRepo(t)
	g, _ := New(tmpDir, DefaultConfig())

	branch, err := g.GetCurrentBranch()
	if err != nil {
		t.Fatalf("GetCurrentBranch() failed: %v", err)
	}

	// Should be main or master depending on git config
	if branch != "main" && branch != "master" {
		t.Errorf("GetCurrentBranch() = %s, want main or master", branch)
	}
}

func TestRewind(t *testing.T) {
	tmpDir := setupTestRepo(t)
	g, _ := New(tmpDir, DefaultConfig())

	g.CreateBranch("TASK-001")

	// Create first checkpoint
	testFile := filepath.Join(tmpDir, "first.txt")
	os.WriteFile(testFile, []byte("first"), 0644)
	checkpoint1, _ := g.CreateCheckpoint("TASK-001", "spec", "first")

	// Create second checkpoint
	testFile2 := filepath.Join(tmpDir, "second.txt")
	os.WriteFile(testFile2, []byte("second"), 0644)
	g.CreateCheckpoint("TASK-001", "implement", "second")

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
	g, _ := New(tmpDir, DefaultConfig())

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
	defer g.CleanupWorktree("TASK-001")

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
	os.WriteFile(testFile, []byte("test"), 0644)

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

// TestPush_ProtectedBranch tests that Push() blocks protected branches
func TestPush_ProtectedBranch(t *testing.T) {
	tmpDir := setupTestRepo(t)
	g, _ := New(tmpDir, DefaultConfig())

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

// TestPushUnsafe_AllowsProtectedBranch tests that PushUnsafe bypasses protection
func TestPushUnsafe_AllowsProtectedBranch(t *testing.T) {
	tmpDir := setupTestRepo(t)
	g, _ := New(tmpDir, DefaultConfig())

	// PushUnsafe should NOT fail with protected branch error
	// (it will fail because there's no remote, but that's a different error)
	err := g.PushUnsafe("origin", "main", false)
	if err != nil && strings.Contains(err.Error(), "protected branch") {
		t.Error("PushUnsafe() should not fail with protected branch error")
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
	defer g.CleanupWorktree("TASK-001")

	// Worktree git instance should be in worktree context
	wtGit := g.InWorktree(worktreePath)
	if !wtGit.IsInWorktreeContext() {
		t.Error("worktree Git instance should be in worktree context")
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

	g, err := New(tmpDir, cfg)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

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

// TestInjectWorktreeHooks_Integration tests hook injection into a real worktree
func TestInjectWorktreeHooks_Integration(t *testing.T) {
	tmpDir := setupTestRepo(t)
	g, _ := New(tmpDir, DefaultConfig())

	baseBranch, _ := g.GetCurrentBranch()
	worktreePath, err := g.CreateWorktree("TASK-002", baseBranch)
	if err != nil {
		t.Fatalf("CreateWorktree() failed: %v", err)
	}
	defer g.CleanupWorktree("TASK-002")

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
	g.CleanupWorktree("TASK-003")
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
	defer g.CleanupWorktree("TASK-004")

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

// TestSyncResult tests the SyncResult struct
func TestSyncResult(t *testing.T) {
	result := &SyncResult{
		Synced:            true,
		ConflictsDetected: false,
		ConflictFiles:     nil,
		CommitsBehind:     0,
		CommitsAhead:      5,
	}

	if !result.Synced {
		t.Error("Synced should be true")
	}
	if result.ConflictsDetected {
		t.Error("ConflictsDetected should be false")
	}
	if result.CommitsAhead != 5 {
		t.Errorf("CommitsAhead = %d, want 5", result.CommitsAhead)
	}
}

// TestGetCommitCounts tests the commit count calculation
func TestGetCommitCounts(t *testing.T) {
	tmpDir := setupTestRepo(t)
	g, _ := New(tmpDir, DefaultConfig())

	baseBranch, _ := g.GetCurrentBranch()

	// Create a task branch
	err := g.CreateBranch("TASK-005")
	if err != nil {
		t.Fatalf("CreateBranch() failed: %v", err)
	}

	// Add commits to task branch
	testFile := filepath.Join(tmpDir, "feature.txt")
	os.WriteFile(testFile, []byte("feature"), 0644)
	_, _ = g.CreateCheckpoint("TASK-005", "implement", "add feature")

	// Get commit counts relative to base
	ahead, behind, err := g.getCommitCounts(baseBranch)
	if err != nil {
		t.Fatalf("getCommitCounts() failed: %v", err)
	}

	// Task branch should be ahead by 1 commit
	if ahead != 1 {
		t.Errorf("ahead = %d, want 1", ahead)
	}
	// Task branch should not be behind
	if behind != 0 {
		t.Errorf("behind = %d, want 0", behind)
	}
}

// TestDetectConflicts_NoConflicts tests conflict detection when no conflicts exist
func TestDetectConflicts_NoConflicts(t *testing.T) {
	tmpDir := setupTestRepo(t)
	g, _ := New(tmpDir, DefaultConfig())

	baseBranch, _ := g.GetCurrentBranch()

	// Create a task branch
	err := g.CreateBranch("TASK-006")
	if err != nil {
		t.Fatalf("CreateBranch() failed: %v", err)
	}

	// Add a commit that doesn't conflict
	testFile := filepath.Join(tmpDir, "new-feature.txt")
	os.WriteFile(testFile, []byte("new feature"), 0644)
	_, _ = g.CreateCheckpoint("TASK-006", "implement", "add new feature")

	// Detect conflicts - should find none
	result, err := g.DetectConflicts(baseBranch)
	if err != nil {
		t.Fatalf("DetectConflicts() failed: %v", err)
	}

	// Should not detect conflicts for new file
	if result.ConflictsDetected {
		t.Errorf("ConflictsDetected = true, want false (files: %v)", result.ConflictFiles)
	}
}

// TestDetectConflicts_WithConflicts tests conflict detection when conflicts exist
func TestDetectConflicts_WithConflicts(t *testing.T) {
	tmpDir := setupTestRepo(t)
	g, _ := New(tmpDir, DefaultConfig())

	baseBranch, _ := g.GetCurrentBranch()

	// Create a task branch
	err := g.CreateBranch("TASK-007")
	if err != nil {
		t.Fatalf("CreateBranch() failed: %v", err)
	}

	// Modify README on task branch
	readmeFile := filepath.Join(tmpDir, "README.md")
	os.WriteFile(readmeFile, []byte("# Task branch changes\n"), 0644)
	_, _ = g.CreateCheckpoint("TASK-007", "implement", "modify readme on task")

	// Switch back to base branch and make conflicting change
	cmd := exec.Command("git", "checkout", baseBranch)
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to checkout base branch: %v", err)
	}

	os.WriteFile(readmeFile, []byte("# Base branch changes\n"), 0644)
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = tmpDir
	cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "modify readme on base")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to commit on base: %v", err)
	}

	// Switch to task branch
	err = g.SwitchBranch("TASK-007")
	if err != nil {
		t.Fatalf("SwitchBranch() failed: %v", err)
	}

	// Detect conflicts - should find conflict
	result, err := g.DetectConflicts(baseBranch)
	if err != nil {
		t.Fatalf("DetectConflicts() failed: %v", err)
	}

	// Should detect conflict on README.md
	if !result.ConflictsDetected {
		t.Error("ConflictsDetected = false, want true")
	}
	if len(result.ConflictFiles) == 0 {
		t.Error("ConflictFiles should not be empty")
	}
}

// TestRebaseWithConflictCheck_Success tests successful rebase
func TestRebaseWithConflictCheck_Success(t *testing.T) {
	tmpDir := setupTestRepo(t)
	g, _ := New(tmpDir, DefaultConfig())

	baseBranch, _ := g.GetCurrentBranch()

	// Create a task branch
	err := g.CreateBranch("TASK-008")
	if err != nil {
		t.Fatalf("CreateBranch() failed: %v", err)
	}

	// Add a non-conflicting commit
	testFile := filepath.Join(tmpDir, "feature.txt")
	os.WriteFile(testFile, []byte("feature"), 0644)
	_, _ = g.CreateCheckpoint("TASK-008", "implement", "add feature")

	// Rebase should succeed with no conflicts
	result, err := g.RebaseWithConflictCheck(baseBranch)
	if err != nil {
		t.Fatalf("RebaseWithConflictCheck() failed: %v", err)
	}

	// Should indicate sync success
	if !result.Synced {
		t.Error("Synced = false, want true")
	}
	if result.ConflictsDetected {
		t.Errorf("ConflictsDetected = true, want false (files: %v)", result.ConflictFiles)
	}
}

// TestRebaseWithConflictCheck_Conflict tests rebase with conflicts
func TestRebaseWithConflictCheck_Conflict(t *testing.T) {
	tmpDir := setupTestRepo(t)
	g, _ := New(tmpDir, DefaultConfig())

	baseBranch, _ := g.GetCurrentBranch()

	// Create a task branch
	err := g.CreateBranch("TASK-009")
	if err != nil {
		t.Fatalf("CreateBranch() failed: %v", err)
	}

	// Modify README on task branch
	readmeFile := filepath.Join(tmpDir, "README.md")
	os.WriteFile(readmeFile, []byte("# Task branch changes\n"), 0644)
	_, _ = g.CreateCheckpoint("TASK-009", "implement", "modify readme on task")

	// Switch back to base branch and make conflicting change
	cmd := exec.Command("git", "checkout", baseBranch)
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to checkout base branch: %v", err)
	}

	os.WriteFile(readmeFile, []byte("# Base branch changes\n"), 0644)
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = tmpDir
	cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "modify readme on base")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to commit on base: %v", err)
	}

	// Switch to task branch
	err = g.SwitchBranch("TASK-009")
	if err != nil {
		t.Fatalf("SwitchBranch() failed: %v", err)
	}

	// Rebase should fail with conflict
	result, err := g.RebaseWithConflictCheck(baseBranch)
	if err == nil {
		t.Fatal("RebaseWithConflictCheck() should fail on conflict")
	}

	// Should return ErrMergeConflict
	if !strings.Contains(err.Error(), "merge conflict") {
		t.Errorf("error should mention merge conflict, got: %v", err)
	}

	// Result should indicate conflicts
	if !result.ConflictsDetected {
		t.Error("ConflictsDetected = false, want true")
	}
	if len(result.ConflictFiles) == 0 {
		t.Error("ConflictFiles should not be empty")
	}
}

// TestAbortRebase tests aborting an in-progress rebase
func TestAbortRebase(t *testing.T) {
	tmpDir := setupTestRepo(t)
	g, _ := New(tmpDir, DefaultConfig())

	// AbortRebase when no rebase is in progress should not panic
	// It may return an error but should not panic
	_ = g.AbortRebase()
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
	g.CleanupWorktree("TASK-STALE")
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
	os.WriteFile(testFile, []byte("test"), 0644)
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
	g.CleanupWorktree("TASK-STALE2")
}

// TestRestoreOrcDir_NoOrcDir tests RestoreOrcDir when .orc/ doesn't exist
func TestRestoreOrcDir_NoOrcDir(t *testing.T) {
	tmpDir := setupTestRepo(t)
	g, _ := New(tmpDir, DefaultConfig())

	baseBranch, _ := g.GetCurrentBranch()

	// Create a task branch
	err := g.CreateBranch("TASK-ORC-001")
	if err != nil {
		t.Fatalf("CreateBranch() failed: %v", err)
	}

	// No .orc/ directory exists, should return false with no error
	restored, err := g.RestoreOrcDir(baseBranch, "TASK-ORC-001")
	if err != nil {
		t.Fatalf("RestoreOrcDir() failed: %v", err)
	}
	if restored {
		t.Error("RestoreOrcDir() should return false when no .orc/ exists")
	}
}

// TestRestoreOrcDir_NoChanges tests RestoreOrcDir when .orc/ has no changes
func TestRestoreOrcDir_NoChanges(t *testing.T) {
	tmpDir := setupTestRepo(t)
	g, _ := New(tmpDir, DefaultConfig())

	baseBranch, _ := g.GetCurrentBranch()

	// Create .orc/ directory on base branch
	orcDir := filepath.Join(tmpDir, ".orc")
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatalf("failed to create .orc/: %v", err)
	}
	configFile := filepath.Join(orcDir, "config.yaml")
	if err := os.WriteFile(configFile, []byte("version: 1\n"), 0644); err != nil {
		t.Fatalf("failed to create config.yaml: %v", err)
	}

	// Commit .orc/ on base branch
	cmd := exec.Command("git", "add", ".orc/")
	cmd.Dir = tmpDir
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "Add .orc/")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to commit .orc/: %v", err)
	}

	// Create a task branch
	err := g.CreateBranch("TASK-ORC-002")
	if err != nil {
		t.Fatalf("CreateBranch() failed: %v", err)
	}

	// Don't modify .orc/ - should return false with no error
	restored, err := g.RestoreOrcDir(baseBranch, "TASK-ORC-002")
	if err != nil {
		t.Fatalf("RestoreOrcDir() failed: %v", err)
	}
	if restored {
		t.Error("RestoreOrcDir() should return false when no changes to .orc/")
	}
}

// TestRestoreOrcDir_WithChanges tests RestoreOrcDir when .orc/ has modifications
func TestRestoreOrcDir_WithChanges(t *testing.T) {
	tmpDir := setupTestRepo(t)
	g, _ := New(tmpDir, DefaultConfig())

	baseBranch, _ := g.GetCurrentBranch()

	// Create .orc/ directory on base branch
	orcDir := filepath.Join(tmpDir, ".orc")
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatalf("failed to create .orc/: %v", err)
	}
	configFile := filepath.Join(orcDir, "config.yaml")
	originalContent := "version: 1\n"
	if err := os.WriteFile(configFile, []byte(originalContent), 0644); err != nil {
		t.Fatalf("failed to create config.yaml: %v", err)
	}

	// Commit .orc/ on base branch
	cmd := exec.Command("git", "add", ".orc/")
	cmd.Dir = tmpDir
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "Add .orc/")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to commit .orc/: %v", err)
	}

	// Create a task branch
	err := g.CreateBranch("TASK-ORC-003")
	if err != nil {
		t.Fatalf("CreateBranch() failed: %v", err)
	}

	// Modify .orc/ on task branch
	modifiedContent := "version: 2\nmodified_by_task: true\n"
	if err := os.WriteFile(configFile, []byte(modifiedContent), 0644); err != nil {
		t.Fatalf("failed to modify config.yaml: %v", err)
	}

	// Commit the modification
	_, _ = g.CreateCheckpoint("TASK-ORC-003", "implement", "modify .orc/")

	// Verify file is modified
	content, _ := os.ReadFile(configFile)
	if string(content) != modifiedContent {
		t.Fatalf("expected modified content, got: %s", string(content))
	}

	// Restore .orc/ from base branch
	restored, err := g.RestoreOrcDir(baseBranch, "TASK-ORC-003")
	if err != nil {
		t.Fatalf("RestoreOrcDir() failed: %v", err)
	}
	if !restored {
		t.Error("RestoreOrcDir() should return true when .orc/ was restored")
	}

	// Verify file is restored to original content
	content, _ = os.ReadFile(configFile)
	if string(content) != originalContent {
		t.Errorf("expected original content after restore, got: %s", string(content))
	}
}

// TestRestoreOrcDir_NewFilesInOrc tests RestoreOrcDir when new files are added to .orc/
func TestRestoreOrcDir_NewFilesInOrc(t *testing.T) {
	tmpDir := setupTestRepo(t)
	g, _ := New(tmpDir, DefaultConfig())

	baseBranch, _ := g.GetCurrentBranch()

	// Create .orc/ directory on base branch
	orcDir := filepath.Join(tmpDir, ".orc")
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatalf("failed to create .orc/: %v", err)
	}
	configFile := filepath.Join(orcDir, "config.yaml")
	if err := os.WriteFile(configFile, []byte("version: 1\n"), 0644); err != nil {
		t.Fatalf("failed to create config.yaml: %v", err)
	}

	// Commit .orc/ on base branch
	cmd := exec.Command("git", "add", ".orc/")
	cmd.Dir = tmpDir
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "Add .orc/")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to commit .orc/: %v", err)
	}

	// Create a task branch
	err := g.CreateBranch("TASK-ORC-004")
	if err != nil {
		t.Fatalf("CreateBranch() failed: %v", err)
	}

	// Add a new file to .orc/ on task branch
	newFile := filepath.Join(orcDir, "tasks", "TASK-999", "task.yaml")
	if err := os.MkdirAll(filepath.Dir(newFile), 0755); err != nil {
		t.Fatalf("failed to create task dir: %v", err)
	}
	if err := os.WriteFile(newFile, []byte("id: TASK-999\n"), 0644); err != nil {
		t.Fatalf("failed to create new task file: %v", err)
	}

	// Commit the new file
	_, _ = g.CreateCheckpoint("TASK-ORC-004", "implement", "add task file to .orc/")

	// Verify new file exists
	if _, err := os.Stat(newFile); os.IsNotExist(err) {
		t.Fatal("new task file should exist before restore")
	}

	// Restore .orc/ from base branch
	restored, err := g.RestoreOrcDir(baseBranch, "TASK-ORC-004")
	if err != nil {
		t.Fatalf("RestoreOrcDir() failed: %v", err)
	}
	if !restored {
		t.Error("RestoreOrcDir() should return true when .orc/ was restored")
	}

	// Verify new file is removed (restored to base state)
	if _, err := os.Stat(newFile); !os.IsNotExist(err) {
		t.Error("new task file should be removed after restore")
	}
}

// TestRestoreOrcDir_InitiativeModification tests RestoreOrcDir with initiative file changes
func TestRestoreOrcDir_InitiativeModification(t *testing.T) {
	tmpDir := setupTestRepo(t)
	g, _ := New(tmpDir, DefaultConfig())

	baseBranch, _ := g.GetCurrentBranch()

	// Create .orc/initiatives/ directory on base branch
	initDir := filepath.Join(tmpDir, ".orc", "initiatives", "INIT-001")
	if err := os.MkdirAll(initDir, 0755); err != nil {
		t.Fatalf("failed to create initiatives dir: %v", err)
	}
	initFile := filepath.Join(initDir, "initiative.yaml")
	originalContent := "id: INIT-001\ntitle: Real Initiative\nstatus: active\n"
	if err := os.WriteFile(initFile, []byte(originalContent), 0644); err != nil {
		t.Fatalf("failed to create initiative.yaml: %v", err)
	}

	// Commit on base branch
	cmd := exec.Command("git", "add", ".orc/")
	cmd.Dir = tmpDir
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "Add initiative")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to commit initiative: %v", err)
	}

	// Create a task branch
	err := g.CreateBranch("TASK-ORC-005")
	if err != nil {
		t.Fatalf("CreateBranch() failed: %v", err)
	}

	// Modify initiative file (simulating accidental modification during task execution)
	modifiedContent := "id: INIT-001\ntitle: Test Initiative (corrupted)\nstatus: completed\ntasks:\n  - TASK-TEST-1\n  - TASK-TEST-2\n"
	if err := os.WriteFile(initFile, []byte(modifiedContent), 0644); err != nil {
		t.Fatalf("failed to modify initiative.yaml: %v", err)
	}

	// Commit the modification
	_, _ = g.CreateCheckpoint("TASK-ORC-005", "implement", "accidentally modify initiative")

	// Restore .orc/ from base branch
	restored, err := g.RestoreOrcDir(baseBranch, "TASK-ORC-005")
	if err != nil {
		t.Fatalf("RestoreOrcDir() failed: %v", err)
	}
	if !restored {
		t.Error("RestoreOrcDir() should return true when initiative was restored")
	}

	// Verify initiative is restored to original content
	content, _ := os.ReadFile(initFile)
	if string(content) != originalContent {
		t.Errorf("expected original initiative content after restore, got: %s", string(content))
	}
}

// TestRestoreOrcDir_CommitMessage tests that RestoreOrcDir creates correct commit message
func TestRestoreOrcDir_CommitMessage(t *testing.T) {
	tmpDir := setupTestRepo(t)
	g, _ := New(tmpDir, DefaultConfig())

	baseBranch, _ := g.GetCurrentBranch()

	// Create .orc/ directory on base branch
	orcDir := filepath.Join(tmpDir, ".orc")
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatalf("failed to create .orc/: %v", err)
	}
	configFile := filepath.Join(orcDir, "config.yaml")
	if err := os.WriteFile(configFile, []byte("version: 1\n"), 0644); err != nil {
		t.Fatalf("failed to create config.yaml: %v", err)
	}

	// Commit on base branch
	cmd := exec.Command("git", "add", ".orc/")
	cmd.Dir = tmpDir
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "Add .orc/")
	cmd.Dir = tmpDir
	cmd.Run()

	// Create a task branch
	g.CreateBranch("TASK-ORC-006")

	// Modify .orc/
	os.WriteFile(configFile, []byte("version: 2\n"), 0644)
	g.CreateCheckpoint("TASK-ORC-006", "implement", "modify .orc/")

	// Restore .orc/
	restored, err := g.RestoreOrcDir(baseBranch, "TASK-ORC-006")
	if err != nil {
		t.Fatalf("RestoreOrcDir() failed: %v", err)
	}
	if !restored {
		t.Fatal("RestoreOrcDir() should return true")
	}

	// Check commit message
	output, err := g.ctx.RunGit("log", "-1", "--format=%s")
	if err != nil {
		t.Fatalf("failed to get commit message: %v", err)
	}
	commitMsg := strings.TrimSpace(output)
	if !strings.Contains(commitMsg, "[orc]") {
		t.Errorf("commit message should contain [orc], got: %s", commitMsg)
	}
	if !strings.Contains(commitMsg, "TASK-ORC-006") {
		t.Errorf("commit message should contain task ID, got: %s", commitMsg)
	}
	if !strings.Contains(commitMsg, "restore .orc/") {
		t.Errorf("commit message should mention restore, got: %s", commitMsg)
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

// TestMainRepoProtection_RebaseBlocked tests that Rebase is blocked on main repo
func TestMainRepoProtection_RebaseBlocked(t *testing.T) {
	tmpDir := setupTestRepo(t)
	g, err := New(tmpDir, DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create Git: %v", err)
	}

	// Rebase should fail on main repo
	err = g.Rebase("HEAD")
	if err == nil {
		t.Fatal("Rebase() should fail on main repo")
	}
	if !errors.Is(err, ErrMainRepoModification) {
		t.Errorf("error should be ErrMainRepoModification, got: %v", err)
	}
}

// TestMainRepoProtection_MergeBlocked tests that Merge is blocked on main repo
func TestMainRepoProtection_MergeBlocked(t *testing.T) {
	tmpDir := setupTestRepo(t)
	g, err := New(tmpDir, DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create Git: %v", err)
	}

	// Merge should fail on main repo
	err = g.Merge("HEAD", false)
	if err == nil {
		t.Fatal("Merge() should fail on main repo")
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

// TestMainRepoProtection_RebaseWithConflictCheckBlocked tests RebaseWithConflictCheck is blocked
func TestMainRepoProtection_RebaseWithConflictCheckBlocked(t *testing.T) {
	tmpDir := setupTestRepo(t)
	g, err := New(tmpDir, DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create Git: %v", err)
	}

	// RebaseWithConflictCheck should fail on main repo
	_, err = g.RebaseWithConflictCheck("HEAD")
	if err == nil {
		t.Fatal("RebaseWithConflictCheck() should fail on main repo")
	}
	if !errors.Is(err, ErrMainRepoModification) {
		t.Errorf("error should be ErrMainRepoModification, got: %v", err)
	}
}

// TestMainRepoProtection_WorktreeContextAllowed tests that worktree context allows operations
func TestMainRepoProtection_WorktreeContextAllowed(t *testing.T) {
	tmpDir := setupTestRepo(t)
	g, err := New(tmpDir, DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create Git: %v", err)
	}

	// Create a worktree
	worktreePath, err := g.CreateWorktree("TASK-TEST", "main")
	if err != nil {
		t.Fatalf("CreateWorktree() failed: %v", err)
	}
	defer g.CleanupWorktree("TASK-TEST")

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
	g, err := New(tmpDir, DefaultConfig())
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
	defer g.CleanupWorktree("TASK-INTEGRITY")

	// Get worktree Git instance
	wg := g.InWorktree(worktreePath)

	// Make changes in worktree
	testFile := filepath.Join(worktreePath, "test-file.txt")
	os.WriteFile(testFile, []byte("test content"), 0644)
	wg.CreateCheckpoint("TASK-INTEGRITY", "implement", "test change")

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
	g, err := New(tmpDir, DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create Git: %v", err)
	}

	// Create a worktree on main branch
	worktreePath, err := g.CreateWorktree("TASK-MAIN", "main")
	if err != nil {
		t.Fatalf("CreateWorktree() failed: %v", err)
	}
	defer g.CleanupWorktree("TASK-MAIN")

	// Get worktree Git instance
	wg := g.InWorktree(worktreePath)

	// Switch to main branch (protected)
	// This shouldn't happen normally but we test the safety anyway
	wg.ctx.Checkout("main")

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
