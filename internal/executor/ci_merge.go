// Package executor provides CI merge functionality for completing PR merges after finalize.
package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/task"
)

// CIMerger handles waiting for CI and merging PRs after finalize.
type CIMerger struct {
	cfg        *config.Config
	logger     *slog.Logger
	workingDir string
	taskDir    string
}

// NewCIMerger creates a new CI merger.
func NewCIMerger(cfg *config.Config, logger *slog.Logger, workingDir, taskDir string) *CIMerger {
	if logger == nil {
		logger = slog.Default()
	}
	return &CIMerger{
		cfg:        cfg,
		logger:     logger,
		workingDir: workingDir,
		taskDir:    taskDir,
	}
}

// CIMergeResult represents the outcome of a CI wait and merge operation.
type CIMergeResult struct {
	Pushed       bool   // Changes were pushed
	CIPassed     bool   // CI checks passed
	Merged       bool   // PR was merged
	MergeCommit  string // SHA of merge commit (if merged)
	Error        string // Error message (if failed)
	CIDetails    string // Details about CI status
	TimedOut     bool   // Whether CI timed out
	SkippedMerge bool   // Merge skipped due to config
}

// WaitForCIAndMerge waits for CI checks to pass and then merges the PR.
// This is called after finalize completes successfully.
func (m *CIMerger) WaitForCIAndMerge(ctx context.Context, t *task.Task) (*CIMergeResult, error) {
	result := &CIMergeResult{}

	// Check if wait_for_ci is enabled
	if !m.cfg.ShouldWaitForCI() {
		m.logger.Debug("wait_for_ci disabled, skipping", "task", t.ID)
		result.SkippedMerge = true
		return result, nil
	}

	prURL := ""
	if t.PR != nil {
		prURL = t.PR.URL
	}
	if prURL == "" {
		return result, fmt.Errorf("no PR URL to wait for")
	}

	m.logger.Info("waiting for CI checks to pass", "task", t.ID, "pr", prURL)

	// Push any finalize changes first
	if err := m.pushChanges(ctx, t); err != nil {
		m.logger.Warn("failed to push finalize changes", "error", err)
		// Continue anyway - changes might already be pushed
	} else {
		result.Pushed = true
	}

	// Wait for CI with timeout
	timeout := m.cfg.GetCITimeout()
	pollInterval := 30 * time.Second

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}

		ciResult, err := m.checkCIStatus(ctx, prURL)
		if err != nil {
			m.logger.Warn("failed to check CI status", "error", err)
			time.Sleep(pollInterval)
			continue
		}

		result.CIDetails = ciResult.Details

		if ciResult.AllPassed {
			m.logger.Info("CI checks passed", "task", t.ID)
			result.CIPassed = true

			if m.cfg.ShouldMergeOnCIPass() {
				mergeCommit, err := m.mergePR(ctx, t, prURL)
				if err != nil {
					result.Error = err.Error()
					return result, err
				}
				result.Merged = true
				result.MergeCommit = mergeCommit
			} else {
				result.SkippedMerge = true
			}
			return result, nil
		}

		if ciResult.AnyFailed {
			m.logger.Error("CI checks failed", "task", t.ID, "failed", ciResult.FailedNames)
			result.Error = fmt.Sprintf("CI checks failed: %s", strings.Join(ciResult.FailedNames, ", "))
			return result, fmt.Errorf("%s", result.Error)
		}

		m.logger.Debug("CI checks pending, waiting...",
			"task", t.ID,
			"timeout_remaining", time.Until(deadline).Round(time.Second),
		)
		time.Sleep(pollInterval)
	}

	result.TimedOut = true
	result.Error = fmt.Sprintf("CI timeout after %v - checks still pending", timeout)
	return result, fmt.Errorf("%s", result.Error)
}

// pushChanges pushes any commits made during finalize.
func (m *CIMerger) pushChanges(ctx context.Context, t *task.Task) error {
	// Get the branch name from the task
	branchPrefix := m.cfg.BranchPrefix
	if branchPrefix == "" {
		branchPrefix = "orc"
	}
	branch := fmt.Sprintf("%s/%s", branchPrefix, t.ID)

	m.logger.Debug("pushing finalize changes", "branch", branch)

	args := []string{"push", "--force-with-lease", "origin", branch}
	cmd := exec.CommandContext(ctx, "git", args...)
	if m.workingDir != "" {
		cmd.Dir = m.workingDir
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(output), "Everything up-to-date") {
			return nil
		}
		return fmt.Errorf("git push: %w: %s", err, output)
	}

	m.logger.Debug("pushed finalize changes", "output", strings.TrimSpace(string(output)))
	return nil
}

// CICheckStatus represents CI check status.
type CICheckStatus struct {
	AllPassed   bool
	AnyFailed   bool
	AnyPending  bool
	FailedNames []string
	Details     string
}

// checkCIStatus checks the current CI status for a PR.
func (m *CIMerger) checkCIStatus(ctx context.Context, prURL string) (*CICheckStatus, error) {
	output, err := m.runGH(ctx, "pr", "checks", prURL, "--json", "name,state,bucket")
	if err != nil {
		if strings.Contains(err.Error(), "no checks") || strings.Contains(output, "[]") {
			return &CICheckStatus{
				AllPassed: true,
				Details:   "No CI checks configured",
			}, nil
		}
		return nil, err
	}

	var checks []struct {
		Name   string `json:"name"`
		State  string `json:"state"`
		Bucket string `json:"bucket"`
	}
	if err := json.Unmarshal([]byte(output), &checks); err != nil {
		return nil, fmt.Errorf("parse checks: %w", err)
	}

	if len(checks) == 0 {
		return &CICheckStatus{
			AllPassed: true,
			Details:   "No CI checks configured",
		}, nil
	}

	result := &CICheckStatus{}
	var passedCount, failedCount, pendingCount int

	for _, c := range checks {
		switch c.Bucket {
		case "pass", "skipping":
			passedCount++
		case "fail", "cancel":
			failedCount++
			result.FailedNames = append(result.FailedNames, c.Name)
		case "pending":
			pendingCount++
		}
	}

	result.AllPassed = failedCount == 0 && pendingCount == 0
	result.AnyFailed = failedCount > 0
	result.AnyPending = pendingCount > 0
	result.Details = fmt.Sprintf("%d passed, %d failed, %d pending", passedCount, failedCount, pendingCount)

	return result, nil
}

// mergePR merges the PR using gh CLI with squash.
func (m *CIMerger) mergePR(ctx context.Context, t *task.Task, prURL string) (string, error) {
	m.logger.Info("merging PR", "task", t.ID, "pr", prURL)

	output, err := m.runGH(ctx, "pr", "merge", prURL, "--squash", "--delete-branch")
	if err != nil {
		if strings.Contains(err.Error(), "already merged") {
			m.logger.Info("PR already merged", "task", t.ID)
			return "", nil
		}
		if strings.Contains(err.Error(), "not mergeable") {
			return "", fmt.Errorf("PR not mergeable (conflicts or branch protection): %w", err)
		}
		return "", fmt.Errorf("merge PR: %w", err)
	}

	m.logger.Info("PR merged successfully", "task", t.ID, "output", strings.TrimSpace(output))

	// Update task status to reflect merge
	t.UpdatePRStatus(task.PRStatusMerged, "success", true, 0, 0)
	if err := t.SaveTo(m.taskDir); err != nil {
		m.logger.Warn("failed to save task after merge", "error", err)
	}

	// Try to extract merge commit SHA from output
	// gh pr merge output typically contains the merge commit info
	mergeCommit := extractMergeCommit(output)

	return mergeCommit, nil
}

// extractMergeCommit tries to extract the merge commit SHA from gh pr merge output.
func extractMergeCommit(output string) string {
	// gh pr merge output varies, but sometimes includes commit info
	// For now, return empty - we could parse it if needed
	return ""
}

// runGH executes a gh CLI command.
func (m *CIMerger) runGH(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "gh", args...)
	if m.workingDir != "" {
		cmd.Dir = m.workingDir
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%w: %s", err, output)
	}

	return string(output), nil
}
