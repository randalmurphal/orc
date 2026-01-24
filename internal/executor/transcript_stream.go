// Package executor provides the execution engine for orc.
// This file provides real-time transcript streaming via the OnEvent callback.
package executor

import (
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/randalmurphal/llmkit/claude"
	"github.com/randalmurphal/orc/internal/storage"
)

// TranscriptStreamHandler captures transcripts in real-time as streaming events arrive.
// This ensures no transcript data is lost if the process dies mid-execution.
type TranscriptStreamHandler struct {
	backend   storage.Backend
	logger    *slog.Logger
	taskID    string
	phaseID   string
	sessionID string
	runID     string // workflow run ID for linking
	model     string
	mu        sync.Mutex // protects writes

	// storedMessageIDs tracks which messages we've already stored to avoid duplicates.
	// Claude streams multiple events with the same message ID as content is generated.
	storedMessageIDs map[string]bool
}

// NewTranscriptStreamHandler creates a handler for streaming transcript capture.
func NewTranscriptStreamHandler(
	backend storage.Backend,
	logger *slog.Logger,
	taskID, phaseID, sessionID, runID, model string,
) *TranscriptStreamHandler {
	return &TranscriptStreamHandler{
		backend:          backend,
		logger:           logger,
		taskID:           taskID,
		phaseID:          phaseID,
		sessionID:        sessionID,
		runID:            runID,
		model:            model,
		storedMessageIDs: make(map[string]bool),
	}
}

// StoreUserPrompt stores the user prompt BEFORE calling Claude.
// This ensures we have a record even if Claude crashes mid-execution.
func (h *TranscriptStreamHandler) StoreUserPrompt(prompt string) {
	if h.backend == nil || h.taskID == "" {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	transcript := &storage.Transcript{
		TaskID:        h.taskID,
		Phase:         h.phaseID,
		SessionID:     h.sessionID,
		WorkflowRunID: h.runID,
		MessageUUID:   uuid.NewString(),
		Type:          "user",
		Role:          "user",
		Content:       prompt,
		Timestamp:     time.Now().UnixMilli(),
	}

	if err := h.backend.AddTranscript(transcript); err != nil {
		h.logger.Warn("failed to store user prompt",
			"task", h.taskID,
			"phase", h.phaseID,
			"error", err,
		)
	}
}

// OnEvent handles streaming events from Claude and stores transcripts in real-time.
// This is passed to CompletionRequest.OnEvent for immediate capture.
func (h *TranscriptStreamHandler) OnEvent(event claude.StreamEvent) {
	if h.backend == nil || h.taskID == "" {
		return
	}

	switch event.Type {
	case claude.StreamEventAssistant:
		h.storeAssistantMessage(event)
	case claude.StreamEventInit:
		// Could log session start if needed
		h.logger.Debug("claude session initialized",
			"session_id", event.SessionID,
			"task", h.taskID,
			"phase", h.phaseID,
		)
	case claude.StreamEventResult:
		// Final result - could store summary if needed
		h.logger.Debug("claude execution complete",
			"session_id", event.SessionID,
			"task", h.taskID,
			"phase", h.phaseID,
			"num_turns", event.Result.NumTurns,
			"cost_usd", event.Result.TotalCostUSD,
		)
	case claude.StreamEventHook:
		// Hook execution - store for debugging
		if event.Hook != nil {
			h.storeHookEvent(event)
		}
	case claude.StreamEventError:
		h.logger.Error("claude streaming error",
			"task", h.taskID,
			"phase", h.phaseID,
			"error", event.Error,
		)
	}
}

// storeAssistantMessage stores a single assistant message from the stream.
// Claude streams multiple events with the same message ID - we only store the first one
// since later events for the same ID are just partial updates that we already have.
func (h *TranscriptStreamHandler) storeAssistantMessage(event claude.StreamEvent) {
	if event.Assistant == nil {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	// Use the API message ID if available, otherwise generate one
	messageUUID := event.Assistant.MessageID
	if messageUUID == "" {
		messageUUID = uuid.NewString()
	}

	// Skip if we've already stored this message (Claude streams multiple events per message)
	if h.storedMessageIDs[messageUUID] {
		return
	}
	h.storedMessageIDs[messageUUID] = true

	// Serialize content blocks to JSON for storage
	contentJSON, err := json.Marshal(event.Assistant.Content)
	if err != nil {
		h.logger.Warn("failed to marshal content blocks",
			"task", h.taskID,
			"error", err,
		)
		contentJSON = []byte(event.Assistant.Text) // Fallback to text
	}

	// Determine model - use from event if available, fall back to handler default
	model := event.Assistant.Model
	if model == "" {
		model = h.model
	}

	transcript := &storage.Transcript{
		TaskID:              h.taskID,
		Phase:               h.phaseID,
		SessionID:           h.sessionID,
		WorkflowRunID:       h.runID,
		MessageUUID:         messageUUID,
		Type:                "assistant",
		Role:                "assistant",
		Content:             string(contentJSON),
		Model:               model,
		InputTokens:         event.Assistant.Usage.InputTokens,
		OutputTokens:        event.Assistant.Usage.OutputTokens,
		CacheCreationTokens: event.Assistant.Usage.CacheCreationInputTokens,
		CacheReadTokens:     event.Assistant.Usage.CacheReadInputTokens,
		Timestamp:           time.Now().UnixMilli(),
	}

	if err := h.backend.AddTranscript(transcript); err != nil {
		h.logger.Warn("failed to store assistant message",
			"task", h.taskID,
			"phase", h.phaseID,
			"message_id", messageUUID,
			"error", err,
		)
	}
}

// storeHookEvent stores hook execution details for debugging.
func (h *TranscriptStreamHandler) storeHookEvent(event claude.StreamEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Serialize hook details
	hookData := map[string]any{
		"hook_name":  event.Hook.HookName,
		"hook_event": event.Hook.HookEvent,
		"stdout":     event.Hook.Stdout,
		"stderr":     event.Hook.Stderr,
		"exit_code":  event.Hook.ExitCode,
	}
	contentJSON, _ := json.Marshal(hookData)

	transcript := &storage.Transcript{
		TaskID:        h.taskID,
		Phase:         h.phaseID,
		SessionID:     h.sessionID,
		WorkflowRunID: h.runID,
		MessageUUID:   uuid.NewString(),
		Type:          "hook",
		Role:          "system",
		Content:       string(contentJSON),
		Timestamp:     time.Now().UnixMilli(),
	}

	if err := h.backend.AddTranscript(transcript); err != nil {
		h.logger.Warn("failed to store hook event",
			"task", h.taskID,
			"phase", h.phaseID,
			"hook", event.Hook.HookName,
			"error", err,
		)
	}
}
