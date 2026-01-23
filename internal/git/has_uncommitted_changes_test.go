package git

import (
	"os"
	"path/filepath"
	"testing"
)

// TestHasUncommittedChanges_CleanWorktree verifies that HasUncommittedChanges
// returns false when the worktree is clean (no staged or unstaged changes).
// Covers: SC-3 (auto-commit skipped when worktree is clean)
func TestHasUncommittedChanges_CleanWorktree(t *testing.T) {
	t.Parallel()
	tmpDir := setupTestRepo(t)
	g, err := New(tmpDir, DefaultConfig())
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	hasChanges, err := g.HasUncommittedChanges()
	if err != nil {
		t.Fatalf("HasUncommittedChanges() error: %v", err)
	}

	if hasChanges {
		t.Error("HasUncommittedChanges() = true, want false for clean worktree")
	}
}

// TestHasUncommittedChanges_UntrackedFile verifies that HasUncommittedChanges
// returns true when there are untracked files in the worktree.
// Covers: SC-1 (auto-commit detects uncommitted changes)
func TestHasUncommittedChanges_UntrackedFile(t *testing.T) {
	t.Parallel()
	tmpDir := setupTestRepo(t)
	g, err := New(tmpDir, DefaultConfig())
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	// Create untracked file
	untrackedFile := filepath.Join(tmpDir, "untracked.txt")
	if err := os.WriteFile(untrackedFile, []byte("new file"), 0644); err != nil {
		t.Fatalf("failed to create untracked file: %v", err)
	}

	hasChanges, err := g.HasUncommittedChanges()
	if err != nil {
		t.Fatalf("HasUncommittedChanges() error: %v", err)
	}

	if !hasChanges {
		t.Error("HasUncommittedChanges() = false, want true for untracked file")
	}
}

// TestHasUncommittedChanges_ModifiedFile verifies that HasUncommittedChanges
// returns true when there are modified but unstaged files.
// Covers: SC-1 (auto-commit detects uncommitted changes)
func TestHasUncommittedChanges_ModifiedFile(t *testing.T) {
	t.Parallel()
	tmpDir := setupTestRepo(t)
	g, err := New(tmpDir, DefaultConfig())
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	// Modify existing file
	readmeFile := filepath.Join(tmpDir, "README.md")
	if err := os.WriteFile(readmeFile, []byte("# Modified\n"), 0644); err != nil {
		t.Fatalf("failed to modify file: %v", err)
	}

	hasChanges, err := g.HasUncommittedChanges()
	if err != nil {
		t.Fatalf("HasUncommittedChanges() error: %v", err)
	}

	if !hasChanges {
		t.Error("HasUncommittedChanges() = false, want true for modified file")
	}
}

// TestHasUncommittedChanges_StagedChanges verifies that HasUncommittedChanges
// returns true when there are staged but uncommitted changes.
// Covers: SC-1 (auto-commit detects uncommitted changes)
func TestHasUncommittedChanges_StagedChanges(t *testing.T) {
	t.Parallel()
	tmpDir := setupTestRepo(t)
	g, err := New(tmpDir, DefaultConfig())
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	// Create and stage a new file
	newFile := filepath.Join(tmpDir, "staged.txt")
	if err := os.WriteFile(newFile, []byte("staged content"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	if _, err := g.ctx.RunGit("add", "staged.txt"); err != nil {
		t.Fatalf("failed to stage file: %v", err)
	}

	hasChanges, err := g.HasUncommittedChanges()
	if err != nil {
		t.Fatalf("HasUncommittedChanges() error: %v", err)
	}

	if !hasChanges {
		t.Error("HasUncommittedChanges() = false, want true for staged changes")
	}
}

// TestHasUncommittedChanges_MixedChanges verifies that HasUncommittedChanges
// returns true when there are both staged and unstaged changes.
// Tests edge case: mixed staged + unstaged changes
// Covers: SC-1 (auto-commit detects uncommitted changes)
func TestHasUncommittedChanges_MixedChanges(t *testing.T) {
	t.Parallel()
	tmpDir := setupTestRepo(t)
	g, err := New(tmpDir, DefaultConfig())
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	// Create and stage a file
	stagedFile := filepath.Join(tmpDir, "staged.txt")
	if err := os.WriteFile(stagedFile, []byte("staged"), 0644); err != nil {
		t.Fatalf("failed to create staged file: %v", err)
	}
	if _, err := g.ctx.RunGit("add", "staged.txt"); err != nil {
		t.Fatalf("failed to stage file: %v", err)
	}

	// Modify existing file without staging
	readmeFile := filepath.Join(tmpDir, "README.md")
	if err := os.WriteFile(readmeFile, []byte("# Unstaged\n"), 0644); err != nil {
		t.Fatalf("failed to modify file: %v", err)
	}

	hasChanges, err := g.HasUncommittedChanges()
	if err != nil {
		t.Fatalf("HasUncommittedChanges() error: %v", err)
	}

	if !hasChanges {
		t.Error("HasUncommittedChanges() = false, want true for mixed changes")
	}
}

// TestHasUncommittedChanges_DeletedFile verifies that HasUncommittedChanges
// returns true when a tracked file has been deleted but not committed.
// Covers: SC-1 (auto-commit detects uncommitted changes)
func TestHasUncommittedChanges_DeletedFile(t *testing.T) {
	t.Parallel()
	tmpDir := setupTestRepo(t)
	g, err := New(tmpDir, DefaultConfig())
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	// Delete the README.md file
	readmeFile := filepath.Join(tmpDir, "README.md")
	if err := os.Remove(readmeFile); err != nil {
		t.Fatalf("failed to delete file: %v", err)
	}

	hasChanges, err := g.HasUncommittedChanges()
	if err != nil {
		t.Fatalf("HasUncommittedChanges() error: %v", err)
	}

	if !hasChanges {
		t.Error("HasUncommittedChanges() = false, want true for deleted file")
	}
}
