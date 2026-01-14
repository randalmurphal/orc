// Package api provides REST API and WebSocket handlers.
package api

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/randalmurphal/orc/internal/github"
	"github.com/randalmurphal/orc/internal/task"
)

// PRPoller periodically polls PR status for tasks with open PRs.
type PRPoller struct {
	workDir  string
	interval time.Duration
	logger   *slog.Logger

	// stopCh signals the poller to stop
	stopCh chan struct{}
	wg     sync.WaitGroup

	// callback for status changes
	onStatusChange func(taskID string, pr *task.PRInfo)
}

// PRPollerConfig configures the PR poller.
type PRPollerConfig struct {
	WorkDir        string
	Interval       time.Duration
	Logger         *slog.Logger
	OnStatusChange func(taskID string, pr *task.PRInfo)
}

// NewPRPoller creates a new PR status poller.
func NewPRPoller(cfg PRPollerConfig) *PRPoller {
	interval := cfg.Interval
	if interval == 0 {
		interval = 60 * time.Second // Default to 1 minute
	}

	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &PRPoller{
		workDir:        cfg.WorkDir,
		interval:       interval,
		logger:         logger,
		stopCh:         make(chan struct{}),
		onStatusChange: cfg.OnStatusChange,
	}
}

// Start begins the polling loop.
func (p *PRPoller) Start(ctx context.Context) {
	p.wg.Add(1)
	go p.run(ctx)
}

// Stop gracefully stops the poller.
func (p *PRPoller) Stop() {
	close(p.stopCh)
	p.wg.Wait()
}

func (p *PRPoller) run(ctx context.Context) {
	defer p.wg.Done()

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	// Run initial poll
	p.pollAll(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.stopCh:
			return
		case <-ticker.C:
			p.pollAll(ctx)
		}
	}
}

func (p *PRPoller) pollAll(ctx context.Context) {
	// Load all tasks from the tasks directory
	tasksDir := p.workDir + "/.orc/tasks"
	tasks, err := task.LoadAllFrom(tasksDir)
	if err != nil {
		p.logger.Debug("failed to load tasks for PR polling", "error", err)
		return
	}

	// Filter tasks that have PRs and need polling
	var tasksToCheck []*task.Task
	for _, t := range tasks {
		if p.shouldPoll(t) {
			tasksToCheck = append(tasksToCheck, t)
		}
	}

	if len(tasksToCheck) == 0 {
		return
	}

	p.logger.Debug("polling PR status", "task_count", len(tasksToCheck))

	// Create GitHub client
	client, err := github.NewClient(p.workDir)
	if err != nil {
		p.logger.Debug("failed to create GitHub client for PR polling", "error", err)
		return
	}

	// Poll each task
	for _, t := range tasksToCheck {
		if err := p.pollTask(ctx, client, t); err != nil {
			if !errors.Is(err, context.Canceled) {
				p.logger.Debug("failed to poll PR for task", "task", t.ID, "error", err)
			}
		}
	}
}

func (p *PRPoller) shouldPoll(t *task.Task) bool {
	// Poll if task has a PR that isn't merged or closed
	if t.PR == nil || t.PR.URL == "" {
		return false
	}

	// Don't poll merged or closed PRs
	if t.PR.Status == task.PRStatusMerged || t.PR.Status == task.PRStatusClosed {
		return false
	}

	// Don't poll if we polled recently (within 30 seconds)
	// This prevents polling too frequently if multiple triggers occur
	if t.PR.LastCheckedAt != nil {
		if time.Since(*t.PR.LastCheckedAt) < 30*time.Second {
			return false
		}
	}

	return true
}

func (p *PRPoller) pollTask(ctx context.Context, client *github.Client, t *task.Task) error {
	// Find PR by branch
	pr, err := client.FindPRByBranch(ctx, t.Branch)
	if err != nil {
		if errors.Is(err, github.ErrNoPRFound) {
			// PR was likely closed/deleted
			t.PR.Status = task.PRStatusClosed
			return p.saveTask(t)
		}
		return err
	}

	// Get PR status summary
	summary, err := client.GetPRStatusSummary(ctx, pr)
	if err != nil {
		return err
	}

	// Determine new PR status
	oldStatus := t.PR.Status
	newStatus := p.determinePRStatus(pr, summary)

	// Update task PR info
	now := time.Now()
	t.PR.Number = pr.Number
	t.PR.Status = newStatus
	t.PR.ChecksStatus = summary.ChecksStatus
	t.PR.Mergeable = summary.Mergeable
	t.PR.ReviewCount = summary.ReviewCount
	t.PR.ApprovalCount = summary.ApprovalCount
	t.PR.LastCheckedAt = &now

	// Save task
	if err := p.saveTask(t); err != nil {
		return err
	}

	// Notify if status changed
	if oldStatus != newStatus && p.onStatusChange != nil {
		p.onStatusChange(t.ID, t.PR)
	}

	return nil
}

func (p *PRPoller) determinePRStatus(pr *github.PR, summary *github.PRStatusSummary) task.PRStatus {
	return DeterminePRStatus(pr, summary)
}

// DeterminePRStatus derives the task.PRStatus from a PR and its review summary.
// This is used by both the poller and the API handler.
func DeterminePRStatus(pr *github.PR, summary *github.PRStatusSummary) task.PRStatus {
	// Check if PR is merged
	if pr.State == "MERGED" {
		return task.PRStatusMerged
	}

	// Check if PR is closed
	if pr.State == "CLOSED" {
		return task.PRStatusClosed
	}

	// Check if PR is draft
	if pr.Draft {
		return task.PRStatusDraft
	}

	// Use review status
	switch summary.ReviewStatus {
	case "approved":
		return task.PRStatusApproved
	case "changes_requested":
		return task.PRStatusChangesRequested
	default:
		return task.PRStatusPendingReview
	}
}

func (p *PRPoller) saveTask(t *task.Task) error {
	taskDir := task.TaskDirIn(p.workDir, t.ID)
	return t.SaveTo(taskDir)
}

// PollTask manually triggers a poll for a specific task.
// This is useful for on-demand refresh.
func (p *PRPoller) PollTask(ctx context.Context, taskID string) error {
	t, err := task.LoadFrom(p.workDir, taskID)
	if err != nil {
		return err
	}

	if t.PR == nil || t.PR.URL == "" {
		return errors.New("task has no PR")
	}

	client, err := github.NewClient(p.workDir)
	if err != nil {
		return err
	}

	return p.pollTask(ctx, client, t)
}
