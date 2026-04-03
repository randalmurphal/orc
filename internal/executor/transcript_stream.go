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
	llmkit "github.com/randalmurphal/llmkit/v2"
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

// OnChunk handles normalized llmkit streaming chunks and stores transcripts in real-time.
func (h *TranscriptStreamHandler) OnChunk(chunk llmkit.StreamChunk) {
	if h.backend == nil || h.taskID == "" {
		return
	}

	if chunk.SessionID != "" {
		h.UpdateSessionID(chunk.SessionID)
	}

	switch chunk.Type {
	case "assistant":
		if chunk.MessageID != "" {
			h.storeAssistantMessage(chunk)
			return
		}
		if chunk.Content != "" {
			h.StoreChunkText(chunk.Content, chunk.Model)
		}
	case "session":
		h.logger.Debug("llm session initialized",
			"session_id", chunk.SessionID,
			"task", h.taskID,
			"phase", h.phaseID,
		)
	case "final":
		h.logger.Debug("llm execution complete",
			"session_id", chunk.SessionID,
			"task", h.taskID,
			"phase", h.phaseID,
			"num_turns", chunk.NumTurns,
			"cost_usd", chunk.CostUSD,
			"has_final_content", chunk.FinalContent != "",
		)
		if chunk.FinalContent != "" && chunk.Session != nil && chunk.Session.Provider == ProviderCodex {
			inputTokens, outputTokens, cacheCreationTokens, cacheReadTokens := 0, 0, 0, 0
			if chunk.Usage != nil {
				inputTokens = chunk.Usage.InputTokens
				outputTokens = chunk.Usage.OutputTokens
				cacheCreationTokens = chunk.Usage.CacheCreationInputTokens
				cacheReadTokens = chunk.Usage.CacheReadInputTokens
			}
			h.StoreAssistantTextWithUsage(
				chunk.FinalContent,
				chunk.Model,
				chunk.MessageID,
				inputTokens,
				outputTokens,
				cacheCreationTokens,
				cacheReadTokens,
			)
		}
	case "hook":
		if h.shouldCaptureHook(metadataString(chunk.Metadata, "hook_event")) {
			h.storeHookEvent(chunk)
		}
	case "tool_call":
		for _, toolCall := range chunk.ToolCalls {
			h.StoreToolCall(toolCall.Name, toolCall.Arguments, chunk.Model)
		}
	case "tool_result":
		for _, toolResult := range chunk.ToolResults {
			h.StoreToolResult(toolResult.Name, toolResult.Output, toolResult.Status, toolResult.ExitCode, chunk.Model)
		}
	case "error":
		h.logger.Error("llm streaming error",
			"task", h.taskID,
			"phase", h.phaseID,
			"error", chunk.Error,
		)
	}
}

// storeAssistantMessage stores a single assistant message from the stream.
// Claude streams multiple events with the same message ID - we only store the first one
// since later events for the same ID are just partial updates that we already have.
func (h *TranscriptStreamHandler) storeAssistantMessage(chunk llmkit.StreamChunk) {
	if chunk.Content == "" {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	if h.err != nil {
		return
	}

	// Use the API message ID if available, otherwise generate one
	messageUUID := chunk.MessageID
	if messageUUID == "" {
		messageUUID = uuid.NewString()
	}

	// Skip if we've already stored this message (Claude streams multiple events per message)
	if h.storedMessageIDs[messageUUID] {
		return
	}
	h.storedMessageIDs[messageUUID] = true

	model := chunk.Model
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
		Content:             chunk.Content,
		Model:               model,
		InputTokens:         usageValue(chunk.Usage, func(u *llmkit.TokenUsage) int { return u.InputTokens }),
		OutputTokens:        usageValue(chunk.Usage, func(u *llmkit.TokenUsage) int { return u.OutputTokens }),
		CacheCreationTokens: usageValue(chunk.Usage, func(u *llmkit.TokenUsage) int { return u.CacheCreationInputTokens }),
		CacheReadTokens:     usageValue(chunk.Usage, func(u *llmkit.TokenUsage) int { return u.CacheReadInputTokens }),
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
			chunk.Content,
			model,
			&events.TokenUpdate{
				Phase:                    h.phaseID,
				InputTokens:              usageValue(chunk.Usage, func(u *llmkit.TokenUsage) int { return u.InputTokens }),
				OutputTokens:             usageValue(chunk.Usage, func(u *llmkit.TokenUsage) int { return u.OutputTokens }),
				CacheCreationInputTokens: usageValue(chunk.Usage, func(u *llmkit.TokenUsage) int { return u.CacheCreationInputTokens }),
				CacheReadInputTokens:     usageValue(chunk.Usage, func(u *llmkit.TokenUsage) int { return u.CacheReadInputTokens }),
				TotalTokens:              usageValue(chunk.Usage, func(u *llmkit.TokenUsage) int { return u.TotalTokens }),
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
func (h *TranscriptStreamHandler) storeHookEvent(chunk llmkit.StreamChunk) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.err != nil {
		return
	}

	// Serialize hook details
	hookData := map[string]any{
		"hook_name":  metadataString(chunk.Metadata, "hook_name"),
		"hook_event": metadataString(chunk.Metadata, "hook_event"),
		"stdout":     metadataString(chunk.Metadata, "stdout"),
		"stderr":     metadataString(chunk.Metadata, "stderr"),
		"exit_code":  metadataInt(chunk.Metadata, "exit_code"),
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
		h.err = fmt.Errorf("store hook transcript %s: %w", metadataString(chunk.Metadata, "hook_name"), err)
	}
}

func metadataString(metadata map[string]any, key string) string {
	if len(metadata) == 0 {
		return ""
	}
	value, ok := metadata[key]
	if !ok || value == nil {
		return ""
	}
	if s, ok := value.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", value)
}

func metadataInt(metadata map[string]any, key string) int {
	if len(metadata) == 0 {
		return 0
	}
	value, ok := metadata[key]
	if !ok || value == nil {
		return 0
	}
	switch v := value.(type) {
	case int:
		return v
	case int32:
		return int(v)
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return 0
	}
}

func usageValue(usage *llmkit.TokenUsage, getter func(*llmkit.TokenUsage) int) int {
	if usage == nil {
		return 0
	}
	return getter(usage)
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
