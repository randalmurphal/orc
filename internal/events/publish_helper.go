// Package events provides event types and publishing infrastructure for orc.
package events

import (
	"time"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
)

// PublishHelper wraps event publishing with nil-safety and convenience methods.
// All methods are safe to call even when the underlying publisher is nil.
//
// Transcript persistence is handled separately via JSONL sync from Claude Code's
// session files, not through this publisher.
//
// Thread-safe: All methods can be called concurrently.
type PublishHelper struct {
	publisher Publisher
}

// NewPublishHelper creates a new PublishHelper wrapping the given publisher.
// If p is nil, all publish operations become no-ops.
func NewPublishHelper(p Publisher) *PublishHelper {
	return &PublishHelper{publisher: p}
}

// Publish sends an event to the underlying publisher.
// Safe to call with nil publisher (no-op).
func (ep *PublishHelper) Publish(ev Event) {
	if ep == nil || ep.publisher == nil {
		return
	}
	ep.publisher.Publish(ev)
}

// PhaseStart publishes a phase start event.
func (ep *PublishHelper) PhaseStart(taskID, phase string) {
	ep.Publish(NewEvent(EventPhase, taskID, PhaseUpdate{
		Phase:  phase,
		Status: "running",
	}))
}

// PhaseComplete publishes a phase completion event with optional commit SHA.
func (ep *PublishHelper) PhaseComplete(taskID, phase, commitSHA string) {
	ep.Publish(NewEvent(EventPhase, taskID, PhaseUpdate{
		Phase:     phase,
		Status:    "completed",
		CommitSHA: commitSHA,
	}))
}

// PhaseFailed publishes a phase failure event with the error message.
func (ep *PublishHelper) PhaseFailed(taskID, phase string, err error) {
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}
	ep.Publish(NewEvent(EventPhase, taskID, PhaseUpdate{
		Phase:  phase,
		Status: "failed",
		Error:  errMsg,
	}))
}

// Transcript publishes a transcript line event (prompt, response, tool, error).
// Database persistence is handled separately via JSONL sync.
func (ep *PublishHelper) Transcript(taskID, phase string, iteration int, msgType, content string) {
	ep.Publish(NewEvent(EventTranscript, taskID, TranscriptLine{
		Phase:     phase,
		Iteration: iteration,
		Type:      msgType,
		Content:   content,
		Timestamp: time.Now(),
	}))
}

// TranscriptChunk publishes a streaming transcript chunk event.
// Database persistence is handled separately via JSONL sync.
func (ep *PublishHelper) TranscriptChunk(taskID, phase string, iteration int, chunk string) {
	ep.Publish(NewEvent(EventTranscript, taskID, TranscriptLine{
		Phase:     phase,
		Iteration: iteration,
		Type:      "chunk",
		Content:   chunk,
		Timestamp: time.Now(),
	}))
}

// Tokens publishes a token usage update event.
func (ep *PublishHelper) Tokens(taskID, phase string, input, output, cacheCreation, cacheRead, total int) {
	ep.Publish(NewEvent(EventTokens, taskID, TokenUpdate{
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
func (ep *PublishHelper) Error(taskID, phase, message string, fatal bool) {
	ep.Publish(NewEvent(EventError, taskID, ErrorData{
		Phase:   phase,
		Message: message,
		Fatal:   fatal,
	}))
}

// State publishes a full state update event.
func (ep *PublishHelper) State(taskID string, s *orcv1.ExecutionState) {
	if s == nil {
		return
	}
	ep.Publish(NewEvent(EventState, taskID, s))
}

// Activity publishes an activity state change event.
func (ep *PublishHelper) Activity(taskID, phase, activity string) {
	ep.Publish(NewEvent(EventActivity, taskID, ActivityUpdate{
		Phase:    phase,
		Activity: activity,
	}))
}

// Heartbeat publishes a heartbeat event indicating the task is still running.
func (ep *PublishHelper) Heartbeat(taskID, phase string, iteration int) {
	ep.Publish(NewEvent(EventHeartbeat, taskID, HeartbeatData{
		Phase:     phase,
		Iteration: iteration,
		Timestamp: time.Now(),
	}))
}

// Warning publishes a warning event (non-fatal).
func (ep *PublishHelper) Warning(taskID, phase, message string) {
	ep.Publish(NewEvent(EventWarning, taskID, WarningData{
		Phase:   phase,
		Message: message,
	}))
}

// Session publishes a session update event with aggregate metrics.
// Session events use an empty task ID as they represent session-level state.
func (ep *PublishHelper) Session(update SessionUpdate) {
	// Use GlobalTaskID so all subscribers receive session updates
	ep.Publish(NewEvent(EventSessionUpdate, GlobalTaskID, update))
}

// FilesChanged publishes a files changed event with file change details.
func (ep *PublishHelper) FilesChanged(taskID string, update FilesChangedUpdate) {
	ep.Publish(NewEvent(EventFilesChanged, taskID, update))
}

// DecisionRequired publishes a decision_required event for human gates.
func (ep *PublishHelper) DecisionRequired(taskID string, data DecisionRequiredData) {
	ep.Publish(NewEvent(EventDecisionRequired, taskID, data))
}

// DecisionResolved publishes a decision_resolved event after gate resolution.
func (ep *PublishHelper) DecisionResolved(taskID string, data DecisionResolvedData) {
	ep.Publish(NewEvent(EventDecisionResolved, taskID, data))
}
