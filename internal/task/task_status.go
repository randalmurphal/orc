// Package task provides task management for orc.
package task

// Status represents the current state of a task.
type Status string

const (
	StatusCreated     Status = "created"
	StatusClassifying Status = "classifying"
	StatusPlanned     Status = "planned"
	StatusRunning     Status = "running"
	StatusPaused      Status = "paused"
	StatusBlocked     Status = "blocked"
	StatusFinalizing  Status = "finalizing" // Post-completion: cleanup, PR creation, branch sync
	StatusCompleted   Status = "completed"  // Terminal: all phases AND sync/PR/merge succeeded
	StatusFailed      Status = "failed"
	StatusResolved    Status = "resolved" // Terminal: failed task marked as resolved without re-running
)

// ValidStatuses returns all valid status values.
func ValidStatuses() []Status {
	return []Status{
		StatusCreated, StatusClassifying, StatusPlanned, StatusRunning,
		StatusPaused, StatusBlocked, StatusFinalizing, StatusCompleted,
		StatusFailed, StatusResolved,
	}
}

// IsValidStatus returns true if the status is a valid status value.
func IsValidStatus(s Status) bool {
	switch s {
	case StatusCreated, StatusClassifying, StatusPlanned, StatusRunning,
		StatusPaused, StatusBlocked, StatusFinalizing, StatusCompleted,
		StatusFailed, StatusResolved:
		return true
	default:
		return false
	}
}

// IsTerminal returns true if the task is in a terminal state.
func (t *Task) IsTerminal() bool {
	return t.Status == StatusCompleted || t.Status == StatusFailed || t.Status == StatusResolved
}

// CanRun returns true if the task can be executed.
func (t *Task) CanRun() bool {
	return t.Status == StatusCreated ||
		t.Status == StatusPlanned ||
		t.Status == StatusPaused ||
		t.Status == StatusBlocked
}

// isDone returns true if the status indicates the task has completed its work.
func isDone(s Status) bool {
	return s == StatusCompleted || s == StatusResolved
}
