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
	g := New("/tmp/test")

	if g.workDir != "/tmp/test" {
		t.Errorf("workDir = %s, want /tmp/test", g.workDir)
	}

	if g.branchPrefix != "orc/" {
		t.Errorf("branchPrefix = %s, want orc/", g.branchPrefix)
	}

	if g.commitPrefix != "[orc]" {
		t.Errorf("commitPrefix = %s, want [orc]", g.commitPrefix)
	}
}

func TestCreateBranch(t *testing.T) {
	tmpDir := setupTestRepo(t)
	g := New(tmpDir)

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
	g := New(tmpDir)

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
	g := New(tmpDir)

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
	g := New(tmpDir)

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
	g := New(tmpDir)

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
	g := New(tmpDir)

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
	g := New(tmpDir)

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

func TestGetCheckpoints(t *testing.T) {
	tmpDir := setupTestRepo(t)
	g := New(tmpDir)

	g.CreateBranch("TASK-001")

	// Create two checkpoints
	testFile := filepath.Join(tmpDir, "first.txt")
	os.WriteFile(testFile, []byte("first"), 0644)
	g.CreateCheckpoint("TASK-001", "spec", "first")

	testFile2 := filepath.Join(tmpDir, "second.txt")
	os.WriteFile(testFile2, []byte("second"), 0644)
	g.CreateCheckpoint("TASK-001", "implement", "second")

	checkpoints, err := g.GetCheckpoints("TASK-001")
	if err != nil {
		t.Fatalf("GetCheckpoints() failed: %v", err)
	}

	// Should have at least 2 checkpoints (may have empty checkpoint from branch creation)
	if len(checkpoints) < 2 {
		t.Errorf("len(checkpoints) = %d, want >= 2", len(checkpoints))
	}

	// Verify our checkpoints are present
	foundSpec := false
	foundImpl := false
	for _, cp := range checkpoints {
		if strings.Contains(cp.Message, "spec") {
			foundSpec = true
		}
		if strings.Contains(cp.Message, "implement") {
			foundImpl = true
		}
	}

	if !foundSpec {
		t.Error("spec checkpoint not found")
	}
	if !foundImpl {
		t.Error("implement checkpoint not found")
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
