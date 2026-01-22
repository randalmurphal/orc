// Package events provides event types and publishing infrastructure for orc.
package events

import (
	"time"
)

// EventType defines the type of event.
type EventType string

const (
	// EventState indicates a full state update.
	EventState EventType = "state"
	// EventTranscript indicates a new transcript line.
	EventTranscript EventType = "transcript"
	// EventPhase indicates a phase status change.
	EventPhase EventType = "phase"
	// EventError indicates an error occurred.
	EventError EventType = "error"
	// EventComplete indicates task completion.
	EventComplete EventType = "complete"
	// EventTokens indicates token usage update.
	EventTokens EventType = "tokens"

	// Progress events (for long-running operations)

	// EventActivity indicates activity state changed (waiting_api, streaming, etc.).
	EventActivity EventType = "activity"
	// EventHeartbeat indicates the task is still running (progress heartbeat).
	EventHeartbeat EventType = "heartbeat"
	// EventWarning indicates a non-fatal warning.
	EventWarning EventType = "warning"

	// File watcher events (triggered by external file changes)

	// EventTaskCreated indicates a new task was created via file system.
	EventTaskCreated EventType = "task_created"
	// EventTaskUpdated indicates a task was modified via file system.
	EventTaskUpdated EventType = "task_updated"
	// EventTaskDeleted indicates a task was deleted via file system.
	EventTaskDeleted EventType = "task_deleted"

	// Initiative events (triggered by initiative file changes)

	// EventInitiativeCreated indicates a new initiative was created.
	EventInitiativeCreated EventType = "initiative_created"
	// EventInitiativeUpdated indicates an initiative was modified.
	EventInitiativeUpdated EventType = "initiative_updated"
	// EventInitiativeDeleted indicates an initiative was deleted.
	EventInitiativeDeleted EventType = "initiative_deleted"

	// Session-level events (not tied to a specific task)

	// EventSessionUpdate indicates session metrics changed (tokens, cost, duration, running tasks).
	EventSessionUpdate EventType = "session_update"

	// Gate decision events (for human approval gates in headless mode)

	// EventDecisionRequired indicates a human gate requires approval.
	EventDecisionRequired EventType = "decision_required"
	// EventDecisionResolved indicates a gate decision was approved or rejected.
	EventDecisionResolved EventType = "decision_resolved"
)

// Event represents a published event.
type Event struct {
	Type   EventType `json:"type"`
	TaskID string    `json:"task_id"`
	Data   any       `json:"data"`
	Time   time.Time `json:"time"`
}

// NewEvent creates a new event with the current timestamp.
func NewEvent(eventType EventType, taskID string, data any) Event {
	return Event{
		Type:   eventType,
		TaskID: taskID,
		Data:   data,
		Time:   time.Now(),
	}
}

// TranscriptLine represents a single transcript entry.
type TranscriptLine struct {
	Phase     string    `json:"phase"`
	Iteration int       `json:"iteration"`
	Type      string    `json:"type"` // prompt, response, tool, error
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// PhaseUpdate represents a phase status change.
type PhaseUpdate struct {
	Phase     string `json:"phase"`
	Status    string `json:"status"` // started, completed, failed, skipped
	CommitSHA string `json:"commit_sha,omitempty"`
	Error     string `json:"error,omitempty"`
}

// TokenUpdate represents token usage information.
type TokenUpdate struct {
	Phase                    string `json:"phase"`
	InputTokens              int    `json:"input_tokens"`
	OutputTokens             int    `json:"output_tokens"`
	CacheCreationInputTokens int    `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int    `json:"cache_read_input_tokens,omitempty"`
	TotalTokens              int    `json:"total_tokens"`
}

// ErrorData represents error information.
type ErrorData struct {
	Phase   string `json:"phase,omitempty"`
	Message string `json:"message"`
	Fatal   bool   `json:"fatal"`
}

// CompleteData represents task completion information.
type CompleteData struct {
	Status    string `json:"status"` // completed, failed
	Duration  string `json:"duration,omitempty"`
	CommitSHA string `json:"commit_sha,omitempty"`
}

// ActivityUpdate represents activity state change information.
type ActivityUpdate struct {
	Phase    string `json:"phase"`
	Activity string `json:"activity"` // idle, waiting_api, streaming, running_tool, processing, spec_analyzing, spec_writing
}

// IsSpecPhaseActivity returns true if this is a spec-phase-specific activity state.
func (a ActivityUpdate) IsSpecPhaseActivity() bool {
	return a.Activity == "spec_analyzing" || a.Activity == "spec_writing"
}

// HeartbeatData represents a progress heartbeat.
type HeartbeatData struct {
	Phase     string    `json:"phase"`
	Iteration int       `json:"iteration"`
	Timestamp time.Time `json:"timestamp"`
}

// WarningData represents a non-fatal warning.
type WarningData struct {
	Phase   string `json:"phase,omitempty"`
	Message string `json:"message"`
}

// SessionUpdate represents session-level metrics for real-time dashboard updates.
// This event is broadcast:
// - Every 10 seconds while tasks are running (heartbeat interval)
// - Immediately when a task starts (tasks_running changes)
// - Immediately when a task completes (tokens/cost change)
// - Immediately when pause/resume is triggered (is_paused changes)
// - No broadcasts when idle (no tasks running)
type SessionUpdate struct {
	// DurationSeconds is the elapsed time since session start (or first task start).
	DurationSeconds int64 `json:"duration_seconds"`
	// TotalTokens is the aggregate token usage across all tasks in this session.
	TotalTokens int `json:"total_tokens"`
	// EstimatedCostUSD is the aggregate estimated cost for this session.
	EstimatedCostUSD float64 `json:"estimated_cost_usd"`
	// InputTokens is the total input tokens used in this session.
	InputTokens int `json:"input_tokens"`
	// OutputTokens is the total output tokens used in this session.
	OutputTokens int `json:"output_tokens"`
	// TasksRunning is the count of tasks with status=running.
	TasksRunning int `json:"tasks_running"`
	// IsPaused indicates whether the executor is in a paused state.
	IsPaused bool `json:"is_paused"`
}

// DecisionRequiredData represents a pending gate decision.
type DecisionRequiredData struct {
	DecisionID  string    `json:"decision_id"` // e.g., "gate_TASK-001_review"
	TaskID      string    `json:"task_id"`
	TaskTitle   string    `json:"task_title"`
	Phase       string    `json:"phase"`
	GateType    string    `json:"gate_type"` // Always "human" for these events
	Question    string    `json:"question"`
	Context     string    `json:"context"`
	RequestedAt time.Time `json:"requested_at"`
}

// DecisionResolvedData represents a resolved gate decision.
type DecisionResolvedData struct {
	DecisionID string    `json:"decision_id"`
	TaskID     string    `json:"task_id"`
	Phase      string    `json:"phase"`
	Approved   bool      `json:"approved"`
	Reason     string    `json:"reason,omitempty"`
	ResolvedBy string    `json:"resolved_by"` // "api" or "cli"
	ResolvedAt time.Time `json:"resolved_at"`
}
