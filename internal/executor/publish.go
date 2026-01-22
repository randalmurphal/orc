// Package executor provides the flowgraph-based execution engine for orc.
package executor

import (
	"time"

	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/state"
)

// EventPublisher wraps event publishing with nil-safety and convenience methods.
// All methods are safe to call even when the underlying publisher is nil.
//
// Transcript persistence is handled separately via JSONL sync from Claude Code's
// session files, not through this publisher.
//
// Thread-safe: All methods can be called concurrently.
type EventPublisher struct {
	publisher events.Publisher
}

// NewEventPublisher creates a new EventPublisher wrapping the given publisher.
// If p is nil, all publish operations become no-ops.
func NewEventPublisher(p events.Publisher) *EventPublisher {
	return &EventPublisher{publisher: p}
}

// Publish sends an event to the underlying publisher.
// Safe to call with nil publisher (no-op).
func (ep *EventPublisher) Publish(ev events.Event) {
	if ep == nil || ep.publisher == nil {
		return
	}
	ep.publisher.Publish(ev)
}

// PhaseStart publishes a phase start event.
func (ep *EventPublisher) PhaseStart(taskID, phase string) {
	ep.Publish(events.NewEvent(events.EventPhase, taskID, events.PhaseUpdate{
		Phase:  phase,
		Status: string(plan.PhaseRunning),
	}))
}

// PhaseComplete publishes a phase completion event with optional commit SHA.
func (ep *EventPublisher) PhaseComplete(taskID, phase, commitSHA string) {
	ep.Publish(events.NewEvent(events.EventPhase, taskID, events.PhaseUpdate{
		Phase:     phase,
		Status:    string(plan.PhaseCompleted),
		CommitSHA: commitSHA,
	}))
}

// PhaseFailed publishes a phase failure event with the error message.
func (ep *EventPublisher) PhaseFailed(taskID, phase string, err error) {
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}
	ep.Publish(events.NewEvent(events.EventPhase, taskID, events.PhaseUpdate{
		Phase:  phase,
		Status: string(plan.PhaseFailed),
		Error:  errMsg,
	}))
}

// Transcript publishes a transcript line event (prompt, response, tool, error).
// Database persistence is handled separately via JSONL sync.
func (ep *EventPublisher) Transcript(taskID, phase string, iteration int, msgType, content string) {
	ep.Publish(events.NewEvent(events.EventTranscript, taskID, events.TranscriptLine{
		Phase:     phase,
		Iteration: iteration,
		Type:      msgType,
		Content:   content,
		Timestamp: time.Now(),
	}))
}

// TranscriptChunk publishes a streaming transcript chunk event.
// Database persistence is handled separately via JSONL sync.
func (ep *EventPublisher) TranscriptChunk(taskID, phase string, iteration int, chunk string) {
	ep.Publish(events.NewEvent(events.EventTranscript, taskID, events.TranscriptLine{
		Phase:     phase,
		Iteration: iteration,
		Type:      "chunk",
		Content:   chunk,
		Timestamp: time.Now(),
	}))
}

// Tokens publishes a token usage update event.
func (ep *EventPublisher) Tokens(taskID, phase string, input, output, cacheCreation, cacheRead, total int) {
	ep.Publish(events.NewEvent(events.EventTokens, taskID, events.TokenUpdate{
		Phase:                    phase,
		InputTokens:              input,
		OutputTokens:             output,
		CacheCreationInputTokens: cacheCreation,
		CacheReadInputTokens:     cacheRead,
		TotalTokens:              total,
	}))
}

// Error publishes an error event.
// Set fatal to true if this error will cause task termination.
func (ep *EventPublisher) Error(taskID, phase, message string, fatal bool) {
	ep.Publish(events.NewEvent(events.EventError, taskID, events.ErrorData{
		Phase:   phase,
		Message: message,
		Fatal:   fatal,
	}))
}

// State publishes a full state update event.
func (ep *EventPublisher) State(taskID string, s *state.State) {
	if s == nil {
		return
	}
	ep.Publish(events.NewEvent(events.EventState, taskID, s))
}

// Activity publishes an activity state change event.
func (ep *EventPublisher) Activity(taskID, phase, activity string) {
	ep.Publish(events.NewEvent(events.EventActivity, taskID, events.ActivityUpdate{
		Phase:    phase,
		Activity: activity,
	}))
}

// Heartbeat publishes a heartbeat event indicating the task is still running.
func (ep *EventPublisher) Heartbeat(taskID, phase string, iteration int) {
	ep.Publish(events.NewEvent(events.EventHeartbeat, taskID, events.HeartbeatData{
		Phase:     phase,
		Iteration: iteration,
		Timestamp: time.Now(),
	}))
}

// Warning publishes a warning event (non-fatal).
func (ep *EventPublisher) Warning(taskID, phase, message string) {
	ep.Publish(events.NewEvent(events.EventWarning, taskID, events.WarningData{
		Phase:   phase,
		Message: message,
	}))
}

// Session publishes a session update event with aggregate metrics.
// Session events use an empty task ID as they represent session-level state.
func (ep *EventPublisher) Session(update events.SessionUpdate) {
	// Use GlobalTaskID so all subscribers receive session updates
	ep.Publish(events.NewEvent(events.EventSessionUpdate, events.GlobalTaskID, update))
}

// FilesChanged publishes a files changed event with file change details.
func (ep *EventPublisher) FilesChanged(taskID string, update events.FilesChangedUpdate) {
	ep.Publish(events.NewEvent(events.EventFilesChanged, taskID, update))
}
