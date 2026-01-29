// Package executor provides CI polling and auto-merge functionality.
package executor

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/hosting"
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
	provider  hosting.Provider
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

// WithCIMergerWorkDir sets the working directory for git commands.
func WithCIMergerWorkDir(dir string) CIMergerOption {
	return func(m *CIMerger) { m.workDir = dir }
}

// WithCIMergerBackend sets the storage backend for task persistence.
func WithCIMergerBackend(b storage.Backend) CIMergerOption {
	return func(m *CIMerger) { m.backend = b }
}

// WithCIMergerHostingProvider sets the hosting provider for CI and merge operations.
func WithCIMergerHostingProvider(p hosting.Provider) CIMergerOption {
	return func(m *CIMerger) { m.provider = p }
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
// 3. Merge PR via hosting provider
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

	// Wait for CI using the task's branch as the ref
	result, err := m.WaitForCI(ctx, t.Branch, t.Id)
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

	if err := m.MergePR(ctx, t); err != nil {
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
func (m *CIMerger) WaitForCI(ctx context.Context, ref, taskID string) (*CICheckResult, error) {
	timeout := m.config.CITimeout()
	pollInterval := m.config.CIPollInterval()

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	// Initial check
	result, err := m.CheckCIStatus(ctx, ref)
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

			result, err = m.CheckCIStatus(ctx, ref)
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

// CheckCIStatus checks the current status of CI checks for a git ref (branch).
func (m *CIMerger) CheckCIStatus(ctx context.Context, ref string) (*CICheckResult, error) {
	if m.provider == nil {
		return nil, fmt.Errorf("hosting provider not configured")
	}

	checks, err := m.provider.GetCheckRuns(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("get check runs: %w", err)
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
		switch c.Status {
		case "completed":
			switch c.Conclusion {
			case "success", "neutral", "skipped":
				result.PassedChecks++
			default:
				result.FailedChecks++
				result.FailedNames = append(result.FailedNames, c.Name)
			}
		default:
			result.PendingChecks++
			result.PendingNames = append(result.PendingNames, c.Name)
		}
	}

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

// MergePR merges a PR using the configured merge method via the hosting provider.
//
// Implements retry logic for "Base branch was modified" errors:
// 1. Attempt merge
// 2. If retryable error:
//   - Wait with exponential backoff (2^attempt seconds, max 8s)
//   - Fetch and rebase onto target branch
//   - Push rebased branch (force-with-lease)
//   - Retry merge (up to 3 attempts)
//
// 3. If rebase has conflicts or max retries exceeded, return ErrMergeFailed
func (m *CIMerger) MergePR(ctx context.Context, t *orcv1.Task) error {
	if m.provider == nil {
		return fmt.Errorf("hosting provider not configured")
	}

	const maxRetries = 3

	method := m.config.MergeMethod()
	if method == "" {
		method = "squash"
	}

	prNumber := int(0)
	if t.Pr != nil && t.Pr.Number != nil {
		prNumber = int(*t.Pr.Number)
	}
	if prNumber == 0 {
		return fmt.Errorf("task has no PR number")
	}

	m.logger.Info("merging PR",
		"task", t.Id,
		"pr_number", prNumber,
		"method", method,
		"delete_branch", m.config.Completion.DeleteBranch,
	)

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
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

			if err := m.rebaseOnTarget(ctx, t); err != nil {
				m.logger.Error("rebase before merge retry failed",
					"task", t.Id,
					"attempt", attempt,
					"error", err,
				)
				return fmt.Errorf("%w: rebase failed during retry: %v", ErrMergeFailed, err)
			}
		}

		err := m.provider.MergePR(ctx, prNumber, hosting.PRMergeOptions{
			Method:       method,
			DeleteBranch: m.config.Completion.DeleteBranch,
		})
		if err != nil {
			errStr := strings.ToLower(err.Error())
			if strings.Contains(errStr, "base branch was modified") || strings.Contains(errStr, "405") {
				lastErr = err
				m.logger.Warn("merge failed with retryable error",
					"task", t.Id,
					"attempt", attempt,
					"error", err,
				)
				continue
			}
			return fmt.Errorf("merge PR: %w", err)
		}

		// Merge succeeded - update task with merge info
		prURL := task.GetPRURLProto(t)
		task.SetMergedInfoProto(t, prURL, m.config.Completion.TargetBranch)
		if m.backend != nil {
			if saveErr := m.backend.SaveTask(t); saveErr != nil {
				m.logger.Warn("failed to save task after merge", "error", saveErr)
			}
		}

		m.logger.Info("PR merged successfully", "task", t.Id, "pr_number", prNumber)
		return nil
	}

	return fmt.Errorf("%w: max retries (%d) exceeded: %v", ErrMergeFailed, maxRetries, lastErr)
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
	fetchOutput, fetchErr := m.runGitCmd(ctx, "fetch", "origin", targetBranch)
	if fetchErr != nil {
		m.logger.Warn("fetch failed", "error", fetchErr, "output", fetchOutput)
		// Continue anyway - we might be able to rebase without fresh fetch
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

// publishProgress publishes a progress message.
func (m *CIMerger) publishProgress(taskID, message string) {
	m.publisher.Transcript(taskID, "ci_merge", 0, "progress", message)
}
