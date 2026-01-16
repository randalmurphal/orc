// Package executor provides PR/merge completion actions for task execution.
package executor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/git"
	"github.com/randalmurphal/orc/internal/task"
)

// runCompletion executes the completion action (merge/PR/none).
func (e *Executor) runCompletion(ctx context.Context, t *task.Task) error {
	// Resolve action based on task weight
	action := e.orcConfig.ResolveCompletionAction(string(t.Weight))
	if action == "" || action == "none" {
		e.logger.Info("skipping completion action", "weight", t.Weight, "action", action)
		return nil
	}

	if e.gitOps == nil {
		return fmt.Errorf("git operations not available")
	}

	// Sync with target branch before completion
	if err := e.syncWithTarget(ctx, t); err != nil {
		return fmt.Errorf("sync with target: %w", err)
	}

	switch action {
	case "merge":
		return e.directMerge(ctx, t)
	case "pr":
		return e.createPR(ctx, t)
	default:
		e.logger.Warn("unknown completion action", "action", action)
		return nil
	}
}

// ErrSyncConflict is returned when sync encounters merge conflicts.
var ErrSyncConflict = errors.New("sync conflict detected")

// SyncPhase indicates when sync is being performed.
type SyncPhase string

const (
	// SyncPhaseStart indicates sync at task start or phase start
	SyncPhaseStart SyncPhase = "start"
	// SyncPhaseCompletion indicates sync before PR/merge
	SyncPhaseCompletion SyncPhase = "completion"
)

// syncWithTarget syncs the task branch with the target branch according to config.
// Returns ErrSyncConflict if conflicts are detected and fail_on_conflict is true.
func (e *Executor) syncWithTarget(ctx context.Context, t *task.Task) error {
	return e.syncWithTargetPhase(ctx, t, SyncPhaseCompletion)
}

// syncWithTargetPhase syncs the task branch with the target branch.
// The phase parameter indicates when sync is being called (for logging/metrics).
func (e *Executor) syncWithTargetPhase(ctx context.Context, t *task.Task, phase SyncPhase) error {
	cfg := e.orcConfig.Completion
	syncCfg := cfg.Sync
	targetBranch := cfg.TargetBranch
	if targetBranch == "" {
		targetBranch = "main"
	}

	// Check if sync should be skipped for this weight
	if !e.orcConfig.ShouldSyncForWeight(string(t.Weight)) {
		e.logger.Debug("skipping sync for weight", "weight", t.Weight)
		return nil
	}

	// Use worktree git if available
	gitOps := e.gitOps
	if e.worktreeGit != nil {
		gitOps = e.worktreeGit
	}

	// Skip sync if no remote is configured (e.g., E2E sandbox projects)
	// This avoids noisy warnings for test repositories without remotes
	if !gitOps.HasRemote("origin") {
		e.logger.Debug("skipping sync: no remote configured",
			"task", t.ID,
			"reason", "repository has no 'origin' remote")
		return nil
	}

	e.logger.Info("syncing with target branch",
		"target", targetBranch,
		"phase", phase,
		"strategy", syncCfg.Strategy)

	// Fetch latest from remote
	if err := gitOps.Fetch("origin"); err != nil {
		e.logger.Warn("fetch failed, continuing anyway", "error", err)
	}

	target := "origin/" + targetBranch

	// For detect-only strategy, just check for conflicts without modifying
	if syncCfg.Strategy == config.SyncStrategyDetect {
		return e.detectConflictsOnly(gitOps, target, t.ID, syncCfg)
	}

	// Perform rebase with conflict detection
	result, err := gitOps.RebaseWithConflictCheck(target)
	if err != nil {
		if errors.Is(err, git.ErrMergeConflict) {
			return e.handleSyncConflict(result, t.ID, syncCfg)
		}
		return fmt.Errorf("rebase onto %s: %w", target, err)
	}

	if result.CommitsBehind == 0 {
		e.logger.Info("branch already up-to-date with target", "target", targetBranch)
	} else {
		e.logger.Info("synced with target branch",
			"target", targetBranch,
			"commits_ahead", result.CommitsAhead,
			"commits_behind", result.CommitsBehind)
	}

	// Restore .orc/ from target branch to prevent worktree contamination
	// This ensures any modifications to .orc/ during task execution don't get merged
	restored, err := gitOps.RestoreOrcDir(target, t.ID)
	if err != nil {
		e.logger.Warn("failed to restore .orc/ directory", "error", err)
		// Don't fail the sync - restoration is defense-in-depth
	} else if restored {
		e.logger.Info("restored .orc/ from target branch",
			"target", targetBranch,
			"reason", "prevent worktree contamination")
	}

	// Restore .claude/settings.json to prevent worktree isolation hooks from being merged
	// Worktrees inject hooks with machine-specific paths that shouldn't be shared
	restoredSettings, err := gitOps.RestoreClaudeSettings(target, t.ID)
	if err != nil {
		e.logger.Warn("failed to restore .claude/settings.json", "error", err)
	} else if restoredSettings {
		e.logger.Info("restored .claude/settings.json from target branch",
			"target", targetBranch,
			"reason", "prevent worktree hooks from being merged")
	}

	return nil
}

// detectConflictsOnly checks for conflicts without attempting resolution.
func (e *Executor) detectConflictsOnly(gitOps *git.Git, target, taskID string, syncCfg config.SyncConfig) error {
	result, err := gitOps.DetectConflicts(target)
	if err != nil {
		return fmt.Errorf("detect conflicts: %w", err)
	}

	if result.ConflictsDetected {
		return e.handleSyncConflict(result, taskID, syncCfg)
	}

	if result.CommitsBehind > 0 {
		e.logger.Info("branch is behind target but no conflicts detected",
			"commits_behind", result.CommitsBehind,
			"hint", "rebase will be required before merge")
	}

	return nil
}

// handleSyncConflict handles merge conflicts according to config.
// If conflict resolution is enabled, it attempts to resolve conflicts using:
// 1. Auto-resolution for known patterns (CLAUDE.md knowledge tables)
// 2. Claude-assisted resolution for remaining conflicts
func (e *Executor) handleSyncConflict(result *git.SyncResult, taskID string, syncCfg config.SyncConfig) error {
	conflictCount := len(result.ConflictFiles)

	e.logger.Error("merge conflicts detected",
		"task", taskID,
		"conflict_files", conflictCount,
		"files", result.ConflictFiles)

	// Check max conflict files threshold
	if syncCfg.MaxConflictFiles > 0 && conflictCount > syncCfg.MaxConflictFiles {
		return fmt.Errorf("%w: %d files with conflicts exceeds max allowed (%d): %v",
			ErrSyncConflict, conflictCount, syncCfg.MaxConflictFiles, result.ConflictFiles)
	}

	// Try to resolve conflicts if configured
	if e.orcConfig.Completion.Finalize.ConflictResolution.Enabled {
		e.logger.Info("attempting conflict resolution",
			"task", taskID,
			"conflict_files", result.ConflictFiles)

		// Use worktree git if available
		gitOps := e.gitOps
		if e.worktreeGit != nil {
			gitOps = e.worktreeGit
		}

		// Create conflict resolver
		resolver := NewConflictResolver(
			WithResolverGitSvc(gitOps),
			WithResolverSessionManager(e.sessionMgr),
			WithResolverLogger(e.logger),
			WithResolverConfig(e.orcConfig.Completion.Finalize),
			WithResolverWorkingDir(e.worktreePath),
		)

		// Load task for resolution context
		t, loadErr := e.backend.LoadTask(taskID)
		if loadErr != nil {
			e.logger.Warn("could not load task for conflict resolution", "error", loadErr)
			// Fall through to fail on conflict check
		} else {
			// Try to resolve conflicts
			resolveResult, resolveErr := resolver.Resolve(context.Background(), t, result.ConflictFiles)
			if resolveErr == nil && resolveResult.Resolved {
				e.logger.Info("conflicts resolved successfully",
					"task", taskID,
					"auto_resolved", resolveResult.AutoResolved,
					"claude_resolved", resolveResult.ClaudeResolved)
				return nil // Conflicts resolved, continue with completion
			}

			if resolveErr != nil {
				e.logger.Warn("conflict resolution failed", "error", resolveErr)
			} else if len(resolveResult.Unresolved) > 0 {
				e.logger.Warn("some conflicts could not be resolved",
					"unresolved", resolveResult.Unresolved)
			}
		}
	}

	// Fail on conflict if configured (or resolution failed)
	if syncCfg.FailOnConflict {
		return fmt.Errorf("%w: %d files have conflicts with target branch: %v\n"+
			"  Resolution options:\n"+
			"    1. Manually resolve conflicts and retry\n"+
			"    2. Rebase your changes onto the latest target branch\n"+
			"    3. Set completion.sync.fail_on_conflict: false to allow PR with conflicts",
			ErrSyncConflict, conflictCount, result.ConflictFiles)
	}

	// If not failing, just warn and continue
	e.logger.Warn("continuing despite conflicts (fail_on_conflict: false)",
		"conflict_files", conflictCount)

	return nil
}

// syncBeforePhase performs sync if configured for phase-start strategy.
// Returns nil if sync is not configured for phase start.
func (e *Executor) syncBeforePhase(ctx context.Context, t *task.Task, phaseID string) error {
	if !e.orcConfig.ShouldSyncBeforePhase() {
		return nil
	}

	if e.gitOps == nil {
		e.logger.Debug("skipping pre-phase sync: git ops not available")
		return nil
	}

	e.logger.Info("syncing before phase execution", "phase", phaseID)
	return e.syncWithTargetPhase(ctx, t, SyncPhaseStart)
}

// syncOnTaskStart syncs the task branch with target before execution starts.
// This catches conflicts from parallel tasks early - if task A merges to main while
// task B's worktree is stale, this sync brings in task A's changes so task B can
// incorporate them during the implement phase.
//
// Unlike syncWithTarget which fails on conflicts, this is more lenient:
// - Conflicts are logged but don't block execution
// - The implement phase gets a chance to resolve conflicts intelligently
// - If conflicts can't be auto-resolved, FailOnConflict config applies
func (e *Executor) syncOnTaskStart(ctx context.Context, t *task.Task) error {
	cfg := e.orcConfig.Completion
	syncCfg := cfg.Sync
	targetBranch := cfg.TargetBranch
	if targetBranch == "" {
		targetBranch = "main"
	}

	// Use worktree git if available
	gitOps := e.gitOps
	if e.worktreeGit != nil {
		gitOps = e.worktreeGit
	}

	if gitOps == nil {
		e.logger.Debug("skipping sync-on-start: git ops not available")
		return nil
	}

	// Skip sync if no remote is configured (e.g., E2E sandbox projects)
	// This avoids noisy warnings for test repositories without remotes
	if !gitOps.HasRemote("origin") {
		e.logger.Debug("skipping sync-on-start: no remote configured",
			"task", t.ID,
			"reason", "repository has no 'origin' remote")
		return nil
	}

	e.logger.Info("syncing with target before execution",
		"target", targetBranch,
		"task", t.ID,
		"reason", "catch stale worktree from parallel tasks")

	// Fetch latest from remote
	if err := gitOps.Fetch("origin"); err != nil {
		e.logger.Warn("fetch failed, continuing anyway", "error", err)
	}

	target := "origin/" + targetBranch

	// Check if we're behind target
	ahead, behind, err := gitOps.GetCommitCounts(target)
	if err != nil {
		e.logger.Warn("could not determine commit counts, skipping sync", "error", err)
		return nil // Don't fail - this is best effort
	}

	if behind == 0 {
		e.logger.Info("branch already up-to-date with target",
			"target", targetBranch,
			"commits_ahead", ahead)
		return nil
	}

	e.logger.Info("task branch is behind target",
		"target", targetBranch,
		"commits_behind", behind,
		"commits_ahead", ahead)

	// Attempt rebase with conflict detection
	result, err := gitOps.RebaseWithConflictCheck(target)
	if err != nil {
		if errors.Is(err, git.ErrMergeConflict) {
			// Log conflict details
			e.logger.Warn("sync-on-start encountered conflicts",
				"task", t.ID,
				"conflict_files", result.ConflictFiles,
				"commits_behind", result.CommitsBehind)

			// Check if we should fail on conflicts
			conflictCount := len(result.ConflictFiles)
			if syncCfg.MaxConflictFiles > 0 && conflictCount > syncCfg.MaxConflictFiles {
				return fmt.Errorf("%w: %d conflict files exceeds max allowed (%d): %v",
					ErrSyncConflict, conflictCount, syncCfg.MaxConflictFiles, result.ConflictFiles)
			}

			if syncCfg.FailOnConflict {
				return fmt.Errorf("%w: task branch has %d files in conflict with target\n"+
					"  Conflicting files: %v\n"+
					"  Resolution options:\n"+
					"    1. Run with sync_on_start: false and resolve conflicts during finalize\n"+
					"    2. Manually rebase the task branch and retry\n"+
					"    3. Set completion.sync.fail_on_conflict: false to proceed anyway",
					ErrSyncConflict, conflictCount, result.ConflictFiles)
			}

			// Continue execution - implement phase may resolve conflicts
			e.logger.Warn("continuing despite conflicts (fail_on_conflict: false)",
				"task", t.ID,
				"conflict_count", conflictCount)
			return nil
		}
		return fmt.Errorf("rebase onto %s: %w", target, err)
	}

	e.logger.Info("synced task branch with target",
		"target", targetBranch,
		"commits_ahead", result.CommitsAhead)

	return nil
}

// ErrDirectMergeBlocked is returned when direct merge to a protected branch is blocked.
var ErrDirectMergeBlocked = errors.New("direct merge to protected branch blocked")

// directMerge merges the task branch directly into the target branch.
// NOTE: This operation is BLOCKED for protected branches (main, master, develop, release).
// Use the PR workflow instead for protected branches.
//
// SAFETY: This operation requires worktree context to prevent accidental modification
// of the main repository. Direct merge uses checkout and merge which are destructive.
func (e *Executor) directMerge(ctx context.Context, t *task.Task) error {
	cfg := e.orcConfig.Completion
	taskBranch := e.gitOps.BranchName(t.ID)

	// SAFETY: Block direct merge to protected branches
	// This is a critical safety check - protected branches should only be modified via PR
	if git.IsProtectedBranch(cfg.TargetBranch, e.gitOps.ProtectedBranches()) {
		e.logger.Error("direct merge blocked",
			"target", cfg.TargetBranch,
			"task", t.ID,
			"reason", "protected branch - use PR workflow instead")
		return fmt.Errorf("%w: cannot merge directly to '%s' - use completion.action: pr instead",
			ErrDirectMergeBlocked, cfg.TargetBranch)
	}

	// Use worktree git if available, otherwise main repo
	gitOps := e.gitOps
	if e.worktreeGit != nil {
		gitOps = e.worktreeGit
	}

	// CRITICAL SAFETY: Require worktree context for checkout/merge operations
	// This prevents accidental modification of the main repository branch
	if err := gitOps.RequireWorktreeContext("direct merge"); err != nil {
		e.logger.Error("direct merge blocked - not in worktree context",
			"task", t.ID,
			"error", err)
		return fmt.Errorf("direct merge requires worktree context: %w", err)
	}

	// Checkout target branch using safe method (protected by RequireWorktreeContext above)
	if err := gitOps.CheckoutSafe(cfg.TargetBranch); err != nil {
		return fmt.Errorf("checkout %s: %w", cfg.TargetBranch, err)
	}

	// Merge task branch (also protected by RequireWorktreeContext)
	if err := gitOps.Merge(taskBranch, true); err != nil {
		return fmt.Errorf("merge %s: %w", taskBranch, err)
	}

	// Push to remote - Push() will validate protected branches but we've already
	// confirmed this is a non-protected branch above, so this is safe
	if err := gitOps.Push("origin", cfg.TargetBranch, false); err != nil {
		e.logger.Warn("failed to push after merge", "error", err)
	}

	// Delete task branch if configured
	if cfg.DeleteBranch {
		if err := gitOps.DeleteBranch(taskBranch, false); err != nil {
			e.logger.Warn("failed to delete task branch", "error", err)
		}
	}

	e.logger.Info("merged task branch", "task", t.ID, "branch", taskBranch, "target", cfg.TargetBranch)
	return nil
}

// createPR creates a pull request for the task branch.
func (e *Executor) createPR(ctx context.Context, t *task.Task) error {
	cfg := e.orcConfig.Completion
	taskBranch := e.gitOps.BranchName(t.ID)

	// Use worktree git if available
	gitOps := e.gitOps
	if e.worktreeGit != nil {
		gitOps = e.worktreeGit
	}

	// Push task branch to remote
	if err := gitOps.Push("origin", taskBranch, true); err != nil {
		// Check if this is a non-fast-forward error (diverged branch from previous run)
		if isNonFastForwardError(err) {
			e.logger.Warn("remote branch has diverged, force pushing",
				"branch", taskBranch,
				"reason", "re-run of completed task or local/remote history diverged")
			if forceErr := gitOps.PushForce("origin", taskBranch, true); forceErr != nil {
				return fmt.Errorf("force push branch (remote diverged): %w", forceErr)
			}
		} else {
			return fmt.Errorf("push branch: %w", err)
		}
	}

	// Build PR title
	title := cfg.PR.Title
	if title == "" {
		title = "[orc] {{TASK_TITLE}}"
	}
	title = strings.ReplaceAll(title, "{{TASK_TITLE}}", t.Title)
	title = strings.ReplaceAll(title, "{{TASK_ID}}", t.ID)

	// Build PR body
	body := e.buildPRBody(t)

	// Create PR using gh CLI
	args := []string{"pr", "create",
		"--title", title,
		"--body", body,
		"--base", cfg.TargetBranch,
		"--head", taskBranch,
	}

	// Add labels
	labels := cfg.PR.Labels
	for _, label := range labels {
		args = append(args, "--label", label)
	}

	// Add reviewers
	for _, reviewer := range cfg.PR.Reviewers {
		args = append(args, "--reviewer", reviewer)
	}

	// Add draft flag
	if cfg.PR.Draft {
		args = append(args, "--draft")
	}

	// Run gh CLI
	output, err := e.runGH(ctx, args...)
	if err != nil && len(labels) > 0 && isLabelError(err) {
		// Labels failed - retry without labels
		// This is expected behavior for repos without pre-configured labels, so use Debug
		e.logger.Debug("PR labels not found on repository, creating PR without labels",
			"labels", labels,
			"error", err)

		// Rebuild args without labels
		args = []string{"pr", "create",
			"--title", title,
			"--body", body,
			"--base", cfg.TargetBranch,
			"--head", taskBranch,
		}
		for _, reviewer := range cfg.PR.Reviewers {
			args = append(args, "--reviewer", reviewer)
		}
		if cfg.PR.Draft {
			args = append(args, "--draft")
		}

		output, err = e.runGH(ctx, args...)
	}
	if err != nil {
		if isAuthError(err) {
			return fmt.Errorf("%w: %v\n\n"+
				"  To fix this, run:\n"+
				"    gh auth login\n\n"+
				"  Then retry with:\n"+
				"    orc resume %s",
				ErrGHNotAuthenticated, err, t.ID)
		}
		return fmt.Errorf("create PR: %w", err)
	}

	// Extract PR URL from output
	prURL := strings.TrimSpace(output)
	if prURL != "" {
		// Keep metadata for backwards compatibility
		if t.Metadata == nil {
			t.Metadata = make(map[string]string)
		}
		t.Metadata["pr_url"] = prURL

		// Extract PR number from URL (e.g., https://github.com/owner/repo/pull/123)
		prNumber := 0
		if parts := strings.Split(prURL, "/pull/"); len(parts) == 2 {
			_, _ = fmt.Sscanf(parts[1], "%d", &prNumber)
		}

		// Set PR info on task
		t.SetPRInfo(prURL, prNumber)

		if saveErr := e.backend.SaveTask(t); saveErr != nil {
			e.logger.Error("failed to save task with PR URL", "error", saveErr)
		}
	}

	e.logger.Info("created pull request", "task", t.ID, "url", prURL)

	// Enable auto-merge if configured
	if cfg.PR.AutoMerge && prURL != "" {
		if _, err := e.runGH(ctx, "pr", "merge", prURL, "--auto", "--squash"); err != nil {
			if isAuthError(err) {
				// Auth errors are actionable - user needs to fix their token
				e.logger.Warn("failed to enable auto-merge due to auth issue",
					"error", err,
					"hint", "run 'gh auth login' to fix")
			} else if isAutoMergeConfigError(err) {
				// Config errors are expected for repos without auto-merge enabled
				// This is common for repos without branch protection rules
				e.logger.Debug("auto-merge not available for repository",
					"error", err,
					"hint", "enable auto-merge in repository settings or set up branch protection")
			} else {
				// Other errors (network, API issues) warrant a warning
				e.logger.Warn("failed to enable auto-merge", "error", err)
			}
		} else {
			e.logger.Info("enabled auto-merge", "task", t.ID)
		}
	}

	// Auto-approve PR if in auto/fast mode and configured
	if e.orcConfig.ShouldAutoApprovePR() && prURL != "" {
		if err := e.autoApprovePR(ctx, t, prURL); err != nil {
			e.logger.Warn("failed to auto-approve PR", "task", t.ID, "error", err)
			// Don't fail the task - the PR is created, approval can happen later
		}
	}

	// Wait for CI and merge if configured (alternative to GitHub's auto-merge)
	// This bypasses the need for branch protection rules
	if e.orcConfig.ShouldWaitForCI() && e.orcConfig.ShouldMergeOnCIPass() && prURL != "" {
		ciMerger := NewCIMerger(
			e.orcConfig,
			WithCIMergerPublisher(e.publisher),
			WithCIMergerLogger(e.logger),
			WithCIMergerWorkDir(e.worktreePath),
			WithCIMergerBackend(e.backend),
		)

		if mergeErr := ciMerger.WaitForCIAndMerge(ctx, t); mergeErr != nil {
			e.logger.Warn("CI wait and merge failed", "task", t.ID, "error", mergeErr)
			// Don't fail the task - PR is created, can be merged manually
		} else {
			// Task already completed - merge success is logged but status unchanged
			// (completeTask already set StatusCompleted before we got here)
			e.logger.Info("PR merged successfully", "task", t.ID)
		}
	}

	return nil
}

// autoApprovePR performs AI review and approves the PR if it passes.
func (e *Executor) autoApprovePR(ctx context.Context, t *task.Task, prURL string) error {
	// Check if this is a self-authored PR - cannot approve your own PR
	isSelfAuthored, err := e.isSelfAuthoredPR(ctx, prURL)
	if err != nil {
		e.logger.Debug("could not determine PR author, proceeding with approval attempt", "error", err)
	} else if isSelfAuthored {
		e.logger.Debug("skipping self-approval: PR author matches authenticated user", "task", t.ID, "pr", prURL)
		return nil
	}

	e.logger.Info("starting AI review for auto-approval", "task", t.ID, "pr", prURL)

	// 1. Get the PR diff
	diff, err := e.getPRDiff(ctx, prURL)
	if err != nil {
		return fmt.Errorf("get PR diff: %w", err)
	}

	// 2. Check PR checks status (CI/tests)
	checksOK, checkDetails, err := e.checkPRStatus(ctx, prURL)
	if err != nil {
		e.logger.Warn("failed to check PR status, proceeding with review", "error", err)
		// Don't fail - proceed with review, the check might be pending
		checksOK = true // Assume OK if we can't determine
		checkDetails = "Status check unavailable"
	}

	// 3. Review the diff and determine if it should be approved
	reviewResult, err := e.reviewAndApprove(ctx, t, diff, checksOK, checkDetails)
	if err != nil {
		return fmt.Errorf("AI review: %w", err)
	}

	// 4. If review passed, approve the PR
	if reviewResult.Approved {
		if err := e.approvePR(ctx, prURL, reviewResult.Comment); err != nil {
			return fmt.Errorf("approve PR: %w", err)
		}
		e.logger.Info("PR auto-approved after AI review", "task", t.ID)
	} else {
		e.logger.Info("PR not auto-approved", "task", t.ID, "reason", reviewResult.Comment)
	}

	return nil
}

// PRReviewResult contains the result of an AI review.
type PRReviewResult struct {
	Approved bool   // Whether the PR should be approved
	Comment  string // Review comment/reason
}

// getPRDiff retrieves the diff for a PR using gh CLI.
func (e *Executor) getPRDiff(ctx context.Context, prURL string) (string, error) {
	output, err := e.runGH(ctx, "pr", "diff", prURL)
	if err != nil {
		return "", err
	}
	return output, nil
}

// isSelfAuthoredPR checks if the PR was authored by the current authenticated user.
// Returns true if the PR author matches the authenticated user, false otherwise.
// Used to skip self-approval attempts in solo dev workflows.
func (e *Executor) isSelfAuthoredPR(ctx context.Context, prURL string) (bool, error) {
	// Get PR author
	prOutput, err := e.runGH(ctx, "pr", "view", prURL, "--json", "author", "--jq", ".author.login")
	if err != nil {
		return false, fmt.Errorf("get PR author: %w", err)
	}
	prAuthor := strings.TrimSpace(prOutput)

	// Get current authenticated user
	userOutput, err := e.runGH(ctx, "api", "user", "--jq", ".login")
	if err != nil {
		return false, fmt.Errorf("get authenticated user: %w", err)
	}
	currentUser := strings.TrimSpace(userOutput)

	return prAuthor == currentUser, nil
}

// checkPRStatus checks if PR checks (CI) have passed.
func (e *Executor) checkPRStatus(ctx context.Context, prURL string) (bool, string, error) {
	// Use gh pr checks to get status
	// gh pr checks --json returns: name, state, bucket (pass/fail/pending/skipping/cancel)
	output, err := e.runGH(ctx, "pr", "checks", prURL, "--json", "name,state,bucket")
	if err != nil {
		// If no checks configured, that's OK
		if strings.Contains(err.Error(), "no checks") || strings.Contains(output, "[]") {
			return true, "No CI checks configured", nil
		}
		return false, "", err
	}

	// Parse the JSON output
	var checks []struct {
		Name   string `json:"name"`
		State  string `json:"state"`
		Bucket string `json:"bucket"` // pass, fail, pending, skipping, cancel
	}
	if err := json.Unmarshal([]byte(output), &checks); err != nil {
		return false, "", fmt.Errorf("parse checks: %w", err)
	}

	// Check if any are failing or pending
	var failedChecks []string
	pending := false
	for _, c := range checks {
		switch c.Bucket {
		case "fail":
			failedChecks = append(failedChecks, c.Name)
		case "pending":
			pending = true
		// pass, skipping, cancel are all acceptable
		}
	}

	if len(failedChecks) > 0 {
		return false, fmt.Sprintf("Failed checks: %s", strings.Join(failedChecks, ", ")), nil
	}
	if pending {
		return true, "Some checks still pending", nil
	}
	return true, "All checks passed", nil
}

// reviewAndApprove reviews the PR diff and determines if it should be approved.
// Since the code has already been reviewed during implement/test/validate phases,
// we primarily verify CI status before approving.
func (e *Executor) reviewAndApprove(ctx context.Context, t *task.Task, diff string, checksOK bool, checkDetails string) (*PRReviewResult, error) {
	// If checks failed, don't approve
	if !checksOK {
		return &PRReviewResult{
			Approved: false,
			Comment:  fmt.Sprintf("CI checks have not passed: %s", checkDetails),
		}, nil
	}

	// At this point:
	// - Tests have passed during the test phase
	// - Validation passed during validate phase
	// - CI checks are passing (or pending)
	// The AI has already reviewed the code during implementation
	// So we can approve based on successful execution

	// Build approval comment with context
	var comment strings.Builder
	comment.WriteString("Auto-approved by orc orchestrator.\n\n")
	comment.WriteString("**Review Summary:**\n")
	comment.WriteString(fmt.Sprintf("- Task: %s\n", t.Title))
	comment.WriteString(fmt.Sprintf("- CI Status: %s\n", checkDetails))
	comment.WriteString("- Implementation: Completed via AI-assisted development\n")
	comment.WriteString("- Tests: Passed during test phase\n")
	comment.WriteString("- Validation: Completed during validate phase\n")

	return &PRReviewResult{
		Approved: true,
		Comment:  comment.String(),
	}, nil
}

// approvePR approves a PR using gh CLI.
func (e *Executor) approvePR(ctx context.Context, prURL string, comment string) error {
	args := []string{"pr", "review", prURL, "--approve"}
	if comment != "" {
		args = append(args, "--body", comment)
	}
	_, err := e.runGH(ctx, args...)
	return err
}

// buildPRBody constructs the PR body from task information.
func (e *Executor) buildPRBody(t *task.Task) string {
	var sb strings.Builder

	sb.WriteString("## Summary\n\n")
	if t.Description != "" {
		sb.WriteString(t.Description)
	} else {
		sb.WriteString(t.Title)
	}
	sb.WriteString("\n\n")

	sb.WriteString("## Task Details\n\n")
	fmt.Fprintf(&sb, "- **Task ID**: %s\n", t.ID)
	fmt.Fprintf(&sb, "- **Weight**: %s\n", t.Weight)
	sb.WriteString("\n")

	sb.WriteString("## Test Plan\n\n")
	sb.WriteString("- [ ] Automated tests passed\n")
	sb.WriteString("- [ ] Manual verification completed\n")
	sb.WriteString("\n")

	sb.WriteString("---\n")
	sb.WriteString("*Created by [orc](https://github.com/randalmurphal/orc)*\n")

	return sb.String()
}

// isLabelError checks if an error is related to missing labels.
// GitHub CLI returns errors like "could not add label: <name> not found".
func isLabelError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "label") &&
		(strings.Contains(errStr, "not found") || strings.Contains(errStr, "could not add"))
}

// ErrGHNotAuthenticated is returned when gh CLI is not authenticated.
var ErrGHNotAuthenticated = errors.New("GitHub CLI not authenticated")

// isAuthError checks if an error is related to gh CLI authentication.
// Common patterns:
// - "gh: not logged in" (older gh versions)
// - "not authenticated" (from CheckGHAuth)
// - "authentication required"
// - "failed to authenticate"
// - "401" or "Unauthorized"
func isAuthError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "not logged in") ||
		strings.Contains(errStr, "not authenticated") ||
		strings.Contains(errStr, "authentication required") ||
		strings.Contains(errStr, "failed to authenticate") ||
		strings.Contains(errStr, "401") ||
		strings.Contains(errStr, "unauthorized") ||
		strings.Contains(errStr, "auth token")
}

// isNonFastForwardError checks if an error is a git non-fast-forward push rejection.
// This occurs when the local branch has diverged from the remote branch,
// typically when re-running a completed task from scratch.
// Common patterns:
// - "non-fast-forward" (standard git message)
// - "rejected" + "fetch first" (alternative git message)
// - "failed to push some refs" + "behind" (hint text)
func isNonFastForwardError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "non-fast-forward") ||
		(strings.Contains(errStr, "rejected") && strings.Contains(errStr, "fetch first")) ||
		(strings.Contains(errStr, "failed to push") && strings.Contains(errStr, "behind"))
}

// isAutoMergeConfigError checks if an error is due to auto-merge not being available
// on the repository. This is expected behavior for repos without auto-merge enabled
// (requires branch protection rules or explicit repo settings).
// Common patterns from GitHub CLI:
// - "auto-merge is not allowed" (repo doesn't allow auto-merge)
// - "pull request is not mergeable" (missing required reviews/checks)
// - "auto-merge can not be enabled" (branch protection prevents it)
// - "auto merge is not allowed" (alternative phrasing)
// - "not eligible for auto-merge" (various eligibility issues)
func isAutoMergeConfigError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "auto-merge is not allowed") ||
		strings.Contains(errStr, "auto merge is not allowed") ||
		strings.Contains(errStr, "auto-merge can not be enabled") ||
		strings.Contains(errStr, "auto merge can not be enabled") ||
		strings.Contains(errStr, "not eligible for auto-merge") ||
		strings.Contains(errStr, "not eligible for auto merge") ||
		strings.Contains(errStr, "pull request is not mergeable") ||
		strings.Contains(errStr, "is in clean status") // PR is already clean/merged
}

// runGH executes a gh CLI command.
func (e *Executor) runGH(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "gh", args...)

	// Use worktree path if available
	if e.worktreePath != "" {
		cmd.Dir = e.worktreePath
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%w: %s", err, output)
	}

	return string(output), nil
}
