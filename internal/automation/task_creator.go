// Package automation provides trigger-based automation for orc tasks.
package automation

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/internal/workflow"
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

	// Create the task using proto type
	t := task.NewProtoTask(taskID, tmpl.Title)
	desc := fmt.Sprintf("%s\n\nTriggered by: %s\nReason: %s", tmpl.Description, triggerID, reason)
	t.Description = &desc
	t.Weight = task.WeightToProto(tmpl.Weight)
	t.Category = task.CategoryToProto(tmpl.Category)
	t.Queue = orcv1.TaskQueue_TASK_QUEUE_ACTIVE

	// Auto-assign workflow based on weight
	wfID := workflow.WeightToWorkflowID(t.Weight)
	if wfID != "" {
		t.WorkflowId = &wfID
	}
	// Priority can be set via trigger action, default to normal
	t.Priority = orcv1.TaskPriority_TASK_PRIORITY_NORMAL
	// Mark as automation task for efficient database querying
	t.IsAutomation = true

	// Mark as automation task with metadata
	if t.Metadata == nil {
		t.Metadata = make(map[string]string)
	}
	t.Metadata["automation_trigger_id"] = triggerID
	t.Metadata["automation_template_id"] = templateID
	t.Metadata["automation_reason"] = reason

	// Save the task (Execution is initialized by NewProtoTask())
	if err := c.backend.SaveTask(t); err != nil {
		return "", fmt.Errorf("save automation task: %w", err)
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

	if c.dbAdapter == nil {
		return "", fmt.Errorf("database adapter required for AUTO task ID generation")
	}

	maxNum, err := c.dbAdapter.GetMaxAutoTaskNumber(ctx)
	if err != nil {
		return "", fmt.Errorf("get max auto task number: %w", err)
	}
	return fmt.Sprintf("AUTO-%03d", maxNum+1), nil
}

// Ensure AutoTaskCreator implements TaskCreator.
var _ TaskCreator = (*AutoTaskCreator)(nil)
