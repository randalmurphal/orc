// Package executor provides the execution engine for orc.
// This file provides a CodexCLI-based executor that implements TurnExecutor
// for the OpenAI Codex CLI provider.
package executor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	llmkit "github.com/randalmurphal/llmkit/v2"
	"github.com/randalmurphal/llmkit/v2/codex"
	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// Ensure CodexExecutor implements TurnExecutor.
var _ TurnExecutor = (*CodexExecutor)(nil)

// CodexExecutor wraps CodexCLI for phase execution with structured output
// via --output-schema. Unlike Claude's constrained decoding, Codex's
// structured output is NOT guaranteed — responses are validated and retried.
type CodexExecutor struct {
	codexPath string
	workdir   string
	model     string
	logger    *slog.Logger

	// Session management for multi-turn
	sessionID string
	resume    bool

	// Phase ID for schema selection
	phaseID string

	// Whether this phase produces an artifact (content field in schema)
	producesArtifact bool

	// Review round for review phase schema selection (1 = findings, 2 = decision)
	reviewRound int

	// Loop configuration for iteration-specific schema selection
	loopConfig *db.LoopConfig

	// Current loop iteration (1-based)
	loopIteration int

	// Transcript storage
	backend           storage.Backend
	taskID            string
	runID             string
	publisher         *events.PublishHelper
	transcriptHandler *TranscriptStreamHandler

	// Codex-specific settings
	bypassApprovalsAndSandbox bool

	// Additional Codex CLI settings
	reasoningEffort string            // model_reasoning_effort
	webSearchMode   string            // web_search mode
	env             map[string]string // extra env vars for process
	addDirs         []string          // additional accessible directories

	// CLI-level timeout (default: 30 minutes). The llmkit default (5m) is too
	// short for extended reasoning on large repos.
	timeout time.Duration
	// Inactivity timeout for silent stalled turns.
	inactivityTimeout time.Duration

	// Schema validation retry limit for non-guaranteed structured output
	schemaRetries int
}

// CodexExecutorOption configures a CodexExecutor.
type CodexExecutorOption func(*CodexExecutor)

// WithCodexWorkdir sets the working directory.
func WithCodexWorkdir(dir string) CodexExecutorOption {
	return func(e *CodexExecutor) { e.workdir = dir }
}

// WithCodexModel sets the model to use.
func WithCodexModel(model string) CodexExecutorOption {
	return func(e *CodexExecutor) { e.model = model }
}

// WithCodexSessionID sets the session ID for conversation tracking.
func WithCodexSessionID(id string) CodexExecutorOption {
	return func(e *CodexExecutor) { e.sessionID = id }
}

// WithCodexResume enables session resume mode.
func WithCodexResume(resume bool) CodexExecutorOption {
	return func(e *CodexExecutor) { e.resume = resume }
}

// WithCodexPath sets the path to codex binary.
func WithCodexPath(path string) CodexExecutorOption {
	return func(e *CodexExecutor) { e.codexPath = path }
}

// WithCodexLogger sets the logger.
func WithCodexLogger(l *slog.Logger) CodexExecutorOption {
	return func(e *CodexExecutor) { e.logger = l }
}

// WithCodexPhaseID sets the phase ID for schema selection.
func WithCodexPhaseID(id string) CodexExecutorOption {
	return func(e *CodexExecutor) { e.phaseID = id }
}

// WithCodexProducesArtifact sets whether the phase produces an artifact.
func WithCodexProducesArtifact(produces bool) CodexExecutorOption {
	return func(e *CodexExecutor) { e.producesArtifact = produces }
}

// WithCodexReviewRound sets the review round for schema selection.
func WithCodexReviewRound(round int) CodexExecutorOption {
	return func(e *CodexExecutor) { e.reviewRound = round }
}

// WithCodexLoopConfig sets the loop configuration.
func WithCodexLoopConfig(cfg *db.LoopConfig) CodexExecutorOption {
	return func(e *CodexExecutor) { e.loopConfig = cfg }
}

// WithCodexLoopIteration sets the current loop iteration.
func WithCodexLoopIteration(iteration int) CodexExecutorOption {
	return func(e *CodexExecutor) { e.loopIteration = iteration }
}

// WithCodexBackend sets the storage backend for transcript storage.
func WithCodexBackend(b storage.Backend) CodexExecutorOption {
	return func(e *CodexExecutor) { e.backend = b }
}

// WithCodexTaskID sets the task ID for transcript storage.
func WithCodexTaskID(id string) CodexExecutorOption {
	return func(e *CodexExecutor) { e.taskID = id }
}

// WithCodexRunID sets the workflow run ID for transcript linking.
func WithCodexRunID(id string) CodexExecutorOption {
	return func(e *CodexExecutor) { e.runID = id }
}

// WithCodexPublisher sets the event publisher for live transcript updates.
func WithCodexPublisher(p *events.PublishHelper) CodexExecutorOption {
	return func(e *CodexExecutor) { e.publisher = p }
}

// WithCodexBypassApprovalsAndSandbox enables --dangerously-bypass-approvals-and-sandbox.
// This is the default and only supported mode for orc execution.
func WithCodexBypassApprovalsAndSandbox(bypass bool) CodexExecutorOption {
	return func(e *CodexExecutor) { e.bypassApprovalsAndSandbox = bypass }
}

// WithCodexReasoningEffort sets the model reasoning effort level.
func WithCodexReasoningEffort(effort string) CodexExecutorOption {
	return func(e *CodexExecutor) { e.reasoningEffort = effort }
}

// WithCodexWebSearchMode sets the web search mode.
func WithCodexWebSearchMode(mode string) CodexExecutorOption {
	return func(e *CodexExecutor) { e.webSearchMode = mode }
}

// WithCodexEnv sets additional environment variables for the codex process.
func WithCodexEnv(env map[string]string) CodexExecutorOption {
	return func(e *CodexExecutor) { e.env = env }
}

// WithCodexAddDirs sets additional accessible directories.
func WithCodexAddDirs(dirs []string) CodexExecutorOption {
	return func(e *CodexExecutor) { e.addDirs = dirs }
}

// WithCodexTimeout sets the CLI-level timeout for codex commands.
// Default is 30 minutes. The llmkit default (5m) is too short for extended
// reasoning with xhigh effort on large repositories.
func WithCodexTimeout(d time.Duration) CodexExecutorOption {
	return func(e *CodexExecutor) { e.timeout = d }
}

// WithCodexInactivityTimeout sets the no-output watchdog timeout for codex turns.
func WithCodexInactivityTimeout(d time.Duration) CodexExecutorOption {
	return func(e *CodexExecutor) { e.inactivityTimeout = d }
}

// WithCodexSchemaRetries sets the number of schema validation retries.
// Default is 2 (total 3 attempts including the first).
func WithCodexSchemaRetries(retries int) CodexExecutorOption {
	return func(e *CodexExecutor) { e.schemaRetries = retries }
}

// NewCodexExecutor creates a new Codex executor.
func NewCodexExecutor(opts ...CodexExecutorOption) *CodexExecutor {
	e := &CodexExecutor{
		codexPath:                 "codex",
		logger:                    slog.Default(),
		bypassApprovalsAndSandbox: true,
		schemaRetries:             2,
		timeout:                   30 * time.Minute,
		inactivityTimeout:         DefaultProviderInactivityTimeout,
	}
	for _, opt := range opts {
		opt(e)
	}
	if e.backend != nil && e.taskID != "" {
		e.transcriptHandler = NewTranscriptStreamHandler(
			e.backend,
			e.logger,
			e.taskID,
			e.phaseID,
			e.sessionID,
			e.runID,
			e.model,
			e.publisher,
			nil,
		)
	}
	return e
}

// ExecuteTurn sends a prompt to Codex and waits for structured JSON output.
// Unlike Claude, Codex does NOT guarantee structured output — responses are
// validated against the expected schema and retried on parse failure.
func (e *CodexExecutor) ExecuteTurn(ctx context.Context, prompt string) (*TurnResult, error) {
	start := time.Now()

	// Select schema for this phase/iteration
	iteration := e.loopIteration
	if iteration == 0 {
		iteration = e.reviewRound
	}
	if iteration == 0 {
		iteration = 1
	}
	schema := GetSchemaForIteration(e.loopConfig, iteration, e.phaseID, e.producesArtifact)

	// Retry loop for schema validation (codex doesn't guarantee structured output)
	var lastResult *TurnResult
	var lastErr error
	maxAttempts := e.schemaRetries + 1

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		if attempt > 0 {
			e.logger.Warn("retrying codex turn due to invalid JSON",
				"attempt", attempt+1,
				"max_attempts", maxAttempts,
				"phase", e.phaseID,
			)
		}

		result, err := e.executeSingleTurn(ctx, prompt, schema, start)
		if err != nil {
			var stalledErr *codexTurnStalledError
			if errors.As(err, &stalledErr) {
				e.logger.Warn("retrying codex turn after stalled stream", "phase", e.phaseID, "attempt", attempt+1)
				result, err = e.executeSingleTurnFresh(ctx, buildCodexStallRetryPrompt(prompt, stalledErr), schema, start)
			}
		}
		if err != nil {
			lastResult = result
			lastErr = err
			// Only retry on JSON parse errors, not transport/execution errors.
			// If result is nil (transport error, auth failure, etc.), fail immediately
			// rather than silently retrying.
			if result == nil || !isJSONParseError(err) {
				if result == nil {
					result = &TurnResult{
						Duration:  time.Since(start),
						IsError:   true,
						ErrorText: err.Error(),
					}
				}
				return result, err
			}
			continue
		}

		// Validate that we got parseable phase completion JSON
		status, reason, parseErr := ParsePhaseSpecificResponse(e.phaseID, e.reviewRound, result.Content)
		result.Status = status
		result.Reason = reason

		if parseErr != nil {
			e.logger.Warn("codex returned invalid phase JSON, will retry",
				"attempt", attempt+1,
				"error", parseErr,
				"content_preview", truncate(result.Content, 200),
			)
			lastResult = result
			lastErr = fmt.Errorf("phase completion JSON parse failed: %w", parseErr)
			continue
		}

		// Success — valid structured output
		result.Duration = time.Since(start)
		return result, nil
	}

	// All retries exhausted
	if lastResult != nil {
		lastResult.IsError = true
		lastResult.ErrorText = lastErr.Error()
		lastResult.Duration = time.Since(start)
		return lastResult, lastErr
	}

	return &TurnResult{
		Duration:  time.Since(start),
		IsError:   true,
		ErrorText: "codex schema validation failed after all retries",
	}, lastErr
}

type codexTurnStalledError struct {
	timeout        time.Duration
	lastToolResult *codex.ToolResult
}

func (e *codexTurnStalledError) Error() string {
	return fmt.Sprintf("codex stalled after %v without output", e.timeout)
}

// ExecuteTurnWithoutSchema sends a prompt without requiring structured output.
func (e *CodexExecutor) ExecuteTurnWithoutSchema(ctx context.Context, prompt string) (*TurnResult, error) {
	start := time.Now()

	result, err := e.executeSingleTurn(ctx, prompt, "", start)
	if err != nil {
		return result, err
	}

	result.Status = PhaseStatusContinue // Default — caller determines actual status
	result.Duration = time.Since(start)
	return result, nil
}

// UpdateSessionID updates the session ID for subsequent calls.
func (e *CodexExecutor) UpdateSessionID(id string) {
	e.logger.Debug("updating codex session ID", "old_id", e.sessionID, "new_id", id, "old_resume", e.resume)
	e.sessionID = id
	e.resume = true
	if e.transcriptHandler != nil {
		e.transcriptHandler.UpdateSessionID(id)
	}
}

func (e *CodexExecutor) persistLiveSessionID(sessionID string) error {
	if e.backend == nil || e.taskID == "" || e.phaseID == "" || sessionID == "" {
		return nil
	}

	t, err := e.backend.LoadTask(e.taskID)
	if err != nil {
		return fmt.Errorf("load task for live codex session persistence: %w", err)
	}
	if t == nil {
		return fmt.Errorf("task %s not found while persisting live codex session", e.taskID)
	}

	sessionMetadata, marshalErr := llmkit.MarshalSessionMetadata(llmkit.SessionMetadataForID(ProviderCodex, sessionID))
	if marshalErr != nil {
		return fmt.Errorf("marshal codex session metadata: %w", marshalErr)
	}
	task.SetPhaseSessionMetadataProto(t.Execution, e.phaseID, sessionMetadata)
	if saveErr := e.backend.SaveTask(t); saveErr != nil {
		return fmt.Errorf("persist live codex session metadata: %w", saveErr)
	}
	return nil
}

// SessionID returns the current session ID.
func (e *CodexExecutor) SessionID() string {
	return e.sessionID
}

// executeSingleTurn runs a single codex completion or resume call.
func (e *CodexExecutor) executeSingleTurn(ctx context.Context, prompt, schema string, start time.Time) (*TurnResult, error) {
	ctx, cancel := e.codexContextWithTimeout(ctx)
	defer cancel()
	watchdog := NewTurnWatchdog(e.inactivityTimeout, cancel)
	watchdog.Start(ctx)
	watchdog.RecordActivity()

	if e.transcriptHandler != nil {
		e.transcriptHandler.StoreUserPrompt(prompt)
		if err := e.transcriptHandler.Err(); err != nil {
			return &TurnResult{
				Duration:  time.Since(start),
				IsError:   true,
				ErrorText: err.Error(),
			}, err
		}
	}

	cli := codex.NewCodexCLI(e.buildCLIOptions()...)
	stream, err := cli.Stream(ctx, codex.CompletionRequest{
		Messages:   []codex.Message{{Role: codex.RoleUser, Content: prompt}},
		JSONSchema: json.RawMessage(schema),
	})
	if err != nil {
		return &TurnResult{
			Duration:  time.Since(start),
			IsError:   true,
			ErrorText: err.Error(),
		}, fmt.Errorf("codex stream: %w", err)
	}

	var (
		contentBuilder strings.Builder
		finalContent   string
		usage          *codex.TokenUsage
		sessionID      = e.sessionID
		numTurns       int
		lastToolResult *codex.ToolResult
		sawTerminal    bool
	)

	for chunk := range stream {
		watchdog.RecordActivity()
		if chunk.SessionID != "" && chunk.SessionID != sessionID {
			sessionID = chunk.SessionID
			e.UpdateSessionID(sessionID)
			if err := e.persistLiveSessionID(sessionID); err != nil {
				return &TurnResult{
					Duration:  time.Since(start),
					IsError:   true,
					ErrorText: err.Error(),
					SessionID: sessionID,
				}, err
			}
		}
		if chunk.Content != "" {
			contentBuilder.WriteString(chunk.Content)
			if e.transcriptHandler != nil {
				e.transcriptHandler.StoreChunkText(chunk.Content, e.model)
				if err := e.transcriptHandler.Err(); err != nil {
					return &TurnResult{
						Duration:  time.Since(start),
						IsError:   true,
						ErrorText: err.Error(),
						SessionID: sessionID,
					}, err
				}
			}
		}
		if len(chunk.ToolCalls) > 0 && e.transcriptHandler != nil {
			for _, toolCall := range chunk.ToolCalls {
				e.transcriptHandler.StoreToolCall(toolCall.Name, toolCall.Arguments, e.model)
				if err := e.transcriptHandler.Err(); err != nil {
					return &TurnResult{
						Duration:  time.Since(start),
						IsError:   true,
						ErrorText: err.Error(),
						SessionID: sessionID,
					}, err
				}
			}
		}
		if len(chunk.ToolResults) > 0 {
			lastToolResult = &chunk.ToolResults[len(chunk.ToolResults)-1]
			if e.transcriptHandler != nil {
				for _, toolResult := range chunk.ToolResults {
					e.transcriptHandler.StoreToolResult(toolResult.Name, toolResult.Output, toolResult.Status, toolResult.ExitCode, e.model)
					if err := e.transcriptHandler.Err(); err != nil {
						return &TurnResult{
							Duration:  time.Since(start),
							IsError:   true,
							ErrorText: err.Error(),
							SessionID: sessionID,
						}, err
					}
				}
			}
		}
		if chunk.FinalContent != "" {
			finalContent = chunk.FinalContent
		}
		if chunk.Usage != nil {
			usage = chunk.Usage
		}
		if chunk.Done {
			sawTerminal = true
			numTurns++
		}
		if chunk.Error != nil {
			content := strings.TrimSpace(contentBuilder.String())
			if finalContent != "" {
				content = strings.TrimSpace(finalContent)
			}
			if watchdog.Tripped() {
				err := &codexTurnStalledError{timeout: watchdog.Timeout(), lastToolResult: lastToolResult}
				return &TurnResult{
					Content:   content,
					Duration:  time.Since(start),
					IsError:   true,
					ErrorText: err.Error(),
					SessionID: sessionID,
				}, err
			}
			return &TurnResult{
				Content:   content,
				Duration:  time.Since(start),
				IsError:   true,
				ErrorText: chunk.Error.Error(),
				SessionID: sessionID,
			}, fmt.Errorf("codex stream: %w", chunk.Error)
		}
	}

	content := strings.TrimSpace(contentBuilder.String())
	if finalContent != "" {
		content = strings.TrimSpace(finalContent)
	}
	if watchdog.Tripped() && !sawTerminal {
		err := &codexTurnStalledError{timeout: watchdog.Timeout(), lastToolResult: lastToolResult}
		return &TurnResult{
			Content:   content,
			Duration:  time.Since(start),
			IsError:   true,
			ErrorText: err.Error(),
			SessionID: sessionID,
		}, err
	}
	result := &TurnResult{
		Content:   content,
		NumTurns:  numTurns,
		SessionID: sessionID,
		Duration:  time.Since(start),
	}
	if usage != nil {
		result.Usage = &orcv1.TokenUsage{
			InputTokens:              int32(usage.InputTokens),
			OutputTokens:             int32(usage.OutputTokens),
			CacheCreationInputTokens: int32(usage.CacheCreationInputTokens),
			CacheReadInputTokens:     int32(usage.CacheReadInputTokens),
			TotalTokens:              int32(usage.TotalTokens),
		}
	}
	if e.transcriptHandler != nil {
		inputTokens, outputTokens, cacheCreationTokens, cacheReadTokens := 0, 0, 0, 0
		if usage != nil {
			inputTokens = usage.InputTokens
			outputTokens = usage.OutputTokens
			cacheCreationTokens = usage.CacheCreationInputTokens
			cacheReadTokens = usage.CacheReadInputTokens
		}
		e.transcriptHandler.StoreAssistantTextWithUsage(content, e.model, "", inputTokens, outputTokens, cacheCreationTokens, cacheReadTokens)
		if err := e.transcriptHandler.Err(); err != nil {
			return &TurnResult{
				Duration:  time.Since(start),
				IsError:   true,
				ErrorText: err.Error(),
				SessionID: sessionID,
			}, err
		}
	}

	return result, nil
}

func (e *CodexExecutor) executeSingleTurnFresh(ctx context.Context, prompt, schema string, start time.Time) (*TurnResult, error) {
	prevSessionID := e.sessionID
	prevResume := e.resume
	e.sessionID = ""
	e.resume = false
	if e.transcriptHandler != nil {
		e.transcriptHandler.UpdateSessionID("")
	}
	defer func() {
		if e.sessionID == "" {
			e.sessionID = prevSessionID
			e.resume = prevResume
			if e.transcriptHandler != nil {
				e.transcriptHandler.UpdateSessionID(prevSessionID)
			}
		}
	}()
	return e.executeSingleTurn(ctx, prompt, schema, start)
}

func buildCodexStallRetryPrompt(originalPrompt string, stalled *codexTurnStalledError) string {
	var prompt strings.Builder
	prompt.WriteString("The previous Codex turn stalled and never emitted a terminal event.\n")
	prompt.WriteString("Do not restart broad reconnaissance. Continue from the current repository state, fix the last failing validation, rerun only the necessary verification, and then return the required completion output.\n")
	if stalled != nil && stalled.lastToolResult != nil {
		prompt.WriteString("\nLast tool result before the stall:\n")
		prompt.WriteString(stalled.lastToolResult.Name)
		if stalled.lastToolResult.Status != "" || stalled.lastToolResult.ExitCode != nil {
			prompt.WriteString("\n")
			if stalled.lastToolResult.Status != "" {
				prompt.WriteString("status: ")
				prompt.WriteString(stalled.lastToolResult.Status)
			}
			if stalled.lastToolResult.ExitCode != nil {
				if stalled.lastToolResult.Status != "" {
					prompt.WriteString(", ")
				}
				prompt.WriteString(fmt.Sprintf("exit_code: %d", *stalled.lastToolResult.ExitCode))
			}
		}
		if stalled.lastToolResult.Output != "" {
			prompt.WriteString("\n")
			prompt.WriteString(truncatePreview(stalled.lastToolResult.Output, 12000))
		}
	}
	prompt.WriteString("\n\nOriginal task prompt:\n")
	prompt.WriteString(originalPrompt)
	return prompt.String()
}

func (e *CodexExecutor) buildCLIOptions() []codex.CodexOption {
	opts := []codex.CodexOption{
		codex.WithWorkdir(e.workdir),
		codex.WithTimeout(e.timeout),
	}
	if e.bypassApprovalsAndSandbox {
		opts = append(opts, codex.WithDangerouslyBypassApprovalsAndSandbox())
	}
	if e.codexPath != "" {
		opts = append(opts, codex.WithCodexPath(e.codexPath))
	}
	if e.model != "" {
		opts = append(opts, codex.WithModel(e.model))
	}
	if e.resume && e.sessionID != "" {
		opts = append(opts, codex.WithSessionID(e.sessionID))
	}
	if e.reasoningEffort != "" {
		opts = append(opts, codex.WithReasoningEffort(e.reasoningEffort))
	}
	if e.webSearchMode != "" {
		opts = append(opts, codex.WithWebSearchMode(codex.WebSearchMode(e.webSearchMode)))
	}
	if len(e.env) > 0 {
		opts = append(opts, codex.WithEnv(e.env))
	}
	if len(e.addDirs) > 0 {
		opts = append(opts, codex.WithAddDirs(e.addDirs))
	}
	return opts
}

func (e *CodexExecutor) codexContextWithTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if e.timeout <= 0 {
		return ctx, func() {}
	}
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, e.timeout)
}

// isJSONParseError checks if an error is related to JSON parsing (worth retrying).
func isJSONParseError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	lower := strings.ToLower(msg)
	return strings.Contains(lower, "json") ||
		strings.Contains(lower, "unmarshal") ||
		strings.Contains(lower, "parse")
}

// truncate shortens a string to maxLen characters, adding "..." if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
