// Package executor provides the flowgraph-based execution engine for orc.
package executor

import (
	"fmt"
	"os"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/git"
)

// WorktreeSetup contains the result of setting up a worktree.
type WorktreeSetup struct {
	// Path is the absolute path to the worktree directory.
	Path string
	// Reused indicates if an existing worktree was reused rather than created.
	Reused bool
}

// SetupWorktree creates or reuses an isolated worktree for the given task.
// It returns the worktree path and whether it was reused.
//
// If cfg is nil, default worktree configuration is used.
// If gitOps is nil, returns an error.
func SetupWorktree(taskID string, cfg *config.Config, gitOps *git.Git) (*WorktreeSetup, error) {
	if gitOps == nil {
		return nil, fmt.Errorf("git operations not available")
	}

	// Determine target branch from config
	targetBranch := "main"
	if cfg != nil && cfg.Completion.TargetBranch != "" {
		targetBranch = cfg.Completion.TargetBranch
	}

	// Check if worktree already exists
	worktreePath := gitOps.WorktreePath(taskID)
	if _, err := os.Stat(worktreePath); err == nil {
		// Worktree exists, reuse it
		return &WorktreeSetup{
			Path:   worktreePath,
			Reused: true,
		}, nil
	}

	// Create new worktree
	path, err := gitOps.CreateWorktree(taskID, targetBranch)
	if err != nil {
		return nil, fmt.Errorf("create worktree for %s: %w", taskID, err)
	}

	return &WorktreeSetup{
		Path:   path,
		Reused: false,
	}, nil
}

// CleanupWorktree removes the worktree for the given task.
// Returns nil if the worktree doesn't exist or was successfully removed.
func CleanupWorktree(taskID string, gitOps *git.Git) error {
	if gitOps == nil {
		return nil // No git ops, nothing to clean up
	}

	// Check if worktree exists before attempting cleanup
	worktreePath := gitOps.WorktreePath(taskID)
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		return nil // Already gone, nothing to do
	}

	if err := gitOps.CleanupWorktree(taskID); err != nil {
		return fmt.Errorf("cleanup worktree for %s: %w", taskID, err)
	}

	return nil
}

// WorktreePath returns the path where a task's worktree would be located.
// This is a convenience function that delegates to git.Git.WorktreePath.
func WorktreePath(taskID string, gitOps *git.Git) string {
	if gitOps == nil {
		return ""
	}
	return gitOps.WorktreePath(taskID)
}

// WorktreeExists checks if a worktree exists for the given task.
func WorktreeExists(taskID string, gitOps *git.Git) bool {
	if gitOps == nil {
		return false
	}
	path := gitOps.WorktreePath(taskID)
	_, err := os.Stat(path)
	return err == nil
}

// ShouldCleanupWorktree determines whether a worktree should be cleaned up
// based on the task status and configuration.
func ShouldCleanupWorktree(completed bool, failed bool, cfg *config.Config) bool {
	if cfg == nil {
		// Default behavior: cleanup on completion, keep on failure
		return completed
	}

	if completed && cfg.Worktree.CleanupOnComplete {
		return true
	}
	if failed && cfg.Worktree.CleanupOnFail {
		return true
	}
	return false
}
