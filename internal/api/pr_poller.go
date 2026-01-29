// Package api provides REST API and WebSocket handlers.
package api

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/hosting"
	_ "github.com/randalmurphal/orc/internal/hosting/github"
	_ "github.com/randalmurphal/orc/internal/hosting/gitlab"
	"github.com/randalmurphal/orc/internal/storage"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// PRPoller periodically polls PR status for tasks with open PRs.
type PRPoller struct {
	workDir   string
	interval  time.Duration
	logger    *slog.Logger
	orcConfig *config.Config
	backend   storage.Backend

	// stopCh signals the poller to stop
	stopCh   chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup

	// callback for status changes
	onStatusChange func(taskID string, pr *orcv1.PRInfo)
}

// PRPollerConfig configures the PR poller.
type PRPollerConfig struct {
	WorkDir        string
	Interval       time.Duration
	Logger         *slog.Logger
	OrcConfig      *config.Config
	Backend        storage.Backend
	OnStatusChange func(taskID string, pr *orcv1.PRInfo)
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
		orcConfig:      cfg.OrcConfig,
		backend:        cfg.Backend,
		stopCh:         make(chan struct{}),
		onStatusChange: cfg.OnStatusChange,
	}
}

// Start begins the polling loop.
func (p *PRPoller) Start(ctx context.Context) {
	p.wg.Add(1)
	go p.run(ctx)
}

// Stop gracefully stops the poller. Safe to call multiple times.
func (p *PRPoller) Stop() {
	p.stopOnce.Do(func() {
		close(p.stopCh)
	})
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
	// Load all tasks from the backend
	tasks, err := p.backend.LoadAllTasks()
	if err != nil {
		p.logger.Debug("failed to load tasks for PR polling", "error", err)
		return
	}

	// Filter tasks that have PRs and need polling
	var tasksToCheck []*orcv1.Task
	for _, t := range tasks {
		if p.shouldPoll(t) {
			tasksToCheck = append(tasksToCheck, t)
		}
	}

	if len(tasksToCheck) == 0 {
		return
	}

	p.logger.Debug("polling PR status", "task_count", len(tasksToCheck))

	// Create hosting provider
	cfg := hosting.Config{}
	if p.orcConfig != nil {
		cfg = hosting.Config{
			Provider:    p.orcConfig.Hosting.Provider,
			BaseURL:     p.orcConfig.Hosting.BaseURL,
			TokenEnvVar: p.orcConfig.Hosting.TokenEnvVar,
		}
	}
	provider, err := hosting.NewProvider(p.workDir, cfg)
	if err != nil {
		p.logger.Debug("failed to create hosting provider for PR polling", "error", err)
		return
	}

	// Poll each task
	for _, t := range tasksToCheck {
		if err := p.pollTask(ctx, provider, t); err != nil {
			if !errors.Is(err, context.Canceled) {
				p.logger.Debug("failed to poll PR for task", "task", t.Id, "error", err)
			}
		}
	}
}

func (p *PRPoller) shouldPoll(t *orcv1.Task) bool {
	// Poll if task has a PR that isn't merged or closed
	if t.Pr == nil || t.Pr.Url == nil || *t.Pr.Url == "" {
		return false
	}

	// Don't poll merged or closed PRs
	if t.Pr.Status == orcv1.PRStatus_PR_STATUS_MERGED || t.Pr.Status == orcv1.PRStatus_PR_STATUS_CLOSED {
		return false
	}

	// Don't poll if we polled recently (within 30 seconds)
	// This prevents polling too frequently if multiple triggers occur
	if t.Pr.LastCheckedAt != nil {
		if time.Since(t.Pr.LastCheckedAt.AsTime()) < 30*time.Second {
			return false
		}
	}

	return true
}

func (p *PRPoller) pollTask(ctx context.Context, provider hosting.Provider, t *orcv1.Task) error {
	// Find PR by branch
	pr, err := provider.FindPRByBranch(ctx, t.Branch)
	if err != nil {
		if errors.Is(err, hosting.ErrNoPRFound) {
			// PR was likely closed/deleted
			t.Pr.Status = orcv1.PRStatus_PR_STATUS_CLOSED
			return p.saveTask(t)
		}
		return err
	}

	// Get PR status summary
	summary, err := provider.GetPRStatusSummary(ctx, pr)
	if err != nil {
		return err
	}

	// Determine new PR status
	oldStatus := t.Pr.Status
	newStatus := DeterminePRStatusProto(pr, summary)

	// Update task PR info
	now := timestamppb.Now()
	prNumber := int32(pr.Number)
	t.Pr.Number = &prNumber
	t.Pr.Status = newStatus
	t.Pr.ChecksStatus = &summary.ChecksStatus
	t.Pr.Mergeable = summary.Mergeable
	t.Pr.ReviewCount = int32(summary.ReviewCount)
	t.Pr.ApprovalCount = int32(summary.ApprovalCount)
	t.Pr.LastCheckedAt = now

	// Save task
	if err := p.saveTask(t); err != nil {
		return err
	}

	// Notify if status changed
	if oldStatus != newStatus && p.onStatusChange != nil {
		p.onStatusChange(t.Id, t.Pr)
	}

	return nil
}

// DeterminePRStatusProto derives the proto PRStatus from a PR and its review summary.
func DeterminePRStatusProto(pr *hosting.PR, summary *hosting.PRStatusSummary) orcv1.PRStatus {
	// Check if PR is merged
	if pr.State == "MERGED" {
		return orcv1.PRStatus_PR_STATUS_MERGED
	}

	// Check if PR is closed
	if pr.State == "CLOSED" {
		return orcv1.PRStatus_PR_STATUS_CLOSED
	}

	// Check if PR is draft
	if pr.Draft {
		return orcv1.PRStatus_PR_STATUS_DRAFT
	}

	// Use review status
	switch summary.ReviewStatus {
	case "approved":
		return orcv1.PRStatus_PR_STATUS_APPROVED
	case "changes_requested":
		return orcv1.PRStatus_PR_STATUS_CHANGES_REQUESTED
	default:
		return orcv1.PRStatus_PR_STATUS_PENDING_REVIEW
	}
}

func (p *PRPoller) saveTask(t *orcv1.Task) error {
	return p.backend.SaveTask(t)
}

// PollTask manually triggers a poll for a specific task.
// This is useful for on-demand refresh.
func (p *PRPoller) PollTask(ctx context.Context, taskID string) error {
	t, err := p.backend.LoadTask(taskID)
	if err != nil {
		return err
	}

	if t.Pr == nil || t.Pr.Url == nil || *t.Pr.Url == "" {
		return errors.New("task has no PR")
	}

	cfg := hosting.Config{}
	if p.orcConfig != nil {
		cfg = hosting.Config{
			Provider:    p.orcConfig.Hosting.Provider,
			BaseURL:     p.orcConfig.Hosting.BaseURL,
			TokenEnvVar: p.orcConfig.Hosting.TokenEnvVar,
		}
	}
	provider, err := hosting.NewProvider(p.workDir, cfg)
	if err != nil {
		return err
	}

	return p.pollTask(ctx, provider, t)
}
