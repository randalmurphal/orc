package executor

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/git"
	"github.com/randalmurphal/orc/internal/task"
)

// Tests for SetupWorktreeForTask

func TestSetupWorktreeForTask_NilGitOps(t *testing.T) {
	t.Parallel()
	tsk := task.NewProtoTask("TASK-001", "Test task")
	_, err := SetupWorktreeForTask(tsk, nil, nil, nil)
	if err == nil {
		t.Error("expected error when gitOps is nil")
	}
}

func TestSetupWorktreeForTask_NilTask(t *testing.T) {
	t.Parallel()
	tmpDir, err := os.MkdirTemp("", "orc-worktree-task-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	if err := initTestRepo(tmpDir); err != nil {
		t.Fatalf("failed to init test repo: %v", err)
	}

	gitCfg := git.DefaultConfig()
	gitCfg.WorktreeDir = filepath.Join(tmpDir, ".orc", "worktrees")
	gitOps, err := git.New(tmpDir, gitCfg)
	if err != nil {
		t.Fatalf("failed to create git ops: %v", err)
	}

	_, err = SetupWorktreeForTask(nil, nil, gitOps, nil)
	if err == nil {
		t.Error("expected error when task is nil")
	}
}

func TestSetupWorktreeForTask_CreatesWorktree(t *testing.T) {
	t.Parallel()
	tmpDir, err := os.MkdirTemp("", "orc-worktree-task-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	if err := initTestRepo(tmpDir); err != nil {
		t.Fatalf("failed to init test repo: %v", err)
	}

	gitCfg := git.DefaultConfig()
	gitCfg.WorktreeDir = filepath.Join(tmpDir, ".orc", "worktrees")
	gitOps, err := git.New(tmpDir, gitCfg)
	if err != nil {
		t.Fatalf("failed to create git ops: %v", err)
	}

	cfg := config.Default()
	cfg.Completion.TargetBranch = "main"

	tsk := task.NewProtoTask("TASK-001", "Test task")

	result, err := SetupWorktreeForTask(tsk, cfg, gitOps, nil)
	if err != nil {
		t.Fatalf("SetupWorktreeForTask failed: %v", err)
	}

	// Verify directory exists
	if _, err := os.Stat(result.Path); os.IsNotExist(err) {
		t.Errorf("worktree directory does not exist: %s", result.Path)
	}

	// Verify it's not marked as reused
	if result.Reused {
		t.Error("expected Reused to be false for new worktree")
	}

	// Verify target branch
	if result.TargetBranch != "main" {
		t.Errorf("expected target branch 'main', got %s", result.TargetBranch)
	}

	// Verify correct branch
	worktreeGit := gitOps.InWorktree(result.Path)
	currentBranch, err := worktreeGit.GetCurrentBranch()
	if err != nil {
		t.Fatalf("failed to get current branch: %v", err)
	}
	expectedBranch := gitOps.BranchName("TASK-001")
	if currentBranch != expectedBranch {
		t.Errorf("expected branch %s, got %s", expectedBranch, currentBranch)
	}
}

func TestSetupWorktreeForTask_ReusesExisting(t *testing.T) {
	t.Parallel()
	tmpDir, err := os.MkdirTemp("", "orc-worktree-task-reuse-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	if err := initTestRepo(tmpDir); err != nil {
		t.Fatalf("failed to init test repo: %v", err)
	}

	gitCfg := git.DefaultConfig()
	gitCfg.WorktreeDir = filepath.Join(tmpDir, ".orc", "worktrees")
	gitOps, err := git.New(tmpDir, gitCfg)
	if err != nil {
		t.Fatalf("failed to create git ops: %v", err)
	}

	tsk := task.NewProtoTask("TASK-REUSE", "Test task")

	// Create worktree first time
	result1, err := SetupWorktreeForTask(tsk, nil, gitOps, nil)
	if err != nil {
		t.Fatalf("first SetupWorktreeForTask failed: %v", err)
	}

	if result1.Reused {
		t.Error("first setup should not be reused")
	}

	// Setup again - should reuse
	result2, err := SetupWorktreeForTask(tsk, nil, gitOps, nil)
	if err != nil {
		t.Fatalf("second SetupWorktreeForTask failed: %v", err)
	}

	if !result2.Reused {
		t.Error("second setup should be reused")
	}

	if result1.Path != result2.Path {
		t.Errorf("paths should match: %s != %s", result1.Path, result2.Path)
	}
}

func TestSetupWorktreeForTask_SwitchesToCorrectBranch(t *testing.T) {
	t.Parallel()
	tmpDir, err := os.MkdirTemp("", "orc-worktree-task-branch-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	if err := initTestRepo(tmpDir); err != nil {
		t.Fatalf("failed to init test repo: %v", err)
	}

	gitCfg := git.DefaultConfig()
	gitCfg.WorktreeDir = filepath.Join(tmpDir, ".orc", "worktrees")
	gitOps, err := git.New(tmpDir, gitCfg)
	if err != nil {
		t.Fatalf("failed to create git ops: %v", err)
	}

	tsk := task.NewProtoTask("TASK-BRANCH2", "Test task")

	// Create worktree
	result1, err := SetupWorktreeForTask(tsk, nil, gitOps, nil)
	if err != nil {
		t.Fatalf("SetupWorktreeForTask failed: %v", err)
	}

	// Verify we're on the expected branch
	worktreeGit := gitOps.InWorktree(result1.Path)
	expectedBranch := gitOps.BranchName("TASK-BRANCH2")
	currentBranch, err := worktreeGit.GetCurrentBranch()
	if err != nil {
		t.Fatalf("failed to get current branch: %v", err)
	}
	if currentBranch != expectedBranch {
		t.Fatalf("expected branch %s, got %s", expectedBranch, currentBranch)
	}

	// Switch to a wrong branch
	wrongBranch := "wrong-branch-2"
	if err := runGitCmd(result1.Path, "checkout", "-b", wrongBranch); err != nil {
		t.Fatalf("failed to create and checkout wrong branch: %v", err)
	}

	// Verify we're on the wrong branch
	currentBranch, err = worktreeGit.GetCurrentBranch()
	if err != nil {
		t.Fatalf("failed to get current branch: %v", err)
	}
	if currentBranch != wrongBranch {
		t.Fatalf("expected to be on %s, got %s", wrongBranch, currentBranch)
	}

	// Reuse worktree - should switch back to the correct branch
	result2, err := SetupWorktreeForTask(tsk, nil, gitOps, nil)
	if err != nil {
		t.Fatalf("SetupWorktreeForTask failed on reuse: %v", err)
	}

	if !result2.Reused {
		t.Error("should be marked as reused")
	}

	// Verify we're back on the expected branch
	currentBranch, err = worktreeGit.GetCurrentBranch()
	if err != nil {
		t.Fatalf("failed to get current branch after reuse: %v", err)
	}
	if currentBranch != expectedBranch {
		t.Errorf("worktree should be on %s after reuse, but is on %s", expectedBranch, currentBranch)
	}
}

func TestSetupWorktreeForTask_WithTargetBranchOverride(t *testing.T) {
	t.Parallel()
	tmpDir, err := os.MkdirTemp("", "orc-worktree-target-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	if err := initTestRepo(tmpDir); err != nil {
		t.Fatalf("failed to init test repo: %v", err)
	}

	gitCfg := git.DefaultConfig()
	gitCfg.WorktreeDir = filepath.Join(tmpDir, ".orc", "worktrees")
	gitOps, err := git.New(tmpDir, gitCfg)
	if err != nil {
		t.Fatalf("failed to create git ops: %v", err)
	}

	// Create a task with explicit target branch
	tsk := task.NewProtoTask("TASK-TARGET", "Test task")
	task.SetTargetBranchProto(tsk, "develop")

	// Create develop branch first (since it's a default branch name)
	if err := runGitCmd(tmpDir, "branch", "develop"); err != nil {
		t.Fatalf("failed to create develop branch: %v", err)
	}

	result, err := SetupWorktreeForTask(tsk, nil, gitOps, nil)
	if err != nil {
		t.Fatalf("SetupWorktreeForTask failed: %v", err)
	}

	// Verify target branch is from task override
	if result.TargetBranch != "develop" {
		t.Errorf("expected target branch 'develop', got %s", result.TargetBranch)
	}
}

func TestSetupWorktreeForTask_ReturnsAbsolutePath(t *testing.T) {
	t.Parallel()
	tmpDir, err := os.MkdirTemp("", "orc-worktree-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	if err := initTestRepo(tmpDir); err != nil {
		t.Fatalf("failed to init test repo: %v", err)
	}

	gitCfg := git.DefaultConfig()
	gitCfg.WorktreeDir = filepath.Join(tmpDir, ".orc", "worktrees")
	gitOps, err := git.New(tmpDir, gitCfg)
	if err != nil {
		t.Fatalf("failed to create git ops: %v", err)
	}

	tsk := task.NewProtoTask("TASK-PATH", "Test task")
	result, err := SetupWorktreeForTask(tsk, nil, gitOps, nil)
	if err != nil {
		t.Fatalf("SetupWorktreeForTask failed: %v", err)
	}

	// Path should be non-empty and absolute
	if result.Path == "" {
		t.Error("expected non-empty path")
	}

	if !filepath.IsAbs(result.Path) {
		t.Errorf("expected absolute path, got: %s", result.Path)
	}

	// Path should contain the task ID pattern
	if !containsTaskID(result.Path, "TASK-PATH") {
		t.Errorf("path should contain task ID, got: %s", result.Path)
	}
}

func TestCleanupWorktree_RemovesDirectory(t *testing.T) {
	t.Parallel()
	tmpDir, err := os.MkdirTemp("", "orc-worktree-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	if err := initTestRepo(tmpDir); err != nil {
		t.Fatalf("failed to init test repo: %v", err)
	}

	gitCfg := git.DefaultConfig()
	gitCfg.WorktreeDir = filepath.Join(tmpDir, ".orc", "worktrees")
	gitOps, err := git.New(tmpDir, gitCfg)
	if err != nil {
		t.Fatalf("failed to create git ops: %v", err)
	}

	// Create worktree
	tsk := task.NewProtoTask("TASK-004", "Test task")
	result, err := SetupWorktreeForTask(tsk, nil, gitOps, nil)
	if err != nil {
		t.Fatalf("SetupWorktreeForTask failed: %v", err)
	}

	// Verify it exists
	if _, err := os.Stat(result.Path); os.IsNotExist(err) {
		t.Fatal("worktree should exist before cleanup")
	}

	// Cleanup
	if err := CleanupWorktree("TASK-004", gitOps); err != nil {
		t.Fatalf("CleanupWorktree failed: %v", err)
	}

	// Verify it's gone
	if _, err := os.Stat(result.Path); !os.IsNotExist(err) {
		t.Error("worktree directory should not exist after cleanup")
	}
}

func TestCleanupWorktree_NonexistentPath_NoError(t *testing.T) {
	t.Parallel()
	tmpDir, err := os.MkdirTemp("", "orc-worktree-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	if err := initTestRepo(tmpDir); err != nil {
		t.Fatalf("failed to init test repo: %v", err)
	}

	gitCfg := git.DefaultConfig()
	gitCfg.WorktreeDir = filepath.Join(tmpDir, ".orc", "worktrees")
	gitOps, err := git.New(tmpDir, gitCfg)
	if err != nil {
		t.Fatalf("failed to create git ops: %v", err)
	}

	// Cleanup a task that was never created
	err = CleanupWorktree("NONEXISTENT-TASK", gitOps)
	if err != nil {
		t.Errorf("CleanupWorktree should not error for nonexistent worktree: %v", err)
	}
}

func TestCleanupWorktree_NilGitOps(t *testing.T) {
	t.Parallel()
	// Should not error when gitOps is nil
	err := CleanupWorktree("TASK-001", nil)
	if err != nil {
		t.Errorf("CleanupWorktree should not error when gitOps is nil: %v", err)
	}
}

func TestWorktreePath_NilGitOps(t *testing.T) {
	t.Parallel()
	path := WorktreePath("TASK-001", nil)
	if path != "" {
		t.Errorf("expected empty path when gitOps is nil, got: %s", path)
	}
}

func TestWorktreeExists_NilGitOps(t *testing.T) {
	t.Parallel()
	exists := WorktreeExists("TASK-001", nil)
	if exists {
		t.Error("expected false when gitOps is nil")
	}
}

func TestWorktreeExists_ReturnsTrueWhenExists(t *testing.T) {
	t.Parallel()
	tmpDir, err := os.MkdirTemp("", "orc-worktree-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	if err := initTestRepo(tmpDir); err != nil {
		t.Fatalf("failed to init test repo: %v", err)
	}

	gitCfg := git.DefaultConfig()
	gitCfg.WorktreeDir = filepath.Join(tmpDir, ".orc", "worktrees")
	gitOps, err := git.New(tmpDir, gitCfg)
	if err != nil {
		t.Fatalf("failed to create git ops: %v", err)
	}

	// Before creation
	if WorktreeExists("TASK-005", gitOps) {
		t.Error("worktree should not exist before creation")
	}

	// Create worktree
	tsk := task.NewProtoTask("TASK-005", "Test task")
	if _, err := SetupWorktreeForTask(tsk, nil, gitOps, nil); err != nil {
		t.Fatalf("SetupWorktreeForTask failed: %v", err)
	}

	// After creation
	if !WorktreeExists("TASK-005", gitOps) {
		t.Error("worktree should exist after creation")
	}
}

func TestShouldCleanupWorktree_NilConfig(t *testing.T) {
	t.Parallel()
	// Default: cleanup on completion, not on failure
	if !ShouldCleanupWorktree(true, false, nil) {
		t.Error("should cleanup when completed with nil config")
	}
	if ShouldCleanupWorktree(false, true, nil) {
		t.Error("should not cleanup when failed with nil config")
	}
	if ShouldCleanupWorktree(false, false, nil) {
		t.Error("should not cleanup when neither completed nor failed")
	}
}

func TestShouldCleanupWorktree_ConfiguredBehavior(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name              string
		completed         bool
		failed            bool
		cleanupOnComplete bool
		cleanupOnFail     bool
		want              bool
	}{
		{"completed+cleanup", true, false, true, false, true},
		{"completed+no-cleanup", true, false, false, false, false},
		{"failed+cleanup", false, true, false, true, true},
		{"failed+no-cleanup", false, true, false, false, false},
		{"both+cleanup-complete", true, true, true, false, true},
		{"both+cleanup-fail", true, true, false, true, true},
		{"neither", false, false, true, true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.Default()
			cfg.Worktree.CleanupOnComplete = tt.cleanupOnComplete
			cfg.Worktree.CleanupOnFail = tt.cleanupOnFail

			got := ShouldCleanupWorktree(tt.completed, tt.failed, cfg)
			if got != tt.want {
				t.Errorf("ShouldCleanupWorktree() = %v, want %v", got, tt.want)
			}
		})
	}
}

// initTestRepo initializes a minimal git repo for testing.
func initTestRepo(dir string) error {
	// git init with explicit 'main' branch to ensure consistent branch name
	if err := runGitCmd(dir, "init", "--initial-branch=main"); err != nil {
		return err
	}

	// Configure git user for commits
	if err := runGitCmd(dir, "config", "user.email", "test@example.com"); err != nil {
		return err
	}
	if err := runGitCmd(dir, "config", "user.name", "Test"); err != nil {
		return err
	}

	// Create initial commit so we have a main branch
	readmePath := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readmePath, []byte("# Test\n"), 0644); err != nil {
		return err
	}
	if err := runGitCmd(dir, "add", "."); err != nil {
		return err
	}
	if err := runGitCmd(dir, "commit", "-m", "Initial commit"); err != nil {
		return err
	}

	return nil
}

// runGitCmd runs a git command in the specified directory.
func runGitCmd(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// containsTaskID checks if the path contains the task ID.
func containsTaskID(path, taskID string) bool {
	return filepath.Base(path) != "" && (filepath.Base(path) == "orc-"+taskID ||
		filepath.Base(filepath.Dir(path)) == "worktrees")
}

// TestSetupWorktreeForTask_StaleWorktree tests that SetupWorktreeForTask handles
// the case where a worktree directory was deleted but git still has metadata about it.
// This simulates what happens when a task times out, the worktree is manually deleted,
// and then the user runs `orc resume`.
func TestSetupWorktreeForTask_StaleWorktree(t *testing.T) {
	t.Parallel()
	tmpDir, err := os.MkdirTemp("", "orc-worktree-task-stale-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	if err := initTestRepo(tmpDir); err != nil {
		t.Fatalf("failed to init test repo: %v", err)
	}

	gitCfg := git.DefaultConfig()
	gitCfg.WorktreeDir = filepath.Join(tmpDir, ".orc", "worktrees")
	gitOps, err := git.New(tmpDir, gitCfg)
	if err != nil {
		t.Fatalf("failed to create git ops: %v", err)
	}

	tsk := task.NewProtoTask("TASK-STALE2", "Test task")

	// Create worktree first time
	result1, err := SetupWorktreeForTask(tsk, nil, gitOps, nil)
	if err != nil {
		t.Fatalf("first SetupWorktreeForTask failed: %v", err)
	}

	// Manually delete the worktree directory (simulating stale registration).
	// This is what happens when a timeout causes cleanup or when someone
	// manually removes the directory without using `git worktree remove`.
	if err := os.RemoveAll(result1.Path); err != nil {
		t.Fatalf("failed to remove worktree directory: %v", err)
	}

	// Verify directory is gone
	if _, err := os.Stat(result1.Path); !os.IsNotExist(err) {
		t.Fatal("worktree directory should not exist after manual deletion")
	}

	// Setup again - this MUST succeed, not fail with
	// "fatal: 'orc/TASK-STALE2' is already used by worktree"
	result2, err := SetupWorktreeForTask(tsk, nil, gitOps, nil)
	if err != nil {
		t.Fatalf("SetupWorktreeForTask should handle stale worktree: %v", err)
	}

	// Verify it was created (not reused since directory was deleted)
	if result2.Reused {
		t.Error("should not be marked as reused when directory was deleted")
	}

	// Verify directory exists
	if _, err := os.Stat(result2.Path); os.IsNotExist(err) {
		t.Error("worktree should exist after re-creation")
	}
}

func TestSetupWorktreeForTask_CleansDirtyWorktree(t *testing.T) {
	t.Parallel()
	tmpDir, err := os.MkdirTemp("", "orc-worktree-dirty-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	if err := initTestRepo(tmpDir); err != nil {
		t.Fatalf("failed to init test repo: %v", err)
	}

	gitCfg := git.DefaultConfig()
	gitCfg.WorktreeDir = filepath.Join(tmpDir, ".orc", "worktrees")
	gitOps, err := git.New(tmpDir, gitCfg)
	if err != nil {
		t.Fatalf("failed to create git ops: %v", err)
	}

	// Create worktree
	tsk := task.NewProtoTask("TASK-DIRTY", "Test task")
	result1, err := SetupWorktreeForTask(tsk, nil, gitOps, nil)
	if err != nil {
		t.Fatalf("SetupWorktreeForTask failed: %v", err)
	}

	// Make the worktree dirty by creating an untracked file
	dirtyFile := filepath.Join(result1.Path, "dirty_file.txt")
	if err := os.WriteFile(dirtyFile, []byte("dirty content"), 0644); err != nil {
		t.Fatalf("failed to create dirty file: %v", err)
	}

	// Also stage a modified file to test staged changes
	readmePath := filepath.Join(result1.Path, "README.md")
	if err := os.WriteFile(readmePath, []byte("modified content\n"), 0644); err != nil {
		t.Fatalf("failed to modify README: %v", err)
	}
	if err := runGitCmd(result1.Path, "add", "README.md"); err != nil {
		t.Fatalf("failed to stage changes: %v", err)
	}

	// Verify worktree is dirty
	worktreeGit := gitOps.InWorktree(result1.Path)
	clean, _ := worktreeGit.IsClean()
	if clean {
		t.Fatal("worktree should be dirty before reuse")
	}

	// Reuse worktree - should clean up dirty state
	result2, err := SetupWorktreeForTask(tsk, nil, gitOps, nil)
	if err != nil {
		t.Fatalf("SetupWorktreeForTask failed on reuse: %v", err)
	}

	if !result2.Reused {
		t.Error("should be marked as reused")
	}

	// Verify worktree is now clean
	clean, err = worktreeGit.IsClean()
	if err != nil {
		t.Fatalf("failed to check clean status: %v", err)
	}
	if !clean {
		t.Error("worktree should be clean after reuse")
	}

	// Verify dirty file was rescued (committed, not discarded)
	if _, err := os.Stat(dirtyFile); err != nil {
		t.Error("dirty file should still exist after rescue commit")
	}

	// Verify a rescue commit was created
	out, err := exec.Command("git", "-C", result2.Path, "log", "--oneline", "-1").Output()
	if err != nil {
		t.Fatalf("failed to get latest commit: %v", err)
	}
	if !strings.Contains(string(out), "Rescue uncommitted changes") {
		t.Errorf("expected rescue commit, got: %s", strings.TrimSpace(string(out)))
	}
}

func TestSetupWorktreeForTask_AbortsRebaseInProgress(t *testing.T) {
	t.Parallel()
	tmpDir, err := os.MkdirTemp("", "orc-worktree-rebase-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	if err := initTestRepo(tmpDir); err != nil {
		t.Fatalf("failed to init test repo: %v", err)
	}

	gitCfg := git.DefaultConfig()
	gitCfg.WorktreeDir = filepath.Join(tmpDir, ".orc", "worktrees")
	gitOps, err := git.New(tmpDir, gitCfg)
	if err != nil {
		t.Fatalf("failed to create git ops: %v", err)
	}

	// Create worktree
	tsk := task.NewProtoTask("TASK-REBASE", "Test task")
	result1, err := SetupWorktreeForTask(tsk, nil, gitOps, nil)
	if err != nil {
		t.Fatalf("SetupWorktreeForTask failed: %v", err)
	}

	// Create a conflicting scenario to trigger rebase conflict
	// 1. Create a commit in main that modifies README.md
	readmePath := filepath.Join(tmpDir, "README.md")
	if err := os.WriteFile(readmePath, []byte("main branch change\n"), 0644); err != nil {
		t.Fatalf("failed to modify README in main: %v", err)
	}
	if err := runGitCmd(tmpDir, "add", "README.md"); err != nil {
		t.Fatalf("failed to stage in main: %v", err)
	}
	if err := runGitCmd(tmpDir, "commit", "-m", "main branch commit"); err != nil {
		t.Fatalf("failed to commit in main: %v", err)
	}

	// 2. In worktree, make a conflicting change
	wtReadmePath := filepath.Join(result1.Path, "README.md")
	if err := os.WriteFile(wtReadmePath, []byte("worktree branch change\n"), 0644); err != nil {
		t.Fatalf("failed to modify README in worktree: %v", err)
	}
	if err := runGitCmd(result1.Path, "add", "README.md"); err != nil {
		t.Fatalf("failed to stage in worktree: %v", err)
	}
	if err := runGitCmd(result1.Path, "commit", "-m", "worktree commit"); err != nil {
		t.Fatalf("failed to commit in worktree: %v", err)
	}

	// 3. Start a rebase that will conflict
	cmd := exec.Command("git", "rebase", "main")
	cmd.Dir = result1.Path
	// We expect this to fail due to conflict
	_ = cmd.Run()

	// Verify rebase is in progress
	worktreeGit := gitOps.InWorktree(result1.Path)
	rebaseInProgress, _ := worktreeGit.IsRebaseInProgress()
	if !rebaseInProgress {
		t.Skip("could not create rebase-in-progress state, skipping test")
	}

	// Reuse worktree - should abort the rebase
	result2, err := SetupWorktreeForTask(tsk, nil, gitOps, nil)
	if err != nil {
		t.Fatalf("SetupWorktreeForTask failed on reuse: %v", err)
	}

	if !result2.Reused {
		t.Error("should be marked as reused")
	}

	// Verify rebase is no longer in progress
	rebaseInProgress, err = worktreeGit.IsRebaseInProgress()
	if err != nil {
		t.Fatalf("failed to check rebase status: %v", err)
	}
	if rebaseInProgress {
		t.Error("rebase should be aborted after reuse")
	}
}

func TestSetupWorktreeForTask_AbortsMergeInProgress(t *testing.T) {
	t.Parallel()
	tmpDir, err := os.MkdirTemp("", "orc-worktree-merge-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	if err := initTestRepo(tmpDir); err != nil {
		t.Fatalf("failed to init test repo: %v", err)
	}

	gitCfg := git.DefaultConfig()
	gitCfg.WorktreeDir = filepath.Join(tmpDir, ".orc", "worktrees")
	gitOps, err := git.New(tmpDir, gitCfg)
	if err != nil {
		t.Fatalf("failed to create git ops: %v", err)
	}

	// Create worktree
	tsk := task.NewProtoTask("TASK-MERGE", "Test task")
	result1, err := SetupWorktreeForTask(tsk, nil, gitOps, nil)
	if err != nil {
		t.Fatalf("SetupWorktreeForTask failed: %v", err)
	}

	// Create a branch in main with conflicting changes
	if err := runGitCmd(tmpDir, "checkout", "-b", "conflict-branch"); err != nil {
		t.Fatalf("failed to create conflict branch: %v", err)
	}
	readmePath := filepath.Join(tmpDir, "README.md")
	if err := os.WriteFile(readmePath, []byte("conflict branch change\n"), 0644); err != nil {
		t.Fatalf("failed to modify README: %v", err)
	}
	if err := runGitCmd(tmpDir, "add", "README.md"); err != nil {
		t.Fatalf("failed to stage: %v", err)
	}
	if err := runGitCmd(tmpDir, "commit", "-m", "conflict branch commit"); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}
	if err := runGitCmd(tmpDir, "checkout", "main"); err != nil {
		t.Fatalf("failed to checkout main: %v", err)
	}

	// In worktree, make a conflicting change
	wtReadmePath := filepath.Join(result1.Path, "README.md")
	if err := os.WriteFile(wtReadmePath, []byte("worktree branch change\n"), 0644); err != nil {
		t.Fatalf("failed to modify README in worktree: %v", err)
	}
	if err := runGitCmd(result1.Path, "add", "README.md"); err != nil {
		t.Fatalf("failed to stage in worktree: %v", err)
	}
	if err := runGitCmd(result1.Path, "commit", "-m", "worktree commit"); err != nil {
		t.Fatalf("failed to commit in worktree: %v", err)
	}

	// Start a merge that will conflict
	cmd := exec.Command("git", "merge", "conflict-branch")
	cmd.Dir = result1.Path
	// We expect this to fail due to conflict
	_ = cmd.Run()

	// Verify merge is in progress
	worktreeGit := gitOps.InWorktree(result1.Path)
	mergeInProgress, _ := worktreeGit.IsMergeInProgress()
	if !mergeInProgress {
		t.Skip("could not create merge-in-progress state, skipping test")
	}

	// Reuse worktree - should abort the merge
	result2, err := SetupWorktreeForTask(tsk, nil, gitOps, nil)
	if err != nil {
		t.Fatalf("SetupWorktreeForTask failed on reuse: %v", err)
	}

	if !result2.Reused {
		t.Error("should be marked as reused")
	}

	// Verify merge is no longer in progress
	mergeInProgress, err = worktreeGit.IsMergeInProgress()
	if err != nil {
		t.Fatalf("failed to check merge status: %v", err)
	}
	if mergeInProgress {
		t.Error("merge should be aborted after reuse")
	}

	// Verify worktree is clean (conflicts resolved by discarding)
	clean, err := worktreeGit.IsClean()
	if err != nil {
		t.Fatalf("failed to check clean status: %v", err)
	}
	if !clean {
		t.Error("worktree should be clean after reuse")
	}
}
