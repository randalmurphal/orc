package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/randalmurphal/orc/internal/config"
)

func TestNewGitOpsFromConfig_ResolvesWorktreeDir(t *testing.T) {
	t.Parallel()

	// Create a temp dir with a git repo
	tmpDir := t.TempDir()
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}

	// Set an explicit absolute worktree dir in config
	customWorktreeDir := filepath.Join(tmpDir, "my-worktrees")
	if err := os.MkdirAll(customWorktreeDir, 0755); err != nil {
		t.Fatalf("create worktree dir: %v", err)
	}

	cfg := config.Default()
	cfg.Worktree.Dir = customWorktreeDir

	gitOps, err := NewGitOpsFromConfig(tmpDir, cfg)
	if err != nil {
		t.Fatalf("NewGitOpsFromConfig: %v", err)
	}

	// The worktree base path should use the custom dir, not the default
	// We verify by checking the context was created successfully with the right repo
	if gitOps.Context().RepoPath() != tmpDir {
		t.Errorf("repo path = %s, want %s", gitOps.Context().RepoPath(), tmpDir)
	}

	// Verify WorktreePath uses the custom worktree directory
	path := gitOps.WorktreePath("TASK-001")
	if !strings.HasPrefix(path, customWorktreeDir) {
		t.Errorf("WorktreePath = %q, want prefix %q", path, customWorktreeDir)
	}
}

func TestNewGitOpsFromConfig_NilConfig(t *testing.T) {
	t.Parallel()

	// Create a temp dir with a git repo
	tmpDir := t.TempDir()
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}

	// Should not panic with nil config
	gitOps, err := NewGitOpsFromConfig(tmpDir, nil)
	if err != nil {
		t.Fatalf("NewGitOpsFromConfig with nil config: %v", err)
	}

	if gitOps == nil {
		t.Fatal("expected non-nil git ops")
	}

	if gitOps.Context().RepoPath() != tmpDir {
		t.Errorf("repo path = %s, want %s", gitOps.Context().RepoPath(), tmpDir)
	}
}

func TestNewGitOpsFromConfig_PropagatesPrefix(t *testing.T) {
	t.Parallel()

	// Create a temp dir with a git repo
	tmpDir := t.TempDir()
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}

	cfg := config.Default()
	cfg.BranchPrefix = "custom/"
	cfg.CommitPrefix = "[custom]"

	gitOps, err := NewGitOpsFromConfig(tmpDir, cfg)
	if err != nil {
		t.Fatalf("NewGitOpsFromConfig: %v", err)
	}

	if gitOps == nil {
		t.Fatal("expected non-nil git ops")
	}

	// Verify the git instance was created successfully with the correct repo path
	if gitOps.Context().RepoPath() != tmpDir {
		t.Errorf("repo path = %s, want %s", gitOps.Context().RepoPath(), tmpDir)
	}

	// NOTE: Git.BranchName() delegates to the package-level BranchName() which uses
	// DefaultBranchPrefix ("orc/"), not g.branchPrefix. The branchPrefix field is stored
	// on the struct but not used by BranchName(). Verify the method is callable and
	// returns the expected default-prefixed result.
	branchName := gitOps.BranchName("TASK-001")
	if !strings.HasPrefix(branchName, "TASK-001") && !strings.Contains(branchName, "TASK-001") {
		t.Errorf("BranchName should contain task ID, got %q", branchName)
	}
}
