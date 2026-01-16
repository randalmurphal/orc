// Package automation provides trigger-based automation for orc tasks.
// It enables automatic execution of maintenance tasks based on configurable
// conditions like task count, initiative completion, events, and thresholds.
package automation

import (
	"fmt"
	"time"
)

// TriggerType defines the type of trigger.
type TriggerType string

const (
	TriggerTypeCount      TriggerType = "count"      // Fire after N tasks/phases complete
	TriggerTypeInitiative TriggerType = "initiative" // Fire on initiative events
	TriggerTypeEvent      TriggerType = "event"      // Fire on specific events (pr_merged, etc.)
	TriggerTypeThreshold  TriggerType = "threshold"  // Fire when metric crosses value
	TriggerTypeSchedule   TriggerType = "schedule"   // Fire on cron schedule (team mode only)
)

// ExecutionMode defines how automation tasks are executed.
type ExecutionMode string

const (
	ModeAuto     ExecutionMode = "auto"     // Fire and execute without prompts
	ModeApproval ExecutionMode = "approval" // Create in pending state, require human approval
	ModeNotify   ExecutionMode = "notify"   // Only notify, human creates task manually
)

// ExecutionStatus represents the execution status of a trigger.
type ExecutionStatus string

const (
	StatusPending   ExecutionStatus = "pending"
	StatusRunning   ExecutionStatus = "running"
	StatusCompleted ExecutionStatus = "completed"
	StatusFailed    ExecutionStatus = "failed"
	StatusSkipped   ExecutionStatus = "skipped"
)

// Trigger represents a trigger definition.
type Trigger struct {
	ID              string        `json:"id" yaml:"id"`
	Type            TriggerType   `json:"type" yaml:"type"`
	Description     string        `json:"description,omitempty" yaml:"description,omitempty"`
	Enabled         bool          `json:"enabled" yaml:"enabled"`
	Mode            ExecutionMode `json:"mode,omitempty" yaml:"mode,omitempty"`
	Condition       Condition     `json:"condition" yaml:"condition"`
	Action          Action        `json:"action" yaml:"action"`
	Cooldown        Cooldown      `json:"cooldown,omitempty" yaml:"cooldown,omitempty"`
	LastTriggeredAt *time.Time    `json:"last_triggered_at,omitempty" yaml:"-"`
	TriggerCount    int           `json:"trigger_count" yaml:"-"`
	CreatedAt       time.Time     `json:"created_at" yaml:"-"`
	UpdatedAt       time.Time     `json:"updated_at" yaml:"-"`
}

// Condition defines when a trigger fires.
type Condition struct {
	// Count-based
	Metric     string   `json:"metric,omitempty" yaml:"metric,omitempty"`         // tasks_completed, large_tasks_completed, phases_completed
	Threshold  int      `json:"threshold,omitempty" yaml:"threshold,omitempty"`   // Number of items before triggering
	Weights    []string `json:"weights,omitempty" yaml:"weights,omitempty"`       // Filter by task weight
	Categories []string `json:"categories,omitempty" yaml:"categories,omitempty"` // Filter by task category

	// Initiative-based / Event-based
	Event  string            `json:"event,omitempty" yaml:"event,omitempty"`   // initiative_completed, pr_merged, task_completed, etc.
	Filter map[string]string `json:"filter,omitempty" yaml:"filter,omitempty"` // Additional filters

	// Threshold-based
	Operator string  `json:"operator,omitempty" yaml:"operator,omitempty"` // lt, gt, eq
	Value    float64 `json:"value,omitempty" yaml:"value,omitempty"`       // Threshold value

	// Schedule-based (team mode only)
	Schedule string `json:"schedule,omitempty" yaml:"schedule,omitempty"` // Cron expression
}

// Action defines what happens when a trigger fires.
type Action struct {
	Template string `json:"template" yaml:"template"` // Template name
	Priority string `json:"priority,omitempty" yaml:"priority,omitempty"`
	Queue    string `json:"queue,omitempty" yaml:"queue,omitempty"`
}

// Cooldown defines the cooldown period for a trigger.
// For solo mode, this is task-count based (e.g., "5 tasks").
// For team mode, this can also be time-based (e.g., "2h").
type Cooldown struct {
	Tasks    int           `json:"tasks,omitempty" yaml:"tasks,omitempty"`       // Number of tasks before retriggering
	Duration time.Duration `json:"duration,omitempty" yaml:"duration,omitempty"` // Time before retriggering (team mode)
}

// UnmarshalYAML handles parsing cooldown from various formats.
func (c *Cooldown) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// Try parsing as string first (e.g., "5 tasks" or "2h")
	var s string
	if err := unmarshal(&s); err == nil {
		return c.parseString(s)
	}

	// Try parsing as struct
	type rawCooldown Cooldown
	var raw rawCooldown
	if err := unmarshal(&raw); err != nil {
		return err
	}
	*c = Cooldown(raw)
	return nil
}

func (c *Cooldown) parseString(s string) error {
	// Parse "N tasks" format
	var tasks int
	if n, _ := fmt.Sscanf(s, "%d tasks", &tasks); n == 1 {
		c.Tasks = tasks
		return nil
	}
	if n, _ := fmt.Sscanf(s, "%d task", &tasks); n == 1 {
		c.Tasks = tasks
		return nil
	}

	// Parse duration format (e.g., "2h", "30m")
	d, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid cooldown format %q: expected 'N tasks' or duration", s)
	}
	c.Duration = d
	return nil
}

// String returns a human-readable representation of the cooldown.
func (c Cooldown) String() string {
	if c.Tasks > 0 {
		return fmt.Sprintf("%d tasks", c.Tasks)
	}
	if c.Duration > 0 {
		return c.Duration.String()
	}
	return "none"
}

// Execution records a trigger firing.
type Execution struct {
	ID            int64           `json:"id"`
	TriggerID     string          `json:"trigger_id"`
	TaskID        string          `json:"task_id,omitempty"`
	TriggeredAt   time.Time       `json:"triggered_at"`
	TriggerReason string          `json:"trigger_reason"`
	Status        ExecutionStatus `json:"status"`
	CompletedAt   *time.Time      `json:"completed_at,omitempty"`
	ErrorMessage  string          `json:"error_message,omitempty"`
}

// Template defines an automation task template.
type Template struct {
	ID          string   `json:"id" yaml:"id"`
	Title       string   `json:"title" yaml:"title"`
	Description string   `json:"description,omitempty" yaml:"description,omitempty"`
	Weight      string   `json:"weight" yaml:"weight"`
	Category    string   `json:"category" yaml:"category"`
	Phases      []string `json:"phases" yaml:"phases"`
	Prompt      string   `json:"prompt" yaml:"prompt"` // Path to prompt template
	Scripts     Scripts  `json:"scripts,omitempty" yaml:"scripts,omitempty"`
}

// Scripts defines pre/post execution scripts.
type Scripts struct {
	Pre  []string `json:"pre,omitempty" yaml:"pre,omitempty"`
	Post []string `json:"post,omitempty" yaml:"post,omitempty"`
}

// Counter tracks task counts for count-based triggers.
type Counter struct {
	TriggerID   string    `json:"trigger_id"`
	Metric      string    `json:"metric"`
	Count       int       `json:"count"`
	LastResetAt time.Time `json:"last_reset_at"`
}

// Metric records a metric value for threshold-based triggers.
type Metric struct {
	ID         int64     `json:"id"`
	Name       string    `json:"metric"`
	Value      float64   `json:"value"`
	TaskID     string    `json:"task_id,omitempty"`
	RecordedAt time.Time `json:"recorded_at"`
}

// Notification represents a notification to the user.
type Notification struct {
	ID         string               `json:"id"`
	Type       string               `json:"type"` // automation_pending, automation_failed, automation_blocked
	Title      string               `json:"title"`
	Message    string               `json:"message,omitempty"`
	SourceType string               `json:"source_type,omitempty"` // trigger, task
	SourceID   string               `json:"source_id,omitempty"`
	Dismissed  bool                 `json:"dismissed"`
	Actions    []NotificationAction `json:"actions,omitempty"`
	CreatedAt  time.Time            `json:"created_at"`
	ExpiresAt  *time.Time           `json:"expires_at,omitempty"`
}

// NotificationAction defines an action button in a notification.
type NotificationAction struct {
	Label  string `json:"label"`
	Href   string `json:"href,omitempty"`
	Action string `json:"action,omitempty"` // dismiss, approve, etc.
}

// Stats provides automation statistics.
type Stats struct {
	TotalTriggers   int `json:"total_triggers"`
	EnabledTriggers int `json:"enabled_triggers"`
	PendingTasks    int `json:"pending_tasks"`
	RunningTasks    int `json:"running_tasks"`
	CompletedTasks  int `json:"completed_tasks"`
	FailedTasks     int `json:"failed_tasks"`
}
