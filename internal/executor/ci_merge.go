// Package executor provides CI polling and auto-merge functionality.
package executor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// CIStatus represents the current state of CI checks.
type CIStatus string

const (
	// CIStatusPending indicates checks are still running.
	CIStatusPending CIStatus = "pending"
	// CIStatusPassed indicates all checks passed.
	CIStatusPassed CIStatus = "passed"
	// CIStatusFailed indicates one or more checks failed.
	CIStatusFailed CIStatus = "failed"
	// CIStatusNoChecks indicates no CI checks are configured.
	CIStatusNoChecks CIStatus = "no_checks"
)

// CICheckResult contains the result of checking CI status.
type CICheckResult struct {
	// Status is the overall CI status.
	Status CIStatus
	// TotalChecks is the number of CI checks.
	TotalChecks int
	// PassedChecks is the number of passed checks.
	PassedChecks int
	// FailedChecks is the number of failed checks.
	FailedChecks int
	// PendingChecks is the number of pending checks.
	PendingChecks int
	// FailedNames lists the names of failed checks.
	FailedNames []string
	// PendingNames lists the names of pending checks.
	PendingNames []string
	// Details contains additional status information.
	Details string
}

// CIMerger handles CI polling and auto-merge operations.
type CIMerger struct {
	config    *config.Config
	publisher *events.PublishHelper
	logger    *slog.Logger
	workDir   string
	backend   storage.Backend
}

// CIMergerOption configures a CIMerger.
type CIMergerOption func(*CIMerger)

// WithCIMergerPublisher sets the event publisher.
func WithCIMergerPublisher(p events.Publisher) CIMergerOption {
	return func(m *CIMerger) { m.publisher = events.NewPublishHelper(p) }
}

// WithCIMergerLogger sets the logger.
func WithCIMergerLogger(l *slog.Logger) CIMergerOption {
	return func(m *CIMerger) { m.logger = l }
}

// WithCIMergerWorkDir sets the working directory for gh commands.
func WithCIMergerWorkDir(dir string) CIMergerOption {
	return func(m *CIMerger) { m.workDir = dir }
}

// WithCIMergerBackend sets the storage backend for task persistence.
func WithCIMergerBackend(b storage.Backend) CIMergerOption {
	return func(m *CIMerger) { m.backend = b }
}

// NewCIMerger creates a new CIMerger.
func NewCIMerger(cfg *config.Config, opts ...CIMergerOption) *CIMerger {
	m := &CIMerger{
		config:    cfg,
		publisher: events.NewPublishHelper(nil),
		logger:    slog.Default(),
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// ErrCITimeout is returned when CI checks timeout.
var ErrCITimeout = errors.New("CI checks timed out")

// ErrCIFailed is returned when CI checks fail.
var ErrCIFailed = errors.New("CI checks failed")

// ErrMergeFailed is returned when PR merge fails and cannot be retried.
// This includes both exhausted retries and non-retryable errors.
var ErrMergeFailed = errors.New("PR merge failed")

// WaitForCIAndMerge waits for CI checks to pass, then merges the PR.
// This is the main entry point for the auto-merge flow after finalize.
//
// Flow:
// 1. Push finalize changes if any
// 2. Poll CI checks until all pass (or timeout)
// 3. Merge PR directly with gh pr merge
func (m *CIMerger) WaitForCIAndMerge(ctx context.Context, t *orcv1.Task) error {
	if !m.config.ShouldWaitForCI() {
		m.logger.Debug("CI wait disabled, skipping", "task", t.Id)
		return nil
	}

	prURL := task.GetPRURLProto(t)
	if prURL == "" {
		m.logger.Debug("no PR URL found, skipping CI wait", "task", t.Id)
		return nil
	}

	m.logger.Info("starting CI wait and merge flow",
		"task", t.Id,
		"pr", prURL,
		"timeout", m.config.CITimeout(),
		"poll_interval", m.config.CIPollInterval(),
	)

	// Publish initial progress
	m.publishProgress(t.Id, "Waiting for CI checks to pass...")

	// Wait for CI
	result, err := m.WaitForCI(ctx, prURL, t.Id)
	if err != nil {
		return err
	}

	// Check if we should merge
	if !m.config.ShouldMergeOnCIPass() {
		m.logger.Info("CI checks passed, merge_on_ci_pass disabled",
			"task", t.Id,
			"status", result.Status,
		)
		m.publishProgress(t.Id, "CI checks passed. Auto-merge disabled.")
		return nil
	}

	// Merge the PR
	m.publishProgress(t.Id, "CI checks passed. Merging PR...")

	if err := m.MergePR(ctx, prURL, t); err != nil {
		return fmt.Errorf("merge PR: %w", err)
	}

	m.publishProgress(t.Id, "PR merged successfully!")
	m.logger.Info("PR merged successfully",
		"task", t.Id,
		"pr", prURL,
		"merge_method", m.config.MergeMethod(),
	)

	return nil
}


// WaitForCI polls CI checks until they pass or timeout.
func (m *CIMerger) WaitForCI(ctx context.Context, prURL, taskID string) (*CICheckResult, error) {
	timeout := m.config.CITimeout()
	pollInterval := m.config.CIPollInterval()

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	// Initial check
	result, err := m.CheckCIStatus(ctx, prURL)
	if err != nil {
		return nil, fmt.Errorf("check CI status: %w", err)
	}

	// Handle immediate completion
	switch result.Status {
	case CIStatusPassed, CIStatusNoChecks:
		m.logger.Info("CI checks already passed", "task", taskID, "status", result.Status)
		return result, nil
	case CIStatusFailed:
		m.logger.Error("CI checks failed",
			"task", taskID,
			"failed_checks", result.FailedNames,
		)
		return result, fmt.Errorf("%w: %v", ErrCIFailed, result.FailedNames)
	}

	// Poll loop
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()

		case <-ticker.C:
			if time.Now().After(deadline) {
				m.logger.Error("CI checks timed out",
					"task", taskID,
					"timeout", timeout,
					"pending_checks", result.PendingNames,
				)
				return result, fmt.Errorf("%w after %v: %d checks still pending (%v)",
					ErrCITimeout, timeout, result.PendingChecks, result.PendingNames)
			}

			result, err = m.CheckCIStatus(ctx, prURL)
			if err != nil {
				m.logger.Warn("failed to check CI status, retrying",
					"task", taskID,
					"error", err,
				)
				continue
			}

			m.logger.Debug("CI status check",
				"task", taskID,
				"status", result.Status,
				"passed", result.PassedChecks,
				"pending", result.PendingChecks,
				"failed", result.FailedChecks,
			)

			switch result.Status {
			case CIStatusPassed, CIStatusNoChecks:
				m.logger.Info("CI checks passed", "task", taskID)
				m.publishProgress(taskID,
					fmt.Sprintf("CI checks passed (%d/%d)", result.PassedChecks, result.TotalChecks))
				return result, nil

			case CIStatusFailed:
				m.logger.Error("CI checks failed",
					"task", taskID,
					"failed_checks", result.FailedNames,
				)
				m.publishProgress(taskID,
					fmt.Sprintf("CI checks failed: %v", result.FailedNames))
				return result, fmt.Errorf("%w: %v", ErrCIFailed, result.FailedNames)

			case CIStatusPending:
				m.publishProgress(taskID,
					fmt.Sprintf("Waiting for CI... %d/%d passed, %d pending",
						result.PassedChecks, result.TotalChecks, result.PendingChecks))
			}
		}
	}
}

// CheckCIStatus checks the current status of CI checks for a PR.
func (m *CIMerger) CheckCIStatus(ctx context.Context, prURL string) (*CICheckResult, error) {
	// Use gh pr checks to get status
	output, err := m.runGH(ctx, "pr", "checks", prURL, "--json", "name,state,bucket")
	if err != nil {
		// If no checks configured, that's OK
		if strings.Contains(err.Error(), "no checks") || strings.Contains(output, "[]") {
			return &CICheckResult{
				Status:  CIStatusNoChecks,
				Details: "No CI checks configured",
			}, nil
		}
		return nil, fmt.Errorf("get PR checks: %w", err)
	}

	// Handle empty response
	output = strings.TrimSpace(output)
	if output == "" || output == "[]" {
		return &CICheckResult{
			Status:  CIStatusNoChecks,
			Details: "No CI checks configured",
		}, nil
	}

	// Parse the JSON output
	var checks []struct {
		Name   string `json:"name"`
		State  string `json:"state"`
		Bucket string `json:"bucket"` // pass, fail, pending, skipping, cancel
	}
	if err := json.Unmarshal([]byte(output), &checks); err != nil {
		return nil, fmt.Errorf("parse checks: %w", err)
	}

	if len(checks) == 0 {
		return &CICheckResult{
			Status:  CIStatusNoChecks,
			Details: "No CI checks configured",
		}, nil
	}

	result := &CICheckResult{
		TotalChecks: len(checks),
	}

	for _, c := range checks {
		switch c.Bucket {
		case "pass", "skipping":
			result.PassedChecks++
		case "fail", "cancel":
			result.FailedChecks++
			result.FailedNames = append(result.FailedNames, c.Name)
		case "pending":
			result.PendingChecks++
			result.PendingNames = append(result.PendingNames, c.Name)
		default:
			// Unknown state, treat as pending
			result.PendingChecks++
			result.PendingNames = append(result.PendingNames, c.Name)
		}
	}

	// Determine overall status
	if result.FailedChecks > 0 {
		result.Status = CIStatusFailed
		result.Details = fmt.Sprintf("%d/%d checks failed", result.FailedChecks, result.TotalChecks)
	} else if result.PendingChecks > 0 {
		result.Status = CIStatusPending
		result.Details = fmt.Sprintf("%d/%d checks pending", result.PendingChecks, result.TotalChecks)
	} else {
		result.Status = CIStatusPassed
		result.Details = fmt.Sprintf("%d/%d checks passed", result.PassedChecks, result.TotalChecks)
	}

	return result, nil
}

// MergePR merges a PR using the configured merge method.
// Uses the GitHub REST API directly to avoid local git checkout issues
// when the target branch is checked out in another worktree.
//
// Implements retry logic for "Base branch was modified" errors (HTTP 405):
// 1. Attempt merge
// 2. If 405 received:
//   - Wait with exponential backoff (2^attempt seconds, max 8s)
//   - Fetch and rebase onto target branch
//   - Push rebased branch (force-with-lease)
//   - Retry merge (up to 3 attempts)
//
// 3. If rebase has conflicts or max retries exceeded, return ErrMergeFailed
func (m *CIMerger) MergePR(ctx context.Context, prURL string, t *orcv1.Task) error {
	const maxRetries = 3

	method := m.config.MergeMethod()

	// Extract owner, repo, and PR number from the URL
	owner, repo, prNumber, err := parsePRURL(prURL)
	if err != nil {
		return fmt.Errorf("parse PR URL: %w", err)
	}

	m.logger.Info("merging PR via API",
		"task", t.Id,
		"pr", prURL,
		"owner", owner,
		"repo", repo,
		"pr_number", prNumber,
		"method", method,
		"delete_branch", m.config.Completion.DeleteBranch,
	)

	// GitHub API uses "merge", "squash", or "rebase" for merge_method
	mergeMethod := method
	if mergeMethod == "" {
		mergeMethod = "squash" // Default to squash
	}

	// Retry loop for handling "Base branch was modified" errors
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 2s, 4s, 8s
			backoff := min(time.Duration(1<<attempt)*time.Second, 8*time.Second)
			m.logger.Info("waiting before merge retry",
				"attempt", attempt,
				"backoff", backoff,
				"task", t.Id,
			)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}

			// Rebase before retry
			if err := m.rebaseOnTarget(ctx, t); err != nil {
				// If rebase fails (conflicts or other error), return ErrMergeFailed
				m.logger.Error("rebase before merge retry failed",
					"task", t.Id,
					"attempt", attempt,
					"error", err,
				)
				return fmt.Errorf("%w: rebase failed during retry: %v", ErrMergeFailed, err)
			}
		}

		// Call GitHub API to merge the PR
		// PUT /repos/{owner}/{repo}/pulls/{pull_number}/merge
		apiPath := fmt.Sprintf("/repos/%s/%s/pulls/%d/merge", owner, repo, prNumber)
		output, err := m.runGH(ctx, "api", "-X", "PUT", apiPath, "-f", fmt.Sprintf("merge_method=%s", mergeMethod))
		if err != nil {
			// Check if this is a retryable error
			if isRetryableMergeError(err, output) {
				lastErr = err
				m.logger.Warn("merge failed with retryable error",
					"task", t.Id,
					"attempt", attempt,
					"error", err,
				)
				continue
			}

			// Non-retryable error - check if it's a validation error
			if isValidationMergeError(err, output) {
				return fmt.Errorf("%w: merge validation failed: %v\nOutput: %s", ErrMergeFailed, err, output)
			}

			// Other non-retryable errors
			return fmt.Errorf("merge PR via API: %w\nOutput: %s", err, output)
		}

		// Parse the response to get the merge commit SHA
		var mergeResponse struct {
			SHA     string `json:"sha"`
			Merged  bool   `json:"merged"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal([]byte(output), &mergeResponse); err != nil {
			m.logger.Warn("failed to parse merge response", "error", err, "output", output)
		} else if mergeResponse.SHA != "" {
			if t.Pr != nil {
				sha := mergeResponse.SHA
				t.Pr.MergeCommitSha = &sha
			}
			m.logger.Info("PR merged", "sha", mergeResponse.SHA)
		}

		// Delete the branch if configured
		if m.config.Completion.DeleteBranch {
			if err := m.deleteBranch(ctx, owner, repo, t.Branch); err != nil {
				// Log but don't fail - the merge succeeded
				m.logger.Warn("failed to delete branch after merge",
					"branch", t.Branch,
					"error", err,
				)
			}
		}

		// Update task with merge info
		task.SetMergedInfoProto(t, prURL, m.config.Completion.TargetBranch)
		if m.backend != nil {
			if saveErr := m.backend.SaveTaskProto(t); saveErr != nil {
				m.logger.Warn("failed to save task after merge", "error", saveErr)
			}
		}

		return nil
	}

	// All retries exhausted
	return fmt.Errorf("%w: max retries (%d) exceeded: %v", ErrMergeFailed, maxRetries, lastErr)
}

// isRetryableMergeError checks if a merge error can be retried.
// HTTP 405 "Base branch was modified" is the primary retryable case.
func isRetryableMergeError(err error, output string) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	outputStr := strings.ToLower(output)

	// GitHub returns 405 "Method Not Allowed" with "Base branch was modified"
	// when another PR merged first, modifying the base branch
	return (strings.Contains(errStr, "405") || strings.Contains(outputStr, "405")) &&
		(strings.Contains(errStr, "base branch was modified") ||
			strings.Contains(outputStr, "base branch was modified") ||
			strings.Contains(errStr, "review and try the merge again") ||
			strings.Contains(outputStr, "review and try the merge again"))
}

// isValidationMergeError checks if a merge error is a validation failure.
// HTTP 422 typically indicates validation errors that won't be fixed by retry.
func isValidationMergeError(err error, output string) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	outputStr := strings.ToLower(output)

	// 422 Unprocessable Entity - validation failed (conflicts, required checks, etc.)
	return strings.Contains(errStr, "422") || strings.Contains(outputStr, "422")
}

// rebaseOnTarget rebases the task branch onto the target branch.
// This is called before merge retries to incorporate upstream changes.
func (m *CIMerger) rebaseOnTarget(ctx context.Context, t *orcv1.Task) error {
	targetBranch := m.config.Completion.TargetBranch
	if targetBranch == "" {
		targetBranch = "main"
	}

	m.logger.Info("rebasing onto target branch before merge retry",
		"task", t.Id,
		"branch", t.Branch,
		"target", targetBranch,
	)

	// Fetch latest from origin
	if _, err := m.runGH(ctx, "api", "-X", "GET", "/rate_limit"); err == nil {
		// gh is available, use git commands via shell
		fetchOutput, fetchErr := m.runGitCmd(ctx, "fetch", "origin", targetBranch)
		if fetchErr != nil {
			m.logger.Warn("fetch failed", "error", fetchErr, "output", fetchOutput)
			// Continue anyway - we might be able to rebase without fresh fetch
		}
	}

	// Attempt rebase onto origin/target
	target := "origin/" + targetBranch
	rebaseOutput, rebaseErr := m.runGitCmd(ctx, "rebase", target)
	if rebaseErr != nil {
		// Check for conflicts
		diffOutput, _ := m.runGitCmd(ctx, "diff", "--name-only", "--diff-filter=U")
		if diffOutput != "" {
			// Abort the failed rebase
			_, _ = m.runGitCmd(ctx, "rebase", "--abort")
			return fmt.Errorf("rebase conflicts detected in files: %s", strings.TrimSpace(diffOutput))
		}

		// Abort and return other rebase errors
		_, _ = m.runGitCmd(ctx, "rebase", "--abort")
		return fmt.Errorf("rebase failed: %s\nOutput: %s", rebaseErr, rebaseOutput)
	}

	// Push the rebased branch with force-with-lease for safety
	pushOutput, pushErr := m.runGitCmd(ctx, "push", "--force-with-lease", "origin", t.Branch)
	if pushErr != nil {
		return fmt.Errorf("push after rebase failed: %s\nOutput: %s", pushErr, pushOutput)
	}

	m.logger.Info("successfully rebased and pushed",
		"task", t.Id,
		"branch", t.Branch,
		"target", targetBranch,
	)

	return nil
}

// runGitCmd executes a git command in the working directory.
func (m *CIMerger) runGitCmd(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	if m.workDir != "" {
		cmd.Dir = m.workDir
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("%w: %s", err, output)
	}

	return string(output), nil
}

// deleteBranch deletes a branch via the GitHub API.
func (m *CIMerger) deleteBranch(ctx context.Context, owner, repo, branch string) error {
	// Strip the "refs/heads/" prefix if present, or add nothing if it's a simple branch name
	branchName := strings.TrimPrefix(branch, "refs/heads/")

	// DELETE /repos/{owner}/{repo}/git/refs/heads/{branch}
	apiPath := fmt.Sprintf("/repos/%s/%s/git/refs/heads/%s", owner, repo, branchName)
	output, err := m.runGH(ctx, "api", "-X", "DELETE", apiPath)
	if err != nil {
		return fmt.Errorf("delete branch: %w\nOutput: %s", err, output)
	}
	m.logger.Info("deleted branch", "branch", branchName)
	return nil
}

// parsePRURL extracts owner, repo, and PR number from a GitHub PR URL.
// Supports formats like:
//   - https://github.com/owner/repo/pull/123
//   - github.com/owner/repo/pull/123
func parsePRURL(prURL string) (owner, repo string, prNumber int, err error) {
	// Regex to match GitHub PR URLs
	// Captures: owner, repo, PR number
	pattern := regexp.MustCompile(`(?:https?://)?github\.com/([^/]+)/([^/]+)/pull/(\d+)`)
	matches := pattern.FindStringSubmatch(prURL)
	if len(matches) != 4 {
		return "", "", 0, fmt.Errorf("invalid PR URL format: %s", prURL)
	}

	prNumber, err = strconv.Atoi(matches[3])
	if err != nil {
		return "", "", 0, fmt.Errorf("invalid PR number in URL: %s", prURL)
	}

	return matches[1], matches[2], prNumber, nil
}

// publishProgress publishes a progress message.
func (m *CIMerger) publishProgress(taskID, message string) {
	m.publisher.Transcript(taskID, "ci_merge", 0, "progress", message)
}

// runGH executes a gh CLI command.
func (m *CIMerger) runGH(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "gh", args...)
	if m.workDir != "" {
		cmd.Dir = m.workDir
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("%w: %s", err, output)
	}

	return string(output), nil
}

