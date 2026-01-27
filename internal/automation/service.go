// Package automation provides trigger-based automation for orc tasks.
package automation

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/randalmurphal/orc/internal/config"
)

// Event represents an event that can trigger automation.
type Event struct {
	Type      string            // Event type (task_completed, initiative_completed, pr_merged, etc.)
	TaskID    string            // Task ID if applicable
	Weight    string            // Task weight if applicable
	Category  string            // Task category if applicable
	Phase     string            // Phase if applicable
	Metadata  map[string]string // Additional event metadata
	Timestamp time.Time         // When the event occurred
}

// EventType constants for common events.
const (
	EventTaskCompleted       = "task_completed"
	EventTaskFailed          = "task_failed"
	EventPhaseCompleted      = "phase_completed"
	EventPhaseFailed         = "phase_failed"
	EventPRMerged            = "pr_merged"
	EventPRApproved          = "pr_approved"
	EventInitiativeCompleted = "initiative_completed"
	EventInitiativeStarted   = "initiative_started"
)

// Evaluator is the interface for trigger condition evaluation.
type Evaluator interface {
	// Type returns the trigger type this evaluator handles.
	Type() TriggerType

	// Evaluate checks if the trigger should fire based on the event.
	// Returns true if the trigger condition is met.
	Evaluate(ctx context.Context, trigger *Trigger, event *Event, svc *Service) (bool, string, error)
}

// TaskCreator is the interface for creating automation tasks.
type TaskCreator interface {
	// CreateAutomationTask creates a new automation task from a template.
	// Returns the created task ID and any error.
	CreateAutomationTask(ctx context.Context, templateID string, triggerID string, reason string) (string, error)

	// StartAutomationTask starts execution of an automation task.
	// This is only called for auto mode.
	StartAutomationTask(ctx context.Context, taskID string) error
}

// Service manages automation triggers and their execution.
type Service struct {
	cfg        *config.Config
	db         Database
	evaluators map[TriggerType]Evaluator
	logger     *slog.Logger
	mu         sync.RWMutex

	// lastGlobalTrigger tracks the last time any trigger fired (for global cooldown)
	lastGlobalTrigger time.Time

	// taskCreator creates automation tasks (optional, nil disables task creation)
	taskCreator TaskCreator
}

// Database is the interface for automation database operations.
type Database interface {
	// Triggers
	SaveTrigger(ctx context.Context, trigger *Trigger) error
	LoadTrigger(ctx context.Context, id string) (*Trigger, error)
	LoadAllTriggers(ctx context.Context) ([]*Trigger, error)
	// IncrementTriggerCount atomically increments trigger count and updates last_triggered_at.
	// Returns the new count. This avoids race conditions from read-modify-write patterns.
	IncrementTriggerCount(ctx context.Context, id string, triggeredAt time.Time) (int, error)
	// SetTriggerEnabled updates the enabled state of a trigger.
	SetTriggerEnabled(ctx context.Context, id string, enabled bool) error

	// Counters
	GetCounter(ctx context.Context, triggerID, metric string) (int, error)
	IncrementCounter(ctx context.Context, triggerID, metric string) error
	// IncrementAndGetCounter atomically increments counter and returns new value.
	// This prevents race conditions between increment and threshold check.
	IncrementAndGetCounter(ctx context.Context, triggerID, metric string) (int, error)
	ResetCounter(ctx context.Context, triggerID, metric string) error

	// Executions
	CreateExecution(ctx context.Context, exec *Execution) error
	UpdateExecutionStatus(ctx context.Context, id int64, status ExecutionStatus, errorMsg string) error
	GetRecentExecutions(ctx context.Context, triggerID string, limit int) ([]*Execution, error)

	// Metrics
	RecordMetric(ctx context.Context, metric *Metric) error
	GetLatestMetric(ctx context.Context, name string) (*Metric, error)

	// Notifications
	CreateNotification(ctx context.Context, notif *Notification) error
	GetActiveNotifications(ctx context.Context) ([]*Notification, error)
	DismissNotification(ctx context.Context, id string) error
	DismissAllNotifications(ctx context.Context) error

	// Stats
	GetExecutionStats(ctx context.Context) (*ExecutionStats, error)
}

// NewService creates a new automation service.
func NewService(cfg *config.Config, db Database, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}

	svc := &Service{
		cfg:        cfg,
		db:         db,
		evaluators: make(map[TriggerType]Evaluator),
		logger:     logger,
	}

	// Register built-in evaluators
	svc.RegisterEvaluator(&CountEvaluator{})
	svc.RegisterEvaluator(&InitiativeEvaluator{})
	svc.RegisterEvaluator(&EventEvaluator{})
	svc.RegisterEvaluator(&ThresholdEvaluator{})

	// Only register schedule evaluator in team mode
	if cfg.IsTeamMode() {
		svc.RegisterEvaluator(&ScheduleEvaluator{})
	}

	return svc
}

// RegisterEvaluator registers a trigger evaluator.
func (s *Service) RegisterEvaluator(eval Evaluator) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.evaluators[eval.Type()] = eval
}

// SetTaskCreator sets the task creator for automation task creation.
func (s *Service) SetTaskCreator(tc TaskCreator) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.taskCreator = tc
}

// SetTriggerEnabled updates the enabled state of a trigger, persisting to the database.
func (s *Service) SetTriggerEnabled(ctx context.Context, id string, enabled bool) error {
	return s.db.SetTriggerEnabled(ctx, id, enabled)
}

// HandleEvent processes an event and fires matching triggers.
func (s *Service) HandleEvent(ctx context.Context, event *Event) error {
	if !s.cfg.AutomationEnabled() {
		return nil
	}

	// Check global cooldown
	s.mu.RLock()
	lastTrigger := s.lastGlobalTrigger
	s.mu.RUnlock()

	if s.cfg.Automation.GlobalCooldown > 0 && time.Since(lastTrigger) < s.cfg.Automation.GlobalCooldown {
		s.logger.Debug("global cooldown active, skipping event",
			"event", event.Type,
			"remaining", s.cfg.Automation.GlobalCooldown-time.Since(lastTrigger))
		return nil
	}

	// Get enabled triggers
	triggers := s.cfg.GetEnabledTriggers()

	for _, triggerCfg := range triggers {
		// Check for context cancellation between trigger evaluations
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		trigger := s.configToTrigger(&triggerCfg)

		// Get evaluator for this trigger type
		s.mu.RLock()
		eval, ok := s.evaluators[trigger.Type]
		s.mu.RUnlock()

		if !ok {
			s.logger.Warn("no evaluator for trigger type",
				"trigger", trigger.ID,
				"type", trigger.Type)
			continue
		}

		// Check if trigger should fire
		shouldFire, reason, err := eval.Evaluate(ctx, trigger, event, s)
		if err != nil {
			s.logger.Error("error evaluating trigger",
				"trigger", trigger.ID,
				"error", err)
			continue
		}

		if !shouldFire {
			continue
		}

		// Check trigger-specific cooldown
		if !s.checkCooldown(ctx, trigger) {
			s.logger.Debug("trigger cooldown active",
				"trigger", trigger.ID)
			continue
		}

		// Fire trigger
		if err := s.fireTrigger(ctx, trigger, reason); err != nil {
			s.logger.Error("error firing trigger",
				"trigger", trigger.ID,
				"error", err)
			continue
		}
	}

	return nil
}

// checkCooldown verifies if a trigger is past its cooldown period.
func (s *Service) checkCooldown(ctx context.Context, trigger *Trigger) bool {
	// Task-based cooldown
	if trigger.Cooldown.Tasks > 0 {
		count, err := s.db.GetCounter(ctx, trigger.ID, "cooldown")
		if err != nil {
			s.logger.Warn("error getting cooldown counter",
				"trigger", trigger.ID,
				"error", err)
			return true // Proceed if we can't check
		}
		if count < trigger.Cooldown.Tasks {
			return false
		}
	}

	// Time-based cooldown (team mode only)
	if trigger.Cooldown.Duration > 0 && s.cfg.IsTeamMode() {
		if trigger.LastTriggeredAt != nil {
			if time.Since(*trigger.LastTriggeredAt) < trigger.Cooldown.Duration {
				return false
			}
		}
	}

	return true
}

// fireTrigger executes a trigger action.
func (s *Service) fireTrigger(ctx context.Context, trigger *Trigger, reason string) error {
	s.logger.Info("firing trigger",
		"trigger", trigger.ID,
		"reason", reason)

	// Update global cooldown
	s.mu.Lock()
	s.lastGlobalTrigger = time.Now()
	s.mu.Unlock()

	// Create execution record
	now := time.Now()
	exec := &Execution{
		TriggerID:     trigger.ID,
		TriggeredAt:   now,
		TriggerReason: reason,
		Status:        StatusPending,
	}

	if err := s.db.CreateExecution(ctx, exec); err != nil {
		return fmt.Errorf("create execution record: %w", err)
	}

	// Update trigger state atomically (prevents race condition in concurrent updates)
	newCount, err := s.db.IncrementTriggerCount(ctx, trigger.ID, now)
	if err != nil {
		s.logger.Warn("error updating trigger state",
			"trigger", trigger.ID,
			"error", err)
	} else {
		// Update in-memory state for logging/debugging purposes only
		trigger.TriggerCount = newCount
		trigger.LastTriggeredAt = &now
	}

	// Reset cooldown counter (critical for preventing trigger storms)
	if trigger.Cooldown.Tasks > 0 {
		if err := s.db.ResetCounter(ctx, trigger.ID, "cooldown"); err != nil {
			// Counter reset failure is critical - log as error, not warning
			// The trigger will still fire, but cooldown tracking may be incorrect
			s.logger.Error("failed to reset cooldown counter - trigger may fire again prematurely",
				"trigger", trigger.ID,
				"error", err)
		}
	}

	// Get execution mode
	mode := s.cfg.GetTriggerMode(config.TriggerConfig{
		ID:   trigger.ID,
		Mode: config.AutomationMode(trigger.Mode),
	})

	// Check if task creator is available
	s.mu.RLock()
	tc := s.taskCreator
	s.mu.RUnlock()

	switch mode {
	case config.AutomationModeAuto:
		if tc == nil {
			s.logger.Warn("automation task creation skipped: no task creator configured",
				"trigger", trigger.ID,
				"template", trigger.Action.Template)
			if err := s.db.UpdateExecutionStatus(ctx, exec.ID, StatusSkipped, "no task creator configured"); err != nil {
				s.logger.Warn("error updating execution status", "error", err)
			}
			return nil
		}

		// Create and run automation task immediately
		taskID, err := tc.CreateAutomationTask(ctx, trigger.Action.Template, trigger.ID, reason)
		if err != nil {
			s.logger.Error("failed to create automation task",
				"trigger", trigger.ID,
				"template", trigger.Action.Template,
				"error", err)
			if dbErr := s.db.UpdateExecutionStatus(ctx, exec.ID, StatusFailed, err.Error()); dbErr != nil {
				s.logger.Warn("error updating execution status", "error", dbErr)
			}
			return fmt.Errorf("create automation task: %w", err)
		}

		s.logger.Info("created automation task",
			"trigger", trigger.ID,
			"template", trigger.Action.Template,
			"task", taskID)

		// Start the task immediately
		if err := tc.StartAutomationTask(ctx, taskID); err != nil {
			s.logger.Error("failed to start automation task",
				"trigger", trigger.ID,
				"task", taskID,
				"error", err)
			if dbErr := s.db.UpdateExecutionStatus(ctx, exec.ID, StatusFailed, err.Error()); dbErr != nil {
				s.logger.Warn("error updating execution status", "error", dbErr)
			}
			return fmt.Errorf("start automation task: %w", err)
		}

		if err := s.db.UpdateExecutionStatus(ctx, exec.ID, StatusRunning, ""); err != nil {
			s.logger.Warn("error updating execution status", "error", err)
		}

	case config.AutomationModeApproval:
		if tc == nil {
			s.logger.Warn("automation task creation skipped: no task creator configured",
				"trigger", trigger.ID,
				"template", trigger.Action.Template)
			if err := s.db.UpdateExecutionStatus(ctx, exec.ID, StatusSkipped, "no task creator configured"); err != nil {
				s.logger.Warn("error updating execution status", "error", err)
			}
			return nil
		}

		// Create pending automation task (don't start it)
		taskID, err := tc.CreateAutomationTask(ctx, trigger.Action.Template, trigger.ID, reason)
		if err != nil {
			s.logger.Error("failed to create pending automation task",
				"trigger", trigger.ID,
				"template", trigger.Action.Template,
				"error", err)
			if dbErr := s.db.UpdateExecutionStatus(ctx, exec.ID, StatusFailed, err.Error()); dbErr != nil {
				s.logger.Warn("error updating execution status", "error", dbErr)
			}
			return fmt.Errorf("create pending automation task: %w", err)
		}

		s.logger.Info("created pending automation task (awaiting approval)",
			"trigger", trigger.ID,
			"template", trigger.Action.Template,
			"task", taskID)

		// Create notification for pending approval
		notif := &Notification{
			ID:         fmt.Sprintf("notif-%s-%d", trigger.ID, now.Unix()),
			Type:       NotificationTypeAutomationPending,
			Title:      "Automation task pending approval",
			Message:    fmt.Sprintf("%s: %s", trigger.Description, reason),
			SourceType: NotificationSourceTask,
			SourceID:   taskID,
			CreatedAt:  now,
		}
		if err := s.db.CreateNotification(ctx, notif); err != nil {
			s.logger.Warn("failed to create notification", "error", err)
		}

		// Task stays in pending status until manually approved
		// Execution stays in pending status

	case config.AutomationModeNotify:
		// Just notify, don't create task
		s.logger.Info("trigger notification (no task created)",
			"trigger", trigger.ID,
			"template", trigger.Action.Template,
			"reason", reason)

		// Create notification for user awareness
		notif := &Notification{
			ID:         fmt.Sprintf("notif-%s-%d", trigger.ID, now.Unix()),
			Type:       NotificationTypeAutomationPending,
			Title:      fmt.Sprintf("Trigger condition met: %s", trigger.Description),
			Message:    fmt.Sprintf("Reason: %s. Template: %s", reason, trigger.Action.Template),
			SourceType: NotificationSourceTrigger,
			SourceID:   trigger.ID,
			CreatedAt:  now,
		}
		if err := s.db.CreateNotification(ctx, notif); err != nil {
			s.logger.Warn("failed to create notification", "error", err)
		}

		if err := s.db.UpdateExecutionStatus(ctx, exec.ID, StatusCompleted, "notification sent"); err != nil {
			s.logger.Warn("error updating execution status", "error", err)
		}
	}

	return nil
}

// IncrementCooldownCounter increments the cooldown counter for all triggers.
// Called after each task completion to track task-based cooldowns.
func (s *Service) IncrementCooldownCounter(ctx context.Context) error {
	for _, trigger := range s.cfg.Automation.Triggers {
		if trigger.Cooldown.Tasks > 0 {
			if err := s.db.IncrementCounter(ctx, trigger.ID, "cooldown"); err != nil {
				return fmt.Errorf("increment cooldown counter for %s: %w", trigger.ID, err)
			}
		}
	}
	return nil
}

// configToTrigger converts a config trigger to an automation trigger.
func (s *Service) configToTrigger(cfg *config.TriggerConfig) *Trigger {
	return &Trigger{
		ID:          cfg.ID,
		Type:        TriggerType(cfg.Type),
		Description: cfg.Description,
		Enabled:     cfg.Enabled,
		Mode:        ExecutionMode(cfg.Mode),
		Condition: Condition{
			Metric:     cfg.Condition.Metric,
			Threshold:  cfg.Condition.Threshold,
			Weights:    cfg.Condition.Weights,
			Categories: cfg.Condition.Categories,
			Event:      cfg.Condition.Event,
			Filter:     cfg.Condition.Filter,
			Operator:   cfg.Condition.Operator,
			Value:      cfg.Condition.Value,
			Schedule:   cfg.Condition.Schedule,
		},
		Action: Action{
			Template: cfg.Action.Template,
			Priority: cfg.Action.Priority,
			Queue:    cfg.Action.Queue,
		},
		Cooldown: Cooldown{
			Tasks:    cfg.Cooldown.Tasks,
			Duration: cfg.Cooldown.Duration,
		},
	}
}

// GetStats returns automation statistics.
func (s *Service) GetStats(ctx context.Context) (*Stats, error) {
	triggers := s.cfg.Automation.Triggers

	stats := &Stats{
		TotalTriggers: len(triggers),
	}

	for _, t := range triggers {
		if t.Enabled {
			stats.EnabledTriggers++
		}
	}

	// Get execution stats from database
	execStats, err := s.db.GetExecutionStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("get execution stats: %w", err)
	}

	stats.PendingTasks = execStats.Pending
	stats.RunningTasks = execStats.Running
	stats.CompletedTasks = execStats.Completed
	stats.FailedTasks = execStats.Failed

	return stats, nil
}

// RunTrigger manually fires a specific trigger, bypassing condition evaluation.
// This is used by the CLI "orc automation run" and the API endpoint.
func (s *Service) RunTrigger(ctx context.Context, triggerID string) error {
	// Find trigger in config
	var triggerCfg *config.TriggerConfig
	for i := range s.cfg.Automation.Triggers {
		if s.cfg.Automation.Triggers[i].ID == triggerID {
			triggerCfg = &s.cfg.Automation.Triggers[i]
			break
		}
	}

	if triggerCfg == nil {
		return fmt.Errorf("trigger %q not found", triggerID)
	}

	trigger := s.configToTrigger(triggerCfg)
	return s.fireTrigger(ctx, trigger, "manual execution via CLI/API")
}
