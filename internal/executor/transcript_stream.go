// Package executor provides the execution engine for orc.
// This file provides real-time transcript streaming via the OnEvent callback.
package executor

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/randalmurphal/llmkit/claude"
	"github.com/randalmurphal/llmkit/codex"
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

	// captureHookEvents filters which hook events to store.
	// If empty, ALL hook events are captured for debugging.
	// If set, only matching hook event types are stored (e.g., "PreToolUse", "PostToolUse").
	captureHookEvents []string
}

// NewTranscriptStreamHandler creates a handler for streaming transcript capture.
func NewTranscriptStreamHandler(
	backend storage.Backend,
	logger *slog.Logger,
	taskID, phaseID, sessionID, runID, model string,
	captureHookEvents []string,
) *TranscriptStreamHandler {
	return &TranscriptStreamHandler{
		backend:           backend,
		logger:            logger,
		taskID:            taskID,
		phaseID:           phaseID,
		sessionID:         sessionID,
		runID:             runID,
		model:             model,
		storedMessageIDs:  make(map[string]bool),
		captureHookEvents: captureHookEvents,
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
		// Final result - log structured output status for debugging
		hasStructuredOutput := len(event.Result.StructuredOutput) > 0
		h.logger.Debug("claude execution complete",
			"session_id", event.SessionID,
			"task", h.taskID,
			"phase", h.phaseID,
			"num_turns", event.Result.NumTurns,
			"cost_usd", event.Result.TotalCostUSD,
			"has_structured_output", hasStructuredOutput,
			"subtype", event.Result.Subtype,
		)
		if !hasStructuredOutput && event.Result.Subtype != "success" {
			h.logger.Warn("claude result without structured output",
				"task", h.taskID,
				"phase", h.phaseID,
				"subtype", event.Result.Subtype,
				"is_error", event.Result.IsError,
			)
		}
	case claude.StreamEventHook:
		// Hook execution - store based on captureHookEvents config
		if event.Hook != nil {
			if h.shouldCaptureHook(event.Hook.HookEvent) {
				h.storeHookEvent(event)
			}
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

// shouldCaptureHook checks if a hook event should be captured based on config.
// If captureHookEvents is empty, all hook events are captured (for debugging).
// If set, only events matching the list are captured.
func (h *TranscriptStreamHandler) shouldCaptureHook(hookEvent string) bool {
	if len(h.captureHookEvents) == 0 {
		return true // Capture all if not specified
	}
	return slices.Contains(h.captureHookEvents, hookEvent)
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

// StoreAssistantText stores a plain text assistant response.
// Used by Codex and other providers that return text instead of content blocks.
func (h *TranscriptStreamHandler) StoreAssistantText(text, model, messageID string, inputTokens, outputTokens int) {
	if h.backend == nil || h.taskID == "" {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if messageID == "" {
		messageID = uuid.NewString()
	}

	// Skip duplicates
	if h.storedMessageIDs[messageID] {
		return
	}
	h.storedMessageIDs[messageID] = true

	if model == "" {
		model = h.model
	}

	transcript := &storage.Transcript{
		TaskID:        h.taskID,
		Phase:         h.phaseID,
		SessionID:     h.sessionID,
		WorkflowRunID: h.runID,
		MessageUUID:   messageID,
		Type:          "assistant",
		Role:          "assistant",
		Content:       text,
		Model:         model,
		InputTokens:   inputTokens,
		OutputTokens:  outputTokens,
		Timestamp:     time.Now().UnixMilli(),
	}

	if err := h.backend.AddTranscript(transcript); err != nil {
		h.logger.Warn("failed to store assistant text",
			"task", h.taskID,
			"phase", h.phaseID,
			"message_id", messageID,
			"error", err,
		)
	}
}

// OnCodexResponse stores transcripts from a Codex completion response.
// Codex returns complete responses (not streamed events), so this handles
// the full response in one call.
func (h *TranscriptStreamHandler) OnCodexResponse(resp *codex.CompletionResponse) {
	if resp == nil || h.backend == nil || h.taskID == "" {
		return
	}

	// Store the assistant response
	messageID := fmt.Sprintf("codex-%s-%d", resp.SessionID, time.Now().UnixMilli())
	h.StoreAssistantText(
		resp.Content,
		resp.Model,
		messageID,
		resp.Usage.InputTokens,
		resp.Usage.OutputTokens,
	)
}
