// Package executor provides the flowgraph-based execution engine for orc.
package executor

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/git"
	"github.com/randalmurphal/orc/internal/hosting"
	"github.com/randalmurphal/orc/internal/initiative"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"

	_ "github.com/randalmurphal/orc/internal/hosting/github"
	_ "github.com/randalmurphal/orc/internal/hosting/gitlab"
)

// InitiativeCompletionResult contains the result of an initiative completion operation.
type InitiativeCompletionResult struct {
	// InitiativeID is the ID of the completed initiative.
	InitiativeID string
	// Merged indicates if the initiative branch was merged.
	Merged bool
	// MergeCommit is the SHA of the merge commit (if merged).
	MergeCommit string
	// PRURL is the URL of the PR created (if created).
	PRURL string
	// PRNumber is the PR number (if created).
	PRNumber int
	// Error contains any error that occurred during completion.
	Error error
}

// InitiativeCompleter handles the completion flow for initiatives with branch bases.
type InitiativeCompleter struct {
	gitOps     *git.Git
	provider   hosting.Provider
	backend    storage.Backend
	cfg        *config.Config
	logger     *slog.Logger
	projectDir string
}

// NewInitiativeCompleter creates a new initiative completer.
func NewInitiativeCompleter(
	gitOps *git.Git,
	provider hosting.Provider,
	backend storage.Backend,
	cfg *config.Config,
	logger *slog.Logger,
	projectDir string,
) *InitiativeCompleter {
	return &InitiativeCompleter{
		gitOps:     gitOps,
		provider:   provider,
		backend:    backend,
		cfg:        cfg,
		logger:     logger,
		projectDir: projectDir,
	}
}

// CheckAndCompleteInitiative checks if an initiative is ready for completion and handles the merge flow.
// Returns a result indicating what action was taken (if any).
func (c *InitiativeCompleter) CheckAndCompleteInitiative(ctx context.Context, initiativeID string) (*InitiativeCompletionResult, error) {
	if c.backend == nil {
		return nil, fmt.Errorf("storage backend is required")
	}

	// Load the initiative
	init, err := c.backend.LoadInitiative(initiativeID)
	if err != nil {
		return nil, fmt.Errorf("load initiative %s: %w", initiativeID, err)
	}
	if init == nil {
		return nil, fmt.Errorf("initiative %s not found", initiativeID)
	}

	// Check if initiative has a branch base configured
	if !init.HasBranchBase() {
		c.logger.Debug("initiative has no branch base, skipping completion",
			"initiative", initiativeID)
		return &InitiativeCompletionResult{InitiativeID: initiativeID}, nil
	}

	// Check if already merged
	if init.MergeStatus == initiative.MergeStatusMerged {
		c.logger.Debug("initiative already merged",
			"initiative", initiativeID,
			"commit", init.MergeCommit)
		return &InitiativeCompletionResult{
			InitiativeID: initiativeID,
			Merged:       true,
			MergeCommit:  init.MergeCommit,
		}, nil
	}

	// Create a task loader to check actual task statuses
	taskLoader := c.createTaskLoader()

	// Check if all tasks are complete
	if !init.AllTasksCompleteWithLoader(taskLoader) {
		c.logger.Debug("initiative has incomplete tasks, not ready for completion",
			"initiative", initiativeID)
		return &InitiativeCompletionResult{InitiativeID: initiativeID}, nil
	}

	// Initiative is ready for completion!
	c.logger.Info("initiative ready for branch merge",
		"initiative", initiativeID,
		"branch", init.BranchBase)

	// Update merge status to pending
	init.MergeStatus = initiative.MergeStatusPending
	init.UpdatedAt = time.Now()
	if err := c.backend.SaveInitiative(init); err != nil {
		return nil, fmt.Errorf("update initiative %s merge status to pending: %w", initiativeID, err)
	}

	// Check automation profile to determine action
	profile := c.getProfile()
	if profile == config.ProfileAuto || profile == config.ProfileFast {
		return c.autoMergeInitiative(ctx, init)
	}

	// For safe/strict profiles, leave in pending state
	c.logger.Info("initiative ready for merge (awaiting manual action)",
		"initiative", initiativeID,
		"profile", profile)
	return &InitiativeCompletionResult{
		InitiativeID: initiativeID,
	}, nil
}

// createTaskLoader creates a TaskLoader function that fetches task status from the backend.
func (c *InitiativeCompleter) createTaskLoader() initiative.TaskLoader {
	return func(taskID string) (status string, title string, err error) {
		t, err := c.backend.LoadTask(taskID)
		if err != nil {
			return "", "", err
		}
		if t == nil {
			return "", "", nil
		}
		return task.StatusFromProto(t.Status), t.Title, nil
	}
}

// getProfile returns the current automation profile.
func (c *InitiativeCompleter) getProfile() config.AutomationProfile {
	if c.cfg == nil {
		return config.ProfileAuto // default
	}
	if c.cfg.Profile != "" {
		return c.cfg.Profile
	}
	return config.ProfileAuto
}

// autoMergeInitiative performs the auto-merge flow for an initiative branch.
func (c *InitiativeCompleter) autoMergeInitiative(ctx context.Context, init *initiative.Initiative) (*InitiativeCompletionResult, error) {
	result := &InitiativeCompletionResult{
		InitiativeID: init.ID,
	}

	// Update status to in_progress
	init.MergeStatus = initiative.MergeStatusInProgress
	init.UpdatedAt = time.Now()
	if err := c.backend.SaveInitiative(init); err != nil {
		// Log but continue - the merge operation will proceed regardless
		c.logger.Warn("failed to record in-progress status, continuing with merge",
			"initiative", init.ID,
			"error", err)
	}

	// Determine target branch for the merge
	targetBranch := c.getTargetBranch()

	c.logger.Info("auto-merging initiative branch",
		"initiative", init.ID,
		"source", init.BranchBase,
		"target", targetBranch)

	// Create a PR for the initiative branch
	if c.provider == nil {
		c.logger.Warn("no hosting provider configured, cannot create PR for initiative",
			"initiative", init.ID)
		init.MergeStatus = initiative.MergeStatusFailed
		init.UpdatedAt = time.Now()
		if saveErr := c.backend.SaveInitiative(init); saveErr != nil {
			c.logger.Error("failed to record failed status after hosting provider error",
				"initiative", init.ID,
				"error", saveErr)
		}
		result.Error = fmt.Errorf("no hosting provider configured")
		return result, nil
	}

	// Build PR title and body
	prTitle := fmt.Sprintf("[initiative] %s", init.Title)
	prBody := c.buildInitiativePRBody(init)

	// Create the PR
	prOpts := hosting.PRCreateOptions{
		Title:  prTitle,
		Body:   prBody,
		Head:   init.BranchBase,
		Base:   targetBranch,
		Labels: c.getPRLabels(),
	}

	pr, err := c.provider.CreatePR(ctx, prOpts)
	if err != nil {
		c.logger.Error("failed to create initiative PR",
			"initiative", init.ID,
			"error", err)
		init.MergeStatus = initiative.MergeStatusFailed
		init.UpdatedAt = time.Now()
		if saveErr := c.backend.SaveInitiative(init); saveErr != nil {
			c.logger.Error("failed to record failed status after PR creation error",
				"initiative", init.ID,
				"error", saveErr)
		}
		result.Error = fmt.Errorf("create PR: %w", err)
		return result, nil
	}

	result.PRURL = pr.HTMLURL
	result.PRNumber = pr.Number

	c.logger.Info("created initiative PR",
		"initiative", init.ID,
		"pr", pr.Number,
		"url", pr.HTMLURL)

	// For auto/fast profiles, wait for CI and merge
	if c.shouldAutoMerge() {
		mergeCommit, err := c.waitAndMerge(ctx, init, pr)
		if err != nil {
			c.logger.Error("failed to auto-merge initiative",
				"initiative", init.ID,
				"error", err)
			init.MergeStatus = initiative.MergeStatusFailed
			init.UpdatedAt = time.Now()
			if saveErr := c.backend.SaveInitiative(init); saveErr != nil {
				c.logger.Error("failed to record failed status after merge error",
					"initiative", init.ID,
					"error", saveErr)
			}
			result.Error = err
			return result, nil
		}

		result.Merged = true
		result.MergeCommit = mergeCommit

		// Update initiative with merge info
		init.MergeStatus = initiative.MergeStatusMerged
		init.MergeCommit = mergeCommit
		init.Status = initiative.StatusCompleted
		init.UpdatedAt = time.Now()
		if err := c.backend.SaveInitiative(init); err != nil {
			c.logger.Error("failed to update initiative after merge",
				"initiative", init.ID,
				"error", err)
		}

		c.logger.Info("initiative branch merged successfully",
			"initiative", init.ID,
			"commit", mergeCommit)
	}

	return result, nil
}

// getTargetBranch returns the target branch for merging initiative branches.
func (c *InitiativeCompleter) getTargetBranch() string {
	if c.cfg != nil && c.cfg.Completion.TargetBranch != "" {
		return c.cfg.Completion.TargetBranch
	}
	return "main"
}

// getPRLabels returns labels to apply to initiative PRs.
func (c *InitiativeCompleter) getPRLabels() []string {
	if c.cfg != nil && len(c.cfg.Completion.PR.Labels) > 0 {
		return c.cfg.Completion.PR.Labels
	}
	return []string{"initiative"}
}

// shouldAutoMerge returns true if PRs should be auto-merged.
func (c *InitiativeCompleter) shouldAutoMerge() bool {
	profile := c.getProfile()
	return profile == config.ProfileAuto || profile == config.ProfileFast
}

// buildInitiativePRBody creates the PR body for an initiative merge.
func (c *InitiativeCompleter) buildInitiativePRBody(init *initiative.Initiative) string {
	body := fmt.Sprintf("## Initiative: %s\n\n", init.Title)

	if init.Vision != "" {
		body += fmt.Sprintf("### Vision\n%s\n\n", init.Vision)
	}

	if len(init.Tasks) > 0 {
		body += "### Tasks Completed\n"
		for _, t := range init.Tasks {
			status := t.Status
			if status == "completed" {
				status = "✅"
			}
			body += fmt.Sprintf("- %s %s: %s\n", status, t.ID, t.Title)
		}
		body += "\n"
	}

	if len(init.Decisions) > 0 {
		body += "### Decisions Made\n"
		for _, d := range init.Decisions {
			body += fmt.Sprintf("- **%s**: %s\n", d.ID, d.Decision)
			if d.Rationale != "" {
				body += fmt.Sprintf("  - Rationale: %s\n", d.Rationale)
			}
		}
		body += "\n"
	}

	body += "---\n🤖 Generated by [orc](https://github.com/randalmurphal/orc)\n"
	return body
}

// waitAndMerge waits for CI to pass and then merges the PR.
// Returns the merge commit SHA on success.
func (c *InitiativeCompleter) waitAndMerge(ctx context.Context, init *initiative.Initiative, pr *hosting.PR) (string, error) {
	// Wait for CI if configured
	if c.cfg != nil && c.cfg.Completion.CI.WaitForCI {
		timeout := c.cfg.Completion.CI.CITimeout
		if timeout == 0 {
			timeout = 10 * time.Minute
		}

		c.logger.Info("waiting for CI to pass",
			"initiative", init.ID,
			"pr", pr.Number,
			"timeout", timeout)

		ciCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		if err := c.waitForCI(ciCtx, pr.HeadBranch); err != nil {
			return "", fmt.Errorf("CI wait: %w", err)
		}
	}

	// Approve PR if configured for auto profiles
	if c.cfg != nil && c.cfg.Completion.PR.AutoApprove {
		comment := fmt.Sprintf("Auto-approving initiative %s after all tasks completed successfully.", init.ID)
		if err := c.approvePR(ctx, pr.Number, comment); err != nil {
			c.logger.Warn("failed to auto-approve initiative PR",
				"initiative", init.ID,
				"pr", pr.Number,
				"error", err)
			// Continue with merge attempt
		}
	}

	// Merge the PR
	mergeMethod := "squash"
	if c.cfg != nil && c.cfg.Completion.CI.MergeMethod != "" {
		mergeMethod = c.cfg.Completion.CI.MergeMethod
	}

	c.logger.Info("merging initiative PR",
		"initiative", init.ID,
		"pr", pr.Number,
		"method", mergeMethod)

	mergeOpts := hosting.PRMergeOptions{
		Method:       mergeMethod,
		DeleteBranch: true, // Clean up initiative branch after merge
	}
	if err := c.provider.MergePR(ctx, pr.Number, mergeOpts); err != nil {
		return "", fmt.Errorf("merge PR: %w", err)
	}

	// Get the merge commit SHA by fetching the updated PR
	mergedPR, err := c.provider.GetPR(ctx, pr.Number)
	if err != nil {
		c.logger.Warn("failed to get merged PR details",
			"initiative", init.ID,
			"pr", pr.Number,
			"error", err)
		return "unknown", nil // PR merged but couldn't get commit
	}

	// For merged PRs, the state will be "MERGED" and we can try to get the commit
	mergeCommit := "merged"
	if mergedPR.State == "MERGED" {
		mergeCommit = "merged"
	}

	return mergeCommit, nil
}

// waitForCI polls CI checks until they pass or timeout.
func (c *InitiativeCompleter) waitForCI(ctx context.Context, branch string) error {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			checks, err := c.provider.GetCheckRuns(ctx, branch)
			if err != nil {
				c.logger.Warn("failed to get CI status, will retry", "error", err)
				continue
			}
			if len(checks) == 0 {
				return nil // no checks configured
			}

			allComplete := true
			anyFailed := false
			for _, check := range checks {
				if check.Status != "completed" {
					allComplete = false
					break
				}
				if check.Conclusion == "failure" || check.Conclusion == "cancelled" {
					anyFailed = true
				}
			}
			if anyFailed {
				return fmt.Errorf("CI checks failed")
			}
			if allComplete {
				return nil
			}
			c.logger.Debug("CI checks still pending", "branch", branch)
		}
	}
}

// approvePR approves a PR with a comment.
func (c *InitiativeCompleter) approvePR(ctx context.Context, prNumber int, comment string) error {
	return c.provider.ApprovePR(ctx, prNumber, comment)
}

// ManualCompleteInitiative triggers completion for an initiative that's in pending state.
// This is used for safe/strict profiles where auto-merge is disabled.
func (c *InitiativeCompleter) ManualCompleteInitiative(ctx context.Context, initiativeID string) (*InitiativeCompletionResult, error) {
	// Load the initiative
	init, err := c.backend.LoadInitiative(initiativeID)
	if err != nil {
		return nil, fmt.Errorf("load initiative %s: %w", initiativeID, err)
	}
	if init == nil {
		return nil, fmt.Errorf("initiative %s not found", initiativeID)
	}

	// Check if initiative has a branch base configured
	if !init.HasBranchBase() {
		return nil, fmt.Errorf("initiative %s has no branch base configured", initiativeID)
	}

	// Verify all tasks are complete
	taskLoader := c.createTaskLoader()
	if !init.AllTasksCompleteWithLoader(taskLoader) {
		return nil, fmt.Errorf("initiative %s has incomplete tasks", initiativeID)
	}

	// Force auto-merge flow
	return c.autoMergeInitiative(ctx, init)
}

// CheckAndCompleteInitiativeNoBranch marks an initiative as completed if:
// 1. The initiative has no BranchBase configured
// 2. All tasks in the initiative are complete
//
// This is called after task completion for initiatives that don't use feature branches.
// Initiatives with BranchBase should use CheckAndCompleteInitiative instead (merge flow).
//
// The function is best-effort: errors are returned but should not fail the calling task.
func (c *InitiativeCompleter) CheckAndCompleteInitiativeNoBranch(ctx context.Context, initiativeID string) error {
	if c.backend == nil {
		return fmt.Errorf("storage backend is required")
	}

	// Load the initiative
	init, err := c.backend.LoadInitiative(initiativeID)
	if err != nil {
		return fmt.Errorf("load initiative %s: %w", initiativeID, err)
	}
	if init == nil {
		return fmt.Errorf("initiative %s not found", initiativeID)
	}

	// Skip if initiative has a branch base - those use the merge flow
	if init.HasBranchBase() {
		c.logger.Debug("initiative has branch base, skipping no-branch completion (use merge flow instead)",
			"initiative", initiativeID,
			"branch", init.BranchBase)
		return nil
	}

	// Skip if already completed - no work to do
	if init.Status == initiative.StatusCompleted {
		c.logger.Debug("initiative already completed",
			"initiative", initiativeID)
		return nil
	}

	// Skip if no tasks - empty initiatives should not auto-complete
	if len(init.Tasks) == 0 {
		c.logger.Debug("initiative has no tasks, skipping auto-completion",
			"initiative", initiativeID)
		return nil
	}

	// Create a task loader to check actual task statuses from backend
	taskLoader := c.createTaskLoader()

	// Check if all tasks are complete
	if !init.AllTasksCompleteWithLoader(taskLoader) {
		c.logger.Debug("initiative has incomplete tasks",
			"initiative", initiativeID)
		return nil
	}

	// All tasks complete - mark initiative as completed
	c.logger.Info("all tasks complete, marking initiative as completed",
		"initiative", initiativeID)

	init.Status = initiative.StatusCompleted
	init.UpdatedAt = time.Now()

	if err := c.backend.SaveInitiative(init); err != nil {
		return fmt.Errorf("save initiative %s: %w", initiativeID, err)
	}

	return nil
}
