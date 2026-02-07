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
	"github.com/randalmurphal/orc/internal/storage"
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
	backend storage.Backend
	taskID  string
	runID   string

	// Codex-specific settings
	sandboxMode  codex.SandboxMode
	approvalMode codex.ApprovalMode

	// Local model routing (ollama, lmstudio)
	localProvider string

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

// WithCodexSandboxMode sets the sandbox mode.
func WithCodexSandboxMode(mode codex.SandboxMode) CodexExecutorOption {
	return func(e *CodexExecutor) { e.sandboxMode = mode }
}

// WithCodexApprovalMode sets the approval mode.
func WithCodexApprovalMode(mode codex.ApprovalMode) CodexExecutorOption {
	return func(e *CodexExecutor) { e.approvalMode = mode }
}

// WithCodexLocalProvider sets the local model provider (ollama, lmstudio).
func WithCodexLocalProvider(provider string) CodexExecutorOption {
	return func(e *CodexExecutor) { e.localProvider = provider }
}

// WithCodexSchemaRetries sets the number of schema validation retries.
// Default is 2 (total 3 attempts including the first).
func WithCodexSchemaRetries(retries int) CodexExecutorOption {
	return func(e *CodexExecutor) { e.schemaRetries = retries }
}

// NewCodexExecutor creates a new Codex executor.
func NewCodexExecutor(opts ...CodexExecutorOption) *CodexExecutor {
	e := &CodexExecutor{
		codexPath:    "codex",
		logger:       slog.Default(),
		sandboxMode:  codex.SandboxWorkspaceWrite,
		approvalMode: codex.ApprovalNever,
		schemaRetries: 2,
	}
	for _, opt := range opts {
		opt(e)
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
}

// SessionID returns the current session ID.
func (e *CodexExecutor) SessionID() string {
	return e.sessionID
}

// executeSingleTurn runs a single codex completion or resume call.
func (e *CodexExecutor) executeSingleTurn(ctx context.Context, prompt, schemaFile string, start time.Time) (*TurnResult, error) {
	// Build common options
	cliOpts := e.buildCLIOptions(schemaFile)
	cli := codex.NewCodexCLI(cliOpts...)

	var resp *codex.CompletionResponse
	var err error

	if e.resume && e.sessionID != "" {
		// Resume existing session
		resp, err = cli.Resume(ctx, e.sessionID, prompt)
	} else {
		// Fresh completion
		req := codex.CompletionRequest{
			Messages: []codex.Message{{Role: codex.RoleUser, Content: prompt}},
		}
		resp, err = cli.Complete(ctx, req)
	}

	if err != nil {
		return &TurnResult{
			Duration:  time.Since(start),
			IsError:   true,
			ErrorText: err.Error(),
		}, fmt.Errorf("codex complete: %w", err)
	}

	result := &TurnResult{
		Content:   resp.Content,
		NumTurns:  resp.NumTurns,
		CostUSD:   resp.CostUSD,
		SessionID: resp.SessionID,
		Duration:  time.Since(start),
		Usage: &orcv1.TokenUsage{
			InputTokens:  int32(resp.Usage.InputTokens),
			OutputTokens: int32(resp.Usage.OutputTokens),
			TotalTokens:  int32(resp.Usage.TotalTokens),
		},
	}

	if resp.FinishReason == "error" {
		result.IsError = true
		result.ErrorText = resp.Content
	}

	// Store transcript if backend is configured
	if e.backend != nil && e.taskID != "" {
		e.storeTranscript(prompt, resp)
	}

	return result, nil
}

// buildCLIOptions constructs codex CLI options for a turn.
func (e *CodexExecutor) buildCLIOptions(schemaFile string) []codex.CodexOption {
	opts := []codex.CodexOption{
		codex.WithWorkdir(e.workdir),
		codex.WithSandboxMode(e.sandboxMode),
		codex.WithApprovalMode(e.approvalMode),
	}

	if e.codexPath != "" {
		opts = append(opts, codex.WithCodexPath(e.codexPath))
	}

	if e.model != "" {
		opts = append(opts, codex.WithModel(e.model))
	}

	// Session handling — codex uses session ID for resume
	if e.sessionID != "" && !e.resume {
		opts = append(opts, codex.WithSessionID(e.sessionID))
	}

	// Schema file for structured output
	if schemaFile != "" {
		opts = append(opts, codex.WithOutputSchema(schemaFile))
	}

	// Local model routing
	if e.localProvider != "" {
		opts = append(opts, codex.WithLocalProvider(e.localProvider))
	}

	return opts
}

// writeSchemaFile writes a JSON schema string to a temp file and returns the path.
func (e *CodexExecutor) writeSchemaFile(schema string) (string, error) {
	// Validate that it's valid JSON
	if !json.Valid([]byte(schema)) {
		return "", fmt.Errorf("invalid JSON schema: %s", truncate(schema, 100))
	}

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

// storeTranscript stores prompt and response as transcript entries.
func (e *CodexExecutor) storeTranscript(prompt string, resp *codex.CompletionResponse) {
	now := time.Now().UnixMilli()

	// Store user prompt
	userTranscript := &storage.Transcript{
		TaskID:        e.taskID,
		Phase:         e.phaseID,
		SessionID:     e.sessionID,
		WorkflowRunID: e.runID,
		MessageUUID:   fmt.Sprintf("codex-user-%d", now),
		Role:          "user",
		Type:          "user",
		Content:       prompt,
		Model:         e.model,
		Timestamp:     now,
	}
	if err := e.backend.AddTranscript(userTranscript); err != nil {
		e.logger.Warn("failed to store codex user transcript", "error", err)
	}

	// Store assistant response
	assistantTranscript := &storage.Transcript{
		TaskID:        e.taskID,
		Phase:         e.phaseID,
		SessionID:     resp.SessionID,
		WorkflowRunID: e.runID,
		MessageUUID:   fmt.Sprintf("codex-assistant-%d", now),
		Role:          "assistant",
		Type:          "assistant",
		Content:       resp.Content,
		Model:         resp.Model,
		Timestamp:     now,
	}
	if err := e.backend.AddTranscript(assistantTranscript); err != nil {
		e.logger.Warn("failed to store codex assistant transcript", "error", err)
	}
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

