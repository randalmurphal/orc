// Package executor provides the execution engine for orc.
// This file provides a clean ClaudeCLI-based executor wrapper using
// headless mode (-p) with JSON schema for structured completion output.
package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	llmkit "github.com/randalmurphal/llmkit/v2"
	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/storage"
)

// TurnExecutor defines the interface for executing Claude turns.
// This abstraction allows for mocking in tests without spawning real Claude CLI.
type TurnExecutor interface {
	// ExecuteTurn sends a prompt and expects structured JSON output.
	ExecuteTurn(ctx context.Context, prompt string) (*TurnResult, error)
	// ExecuteTurnWithoutSchema sends a prompt without requiring structured output.
	ExecuteTurnWithoutSchema(ctx context.Context, prompt string) (*TurnResult, error)
	// UpdateSessionID updates the session ID for multi-turn conversations.
	UpdateSessionID(id string)
	// SessionID returns the current session ID.
	SessionID() string
}

// Ensure ClaudeExecutor implements TurnExecutor
var _ TurnExecutor = (*ClaudeExecutor)(nil)

// ClaudeExecutor wraps ClaudeCLI for phase execution with proper
// structured output via --json-schema. Handles transcript storage
// automatically when backend is provided.
type ClaudeExecutor struct {
	claudePath string
	workdir    string
	model      string
	logger     *slog.Logger

	// Session management for multi-turn
	sessionID string
	resume    bool

	// Max turns (budget control)
	maxTurns int

	// Phase ID for schema selection (artifact vs non-artifact phases)
	phaseID string

	// Whether this phase produces an artifact (content field in schema)
	producesArtifact bool

	// Review round for review phase schema selection (1 = findings, 2 = decision)
	reviewRound int

	// Loop configuration for iteration-specific schema selection
	loopConfig *db.LoopConfig

	// Current loop iteration (1-based)
	loopIteration int

	// Transcript storage - if backend is set, transcripts are stored automatically
	backend storage.Backend
	taskID  string
	runID   string // workflow run ID (optional - for linking)

	// transcriptHandler is created internally when backend is provided
	transcriptHandler *TranscriptStreamHandler

	// phaseConfig contains per-phase Claude CLI configuration
	// (system prompts, tool restrictions, MCP servers, budgets, etc.)
	phaseConfig *PhaseRuntimeConfig

	// Inactivity timeout for silent stalled turns.
	inactivityTimeout time.Duration
}

// ClaudeExecutorOption configures a ClaudeExecutor.
type ClaudeExecutorOption func(*ClaudeExecutor)

// WithClaudeWorkdir sets the working directory.
func WithClaudeWorkdir(dir string) ClaudeExecutorOption {
	return func(e *ClaudeExecutor) { e.workdir = dir }
}

// WithClaudeModel sets the model to use.
func WithClaudeModel(model string) ClaudeExecutorOption {
	return func(e *ClaudeExecutor) { e.model = model }
}

// WithClaudeSessionID sets the session ID for conversation tracking.
func WithClaudeSessionID(id string) ClaudeExecutorOption {
	return func(e *ClaudeExecutor) { e.sessionID = id }
}

// WithClaudeResume enables session resume mode.
func WithClaudeResume(resume bool) ClaudeExecutorOption {
	return func(e *ClaudeExecutor) { e.resume = resume }
}

// WithClaudePath sets the path to claude binary.
func WithClaudePath(path string) ClaudeExecutorOption {
	return func(e *ClaudeExecutor) { e.claudePath = path }
}

// WithClaudeMaxTurns sets the maximum turns for budget control.
func WithClaudeMaxTurns(maxTurns int) ClaudeExecutorOption {
	return func(e *ClaudeExecutor) { e.maxTurns = maxTurns }
}

// WithClaudeLogger sets the logger.
func WithClaudeLogger(l *slog.Logger) ClaudeExecutorOption {
	return func(e *ClaudeExecutor) { e.logger = l }
}

// WithClaudeInactivityTimeout sets the no-output watchdog timeout for Claude turns.
func WithClaudeInactivityTimeout(d time.Duration) ClaudeExecutorOption {
	return func(e *ClaudeExecutor) { e.inactivityTimeout = d }
}

// WithClaudePhaseID sets the phase ID for schema selection.
func WithClaudePhaseID(id string) ClaudeExecutorOption {
	return func(e *ClaudeExecutor) { e.phaseID = id }
}

// WithClaudeProducesArtifact sets whether the phase produces an artifact.
// Content-producing phases use a schema with a content field for capturing output.
func WithClaudeProducesArtifact(produces bool) ClaudeExecutorOption {
	return func(e *ClaudeExecutor) { e.producesArtifact = produces }
}

// WithClaudeReviewRound sets the review round for review phase schema selection.
// Round 1 uses ReviewFindingsSchema, Round 2 uses ReviewDecisionSchema.
func WithClaudeReviewRound(round int) ClaudeExecutorOption {
	return func(e *ClaudeExecutor) { e.reviewRound = round }
}

// WithClaudeLoopConfig sets the loop configuration for iteration-specific schema selection.
// The LoopConfig's LoopSchemas map determines which schema is used for each iteration.
func WithClaudeLoopConfig(cfg *db.LoopConfig) ClaudeExecutorOption {
	return func(e *ClaudeExecutor) { e.loopConfig = cfg }
}

// WithClaudeLoopIteration sets the current loop iteration (1-based).
// Used with LoopConfig for iteration-specific schema selection.
func WithClaudeLoopIteration(iteration int) ClaudeExecutorOption {
	return func(e *ClaudeExecutor) { e.loopIteration = iteration }
}

// WithClaudeBackend sets the storage backend for automatic transcript storage.
// When set along with WithClaudeTaskID, transcripts are stored in real-time
// as Claude streams responses. This is the unified path for all Claude calls.
func WithClaudeBackend(b storage.Backend) ClaudeExecutorOption {
	return func(e *ClaudeExecutor) { e.backend = b }
}

// WithClaudeTaskID sets the task ID for transcript storage.
// Required for transcript storage to work (along with backend).
func WithClaudeTaskID(id string) ClaudeExecutorOption {
	return func(e *ClaudeExecutor) { e.taskID = id }
}

// WithClaudeRunID sets the workflow run ID for transcript linking.
// Optional - transcripts are linked to runs when provided.
func WithClaudeRunID(id string) ClaudeExecutorOption {
	return func(e *ClaudeExecutor) { e.runID = id }
}

// WithPhaseRuntimeConfig sets the per-phase Claude configuration.
// This enables fine-grained control over Claude's behavior per-phase including:
// - System prompts (inline or file-based)
// - Tool restrictions (allowed, disallowed, tools list)
// - MCP servers (per-phase server configs)
// - Budget and limits (max_budget_usd, max_turns)
// - Environment variables and additional directories
// - Agent assignment (agent_ref, inline_agents - requires llmkit support)
// - Skill injection (skill_refs - resolved before passing to config)
func WithPhaseRuntimeConfig(cfg *PhaseRuntimeConfig) ClaudeExecutorOption {
	return func(e *ClaudeExecutor) { e.phaseConfig = cfg }
}

// NewClaudeExecutor creates a new Claude executor.
// If backend and taskID are provided, transcripts are stored automatically.
func NewClaudeExecutor(opts ...ClaudeExecutorOption) *ClaudeExecutor {
	e := &ClaudeExecutor{
		claudePath:        "claude",
		logger:            slog.Default(),
		inactivityTimeout: DefaultProviderInactivityTimeout,
	}
	for _, opt := range opts {
		opt(e)
	}

	// Create transcript handler if we have backend and taskID
	if e.backend != nil && e.taskID != "" {
		var captureHookEvents []string
		if e.phaseConfig != nil && e.phaseConfig.Providers.Claude != nil && len(e.phaseConfig.Providers.Claude.Hooks) > 0 {
			for event := range e.phaseConfig.Providers.Claude.Hooks {
				captureHookEvents = append(captureHookEvents, event)
			}
		}
		e.transcriptHandler = NewTranscriptStreamHandler(
			e.backend, e.logger,
			e.taskID, e.phaseID, e.sessionID, e.runID,
			e.model,
			nil,
			captureHookEvents,
		)
	}

	return e
}

// TurnResult contains the outcome of a single turn.
// This is kept for compatibility with existing executor code.
type TurnResult struct {
	Content   string
	Status    PhaseCompletionStatus
	Reason    string // For blocked status or continue reason
	NumTurns  int
	CostUSD   float64
	Usage     *orcv1.TokenUsage
	Duration  time.Duration
	IsError   bool
	ErrorText string
	SessionID string // Session ID from response (for tracking)
}

// ExecuteTurn sends a prompt to Claude and waits for the response.
// Uses --json-schema to force structured output for completion detection.
// The schema varies by phase: content-producing phases (spec, research, docs)
// use a schema with a content field to capture output.
// Transcripts are stored automatically if backend was configured.
func (e *ClaudeExecutor) ExecuteTurn(ctx context.Context, prompt string) (*TurnResult, error) {
	start := time.Now()

	// Select schema using loop-aware selection (falls back to round-based for backward compat)
	// Priority: loopIteration > reviewRound > 1
	iteration := e.loopIteration
	if iteration == 0 {
		iteration = e.reviewRound
	}
	if iteration == 0 {
		iteration = 1
	}
	schema := GetSchemaForIteration(e.loopConfig, iteration, e.phaseID, e.producesArtifact)
	result, err := e.executeStream(ctx, prompt, schema, start)
	if err != nil {
		return result, err
	}

	// Parse completion status from JSON response using phase-specific parser
	// Different phases use different schemas (review has findings/decision, QA has its own, etc.)
	// Error on parse failure - no silent continue
	status, reason, parseErr := ParsePhaseSpecificResponse(e.phaseID, e.reviewRound, result.Content)
	result.Status = status
	result.Reason = reason

	if parseErr != nil {
		// JSON parse failed - this is a critical error, not a "continue" situation
		result.IsError = true
		result.ErrorText = parseErr.Error()
		return result, fmt.Errorf("phase completion JSON parse failed: %w", parseErr)
	}

	return result, nil
}

// ExecuteTurnWithoutSchema sends a prompt without requiring structured output.
// Used for phases that don't need completion detection (e.g., conflict resolution).
// Transcripts are stored automatically if backend was configured.
func (e *ClaudeExecutor) ExecuteTurnWithoutSchema(ctx context.Context, prompt string) (*TurnResult, error) {
	start := time.Now()

	result, err := e.executeStream(ctx, prompt, "", start)
	if err != nil {
		return result, err
	}
	result.Status = PhaseStatusContinue // Default - caller determines actual status
	return result, nil
}

// UpdateSessionID updates the session ID for subsequent calls.
// Used after getting a session ID from the first response.
func (e *ClaudeExecutor) UpdateSessionID(id string) {
	e.logger.Debug("updating session ID", "old_id", e.sessionID, "new_id", id, "old_resume", e.resume)
	e.sessionID = id
	e.resume = true // Enable resume mode for subsequent calls
	if e.transcriptHandler != nil {
		e.transcriptHandler.UpdateSessionID(id)
	}
}

// SessionID returns the current session ID.
func (e *ClaudeExecutor) SessionID() string {
	return e.sessionID
}

func (e *ClaudeExecutor) executeStream(ctx context.Context, prompt, schema string, start time.Time) (*TurnResult, error) {
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

	clientCfg, err := e.buildClientConfig()
	if err != nil {
		return &TurnResult{
			Duration:  time.Since(start),
			IsError:   true,
			ErrorText: err.Error(),
		}, err
	}

	client, err := llmkit.New(ProviderClaude, clientCfg)
	if err != nil {
		return &TurnResult{
			Duration:  time.Since(start),
			IsError:   true,
			ErrorText: err.Error(),
		}, fmt.Errorf("create llmkit claude client: %w", err)
	}
	defer client.Close()

	req := llmkit.Request{
		Messages: []llmkit.Message{llmkit.NewTextMessage(llmkit.RoleUser, prompt)},
	}
	if schema != "" {
		req.JSONSchema = json.RawMessage(schema)
	}

	watchCtx, watchCancel := context.WithCancel(ctx)
	defer watchCancel()
	watchdog := NewTurnWatchdog(e.inactivityTimeout, watchCancel)
	watchdog.Start(watchCtx)

	stream, err := client.Stream(watchCtx, req)
	if err != nil {
		return &TurnResult{
			Duration:  time.Since(start),
			IsError:   true,
			ErrorText: err.Error(),
		}, fmt.Errorf("claude stream: %w", err)
	}

	var (
		contentBuilder strings.Builder
		finalContent   string
		usage          *llmkit.TokenUsage
		sessionID      = e.sessionID
		numTurns       int
		costUSD        float64
	)

	for chunk := range stream {
		watchdog.RecordActivity()
		if chunk.SessionID != "" && chunk.SessionID != sessionID {
			sessionID = chunk.SessionID
			e.UpdateSessionID(sessionID)
		}
		if e.transcriptHandler != nil {
			e.transcriptHandler.OnChunk(chunk)
			if transcriptErr := e.transcriptHandler.Err(); transcriptErr != nil {
				watchCancel()
			}
		}
		if chunk.Content != "" {
			contentBuilder.WriteString(chunk.Content)
		}
		if chunk.FinalContent != "" {
			finalContent = chunk.FinalContent
		}
		if chunk.Usage != nil {
			usage = chunk.Usage
		}
		if chunk.NumTurns > 0 {
			numTurns = chunk.NumTurns
		}
		if chunk.CostUSD > 0 {
			costUSD = chunk.CostUSD
		}
		if chunk.Error != nil {
			err := chunk.Error
			if transcriptErr := e.transcriptHandler.Err(); transcriptErr != nil {
				err = transcriptErr
			}
			if stallErr := watchdog.Error("claude"); stallErr != nil {
				err = stallErr
			}
			content := strings.TrimSpace(contentBuilder.String())
			if finalContent != "" {
				content = strings.TrimSpace(finalContent)
			}
			return &TurnResult{
				Content:   content,
				Duration:  time.Since(start),
				IsError:   true,
				ErrorText: err.Error(),
				SessionID: sessionID,
			}, fmt.Errorf("claude stream: %w", err)
		}
	}

	if transcriptErr := e.transcriptHandler.Err(); transcriptErr != nil {
		return &TurnResult{
			Duration:  time.Since(start),
			IsError:   true,
			ErrorText: transcriptErr.Error(),
			SessionID: sessionID,
		}, transcriptErr
	}
	if stallErr := watchdog.Error("claude"); stallErr != nil {
		return &TurnResult{
			Duration:  time.Since(start),
			IsError:   true,
			ErrorText: stallErr.Error(),
			SessionID: sessionID,
		}, stallErr
	}

	content := strings.TrimSpace(contentBuilder.String())
	if finalContent != "" {
		content = strings.TrimSpace(finalContent)
	}

	result := &TurnResult{
		Content:   content,
		NumTurns:  numTurns,
		CostUSD:   costUSD,
		SessionID: sessionID,
		Duration:  time.Since(start),
	}
	if usage != nil {
		result.Usage = &orcv1.TokenUsage{
			InputTokens:              int32(usage.InputTokens),
			OutputTokens:             int32(usage.OutputTokens),
			TotalTokens:              int32(usage.TotalTokens),
			CacheCreationInputTokens: int32(usage.CacheCreationInputTokens),
			CacheReadInputTokens:     int32(usage.CacheReadInputTokens),
		}
	}

	return result, nil
}

func (e *ClaudeExecutor) buildClientConfig() (llmkit.Config, error) {
	runtime := llmkit.RuntimeConfig{}
	if e.phaseConfig != nil {
		runtime = e.phaseConfig.ToLLMKit()
	}
	if runtime.Shared.MaxTurns == 0 && e.maxTurns > 0 {
		runtime.Shared.MaxTurns = e.maxTurns
	}
	if runtime.Providers.Claude == nil {
		runtime.Providers.Claude = &llmkit.ClaudeRuntimeConfig{}
	}
	runtime.Providers.Claude.DangerouslySkipPermissions = true
	if len(runtime.Providers.Claude.SettingSources) == 0 {
		runtime.Providers.Claude.SettingSources = []string{"project", "local", "user"}
	}

	var session *llmkit.SessionMetadata
	if e.sessionID != "" {
		session = llmkit.SessionMetadataForID(ProviderClaude, e.sessionID)
	}

	cfg, err := llmkit.BuildConfig(ProviderClaude, e.model, e.workdir, runtime, session)
	if err != nil {
		return llmkit.Config{}, fmt.Errorf("build claude runtime config: %w", err)
	}
	cfg.ResumeSession = e.resume
	cfg.BinaryPath = e.claudePath
	return cfg, nil
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func mapHeadersToSlice(in map[string]string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, 0, len(in))
	for name, value := range in {
		out = append(out, fmt.Sprintf("%s: %s", name, value))
	}
	return out
}

// MockTurnExecutor is a test double for TurnExecutor that returns configurable responses.
// Use this in tests to avoid spawning real Claude CLI processes.
type MockTurnExecutor struct {
	// Responses is a queue of responses to return. Each call pops the first response.
	// If empty, returns DefaultResponse.
	Responses []string
	// DefaultResponse is returned when Responses is empty.
	DefaultResponse string
	// CallCount tracks how many times ExecuteTurn was called.
	callCount int
	// Prompts stores all prompts received for verification.
	Prompts []string
	// SessionIDValue is the session ID to return.
	SessionIDValue string
	// Error to return (if set, returned on every call).
	Error error
	// Delay is how long to wait before returning. If set, respects context cancellation.
	Delay time.Duration
	// PhaseID for phase-specific response parsing (review, qa use different schemas)
	PhaseID string
	// ReviewRound for review phase (1 = findings, 2 = decision)
	ReviewRound int
	// UsageOverrides is a queue of token usage values. Each call pops the first entry.
	// If empty, uses the default {InputTokens: 100, OutputTokens: 50}.
	UsageOverrides []*orcv1.TokenUsage
	// CostOverrides is a queue of CostUSD values. Each call pops the first entry.
	// If empty, CostUSD defaults to 0.
	CostOverrides []float64
}

// Ensure MockTurnExecutor implements TurnExecutor
var _ TurnExecutor = (*MockTurnExecutor)(nil)

// NewMockTurnExecutor creates a mock that returns the given response.
func NewMockTurnExecutor(response string) *MockTurnExecutor {
	return &MockTurnExecutor{
		DefaultResponse: response,
		SessionIDValue:  "mock-session-123",
	}
}

// ExecuteTurn returns the next response from the queue or DefaultResponse.
func (m *MockTurnExecutor) ExecuteTurn(ctx context.Context, prompt string) (*TurnResult, error) {
	m.callCount++
	m.Prompts = append(m.Prompts, prompt)

	// Honor Delay if set, respecting context cancellation
	if m.Delay > 0 {
		select {
		case <-time.After(m.Delay):
			// Continue to return response
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	if m.Error != nil {
		return &TurnResult{IsError: true, ErrorText: m.Error.Error()}, m.Error
	}

	var content string
	if len(m.Responses) > 0 {
		content = m.Responses[0]
		m.Responses = m.Responses[1:]
	} else {
		content = m.DefaultResponse
	}

	var usage *orcv1.TokenUsage
	if len(m.UsageOverrides) > 0 {
		usage = m.UsageOverrides[0]
		m.UsageOverrides = m.UsageOverrides[1:]
	} else {
		usage = &orcv1.TokenUsage{
			InputTokens:  100,
			OutputTokens: 50,
		}
	}

	var costUSD float64
	if len(m.CostOverrides) > 0 {
		costUSD = m.CostOverrides[0]
		m.CostOverrides = m.CostOverrides[1:]
	}

	result := &TurnResult{
		Content:   content,
		NumTurns:  1,
		SessionID: m.SessionIDValue,
		CostUSD:   costUSD,
		Usage:     usage,
	}

	// Parse status from content using phase-specific parser (same as real executor).
	// When PhaseID is empty, skip parsing and let the executor's own
	// ParsePhaseSpecificResponse handle it with the correct phase ID.
	// This supports integration tests where the mock serves multiple phases.
	if m.PhaseID != "" {
		status, reason, parseErr := ParsePhaseSpecificResponse(m.PhaseID, m.ReviewRound, content)
		result.Status = status
		result.Reason = reason
		if parseErr != nil {
			result.IsError = true
			result.ErrorText = parseErr.Error()
			return result, parseErr
		}
	}

	return result, nil
}

// ExecuteTurnWithoutSchema is the same as ExecuteTurn for the mock.
func (m *MockTurnExecutor) ExecuteTurnWithoutSchema(ctx context.Context, prompt string) (*TurnResult, error) {
	return m.ExecuteTurn(ctx, prompt)
}

// UpdateSessionID updates the session ID.
func (m *MockTurnExecutor) UpdateSessionID(id string) {
	m.SessionIDValue = id
}

// SessionID returns the current session ID.
func (m *MockTurnExecutor) SessionID() string {
	return m.SessionIDValue
}

// CallCount returns how many times ExecuteTurn was called.
func (m *MockTurnExecutor) CallCount() int {
	return m.callCount
}
