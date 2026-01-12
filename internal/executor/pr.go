// Package executor provides PR/merge completion actions for task execution.
package executor

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

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

// syncWithTarget rebases the task branch onto the target branch.
func (e *Executor) syncWithTarget(ctx context.Context, t *task.Task) error {
	cfg := e.orcConfig.Completion
	targetBranch := cfg.TargetBranch
	if targetBranch == "" {
		targetBranch = "main"
	}

	// Use worktree git if available
	gitOps := e.gitOps
	if e.worktreeGit != nil {
		gitOps = e.worktreeGit
	}

	e.logger.Info("syncing with target branch", "target", targetBranch)

	// Fetch latest from remote
	if err := gitOps.Fetch("origin"); err != nil {
		e.logger.Warn("fetch failed, continuing anyway", "error", err)
	}

	// Rebase onto target
	target := "origin/" + targetBranch
	if err := gitOps.Rebase(target); err != nil {
		return fmt.Errorf("rebase onto %s: %w", target, err)
	}

	e.logger.Info("synced with target branch", "target", targetBranch)
	return nil
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

	// Push to remote - use PushUnsafe since we've already validated the branch
	// and this is a non-protected branch
	if err := gitOps.PushUnsafe("origin", cfg.TargetBranch, false); err != nil {
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
	for _, label := range cfg.PR.Labels {
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
	if err != nil {
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
			e.logger.Warn("failed to enable auto-merge", "error", err)
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
