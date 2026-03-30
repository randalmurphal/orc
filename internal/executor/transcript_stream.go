// Package executor provides the execution engine for orc.
// This file provides real-time transcript streaming via the OnEvent callback.
package executor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/randalmurphal/llmkit/v2/claude"
	"github.com/randalmurphal/orc/internal/events"
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
	publisher *events.PublishHelper
	mu        sync.Mutex // protects writes
	err       error

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
	publisher *events.PublishHelper,
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
		publisher:         publisher,
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
	if h.err != nil {
		return
	}

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
		h.err = fmt.Errorf("store user prompt transcript: %w", err)
		return
	}
	if h.publisher != nil {
		h.publisher.Transcript(h.taskID, h.phaseID, 1, "prompt", prompt)
	}
}

// UpdateSessionID updates the session ID used for subsequent transcript rows.
func (h *TranscriptStreamHandler) UpdateSessionID(sessionID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.sessionID = sessionID
}

// Err returns the first execution-critical transcript persistence error.
func (h *TranscriptStreamHandler) Err() error {
	if h == nil {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.err
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
	if h.err != nil {
		return
	}

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
		h.err = fmt.Errorf("marshal assistant transcript content: %w", err)
		return
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
		h.err = fmt.Errorf("store assistant transcript %s: %w", messageUUID, err)
		return
	}
	if h.publisher != nil {
		h.publisher.TranscriptWithUsage(
			h.taskID,
			h.phaseID,
			1,
			"response",
			event.Assistant.Text,
			model,
			&events.TokenUpdate{
				Phase:                    h.phaseID,
				InputTokens:              event.Assistant.Usage.InputTokens,
				OutputTokens:             event.Assistant.Usage.OutputTokens,
				CacheCreationInputTokens: event.Assistant.Usage.CacheCreationInputTokens,
				CacheReadInputTokens:     event.Assistant.Usage.CacheReadInputTokens,
				TotalTokens:              event.Assistant.Usage.InputTokens + event.Assistant.Usage.OutputTokens + event.Assistant.Usage.CacheCreationInputTokens + event.Assistant.Usage.CacheReadInputTokens,
			},
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
	if h.err != nil {
		return
	}

	// Serialize hook details
	hookData := map[string]any{
		"hook_name":  event.Hook.HookName,
		"hook_event": event.Hook.HookEvent,
		"stdout":     event.Hook.Stdout,
		"stderr":     event.Hook.Stderr,
		"exit_code":  event.Hook.ExitCode,
	}
	contentJSON, err := json.Marshal(hookData)
	if err != nil {
		h.err = fmt.Errorf("marshal hook transcript content: %w", err)
		return
	}

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
		h.err = fmt.Errorf("store hook transcript %s: %w", event.Hook.HookName, err)
	}
}

// StoreAssistantText stores a plain text assistant response.
// Used by Codex and other providers that return text instead of content blocks.
func (h *TranscriptStreamHandler) StoreAssistantText(text, model, messageID string, inputTokens, outputTokens int) {
	h.StoreAssistantTextWithUsage(text, model, messageID, inputTokens, outputTokens, 0, 0)
}

// StoreAssistantTextWithUsage stores a plain text assistant response with full token usage.
func (h *TranscriptStreamHandler) StoreAssistantTextWithUsage(
	text, model, messageID string,
	inputTokens, outputTokens, cacheCreationTokens, cacheReadTokens int,
) {
	if h.backend == nil || h.taskID == "" || text == "" {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	if h.err != nil {
		return
	}

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
		TaskID:              h.taskID,
		Phase:               h.phaseID,
		SessionID:           h.sessionID,
		WorkflowRunID:       h.runID,
		MessageUUID:         messageID,
		Type:                "assistant",
		Role:                "assistant",
		Content:             text,
		Model:               model,
		InputTokens:         inputTokens,
		OutputTokens:        outputTokens,
		CacheCreationTokens: cacheCreationTokens,
		CacheReadTokens:     cacheReadTokens,
		Timestamp:           time.Now().UnixMilli(),
	}

	if err := h.backend.AddTranscript(transcript); err != nil {
		h.err = fmt.Errorf("store assistant transcript %s: %w", messageID, err)
		return
	}
	if h.publisher != nil {
		h.publisher.TranscriptWithUsage(
			h.taskID,
			h.phaseID,
			1,
			"response",
			text,
			model,
			&events.TokenUpdate{
				Phase:                    h.phaseID,
				InputTokens:              inputTokens,
				OutputTokens:             outputTokens,
				CacheCreationInputTokens: cacheCreationTokens,
				CacheReadInputTokens:     cacheReadTokens,
				TotalTokens:              inputTokens + outputTokens + cacheCreationTokens + cacheReadTokens,
			},
		)
	}
}

// StoreChunkText stores a streaming assistant chunk for live transcript visibility.
func (h *TranscriptStreamHandler) StoreChunkText(text, model string) {
	if h.backend == nil || h.taskID == "" || text == "" {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	if h.err != nil {
		return
	}

	if model == "" {
		model = h.model
	}

	transcript := &storage.Transcript{
		TaskID:        h.taskID,
		Phase:         h.phaseID,
		SessionID:     h.sessionID,
		WorkflowRunID: h.runID,
		MessageUUID:   uuid.NewString(),
		Type:          "chunk",
		Role:          "assistant",
		Content:       text,
		Model:         model,
		Timestamp:     time.Now().UnixMilli(),
	}

	if err := h.backend.AddTranscript(transcript); err != nil {
		h.err = fmt.Errorf("store assistant chunk transcript: %w", err)
		return
	}
	if h.publisher != nil {
		h.publisher.TranscriptChunk(h.taskID, h.phaseID, 1, text)
	}
}

// StoreToolCall stores a Codex tool invocation for live transcript visibility.
func (h *TranscriptStreamHandler) StoreToolCall(name string, arguments json.RawMessage, model string) {
	if h.backend == nil || h.taskID == "" || name == "" {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	if h.err != nil {
		return
	}

	if model == "" {
		model = h.model
	}

	content := formatToolCallContent(name, arguments)
	transcript := &storage.Transcript{
		TaskID:        h.taskID,
		Phase:         h.phaseID,
		SessionID:     h.sessionID,
		WorkflowRunID: h.runID,
		MessageUUID:   uuid.NewString(),
		Type:          "tool",
		Role:          "assistant",
		Content:       content,
		Model:         model,
		Timestamp:     time.Now().UnixMilli(),
	}

	if err := h.backend.AddTranscript(transcript); err != nil {
		h.err = fmt.Errorf("store tool call transcript %s: %w", name, err)
		return
	}
	if h.publisher != nil {
		h.publisher.Transcript(h.taskID, h.phaseID, 1, "tool", content)
	}
}

// StoreToolResult stores a Codex tool result for live transcript visibility.
func (h *TranscriptStreamHandler) StoreToolResult(name, output, status string, exitCode *int, model string) {
	if h.backend == nil || h.taskID == "" {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	if h.err != nil {
		return
	}

	if model == "" {
		model = h.model
	}

	content := formatToolResultPreview(name, output, status, exitCode)
	metadata := map[string]any{
		"name":   name,
		"output": output,
		"status": status,
	}
	if exitCode != nil {
		metadata["exit_code"] = *exitCode
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		h.err = fmt.Errorf("marshal tool result transcript metadata for %s: %w", name, err)
		return
	}

	transcript := &storage.Transcript{
		TaskID:        h.taskID,
		Phase:         h.phaseID,
		SessionID:     h.sessionID,
		WorkflowRunID: h.runID,
		MessageUUID:   uuid.NewString(),
		Type:          "tool_result",
		Role:          "tool",
		Content:       content,
		Model:         model,
		ToolResults:   string(metadataJSON),
		Timestamp:     time.Now().UnixMilli(),
	}

	if err := h.backend.AddTranscript(transcript); err != nil {
		h.err = fmt.Errorf("store tool result transcript %s: %w", name, err)
		return
	}
	if h.publisher != nil {
		h.publisher.Transcript(h.taskID, h.phaseID, 1, "tool_result", content)
	}
}

func formatToolCallContent(name string, arguments json.RawMessage) string {
	if len(arguments) == 0 {
		return name
	}

	var pretty bytes.Buffer
	if err := json.Indent(&pretty, arguments, "", "  "); err == nil && pretty.Len() > 0 {
		return name + "\n" + pretty.String()
	}

	return name + "\n" + string(arguments)
}

func formatToolResultPreview(name, output, status string, exitCode *int) string {
	var preview bytes.Buffer
	if name != "" {
		preview.WriteString(name)
	}
	if status != "" || exitCode != nil {
		if preview.Len() > 0 {
			preview.WriteString("\n")
		}
		if status != "" {
			preview.WriteString("status: ")
			preview.WriteString(status)
		}
		if exitCode != nil {
			if status != "" {
				preview.WriteString(", ")
			}
			preview.WriteString(fmt.Sprintf("exit_code: %d", *exitCode))
		}
	}
	if output != "" {
		if preview.Len() > 0 {
			preview.WriteString("\n")
		}
		preview.WriteString(truncatePreview(output, 8192))
	}
	return preview.String()
}

func truncatePreview(text string, maxBytes int) string {
	if maxBytes <= 0 || len(text) <= maxBytes {
		return text
	}
	return text[:maxBytes] + "\n[truncated]"
}
