// workflow_completion.go contains completion actions for workflow execution.
// This includes PR creation, direct merge, worktree management, and sync operations.
package executor

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/randalmurphal/orc/internal/git"
	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/internal/variable"
)

// runCompletion executes the completion action (sync, PR/merge) for a task.
func (we *WorkflowExecutor) runCompletion(ctx context.Context, t *task.Task) error {
	if we.orcConfig == nil {
		return nil
	}

	// Resolve action based on task weight
	action := we.orcConfig.ResolveCompletionAction(string(t.Weight))
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
					"task", t.ID,
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
func (we *WorkflowExecutor) directMerge(ctx context.Context, t *task.Task, gitOps *git.Git, targetBranch string) error {
	we.logger.Info("direct merge to target branch", "target", targetBranch)

	// Push task branch first
	if err := gitOps.Push("origin", t.Branch, false); err != nil {
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
	t.Status = task.StatusResolved
	now := time.Now()
	t.CompletedAt = &now
	we.backend.SaveTask(t)

	we.logger.Info("direct merge completed", "task", t.ID, "target", targetBranch)
	return nil
}

// createPR creates a pull request for the task branch.
func (we *WorkflowExecutor) createPR(ctx context.Context, t *task.Task, gitOps *git.Git, targetBranch string) error {
	// Check if PR already exists
	if t.PR != nil && t.PR.URL != "" {
		we.logger.Info("PR already exists", "url", t.PR.URL)
		return nil
	}

	we.logger.Info("creating PR", "branch", t.Branch, "target", targetBranch)

	// Push task branch
	if err := gitOps.Push("origin", t.Branch, true); err != nil {
		return fmt.Errorf("push failed: %w", err)
	}

	// Build PR body
	body := fmt.Sprintf("## Task: %s\n\n%s\n\n---\nCreated by orc workflow execution.",
		t.Title, t.Description)

	// Create PR via gh cli
	prTitle := fmt.Sprintf("[orc] %s: %s", t.ID, t.Title)
	prURL, err := we.runGHCreatePR(ctx, prTitle, body, targetBranch)
	if err != nil {
		return fmt.Errorf("create PR: %w", err)
	}

	// Update task with PR info
	t.PR = &task.PRInfo{
		URL:    prURL,
		Status: task.PRStatusPendingReview,
	}
	we.backend.SaveTask(t)

	we.logger.Info("PR created", "url", prURL)
	return nil
}

// runGHCreatePR creates a PR using the gh CLI.
func (we *WorkflowExecutor) runGHCreatePR(ctx context.Context, title, body, targetBranch string) (string, error) {
	workDir := we.effectiveWorkingDir()

	args := []string{
		"pr", "create",
		"--title", title,
		"--body", body,
		"--base", targetBranch,
	}

	cmd := exec.CommandContext(ctx, "gh", args...)
	cmd.Dir = workDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("gh pr create: %w: %s", err, string(out))
	}

	// Extract URL from output (gh pr create outputs the PR URL)
	prURL := strings.TrimSpace(string(out))
	return prURL, nil
}

// setupWorktree creates or reuses an isolated worktree for the given task.
func (we *WorkflowExecutor) setupWorktree(t *task.Task) error {
	result, err := SetupWorktreeForTask(t, we.orcConfig, we.gitOps, we.backend)
	if err != nil {
		return fmt.Errorf("setup worktree: %w", err)
	}

	we.worktreePath = result.Path
	we.worktreeGit = we.gitOps.InWorktree(result.Path)

	// Calculate and set task branch for git operations (push, PR creation, etc.)
	// Get initiative prefix for branch name calculation
	var initiativePrefix string
	if t.InitiativeID != "" {
		if init, loadErr := we.backend.LoadInitiative(t.InitiativeID); loadErr == nil && init != nil {
			initiativePrefix = init.BranchPrefix
		}
	}

	// Set task branch before any git operations reference it
	t.Branch = we.gitOps.BranchNameWithInitiativePrefix(t.ID, initiativePrefix)
	if err := we.backend.SaveTask(t); err != nil {
		we.logger.Warn("failed to save task branch", "task_id", t.ID, "error", err)
	}

	logMsg := "created worktree"
	if result.Reused {
		logMsg = "reusing existing worktree"
	}
	we.logger.Info(logMsg, "task", t.ID, "path", result.Path, "target_branch", result.TargetBranch, "branch", t.Branch)

	// Generate per-worktree MCP config for isolated Playwright sessions
	if ShouldGenerateMCPConfig(t, we.orcConfig) {
		if err := GenerateWorktreeMCPConfig(result.Path, t.ID, t, we.orcConfig); err != nil {
			we.logger.Warn("failed to generate MCP config", "task", t.ID, "error", err)
			// Non-fatal: continue without MCP config
		} else {
			we.logger.Info("generated MCP config", "task", t.ID, "path", result.Path+"/.mcp.json")
		}
	}

	// Update the resolver to use worktree path
	we.resolver = variable.NewResolver(result.Path)

	return nil
}

// cleanupWorktree removes the worktree based on config and task status.
func (we *WorkflowExecutor) cleanupWorktree(t *task.Task) {
	if we.worktreePath == "" {
		return
	}

	// StatusResolved is treated like StatusCompleted for cleanup - both are terminal success states
	shouldCleanup := ((t.Status == task.StatusCompleted || t.Status == task.StatusResolved) && we.orcConfig.Worktree.CleanupOnComplete) ||
		(t.Status == task.StatusFailed && we.orcConfig.Worktree.CleanupOnFail)
	if !shouldCleanup {
		return
	}

	// Cleanup Playwright user data directory (task-specific browser profile)
	if err := CleanupPlaywrightUserData(t.ID); err != nil {
		we.logger.Warn("failed to cleanup playwright user data", "task", t.ID, "error", err)
	}

	// Use stored worktree path directly instead of reconstructing from task ID.
	// This handles initiative-prefixed worktrees correctly.
	if err := we.gitOps.CleanupWorktreeAtPath(we.worktreePath); err != nil {
		we.logger.Warn("failed to cleanup worktree", "path", we.worktreePath, "error", err)
	} else {
		we.logger.Info("cleaned up worktree", "task", t.ID, "path", we.worktreePath)
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
func (we *WorkflowExecutor) syncOnTaskStart(ctx context.Context, t *task.Task) error {
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
			"task", t.ID,
			"reason", "repository has no 'origin' remote")
		return nil
	}

	we.logger.Info("syncing with target before execution",
		"target", targetBranch,
		"task", t.ID,
		"reason", "catch stale worktree from parallel tasks")

	// Fetch latest from remote
	if err := gitOps.Fetch("origin"); err != nil {
		we.logger.Warn("fetch failed, continuing anyway", "error", err)
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
				"task", t.ID,
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
				"task", t.ID,
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
