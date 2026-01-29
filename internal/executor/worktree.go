// Package executor provides the flowgraph-based execution engine for orc.
package executor

import (
	"fmt"
	"log/slog"
	"os"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
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
func SetupWorktreeForTask(t *orcv1.Task, cfg *config.Config, gitOps *git.Git, backend storage.Backend) (*WorktreeSetup, error) {
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
	initiativeID := task.GetInitiativeIDProto(t)
	if initiativeID != "" && backend != nil {
		init, err := backend.LoadInitiative(initiativeID)
		if err != nil {
			slog.Warn("failed to load initiative for branch prefix, using default 'orc/' prefix",
				"task_id", t.Id,
				"initiative_id", initiativeID,
				"error", err,
			)
		} else if init != nil {
			initiativePrefix = init.BranchPrefix
		}
	}

	// For non-default branches (initiative/staging), ensure they exist
	if !IsDefaultBranch(targetBranch) {
		baseBranch := "main"
		if cfg != nil && cfg.Completion.TargetBranch != "" {
			baseBranch = cfg.Completion.TargetBranch
		}
		if err := gitOps.EnsureBranchExists(targetBranch, baseBranch); err != nil {
			return nil, fmt.Errorf("ensure target branch %s exists: %w", targetBranch, err)
		}
	}

	// Calculate expected branch name for this task
	expectedBranch := gitOps.BranchNameWithInitiativePrefix(t.Id, initiativePrefix)

	// Prune stale worktree entries
	if err := gitOps.PruneWorktrees(); err != nil {
		slog.Debug("failed to prune stale worktrees (non-fatal)",
			"task_id", t.Id,
			"error", err,
		)
	}

	// Check if worktree already exists
	worktreePath := gitOps.WorktreePathWithInitiativePrefix(t.Id, initiativePrefix)
	if info, err := os.Stat(worktreePath); err == nil {
		if !info.IsDir() {
			return nil, fmt.Errorf("worktree path exists but is not a directory: %s", worktreePath)
		}
		if err := cleanWorktreeState(worktreePath, gitOps, expectedBranch); err != nil {
			return nil, fmt.Errorf("clean worktree state for %s: %w", t.Id, err)
		}
		return &WorktreeSetup{
			Path:         worktreePath,
			Reused:       true,
			TargetBranch: targetBranch,
		}, nil
	}

	// Create new worktree with initiative prefix
	path, err := gitOps.CreateWorktreeWithInitiativePrefix(t.Id, targetBranch, initiativePrefix)
	if err != nil {
		return nil, fmt.Errorf("create worktree for %s: %w", t.Id, err)
	}

	// SAFETY: Validate branch after creation
	worktreeGit := gitOps.InWorktree(path)
	currentBranch, err := worktreeGit.GetCurrentBranch()
	if err != nil {
		return nil, fmt.Errorf("verify worktree branch for %s: %w", t.Id, err)
	}
	if currentBranch != expectedBranch {
		return nil, fmt.Errorf("INTERNAL BUG: worktree created on wrong branch: expected %s, got %s",
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
	slog.Debug("cleaning worktree state",
		"worktree", worktreePath,
		"expected_branch", expectedBranch,
	)

	worktreeGit := gitOps.InWorktree(worktreePath)

	// Check and abort any in-progress rebase
	rebaseInProgress, err := worktreeGit.IsRebaseInProgress()
	if err != nil {
		// Log but continue - check may fail due to unexpected git state
		slog.Warn("failed to check rebase status, assuming none in progress",
			"worktree", worktreePath,
			"error", err,
		)
	} else if rebaseInProgress {
		slog.Debug("aborting in-progress rebase", "worktree", worktreePath)
		if err := worktreeGit.AbortRebase(); err != nil {
			return fmt.Errorf("abort rebase: %w", err)
		}
	}

	// Check and abort any in-progress merge
	mergeInProgress, err := worktreeGit.IsMergeInProgress()
	if err != nil {
		// Log but continue - check may fail due to unexpected git state
		slog.Warn("failed to check merge status, assuming none in progress",
			"worktree", worktreePath,
			"error", err,
		)
	} else if mergeInProgress {
		slog.Debug("aborting in-progress merge", "worktree", worktreePath)
		if err := worktreeGit.AbortMerge(); err != nil {
			return fmt.Errorf("abort merge: %w", err)
		}
	}

	// Check if working directory has uncommitted changes.
	// On resume after a crash, Claude may have written files that weren't committed.
	// We preserve these by committing them as a rescue commit instead of discarding.
	clean, isCleanErr := worktreeGit.IsClean()
	if isCleanErr != nil {
		slog.Debug("IsClean check failed, attempting cleanup anyway",
			"worktree", worktreePath,
			"error", isCleanErr,
		)
	}
	if isCleanErr != nil || !clean {
		// Try to rescue uncommitted changes by committing them
		rescued := false
		if isCleanErr == nil {
			ctx := worktreeGit.Context()
			if _, addErr := ctx.RunGit("add", "-A"); addErr == nil {
				msg := "[orc] Rescue uncommitted changes from interrupted execution"
				if _, commitErr := ctx.RunGit("commit", "-m", msg, "--allow-empty-message"); commitErr == nil {
					slog.Info("rescued uncommitted changes as commit before resume",
						"worktree", worktreePath,
					)
					rescued = true
				} else {
					slog.Debug("rescue commit failed, will discard changes",
						"worktree", worktreePath,
						"error", commitErr,
					)
				}
			}
		}
		// If rescue failed (e.g., conflicted state), fall back to discard
		if !rescued {
			if discardErr := worktreeGit.DiscardChanges(); discardErr != nil {
				if isCleanErr != nil {
					return fmt.Errorf("discard changes (IsClean also failed: %v): %w", isCleanErr, discardErr)
				}
				return fmt.Errorf("discard changes: %w", discardErr)
			}
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

		// Verify the expected branch exists before attempting checkout
		exists, existsErr := worktreeGit.BranchExists(expectedBranch)
		if existsErr != nil {
			return fmt.Errorf("check if branch %s exists: %w", expectedBranch, existsErr)
		}
		if !exists {
			return fmt.Errorf("expected branch %s does not exist - worktree at %s needs manual cleanup (currently on %s)",
				expectedBranch, worktreePath, currentBranch)
		}

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
