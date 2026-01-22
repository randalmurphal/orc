// Package executor provides the flowgraph-based execution engine for orc.
package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/randalmurphal/llmkit/claude/jsonl"
	"github.com/randalmurphal/llmkit/claude/session"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/storage"
)

// JSONLSyncer synchronizes Claude Code JSONL session files to the database.
// This is the primary mechanism for transcript persistence in the new architecture.
type JSONLSyncer struct {
	backend storage.Backend
	logger  *slog.Logger
}

// NewJSONLSyncer creates a new JSONL syncer.
func NewJSONLSyncer(backend storage.Backend, logger *slog.Logger) *JSONLSyncer {
	if logger == nil {
		logger = slog.Default()
	}
	return &JSONLSyncer{
		backend: backend,
		logger:  logger,
	}
}

// SyncOptions configures the sync behavior.
type SyncOptions struct {
	TaskID  string // Task ID for these transcripts
	Phase   string // Phase ID
	Append  bool   // If true, only sync messages not already in DB
}

// SyncFromFile reads a JSONL file and syncs all messages to the database.
func (s *JSONLSyncer) SyncFromFile(ctx context.Context, jsonlPath string, opts SyncOptions) error {
	if s.backend == nil {
		return fmt.Errorf("no backend configured")
	}

	// Read JSONL file
	messages, err := jsonl.ReadFile(jsonlPath)
	if err != nil {
		return fmt.Errorf("read jsonl file: %w", err)
	}

	return s.SyncMessages(ctx, messages, opts)
}

// TranscriptStreamer streams JSONL transcripts to the database in real-time.
// Uses file watching (fsnotify with polling fallback) to sync new messages
// as they are written by Claude.
type TranscriptStreamer struct {
	syncer   *JSONLSyncer
	opts     SyncOptions
	cancel   context.CancelFunc
	done     chan struct{}
	logger   *slog.Logger
}

// StartStreaming begins watching a JSONL file and streaming new messages to the database.
// The returned TranscriptStreamer can be stopped with Stop().
// Streams messages in real-time as they appear in the file.
func (s *JSONLSyncer) StartStreaming(jsonlPath string, opts SyncOptions) (*TranscriptStreamer, error) {
	if s.backend == nil {
		return nil, fmt.Errorf("no backend configured")
	}

	// Check file exists (may not exist yet if session just starting)
	if _, err := os.Stat(jsonlPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("jsonl file not found: %s", jsonlPath)
	}

	reader, err := jsonl.NewReader(jsonlPath)
	if err != nil {
		return nil, fmt.Errorf("create jsonl reader: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	streamer := &TranscriptStreamer{
		syncer: s,
		opts:   opts,
		cancel: cancel,
		done:   make(chan struct{}),
		logger: s.logger,
	}

	// Start background goroutine to watch and sync
	go streamer.run(ctx, reader)

	return streamer, nil
}

// run watches the JSONL file and syncs new messages to the database.
func (ts *TranscriptStreamer) run(ctx context.Context, reader *jsonl.Reader) {
	defer close(ts.done)
	defer reader.Close()

	// Tail the file for new messages
	msgCh := reader.Tail(ctx)

	// Batch messages for efficiency (sync every 100ms or 10 messages)
	var batch []session.JSONLMessage
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	syncBatch := func() {
		if len(batch) == 0 {
			return
		}
		if err := ts.syncer.SyncMessages(ctx, batch, ts.opts); err != nil {
			ts.logger.Warn("failed to sync transcript batch",
				"task", ts.opts.TaskID,
				"phase", ts.opts.Phase,
				"count", len(batch),
				"error", err,
			)
		}
		batch = batch[:0] // Clear batch
	}

	for {
		select {
		case <-ctx.Done():
			// Final sync of any remaining messages
			syncBatch()
			return

		case msg, ok := <-msgCh:
			if !ok {
				// Channel closed
				syncBatch()
				return
			}
			batch = append(batch, msg)
			// Sync immediately if batch is large enough
			if len(batch) >= 10 {
				syncBatch()
			}

		case <-ticker.C:
			// Periodic sync of accumulated messages
			syncBatch()
		}
	}
}

// Stop stops the streamer and waits for it to finish.
func (ts *TranscriptStreamer) Stop() {
	ts.cancel()
	<-ts.done
}

// SyncMessages syncs a slice of JSONL messages to the database.
func (s *JSONLSyncer) SyncMessages(ctx context.Context, messages []session.JSONLMessage, opts SyncOptions) error {
	if len(messages) == 0 {
		return nil
	}

	// Get existing message UUIDs if append mode
	existingUUIDs := make(map[string]bool)
	if opts.Append {
		existing, err := s.backend.GetTranscripts(opts.TaskID)
		if err != nil {
			s.logger.Warn("failed to load existing transcripts", "error", err)
		} else {
			for _, t := range existing {
				if t.MessageUUID != "" {
					existingUUIDs[t.MessageUUID] = true
				}
			}
		}
	}

	// Convert and filter messages
	var transcripts []storage.Transcript
	var todoSnapshots []*db.TodoSnapshot

	for _, msg := range messages {
		// Skip if already exists
		if opts.Append && existingUUIDs[msg.UUID] {
			continue
		}

		// Skip queue-operation messages (internal Claude Code bookkeeping)
		if msg.Type == "queue-operation" {
			continue
		}

		// Convert to transcript
		transcript := convertJSONLToTranscript(msg, opts.TaskID, opts.Phase)
		transcripts = append(transcripts, transcript)

		// Extract todo snapshots from assistant messages with TodoWrite
		if msg.HasTodoUpdate() {
			todos := msg.GetTodos()
			if len(todos) > 0 {
				snapshot := &db.TodoSnapshot{
					TaskID:      opts.TaskID,
					Phase:       opts.Phase,
					MessageUUID: msg.UUID,
					Items:       convertTodos(todos),
					Timestamp:   parseTimestamp(msg.Timestamp),
				}
				todoSnapshots = append(todoSnapshots, snapshot)
			}
		}
	}

	// Batch insert transcripts
	if len(transcripts) > 0 {
		if err := s.backend.AddTranscriptBatch(ctx, transcripts); err != nil {
			return fmt.Errorf("add transcript batch: %w", err)
		}
		s.logger.Debug("synced transcripts", "count", len(transcripts), "task", opts.TaskID, "phase", opts.Phase)
	}

	// Insert todo snapshots (need to use db.ProjectDB directly for this)
	if len(todoSnapshots) > 0 {
		if pdb, ok := s.getProjectDB(); ok {
			var failedCount int
			for _, snapshot := range todoSnapshots {
				if err := pdb.AddTodoSnapshot(snapshot); err != nil {
					s.logger.Warn("failed to add todo snapshot", "error", err, "task", opts.TaskID)
					failedCount++
				}
			}
			if failedCount > 0 {
				s.logger.Warn("some todo snapshots failed to persist", "failed", failedCount, "total", len(todoSnapshots), "task", opts.TaskID)
			} else {
				s.logger.Debug("synced todo snapshots", "count", len(todoSnapshots), "task", opts.TaskID)
			}
		} else {
			s.logger.Debug("skipping todo snapshots - no project db available", "count", len(todoSnapshots), "task", opts.TaskID)
		}
	}

	return nil
}

// convertJSONLToTranscript converts a JSONL message to our Transcript format.
func convertJSONLToTranscript(msg session.JSONLMessage, taskID, phase string) storage.Transcript {
	// Get content as JSON string (preserves structure)
	contentJSON := ""
	if msg.Message != nil && len(msg.Message.Content) > 0 {
		contentJSON = string(msg.Message.Content)
	}

	// Get token usage
	var inputTokens, outputTokens, cacheCreation, cacheRead int
	if usage := msg.GetUsage(); usage != nil {
		inputTokens = usage.InputTokens
		outputTokens = usage.OutputTokens
		cacheCreation = usage.CacheCreationInputTokens
		cacheRead = usage.CacheReadInputTokens
	}

	// Extract tool calls
	toolCallsJSON := ""
	if toolCalls := msg.GetToolCalls(); len(toolCalls) > 0 {
		if data, err := json.Marshal(toolCalls); err == nil {
			toolCallsJSON = string(data)
		}
	}

	// Extract tool results
	toolResultsJSON := ""
	if msg.ToolResult != nil {
		if data, err := json.Marshal(msg.ToolResult); err == nil {
			toolResultsJSON = string(data)
		}
	}

	// Determine type and role
	msgType := msg.Type
	role := ""
	model := ""
	if msg.Message != nil {
		role = msg.Message.Role
		model = msg.Message.Model
	}

	// Get parent UUID (already a *string)
	parentUUID := msg.ParentUUID

	return storage.Transcript{
		TaskID:              taskID,
		Phase:               phase,
		SessionID:           msg.SessionID,
		MessageUUID:         msg.UUID,
		ParentUUID:          parentUUID,
		Type:                msgType,
		Role:                role,
		Content:             contentJSON,
		Model:               model,
		InputTokens:         inputTokens,
		OutputTokens:        outputTokens,
		CacheCreationTokens: cacheCreation,
		CacheReadTokens:     cacheRead,
		ToolCalls:           toolCallsJSON,
		ToolResults:         toolResultsJSON,
		Timestamp:           parseTimestamp(msg.Timestamp).UnixMilli(),
	}
}

// convertTodos converts session TodoItems to db TodoItems.
func convertTodos(todos []session.TodoItem) []db.TodoItem {
	items := make([]db.TodoItem, len(todos))
	for i, t := range todos {
		items[i] = db.TodoItem{
			Content:    t.Content,
			Status:     t.Status,
			ActiveForm: t.ActiveForm,
		}
	}
	return items
}

// parseTimestamp parses ISO 8601 timestamp from JSONL.
func parseTimestamp(ts string) time.Time {
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		// Try alternative formats
		t, err = time.Parse("2006-01-02T15:04:05.000Z", ts)
		if err != nil {
			return time.Now()
		}
	}
	return t
}

// getProjectDB attempts to get the underlying ProjectDB for todo operations.
func (s *JSONLSyncer) getProjectDB() (*db.ProjectDB, bool) {
	// The storage backend wraps a ProjectDB - try to access it
	if dbBackend, ok := s.backend.(*storage.DatabaseBackend); ok {
		return dbBackend.DB(), true
	}
	return nil, false
}

// ComputeTokenUsage calculates aggregated token usage from transcripts.
func ComputeTokenUsage(transcripts []storage.Transcript) TokenUsageSummary {
	var summary TokenUsageSummary
	for _, t := range transcripts {
		if t.Type == "assistant" {
			summary.InputTokens += t.InputTokens
			summary.OutputTokens += t.OutputTokens
			summary.CacheCreationTokens += t.CacheCreationTokens
			summary.CacheReadTokens += t.CacheReadTokens
			summary.MessageCount++
		}
	}
	return summary
}

// TokenUsageSummary holds aggregated token counts.
type TokenUsageSummary struct {
	InputTokens         int
	OutputTokens        int
	CacheCreationTokens int
	CacheReadTokens     int
	MessageCount        int
}

// TotalTokens returns the total token count including cache tokens.
func (s TokenUsageSummary) TotalTokens() int {
	return s.InputTokens + s.OutputTokens + s.CacheCreationTokens + s.CacheReadTokens
}

// ComputeJSONLPath computes the JSONL file path for a Claude session.
// Claude Code stores JSONL files at: ~/.claude/projects/{normalized-workdir}/{sessionId}.jsonl
// The workdir is normalized by replacing "/" with "-" and prepending "-".
func ComputeJSONLPath(workdir, sessionID string) (string, error) {
	if sessionID == "" {
		return "", fmt.Errorf("empty session ID")
	}

	homeDir, err := os.Getenv("HOME"), error(nil)
	if homeDir == "" {
		homeDir, err = os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("get home directory: %w", err)
		}
	}

	normalizedPath := normalizeProjectPath(workdir)
	return fmt.Sprintf("%s/.claude/projects/%s/%s.jsonl", homeDir, normalizedPath, sessionID), nil
}

// normalizeProjectPath converts an absolute path to Claude Code's normalized format.
// Example: /home/user/repos/project -> -home-user-repos-project
// Example: /home/user/repos/orc/.orc/worktrees -> -home-user-repos-orc--orc-worktrees
func normalizeProjectPath(path string) string {
	// Remove leading slash and replace remaining slashes with dashes
	normalized := strings.TrimPrefix(path, "/")
	normalized = strings.ReplaceAll(normalized, "/", "-")
	// Claude also replaces dots with dashes (e.g., .orc -> -orc)
	normalized = strings.ReplaceAll(normalized, ".", "-")
	// Prepend dash to match Claude Code's format
	return "-" + normalized
}
