// workflow_completion.go contains completion actions for workflow execution.
// This includes PR creation, direct merge, worktree management, and sync operations.
package executor

import (
	"context"
	"errors"
	"fmt"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/git"
	"github.com/randalmurphal/orc/internal/hosting"
	_ "github.com/randalmurphal/orc/internal/hosting/github"
	_ "github.com/randalmurphal/orc/internal/hosting/gitlab"
	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/internal/variable"
)

// runCompletion executes the completion action (sync, PR/merge) for a task.
func (we *WorkflowExecutor) runCompletion(ctx context.Context, t *orcv1.Task) error {
	if we.orcConfig == nil {
		return nil
	}

	// Resolve action based on task weight
	action := we.orcConfig.ResolveCompletionAction(t.Weight.String())
	if action == "" || action == "none" {
		we.logger.Info("skipping completion action", "weight", t.Weight, "action", action)
		return nil
	}

	// Get effective git operations (worktree or main)
	gitOps := we.gitOps
	if we.worktreeGit != nil {
		gitOps = we.worktreeGit
	}

	if gitOps == nil {
		return fmt.Errorf("git operations not available")
	}

	// Skip if no remote is configured
	if !gitOps.HasRemote("origin") {
		we.logger.Debug("skipping completion: no remote configured")
		return nil
	}

	// Auto-commit any uncommitted changes before PR/merge
	// This prevents work loss when Claude forgets to commit
	if err := we.autoCommitBeforeCompletion(gitOps, t); err != nil {
		we.logger.Warn("auto-commit failed, continuing anyway", "error", err)
		// Non-fatal: PR might still succeed if changes were committed by Claude
	}

	// Sync with target branch before completion
	targetBranch := we.orcConfig.Completion.TargetBranch
	if targetBranch == "" {
		targetBranch = "main"
	}

	we.logger.Info("syncing with target branch before completion",
		"target", targetBranch,
		"action", action)

	// Fetch latest
	if err := gitOps.Fetch("origin"); err != nil {
		we.logger.Warn("fetch failed, continuing anyway", "error", err)
	}

	target := "origin/" + targetBranch

	// Check divergence
	ahead, behind, err := gitOps.GetCommitCounts(target)
	if err != nil {
		we.logger.Warn("could not determine divergence", "error", err)
	} else if behind > 0 {
		// Attempt rebase
		result, err := gitOps.RebaseWithConflictCheck(target)
		if err != nil {
			if errors.Is(err, git.ErrMergeConflict) {
				we.logger.Warn("sync encountered conflicts",
					"task", t.Id,
					"conflict_files", result.ConflictFiles)
				if we.orcConfig.Completion.Sync.FailOnConflict {
					return fmt.Errorf("%w: %d conflict files", ErrSyncConflict, len(result.ConflictFiles))
				}
			} else {
				return fmt.Errorf("rebase failed: %w", err)
			}
		} else {
			we.logger.Info("synced with target branch",
				"commits_behind", behind,
				"commits_ahead", ahead)
		}
	}

	// Execute completion action
	switch action {
	case "merge":
		return we.directMerge(ctx, t, gitOps, targetBranch)
	case "pr":
		return we.createPR(ctx, t, gitOps, targetBranch)
	default:
		we.logger.Warn("unknown completion action", "action", action)
		return nil
	}
}

// directMerge merges the task branch directly into target.
func (we *WorkflowExecutor) directMerge(ctx context.Context, t *orcv1.Task, gitOps *git.Git, targetBranch string) error {
	we.logger.Info("direct merge to target branch", "target", targetBranch)

	// Push task branch first (with force fallback for divergent history from previous runs)
	if err := gitOps.PushWithForceFallback("origin", t.Branch, false, we.logger); err != nil {
		return fmt.Errorf("push failed: %w", err)
	}

	// Switch to target and merge
	if err := gitOps.CheckoutSafe(targetBranch); err != nil {
		return fmt.Errorf("checkout target: %w", err)
	}

	// Fetch and rebase to get latest changes
	if err := gitOps.Fetch("origin"); err != nil {
		we.logger.Warn("fetch failed", "error", err)
	}
	if err := gitOps.Rebase("origin/" + targetBranch); err != nil {
		we.logger.Warn("rebase failed", "error", err)
	}

	if err := gitOps.Merge(t.Branch, false); err != nil {
		return fmt.Errorf("merge failed: %w", err)
	}

	if err := gitOps.Push("origin", targetBranch, false); err != nil {
		return fmt.Errorf("push target: %w", err)
	}

	// Update task with merge info
	t.Status = orcv1.TaskStatus_TASK_STATUS_RESOLVED
	task.MarkCompletedProto(t)
	if err := we.backend.SaveTask(t); err != nil {
		we.logger.Warn("failed to save task after direct merge", "task", t.Id, "error", err)
	}

	we.logger.Info("direct merge completed", "task", t.Id, "target", targetBranch)
	return nil
}

// createPR creates a pull request for the task branch.
func (we *WorkflowExecutor) createPR(ctx context.Context, t *orcv1.Task, gitOps *git.Git, targetBranch string) error {
	if task.HasPRProto(t) {
		we.logger.Info("PR already exists", "url", task.GetPRURLProto(t))
		return nil
	}

	we.logger.Info("creating PR", "branch", t.Branch, "target", targetBranch)

	// Push task branch (with force fallback for divergent history from previous runs)
	if err := gitOps.PushWithForceFallback("origin", t.Branch, true, we.logger); err != nil {
		return fmt.Errorf("push failed: %w", err)
	}

	// Get hosting provider
	provider, err := we.getHostingProvider()
	if err != nil {
		return fmt.Errorf("create hosting provider: %w", err)
	}

	// Build PR options from config
	prCfg := we.orcConfig.Completion.PR
	ciCfg := we.orcConfig.Completion.CI

	description := task.GetDescriptionProto(t)
	body := fmt.Sprintf("## Task: %s\n\n%s\n\n---\nCreated by orc workflow execution.",
		t.Title, description)
	prTitle := fmt.Sprintf("[orc] %s: %s", t.Id, t.Title)

	pr, err := provider.CreatePR(ctx, hosting.PRCreateOptions{
		Title:               prTitle,
		Body:                body,
		Head:                t.Branch,
		Base:                targetBranch,
		Draft:               prCfg.Draft,
		Labels:              prCfg.Labels,
		Reviewers:           prCfg.Reviewers,
		TeamReviewers:       prCfg.TeamReviewers,
		Assignees:           prCfg.Assignees,
		MaintainerCanModify: prCfg.MaintainerCanModify,
	})
	if err != nil {
		return fmt.Errorf("create PR: %w", err)
	}

	// Enable auto-merge if configured (GitLab only; GitHub returns ErrAutoMergeNotSupported)
	if prCfg.AutoMerge {
		if amErr := provider.EnableAutoMerge(ctx, pr.Number, ciCfg.MergeMethod); amErr != nil {
			if !errors.Is(amErr, hosting.ErrAutoMergeNotSupported) {
				we.logger.Warn("failed to enable auto-merge", "pr", pr.Number, "error", amErr)
			}
		}
	}

	// Auto-approve if configured
	if prCfg.AutoApprove {
		if apErr := provider.ApprovePR(ctx, pr.Number, "Auto-approved by orc"); apErr != nil {
			we.logger.Warn("failed to auto-approve PR", "pr", pr.Number, "error", apErr)
		}
	}

	// Update task with PR info
	task.SetPRInfoProto(t, pr.HTMLURL, pr.Number)
	if err := we.backend.SaveTask(t); err != nil {
		we.logger.Warn("failed to save task with PR info", "task", t.Id, "error", err)
	}

	we.logger.Info("PR created", "url", pr.HTMLURL, "number", pr.Number)
	return nil
}

// getHostingProvider creates a hosting provider from the executor's config and working directory.
func (we *WorkflowExecutor) getHostingProvider() (hosting.Provider, error) {
	cfg := hosting.Config{}
	if we.orcConfig != nil {
		cfg = hosting.Config{
			Provider:    we.orcConfig.Hosting.Provider,
			BaseURL:     we.orcConfig.Hosting.BaseURL,
			TokenEnvVar: we.orcConfig.Hosting.TokenEnvVar,
		}
	}
	return hosting.NewProvider(we.effectiveWorkingDir(), cfg)
}

// setupWorktree creates or reuses an isolated worktree for the given task.
func (we *WorkflowExecutor) setupWorktree(t *orcv1.Task) error {
	result, err := SetupWorktreeForTask(t, we.orcConfig, we.gitOps, we.backend)
	if err != nil {
		return fmt.Errorf("setup worktree: %w", err)
	}

	we.worktreePath = result.Path
	we.worktreeGit = we.gitOps.InWorktree(result.Path)

	// Calculate and set task branch for git operations (push, PR creation, etc.)
	// Get initiative prefix for branch name calculation
	var initiativePrefix string
	initiativeID := task.GetInitiativeIDProto(t)
	if initiativeID != "" {
		if init, loadErr := we.backend.LoadInitiative(initiativeID); loadErr == nil && init != nil {
			initiativePrefix = init.BranchPrefix
		}
	}

	// Set task branch before any git operations reference it
	t.Branch = we.gitOps.BranchNameWithInitiativePrefix(t.Id, initiativePrefix)
	if err := we.backend.SaveTask(t); err != nil {
		we.logger.Warn("failed to save task branch", "task_id", t.Id, "error", err)
	}

	logMsg := "created worktree"
	if result.Reused {
		logMsg = "reusing existing worktree"
	}
	we.logger.Info(logMsg, "task", t.Id, "path", result.Path, "target_branch", result.TargetBranch, "branch", t.Branch)

	// Update the resolver to use worktree path
	we.resolver = variable.NewResolver(result.Path)

	return nil
}

// cleanupWorktree removes the worktree based on config and task status.
func (we *WorkflowExecutor) cleanupWorktree(t *orcv1.Task) {
	if we.worktreePath == "" {
		return
	}

	// StatusResolved is treated like StatusCompleted for cleanup - both are terminal success states
	shouldCleanup := ((t.Status == orcv1.TaskStatus_TASK_STATUS_COMPLETED || t.Status == orcv1.TaskStatus_TASK_STATUS_RESOLVED) && we.orcConfig.Worktree.CleanupOnComplete) ||
		(t.Status == orcv1.TaskStatus_TASK_STATUS_FAILED && we.orcConfig.Worktree.CleanupOnFail)
	if !shouldCleanup {
		return
	}

	// Cleanup Playwright user data directory (task-specific browser profile)
	if err := CleanupPlaywrightUserData(t.Id); err != nil {
		we.logger.Warn("failed to cleanup playwright user data", "task", t.Id, "error", err)
	}

	// Use stored worktree path directly instead of reconstructing from task ID.
	// This handles initiative-prefixed worktrees correctly.
	if err := we.gitOps.CleanupWorktreeAtPath(we.worktreePath); err != nil {
		we.logger.Warn("failed to cleanup worktree", "path", we.worktreePath, "error", err)
	} else {
		we.logger.Info("cleaned up worktree", "task", t.Id, "path", we.worktreePath)
	}
}

// effectiveWorkingDir returns the working directory for phase execution.
// Returns worktree path if one was created, otherwise the original working dir.
func (we *WorkflowExecutor) effectiveWorkingDir() string {
	if we.worktreePath != "" {
		return we.worktreePath
	}
	return we.workingDir
}

// syncOnTaskStart syncs the task branch with target before execution starts.
// This catches conflicts from parallel tasks early.
func (we *WorkflowExecutor) syncOnTaskStart(ctx context.Context, t *orcv1.Task) error {
	cfg := we.orcConfig.Completion
	targetBranch := cfg.TargetBranch
	if targetBranch == "" {
		targetBranch = "main"
	}

	// Use worktree git if available
	gitOps := we.gitOps
	if we.worktreeGit != nil {
		gitOps = we.worktreeGit
	}

	if gitOps == nil {
		we.logger.Debug("skipping sync-on-start: git ops not available")
		return nil
	}

	// Skip sync if no remote is configured (e.g., E2E sandbox projects)
	if !gitOps.HasRemote("origin") {
		we.logger.Debug("skipping sync-on-start: no remote configured",
			"task", t.Id,
			"reason", "repository has no 'origin' remote")
		return nil
	}

	we.logger.Info("syncing with target before execution",
		"target", targetBranch,
		"task", t.Id,
		"reason", "catch stale worktree from parallel tasks")

	// Fetch latest from remote
	if err := gitOps.Fetch("origin"); err != nil {
		we.logger.Warn("fetch failed, continuing anyway", "error", err)
	}

	// CRITICAL: Sync with remote feature branch first
	// This prevents push failures when the remote branch already exists with different commits
	// (e.g., from a previous run that was interrupted/resumed)
	if t.Branch != "" {
		remoteFeature := "origin/" + t.Branch
		featureExists, err := gitOps.RemoteBranchExists("origin", t.Branch)
		if err != nil {
			we.logger.Debug("could not check remote feature branch, continuing", "error", err)
		} else if featureExists {
			// Check if we're behind the remote feature branch
			featureAhead, featureBehind, err := gitOps.GetCommitCounts(remoteFeature)
			if err != nil {
				we.logger.Debug("could not determine feature branch commit counts", "error", err)
			} else if featureBehind > 0 {
				we.logger.Info("task branch is behind remote feature branch",
					"remote", remoteFeature,
					"commits_behind", featureBehind,
					"commits_ahead", featureAhead)

				// Merge from remote feature branch to incorporate previous work
				if _, err := gitOps.Context().RunGit("merge", remoteFeature, "--no-edit"); err != nil {
					// If merge fails, try reset to remote (previous work takes precedence)
					we.logger.Warn("merge from remote feature branch failed, resetting to remote",
						"error", err,
						"remote", remoteFeature)
					if _, resetErr := gitOps.Context().RunGit("merge", "--abort"); resetErr != nil {
						we.logger.Debug("merge abort failed (may not be in conflict state)", "error", resetErr)
					}
					if _, resetErr := gitOps.Context().RunGit("reset", "--hard", remoteFeature); resetErr != nil {
						we.logger.Warn("reset to remote feature branch failed", "error", resetErr)
					} else {
						we.logger.Info("reset to remote feature branch", "remote", remoteFeature)
					}
				} else {
					we.logger.Info("merged remote feature branch", "remote", remoteFeature)
				}
			} else {
				we.logger.Debug("local branch is up-to-date with remote feature branch",
					"remote", remoteFeature)
			}
		}
	}

	target := "origin/" + targetBranch

	// Check if we're behind target
	ahead, behind, err := gitOps.GetCommitCounts(target)
	if err != nil {
		we.logger.Warn("could not determine commit counts, skipping sync", "error", err)
		return nil // Don't fail - this is best effort
	}

	if behind == 0 {
		we.logger.Info("branch already up-to-date with target",
			"target", targetBranch,
			"commits_ahead", ahead)
		return nil
	}

	we.logger.Info("task branch is behind target",
		"target", targetBranch,
		"commits_behind", behind,
		"commits_ahead", ahead)

	// Attempt rebase with conflict detection
	result, err := gitOps.RebaseWithConflictCheck(target)
	if err != nil {
		if errors.Is(err, git.ErrMergeConflict) {
			// Log conflict details
			we.logger.Warn("sync-on-start encountered conflicts",
				"task", t.Id,
				"conflict_files", result.ConflictFiles,
				"commits_behind", result.CommitsBehind)

			syncCfg := cfg.Sync
			conflictCount := len(result.ConflictFiles)

			// Check if we should fail on conflicts
			if syncCfg.MaxConflictFiles > 0 && conflictCount > syncCfg.MaxConflictFiles {
				return fmt.Errorf("sync conflict: %d conflict files exceeds max allowed (%d): %v",
					conflictCount, syncCfg.MaxConflictFiles, result.ConflictFiles)
			}

			if syncCfg.FailOnConflict {
				return fmt.Errorf("sync conflict: task branch has %d files in conflict with target",
					conflictCount)
			}

			// Continue execution - implement phase may resolve conflicts
			we.logger.Warn("continuing despite conflicts (fail_on_conflict: false)",
				"task", t.Id,
				"conflict_count", conflictCount)
			return nil
		}
		return fmt.Errorf("rebase onto %s: %w", target, err)
	}

	we.logger.Info("synced task branch with target",
		"target", targetBranch,
		"commits_behind", result.CommitsBehind)

	return nil
}

// autoCommitBeforeCompletion commits any uncommitted changes before PR/merge.
// This is a safety net for when Claude doesn't commit during implement phase.
func (we *WorkflowExecutor) autoCommitBeforeCompletion(gitOps *git.Git, t *orcv1.Task) error {
	hasChanges, err := gitOps.HasUncommittedChanges()
	if err != nil {
		return fmt.Errorf("check uncommitted changes: %w", err)
	}

	if !hasChanges {
		return nil // Clean worktree, nothing to commit
	}

	we.logger.Info("uncommitted changes detected, auto-committing",
		"task", t.Id)

	// Stage all changes
	ctx := gitOps.Context()
	if _, err := ctx.RunGit("add", "-A"); err != nil {
		return fmt.Errorf("stage changes: %w", err)
	}

	// Commit with standard message format
	msg := fmt.Sprintf("[orc] %s: Auto-commit before PR creation\n\nCo-Authored-By: Claude Sonnet 4.5 <noreply@anthropic.com>", t.Id)
	if _, err := ctx.RunGit("commit", "-m", msg); err != nil {
		return fmt.Errorf("commit changes: %w", err)
	}

	we.logger.Info("auto-committed changes", "task", t.Id)
	return nil
}
