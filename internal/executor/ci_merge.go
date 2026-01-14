// Package executor provides CI polling and auto-merge functionality.
package executor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/events"
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
	publisher *EventPublisher
	logger    *slog.Logger
	workDir   string
}

// CIMergerOption configures a CIMerger.
type CIMergerOption func(*CIMerger)

// WithCIMergerPublisher sets the event publisher.
func WithCIMergerPublisher(p events.Publisher) CIMergerOption {
	return func(m *CIMerger) { m.publisher = NewEventPublisher(p) }
}

// WithCIMergerLogger sets the logger.
func WithCIMergerLogger(l *slog.Logger) CIMergerOption {
	return func(m *CIMerger) { m.logger = l }
}

// WithCIMergerWorkDir sets the working directory for gh commands.
func WithCIMergerWorkDir(dir string) CIMergerOption {
	return func(m *CIMerger) { m.workDir = dir }
}

// NewCIMerger creates a new CIMerger.
func NewCIMerger(cfg *config.Config, opts ...CIMergerOption) *CIMerger {
	m := &CIMerger{
		config:    cfg,
		publisher: NewEventPublisher(nil),
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

// WaitForCIAndMerge waits for CI checks to pass, then merges the PR.
// This is the main entry point for the auto-merge flow after finalize.
//
// Flow:
// 1. Push finalize changes if any
// 2. Poll CI checks until all pass (or timeout)
// 3. Merge PR directly with gh pr merge
func (m *CIMerger) WaitForCIAndMerge(ctx context.Context, t *task.Task) error {
	if !m.config.ShouldWaitForCI() {
		m.logger.Debug("CI wait disabled, skipping", "task", t.ID)
		return nil
	}

	prURL := t.GetPRURL()
	if prURL == "" {
		m.logger.Debug("no PR URL found, skipping CI wait", "task", t.ID)
		return nil
	}

	m.logger.Info("starting CI wait and merge flow",
		"task", t.ID,
		"pr", prURL,
		"timeout", m.config.CITimeout(),
		"poll_interval", m.config.CIPollInterval(),
	)

	// Publish initial progress
	m.publishProgress(t.ID, "Waiting for CI checks to pass...")

	// Wait for CI
	result, err := m.WaitForCI(ctx, prURL, t.ID)
	if err != nil {
		return err
	}

	// Check if we should merge
	if !m.config.ShouldMergeOnCIPass() {
		m.logger.Info("CI checks passed, merge_on_ci_pass disabled",
			"task", t.ID,
			"status", result.Status,
		)
		m.publishProgress(t.ID, "CI checks passed. Auto-merge disabled.")
		return nil
	}

	// Merge the PR
	m.publishProgress(t.ID, "CI checks passed. Merging PR...")

	if err := m.MergePR(ctx, prURL, t); err != nil {
		return fmt.Errorf("merge PR: %w", err)
	}

	m.publishProgress(t.ID, "PR merged successfully!")
	m.logger.Info("PR merged successfully",
		"task", t.ID,
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
func (m *CIMerger) MergePR(ctx context.Context, prURL string, t *task.Task) error {
	method := m.config.MergeMethod()

	// Build merge command
	args := []string{"pr", "merge", prURL}

	switch method {
	case "squash":
		args = append(args, "--squash")
	case "merge":
		args = append(args, "--merge")
	case "rebase":
		args = append(args, "--rebase")
	default:
		args = append(args, "--squash") // Default to squash
	}

	// Add delete branch if configured
	if m.config.Completion.DeleteBranch {
		args = append(args, "--delete-branch")
	}

	m.logger.Info("merging PR",
		"task", t.ID,
		"pr", prURL,
		"method", method,
		"delete_branch", m.config.Completion.DeleteBranch,
	)

	output, err := m.runGH(ctx, args...)
	if err != nil {
		return fmt.Errorf("gh pr merge: %w\nOutput: %s", err, output)
	}

	// Update task with merge info
	t.SetMergedInfo(prURL, m.config.Completion.TargetBranch)
	if saveErr := t.Save(); saveErr != nil {
		m.logger.Warn("failed to save task after merge", "error", saveErr)
	}

	return nil
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
