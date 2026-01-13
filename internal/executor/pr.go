// Package executor provides PR/merge completion actions for task execution.
package executor

import (
	"context"
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

	// Fail on conflict if configured
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

// ErrDirectMergeBlocked is returned when direct merge to a protected branch is blocked.
var ErrDirectMergeBlocked = errors.New("direct merge to protected branch blocked")

// directMerge merges the task branch directly into the target branch.
// NOTE: This operation is BLOCKED for protected branches (main, master, develop, release).
// Use the PR workflow instead for protected branches.
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

	// Checkout target branch
	if err := gitOps.Context().Checkout(cfg.TargetBranch); err != nil {
		return fmt.Errorf("checkout %s: %w", cfg.TargetBranch, err)
	}

	// Merge task branch
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
		return fmt.Errorf("push branch: %w", err)
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
		e.logger.Warn("PR labels not found on repository, creating PR without labels",
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
		if t.Metadata == nil {
			t.Metadata = make(map[string]string)
		}
		t.Metadata["pr_url"] = prURL
		if saveErr := t.SaveTo(e.currentTaskDir); saveErr != nil {
			e.logger.Error("failed to save task with PR URL", "error", saveErr)
		}
	}

	e.logger.Info("created pull request", "task", t.ID, "url", prURL)

	// Enable auto-merge if configured
	if cfg.PR.AutoMerge && prURL != "" {
		if _, err := e.runGH(ctx, "pr", "merge", prURL, "--auto", "--squash"); err != nil {
			if isAuthError(err) {
				e.logger.Warn("failed to enable auto-merge due to auth issue",
					"error", err,
					"hint", "run 'gh auth login' to fix")
			} else {
				e.logger.Warn("failed to enable auto-merge", "error", err)
			}
		} else {
			e.logger.Info("enabled auto-merge", "task", t.ID)
		}
	}

	return nil
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
