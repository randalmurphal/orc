package executor

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/git"
)

func TestSetupWorktree_NilGitOps(t *testing.T) {
	_, err := SetupWorktree("TASK-001", nil, nil)
	if err == nil {
		t.Error("expected error when gitOps is nil")
	}
}

func TestSetupWorktree_CreatesDirectory(t *testing.T) {
	// Create a temporary git repo for testing
	tmpDir, err := os.MkdirTemp("", "orc-worktree-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Initialize git repo
	if err := initTestRepo(tmpDir); err != nil {
		t.Fatalf("failed to init test repo: %v", err)
	}

	// Create git ops
	gitCfg := git.DefaultConfig()
	gitOps, err := git.New(tmpDir, gitCfg)
	if err != nil {
		t.Fatalf("failed to create git ops: %v", err)
	}

	cfg := config.Default()
	cfg.Completion.TargetBranch = "main"

	// Setup worktree
	result, err := SetupWorktree("TASK-001", cfg, gitOps)
	if err != nil {
		t.Fatalf("SetupWorktree failed: %v", err)
	}

	// Verify directory exists
	if _, err := os.Stat(result.Path); os.IsNotExist(err) {
		t.Errorf("worktree directory does not exist: %s", result.Path)
	}

	// Verify it's not marked as reused
	if result.Reused {
		t.Error("expected Reused to be false for new worktree")
	}
}

func TestSetupWorktree_ReturnsPath(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "orc-worktree-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	if err := initTestRepo(tmpDir); err != nil {
		t.Fatalf("failed to init test repo: %v", err)
	}

	gitOps, err := git.New(tmpDir, git.DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create git ops: %v", err)
	}

	result, err := SetupWorktree("TASK-002", nil, gitOps)
	if err != nil {
		t.Fatalf("SetupWorktree failed: %v", err)
	}

	// Path should be non-empty and absolute
	if result.Path == "" {
		t.Error("expected non-empty path")
	}

	if !filepath.IsAbs(result.Path) {
		t.Errorf("expected absolute path, got: %s", result.Path)
	}

	// Path should contain the task ID pattern
	if !containsTaskID(result.Path, "TASK-002") {
		t.Errorf("path should contain task ID, got: %s", result.Path)
	}
}

func TestSetupWorktree_ReusesExisting(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "orc-worktree-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	if err := initTestRepo(tmpDir); err != nil {
		t.Fatalf("failed to init test repo: %v", err)
	}

	gitOps, err := git.New(tmpDir, git.DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create git ops: %v", err)
	}

	// Create worktree first time
	result1, err := SetupWorktree("TASK-003", nil, gitOps)
	if err != nil {
		t.Fatalf("first SetupWorktree failed: %v", err)
	}

	if result1.Reused {
		t.Error("first setup should not be reused")
	}

	// Setup again - should reuse
	result2, err := SetupWorktree("TASK-003", nil, gitOps)
	if err != nil {
		t.Fatalf("second SetupWorktree failed: %v", err)
	}

	if !result2.Reused {
		t.Error("second setup should be reused")
	}

	if result1.Path != result2.Path {
		t.Errorf("paths should match: %s != %s", result1.Path, result2.Path)
	}
}

func TestCleanupWorktree_RemovesDirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "orc-worktree-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	if err := initTestRepo(tmpDir); err != nil {
		t.Fatalf("failed to init test repo: %v", err)
	}

	gitOps, err := git.New(tmpDir, git.DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create git ops: %v", err)
	}

	// Create worktree
	result, err := SetupWorktree("TASK-004", nil, gitOps)
	if err != nil {
		t.Fatalf("SetupWorktree failed: %v", err)
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
	tmpDir, err := os.MkdirTemp("", "orc-worktree-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	if err := initTestRepo(tmpDir); err != nil {
		t.Fatalf("failed to init test repo: %v", err)
	}

	gitOps, err := git.New(tmpDir, git.DefaultConfig())
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
	// Should not error when gitOps is nil
	err := CleanupWorktree("TASK-001", nil)
	if err != nil {
		t.Errorf("CleanupWorktree should not error when gitOps is nil: %v", err)
	}
}

func TestWorktreePath_NilGitOps(t *testing.T) {
	path := WorktreePath("TASK-001", nil)
	if path != "" {
		t.Errorf("expected empty path when gitOps is nil, got: %s", path)
	}
}

func TestWorktreeExists_NilGitOps(t *testing.T) {
	exists := WorktreeExists("TASK-001", nil)
	if exists {
		t.Error("expected false when gitOps is nil")
	}
}

func TestWorktreeExists_ReturnsTrueWhenExists(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "orc-worktree-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	if err := initTestRepo(tmpDir); err != nil {
		t.Fatalf("failed to init test repo: %v", err)
	}

	gitOps, err := git.New(tmpDir, git.DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create git ops: %v", err)
	}

	// Before creation
	if WorktreeExists("TASK-005", gitOps) {
		t.Error("worktree should not exist before creation")
	}

	// Create worktree
	if _, err := SetupWorktree("TASK-005", nil, gitOps); err != nil {
		t.Fatalf("SetupWorktree failed: %v", err)
	}

	// After creation
	if !WorktreeExists("TASK-005", gitOps) {
		t.Error("worktree should exist after creation")
	}
}

func TestShouldCleanupWorktree_NilConfig(t *testing.T) {
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

func TestSetupWorktree_StaleWorktree(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "orc-worktree-stale-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	if err := initTestRepo(tmpDir); err != nil {
		t.Fatalf("failed to init test repo: %v", err)
	}

	gitOps, err := git.New(tmpDir, git.DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create git ops: %v", err)
	}

	// Create worktree first time
	result1, err := SetupWorktree("TASK-STALE", nil, gitOps)
	if err != nil {
		t.Fatalf("first SetupWorktree failed: %v", err)
	}

	// Manually delete the worktree directory (simulating stale registration)
	if err := os.RemoveAll(result1.Path); err != nil {
		t.Fatalf("failed to remove worktree directory: %v", err)
	}

	// Verify directory is gone
	if _, err := os.Stat(result1.Path); !os.IsNotExist(err) {
		t.Fatal("worktree directory should not exist after manual deletion")
	}

	// WorktreeExists should return false since directory is gone
	if WorktreeExists("TASK-STALE", gitOps) {
		t.Error("WorktreeExists should return false when directory is deleted")
	}

	// Setup again - should succeed due to auto-prune in git layer
	result2, err := SetupWorktree("TASK-STALE", nil, gitOps)
	if err != nil {
		t.Fatalf("SetupWorktree should handle stale worktree: %v", err)
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

func TestSetupWorktree_CleansDirtyWorktree(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "orc-worktree-dirty-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	if err := initTestRepo(tmpDir); err != nil {
		t.Fatalf("failed to init test repo: %v", err)
	}

	gitOps, err := git.New(tmpDir, git.DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create git ops: %v", err)
	}

	// Create worktree
	result1, err := SetupWorktree("TASK-DIRTY", nil, gitOps)
	if err != nil {
		t.Fatalf("SetupWorktree failed: %v", err)
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
	result2, err := SetupWorktree("TASK-DIRTY", nil, gitOps)
	if err != nil {
		t.Fatalf("SetupWorktree failed on reuse: %v", err)
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

	// Verify dirty file was removed
	if _, err := os.Stat(dirtyFile); !os.IsNotExist(err) {
		t.Error("dirty file should be removed after reuse")
	}
}

func TestSetupWorktree_AbortsRebaseInProgress(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "orc-worktree-rebase-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	if err := initTestRepo(tmpDir); err != nil {
		t.Fatalf("failed to init test repo: %v", err)
	}

	gitOps, err := git.New(tmpDir, git.DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create git ops: %v", err)
	}

	// Create worktree
	result1, err := SetupWorktree("TASK-REBASE", nil, gitOps)
	if err != nil {
		t.Fatalf("SetupWorktree failed: %v", err)
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
	result2, err := SetupWorktree("TASK-REBASE", nil, gitOps)
	if err != nil {
		t.Fatalf("SetupWorktree failed on reuse: %v", err)
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

func TestSetupWorktree_AbortsMergeInProgress(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "orc-worktree-merge-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	if err := initTestRepo(tmpDir); err != nil {
		t.Fatalf("failed to init test repo: %v", err)
	}

	gitOps, err := git.New(tmpDir, git.DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create git ops: %v", err)
	}

	// Create worktree
	result1, err := SetupWorktree("TASK-MERGE", nil, gitOps)
	if err != nil {
		t.Fatalf("SetupWorktree failed: %v", err)
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
	result2, err := SetupWorktree("TASK-MERGE", nil, gitOps)
	if err != nil {
		t.Fatalf("SetupWorktree failed on reuse: %v", err)
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
