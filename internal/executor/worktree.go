// Package executor provides the flowgraph-based execution engine for orc.
package executor

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/git"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// WorktreeSetup contains the result of setting up a worktree.
type WorktreeSetup struct {
	// Path is the absolute path to the worktree directory.
	Path string
	// Reused indicates if an existing worktree was reused rather than created.
	Reused bool
	// TargetBranch is the resolved target branch for this task's PR.
	TargetBranch string
}

// SetupWorktreeForTask creates or reuses an isolated worktree for the given task,
// using the full 5-level branch resolution hierarchy:
//
//  1. Task.TargetBranch (explicit override)
//  2. Initiative.BranchBase (inherited from initiative)
//  3. Developer.StagingBranch (personal staging area)
//  4. Config.Completion.TargetBranch (project default)
//  5. "main" (hardcoded fallback)
//
// If the resolved target branch doesn't exist locally and is not a default branch
// (main/master/develop), it will be auto-created from the configured base branch.
//
// When the task belongs to an initiative with a BranchPrefix, the task branch will
// use that prefix instead of the default "orc/" prefix. For example, an initiative
// with BranchPrefix "feature/auth-" will create branches like "feature/auth-TASK-001".
//
// This is the preferred function for task execution as it supports initiative-level
// and developer staging branches.
func SetupWorktreeForTask(t *task.Task, cfg *config.Config, gitOps *git.Git, backend storage.Backend) (*WorktreeSetup, error) {
	if gitOps == nil {
		return nil, fmt.Errorf("git operations not available")
	}
	if t == nil {
		return nil, fmt.Errorf("task is required")
	}

	// Resolve target branch using 5-level hierarchy
	targetBranch := ResolveTargetBranchForTask(t, backend, cfg)

	// Get initiative prefix if task belongs to an initiative
	var initiativePrefix string
	if t.InitiativeID != "" && backend != nil {
		init, err := backend.LoadInitiative(t.InitiativeID)
		if err == nil && init != nil {
			initiativePrefix = init.BranchPrefix
		}
		// Ignore errors - just use default prefix if initiative can't be loaded
	}

	// For non-default branches (initiative/staging), ensure they exist
	// Default branches (main, master, develop) should already exist
	if !IsDefaultBranch(targetBranch) {
		// Determine base branch for creating new branches
		baseBranch := "main"
		if cfg != nil && cfg.Completion.TargetBranch != "" {
			baseBranch = cfg.Completion.TargetBranch
		}

		// Auto-create the branch if it doesn't exist
		if err := gitOps.EnsureBranchExists(targetBranch, baseBranch); err != nil {
			return nil, fmt.Errorf("ensure target branch %s exists: %w", targetBranch, err)
		}
	}

	// Calculate expected branch name for this task
	expectedBranch := gitOps.BranchNameWithInitiativePrefix(t.ID, initiativePrefix)

	// Check if worktree already exists
	// Use initiative prefix for consistent path resolution
	worktreePath := gitOps.WorktreePathWithInitiativePrefix(t.ID, initiativePrefix)
	if _, err := os.Stat(worktreePath); err == nil {
		// Worktree exists - clean up any problematic state before reusing
		// CRITICAL: Also verifies worktree is on the correct branch
		if err := cleanWorktreeState(worktreePath, gitOps, expectedBranch); err != nil {
			return nil, fmt.Errorf("clean worktree state for %s: %w", t.ID, err)
		}
		return &WorktreeSetup{
			Path:         worktreePath,
			Reused:       true,
			TargetBranch: targetBranch,
		}, nil
	}

	// Create new worktree with initiative prefix
	path, err := gitOps.CreateWorktreeWithInitiativePrefix(t.ID, targetBranch, initiativePrefix)
	if err != nil {
		return nil, fmt.Errorf("create worktree for %s: %w", t.ID, err)
	}

	// SAFETY: Validate branch after creation
	// This catches any issues with worktree creation leaving us on the wrong branch
	worktreeGit := gitOps.InWorktree(path)
	currentBranch, err := worktreeGit.GetCurrentBranch()
	if err != nil {
		return nil, fmt.Errorf("verify worktree branch for %s: %w", t.ID, err)
	}
	if currentBranch != expectedBranch {
		return nil, fmt.Errorf("INTERNAL BUG: worktree created on wrong branch: expected %s, got %s - this indicates a bug in CreateWorktreeWithInitiativePrefix",
			expectedBranch, currentBranch)
	}

	return &WorktreeSetup{
		Path:         path,
		Reused:       false,
		TargetBranch: targetBranch,
	}, nil
}

// SetupWorktree creates or reuses an isolated worktree for the given task.
// It returns the worktree path and whether it was reused.
//
// When reusing an existing worktree, the function checks for and cleans up
// any problematic git state that might block execution (rebase/merge in progress,
// uncommitted changes, conflicts). This ensures that resumed tasks start with
// a clean worktree state.
//
// If cfg is nil, default worktree configuration is used.
// If gitOps is nil, returns an error.
//
// Deprecated: Use SetupWorktreeForTask for task execution as it supports
// the full 5-level branch resolution hierarchy including initiative and
// developer staging branches.
func SetupWorktree(taskID string, cfg *config.Config, gitOps *git.Git) (*WorktreeSetup, error) {
	if gitOps == nil {
		return nil, fmt.Errorf("git operations not available")
	}

	// Determine target branch from config
	targetBranch := "main"
	if cfg != nil && cfg.Completion.TargetBranch != "" {
		targetBranch = cfg.Completion.TargetBranch
	}

	// Calculate expected branch name for this task (no initiative prefix in deprecated path)
	expectedBranch := gitOps.BranchName(taskID)

	// Check if worktree already exists
	worktreePath := gitOps.WorktreePath(taskID)
	if _, err := os.Stat(worktreePath); err == nil {
		// Worktree exists - clean up any problematic state before reusing
		// This handles cases where a previous execution failed and left the
		// worktree in a bad state (e.g., rebase in progress, conflicts, dirty)
		// CRITICAL: Also verifies worktree is on the correct branch
		if err := cleanWorktreeState(worktreePath, gitOps, expectedBranch); err != nil {
			return nil, fmt.Errorf("clean worktree state for %s: %w", taskID, err)
		}
		return &WorktreeSetup{
			Path:         worktreePath,
			Reused:       true,
			TargetBranch: targetBranch,
		}, nil
	}

	// Create new worktree
	path, err := gitOps.CreateWorktree(taskID, targetBranch)
	if err != nil {
		return nil, fmt.Errorf("create worktree for %s: %w", taskID, err)
	}

	// SAFETY: Validate branch after creation
	// This catches any issues with worktree creation leaving us on the wrong branch
	worktreeGit := gitOps.InWorktree(path)
	currentBranch, err := worktreeGit.GetCurrentBranch()
	if err != nil {
		return nil, fmt.Errorf("verify worktree branch for %s: %w", taskID, err)
	}
	if currentBranch != expectedBranch {
		return nil, fmt.Errorf("INTERNAL BUG: worktree created on wrong branch: expected %s, got %s - this indicates a bug in CreateWorktree",
			expectedBranch, currentBranch)
	}

	return &WorktreeSetup{
		Path:         path,
		Reused:       false,
		TargetBranch: targetBranch,
	}, nil
}

// cleanWorktreeState checks for and cleans up any problematic git state
// in the worktree that might block execution.
//
// This function handles:
// - Rebase in progress: aborts the rebase
// - Merge in progress: aborts the merge
// - Uncommitted changes/conflicts: discards all changes
// - Wrong branch: switches to the expected branch
//
// This ensures that resumed tasks start with a clean worktree state,
// preventing errors like "rebase already in progress" or "you have
// unstaged changes" when syncing with the target branch.
//
// CRITICAL: The expectedBranch parameter ensures the worktree is on the correct
// task branch. If the worktree ended up on a different branch (e.g., main),
// this function will switch it back to prevent issues like:
// - Review phase seeing no changes (diffing main against main)
// - Commits going to the wrong branch
// - Infinite review/implement loops
func cleanWorktreeState(worktreePath string, gitOps *git.Git, expectedBranch string) error {
	worktreeGit := gitOps.InWorktree(worktreePath)

	// Check and abort any in-progress rebase
	rebaseInProgress, err := worktreeGit.IsRebaseInProgress()
	if err != nil {
		// Non-fatal: continue checking other states
	} else if rebaseInProgress {
		if err := worktreeGit.AbortRebase(); err != nil {
			return fmt.Errorf("abort rebase: %w", err)
		}
	}

	// Check and abort any in-progress merge
	mergeInProgress, err := worktreeGit.IsMergeInProgress()
	if err != nil {
		// Non-fatal: continue checking other states
	} else if mergeInProgress {
		if err := worktreeGit.AbortMerge(); err != nil {
			return fmt.Errorf("abort merge: %w", err)
		}
	}

	// Check if working directory has uncommitted changes or conflicts
	// and discard them to ensure clean state for execution.
	// If IsClean fails, we still try DiscardChanges as a fallback.
	clean, err := worktreeGit.IsClean()
	if err != nil || !clean {
		if discardErr := worktreeGit.DiscardChanges(); discardErr != nil {
			return fmt.Errorf("discard changes: %w", discardErr)
		}
	}

	// CRITICAL: Verify worktree is on the expected branch
	// This catches cases where a worktree ended up on the wrong branch
	// (e.g., due to manual checkout or previous execution bugs)
	currentBranch, err := worktreeGit.GetCurrentBranch()
	if err != nil {
		return fmt.Errorf("get current branch: %w", err)
	}

	if currentBranch != expectedBranch {
		slog.Warn("worktree on wrong branch, switching",
			"worktree", worktreePath,
			"current", currentBranch,
			"expected", expectedBranch,
		)
		// Use CheckoutSafe which has proper worktree context protection
		if err := worktreeGit.CheckoutSafe(expectedBranch); err != nil {
			return fmt.Errorf("checkout expected branch %s (was on %s): %w",
				expectedBranch, currentBranch, err)
		}
	}

	return nil
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
