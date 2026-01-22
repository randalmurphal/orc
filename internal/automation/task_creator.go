// Package automation provides trigger-based automation for orc tasks.
package automation

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// AutoTaskCreator implements TaskCreator for creating automation tasks.
type AutoTaskCreator struct {
	cfg     *config.Config
	backend storage.Backend
	logger  *slog.Logger

	// onTaskStart is called when a task should be started (auto mode)
	// This allows decoupling from the executor
	onTaskStart func(ctx context.Context, taskID string) error

	// dbAdapter provides direct database access for efficient queries
	dbAdapter *ProjectDBAdapter

	// mu protects concurrent task ID generation
	mu sync.Mutex
}

// AutoTaskCreatorOption configures an AutoTaskCreator.
type AutoTaskCreatorOption func(*AutoTaskCreator)

// WithTaskStartFunc sets the function called to start an automation task.
func WithTaskStartFunc(fn func(ctx context.Context, taskID string) error) AutoTaskCreatorOption {
	return func(c *AutoTaskCreator) {
		c.onTaskStart = fn
	}
}

// WithDBAdapter sets the database adapter for efficient queries.
func WithDBAdapter(adapter *ProjectDBAdapter) AutoTaskCreatorOption {
	return func(c *AutoTaskCreator) {
		c.dbAdapter = adapter
	}
}

// NewAutoTaskCreator creates a new automation task creator.
func NewAutoTaskCreator(cfg *config.Config, backend storage.Backend, logger *slog.Logger, opts ...AutoTaskCreatorOption) *AutoTaskCreator {
	if logger == nil {
		logger = slog.Default()
	}

	c := &AutoTaskCreator{
		cfg:     cfg,
		backend: backend,
		logger:  logger,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// CreateAutomationTask creates a new automation task from a template.
// Returns the created task ID and any error.
// Note: Workflow execution is handled by WorkflowExecutor when the task is run.
func (c *AutoTaskCreator) CreateAutomationTask(ctx context.Context, templateID string, triggerID string, reason string) (string, error) {
	// Get template from config
	tmpl := c.cfg.GetAutomationTemplate(templateID)
	if tmpl == nil {
		return "", fmt.Errorf("automation template not found: %s", templateID)
	}

	// Generate automation task ID (AUTO-XXX)
	taskID, err := c.nextAutoTaskID(ctx)
	if err != nil {
		return "", fmt.Errorf("generate automation task ID: %w", err)
	}

	// Create the task
	t := task.New(taskID, tmpl.Title)
	t.Description = fmt.Sprintf("%s\n\nTriggered by: %s\nReason: %s", tmpl.Description, triggerID, reason)
	t.Weight = task.Weight(tmpl.Weight)
	t.Category = task.Category(tmpl.Category)
	t.Queue = task.QueueActive
	// Priority can be set via trigger action, default to normal
	t.Priority = task.PriorityNormal
	// Mark as automation task for efficient database querying
	t.IsAutomation = true

	// Mark as automation task with metadata
	if t.Metadata == nil {
		t.Metadata = make(map[string]string)
	}
	t.Metadata["automation_trigger_id"] = triggerID
	t.Metadata["automation_template_id"] = templateID
	t.Metadata["automation_reason"] = reason

	// Save the task
	if err := c.backend.SaveTask(t); err != nil {
		return "", fmt.Errorf("save automation task: %w", err)
	}

	// Create initial state
	s := &state.State{
		TaskID:       taskID,
		Status:       state.StatusPending,
		CurrentPhase: "",
		StartedAt:    time.Time{},
	}
	if err := c.backend.SaveState(s); err != nil {
		c.logger.Warn("failed to save initial state for automation task",
			"task", taskID,
			"error", err)
	}

	c.logger.Info("created automation task",
		"task", taskID,
		"template", templateID,
		"trigger", triggerID)

	return taskID, nil
}

// StartAutomationTask starts execution of an automation task.
// This is only called for auto mode.
func (c *AutoTaskCreator) StartAutomationTask(ctx context.Context, taskID string) error {
	if c.onTaskStart == nil {
		c.logger.Warn("no task start function configured, task created but not started",
			"task", taskID)
		return nil
	}

	return c.onTaskStart(ctx, taskID)
}

// nextAutoTaskID generates the next AUTO-XXX task ID.
// The mutex prevents race conditions when multiple automation tasks are created concurrently.
func (c *AutoTaskCreator) nextAutoTaskID(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Use efficient database query if adapter is available
	if c.dbAdapter != nil {
		maxNum, err := c.dbAdapter.GetMaxAutoTaskNumber(ctx)
		if err != nil {
			return "", fmt.Errorf("get max auto task number: %w", err)
		}
		return fmt.Sprintf("AUTO-%03d", maxNum+1), nil
	}

	// Fallback: Get all tasks and find the highest AUTO-XXX number
	// This is less efficient but ensures backward compatibility
	tasks, err := c.backend.LoadAllTasks()
	if err != nil {
		return "", fmt.Errorf("load tasks: %w", err)
	}

	maxNum := 0
	for _, t := range tasks {
		if len(t.ID) > 5 && t.ID[:5] == "AUTO-" {
			var num int
			if _, scanErr := fmt.Sscanf(t.ID[5:], "%d", &num); scanErr == nil {
				if num > maxNum {
					maxNum = num
				}
			}
		}
	}

	return fmt.Sprintf("AUTO-%03d", maxNum+1), nil
}

// Ensure AutoTaskCreator implements TaskCreator.
var _ TaskCreator = (*AutoTaskCreator)(nil)
