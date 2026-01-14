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

	// File watcher events (triggered by external file changes)

	// EventTaskCreated indicates a new task was created via file system.
	EventTaskCreated EventType = "task_created"
	// EventTaskUpdated indicates a task was modified via file system.
	EventTaskUpdated EventType = "task_updated"
	// EventTaskDeleted indicates a task was deleted via file system.
	EventTaskDeleted EventType = "task_deleted"
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
