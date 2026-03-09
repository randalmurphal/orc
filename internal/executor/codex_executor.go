// Package executor provides the execution engine for orc.
// This file provides a CodexCLI-based executor that implements TurnExecutor
// for the OpenAI Codex CLI provider (GPT-5, local OSS models via ollama).
package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/randalmurphal/llmkit/codex"
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

	// Local model routing (ollama, lmstudio)
	localProvider string

	// Additional Codex CLI settings
	reasoningEffort string            // model_reasoning_effort
	webSearchMode   string            // web_search mode
	env             map[string]string // extra env vars for process
	addDirs         []string          // additional accessible directories

	// CLI-level timeout (default: 30 minutes). The llmkit default (5m) is too
	// short for extended reasoning on large repos.
	timeout time.Duration

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

// WithCodexLocalProvider sets the local model provider (ollama, lmstudio).
func WithCodexLocalProvider(provider string) CodexExecutorOption {
	return func(e *CodexExecutor) { e.localProvider = provider }
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

	// Write schema to temp file (codex uses --output-schema <path>, not inline JSON)
	schemaFile, err := e.writeSchemaFile(schema)
	if err != nil {
		return &TurnResult{
			Duration:  time.Since(start),
			IsError:   true,
			ErrorText: err.Error(),
		}, fmt.Errorf("write schema file: %w", err)
	}
	defer os.Remove(schemaFile)

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

		result, err := e.executeSingleTurn(ctx, prompt, schemaFile, start)
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

		// Codex with --output-schema may return one JSON per turn, producing
		// concatenated objects like "{json1}{json2}". Extract the last one
		// and store it back so downstream consumers see clean JSON.
		content := extractLastJSON(result.Content)
		result.Content = content

		// Validate that we got parseable phase completion JSON
		status, reason, parseErr := ParsePhaseSpecificResponse(e.phaseID, e.reviewRound, content)
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

func (e *CodexExecutor) persistLiveSessionID(sessionID string) {
	if e.backend == nil || e.taskID == "" || e.phaseID == "" || sessionID == "" {
		return
	}

	t, err := e.backend.LoadTask(e.taskID)
	if err != nil {
		e.logger.Warn("failed to load task for live codex session persistence", "task", e.taskID, "phase", e.phaseID, "error", err)
		return
	}
	if t == nil {
		e.logger.Warn("task not found while persisting live codex session", "task", e.taskID, "phase", e.phaseID)
		return
	}

	task.SetPhaseSessionIDProto(t.Execution, e.phaseID, sessionID)
	if saveErr := e.backend.SaveTask(t); saveErr != nil {
		e.logger.Warn("failed to persist live codex session ID", "task", e.taskID, "phase", e.phaseID, "session_id", sessionID, "error", saveErr)
	}
}

// SessionID returns the current session ID.
func (e *CodexExecutor) SessionID() string {
	return e.sessionID
}

// executeSingleTurn runs a single codex completion or resume call.
func (e *CodexExecutor) executeSingleTurn(ctx context.Context, prompt, schemaFile string, start time.Time) (*TurnResult, error) {
	ctx, cancel := e.codexContextWithTimeout(ctx)
	defer cancel()

	if e.transcriptHandler != nil {
		e.transcriptHandler.StoreUserPrompt(prompt)
	}

	cli := codex.NewCodexCLI(e.buildCLIOptions(schemaFile)...)
	stream, err := cli.Stream(ctx, codex.CompletionRequest{
		Messages: []codex.Message{{Role: codex.RoleUser, Content: prompt}},
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
	)

	for chunk := range stream {
		if chunk.SessionID != "" && chunk.SessionID != sessionID {
			sessionID = chunk.SessionID
			e.UpdateSessionID(sessionID)
			e.persistLiveSessionID(sessionID)
		}
		if chunk.Content != "" {
			contentBuilder.WriteString(chunk.Content)
			if e.transcriptHandler != nil {
				e.transcriptHandler.StoreChunkText(chunk.Content, e.model)
			}
		}
		if chunk.FinalContent != "" {
			finalContent = chunk.FinalContent
		}
		if chunk.Usage != nil {
			usage = chunk.Usage
		}
		if chunk.Done {
			numTurns++
		}
		if chunk.Error != nil {
			content := strings.TrimSpace(contentBuilder.String())
			if finalContent != "" {
				content = strings.TrimSpace(finalContent)
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
	}

	return result, nil
}

func (e *CodexExecutor) buildCLIOptions(schemaFile string) []codex.CodexOption {
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
	if schemaFile != "" {
		opts = append(opts, codex.WithOutputSchema(schemaFile))
	}
	if e.localProvider != "" {
		opts = append(opts, codex.WithLocalProvider(e.localProvider))
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

// writeSchemaFile writes a JSON schema string to a temp file and returns the path.
// OpenAI structured outputs require additionalProperties:false at every object level.
// This is applied here so the shared schemas (used by both Claude and Codex) don't need modification.
func (e *CodexExecutor) writeSchemaFile(schema string) (string, error) {
	// Validate that it's valid JSON
	if !json.Valid([]byte(schema)) {
		return "", fmt.Errorf("invalid JSON schema: %s", truncate(schema, 100))
	}

	// OpenAI requires additionalProperties:false at every object level in the schema.
	// Transform the schema to add it where missing.
	schema = ensureAdditionalPropertiesFalse(schema)

	dir := os.TempDir()
	f, err := os.CreateTemp(dir, "orc-codex-schema-*.json")
	if err != nil {
		return "", fmt.Errorf("create temp schema file: %w", err)
	}

	if _, err := f.WriteString(schema); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", fmt.Errorf("write schema: %w", err)
	}

	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return "", fmt.Errorf("close schema file: %w", err)
	}

	return f.Name(), nil
}

// ensureAdditionalPropertiesFalse walks a JSON schema and applies the OpenAI
// structured output rules orc relies on:
//  1. Every object with properties gets additionalProperties:false.
//  2. Every object requires all declared property keys.
//  3. Properties that were optional in the original schema are made nullable so
//     the required-all-keys rule does not change their semantics.
func ensureAdditionalPropertiesFalse(schema string) string {
	var root map[string]any
	if err := json.Unmarshal([]byte(schema), &root); err != nil {
		return schema // return unchanged if unparseable
	}
	enforceOpenAISchemaRules(root)
	out, err := json.Marshal(root)
	if err != nil {
		return schema
	}
	return string(out)
}

// enforceOpenAISchemaRules recursively fixes a JSON schema node for OpenAI compatibility.
func enforceOpenAISchemaRules(node map[string]any) {
	props, hasProps := node["properties"].(map[string]any)
	if hasProps {
		originalRequired := map[string]bool{}
		if requiredList, ok := node["required"].([]any); ok {
			for _, value := range requiredList {
				key, ok := value.(string)
				if ok {
					originalRequired[key] = true
				}
			}
		}

		// Add additionalProperties: false if missing
		if _, hasAP := node["additionalProperties"]; !hasAP {
			node["additionalProperties"] = false
		}

		// Ensure required lists ALL property keys, but preserve original optional
		// semantics by making non-required properties nullable.
		allKeys := make([]string, 0, len(props))
		for k, rawProp := range props {
			allKeys = append(allKeys, k)
			if !originalRequired[k] {
				if propSchema, ok := rawProp.(map[string]any); ok {
					makeSchemaNullable(propSchema)
				}
			}
		}
		node["required"] = allKeys

		// Recurse into each property
		for _, v := range props {
			if obj, ok := v.(map[string]any); ok {
				enforceOpenAISchemaRules(obj)
			}
		}
	}
	// Recurse into items (arrays of objects)
	if items, ok := node["items"].(map[string]any); ok {
		enforceOpenAISchemaRules(items)
	}
}

func makeSchemaNullable(node map[string]any) {
	switch typed := node["type"].(type) {
	case string:
		if typed == "null" {
			return
		}
		node["type"] = []any{typed, "null"}
	case []any:
		for _, value := range typed {
			if typeName, ok := value.(string); ok && typeName == "null" {
				return
			}
		}
		node["type"] = append(typed, "null")
	}
}

// extractLastJSON extracts the last valid JSON object from a potentially
// concatenated response. Codex with --output-schema returns one JSON object
// per turn; multi-turn execution produces "{json1}{json2}{json3}".
// We want only the final turn's result.
func extractLastJSON(content string) string {
	content = strings.TrimSpace(content)
	if len(content) == 0 || json.Valid([]byte(content)) {
		return content
	}

	// Scan backwards for the last '{' that starts a valid JSON object.
	for i := len(content) - 1; i >= 0; i-- {
		if content[i] == '{' {
			candidate := content[i:]
			if json.Valid([]byte(candidate)) {
				return candidate
			}
		}
	}

	return content // Return as-is if no valid JSON found; caller will handle error.
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
