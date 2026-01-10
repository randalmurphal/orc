package git

import (
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
	if !strings.Contains(path, "orc-task-001") {
		t.Errorf("WorktreePath() = %s, should contain sanitized branch name", path)
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

func TestCreateWorktree(t *testing.T) {
	tmpDir := setupTestRepo(t)
	g := New(tmpDir)

	// Create the .orc/worktrees directory
	worktreesDir := filepath.Join(tmpDir, ".orc", "worktrees")
	os.MkdirAll(worktreesDir, 0755)

	// Get current branch for base
	baseBranch, err := g.GetCurrentBranch()
	if err != nil {
		t.Fatalf("GetCurrentBranch() failed: %v", err)
	}

	// Create worktree
	path, err := g.CreateWorktree("TASK-001", baseBranch)
	if err != nil {
		t.Fatalf("CreateWorktree() failed: %v", err)
	}

	// Verify path is correct
	expectedPath := ".orc/worktrees/TASK-001"
	if path != expectedPath {
		t.Errorf("worktree path = %s, want %s", path, expectedPath)
	}

	// Verify the worktree was actually created
	worktreeFullPath := filepath.Join(tmpDir, path)
	if _, err := os.Stat(worktreeFullPath); os.IsNotExist(err) {
		t.Error("worktree directory was not created")
	}
}

func TestRemoveWorktree(t *testing.T) {
	tmpDir := setupTestRepo(t)
	g := New(tmpDir)

	// Create the .orc/worktrees directory
	os.MkdirAll(filepath.Join(tmpDir, ".orc", "worktrees"), 0755)

	baseBranch, _ := g.GetCurrentBranch()

	// Create a worktree first
	_, err := g.CreateWorktree("TASK-002", baseBranch)
	if err != nil {
		t.Fatalf("CreateWorktree() failed: %v", err)
	}

	// Verify worktree exists
	worktreePath := filepath.Join(tmpDir, ".orc", "worktrees", "TASK-002")
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		t.Fatal("worktree should exist before removal")
	}

	// Remove worktree
	err = g.RemoveWorktree("TASK-002")
	if err != nil {
		t.Fatalf("RemoveWorktree() failed: %v", err)
	}

	// Verify worktree was removed
	if _, err := os.Stat(worktreePath); !os.IsNotExist(err) {
		t.Error("worktree should be removed")
	}
}

func TestRemoveWorktree_NotExists(t *testing.T) {
	tmpDir := setupTestRepo(t)
	g := New(tmpDir)

	// Try to remove a worktree that doesn't exist
	err := g.RemoveWorktree("NONEXISTENT")
	if err == nil {
		t.Error("RemoveWorktree() should fail for non-existent worktree")
	}
}
