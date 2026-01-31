// Package executor provides the execution engine for orc.
// This file provides a clean ClaudeCLI-based executor wrapper using
// headless mode (-p) with JSON schema for structured completion output.
package executor

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/randalmurphal/llmkit/claude"
	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
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

	// Review round for review phase schema selection (1 = findings, 2 = decision)
	reviewRound int

	// Transcript storage - if backend is set, transcripts are stored automatically
	backend storage.Backend
	taskID  string
	runID   string // workflow run ID (optional - for linking)

	// transcriptHandler is created internally when backend is provided
	transcriptHandler *TranscriptStreamHandler

	// phaseConfig contains per-phase Claude CLI configuration
	// (system prompts, tool restrictions, MCP servers, budgets, etc.)
	phaseConfig *PhaseClaudeConfig
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

// WithClaudePhaseID sets the phase ID for schema selection.
// Content-producing phases (spec, research, docs) use a schema
// that includes a content field for capturing output.
func WithClaudePhaseID(id string) ClaudeExecutorOption {
	return func(e *ClaudeExecutor) { e.phaseID = id }
}

// WithClaudeReviewRound sets the review round for review phase schema selection.
// Round 1 uses ReviewFindingsSchema, Round 2 uses ReviewDecisionSchema.
func WithClaudeReviewRound(round int) ClaudeExecutorOption {
	return func(e *ClaudeExecutor) { e.reviewRound = round }
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

// WithPhaseClaudeConfig sets the per-phase Claude configuration.
// This enables fine-grained control over Claude's behavior per-phase including:
// - System prompts (inline or file-based)
// - Tool restrictions (allowed, disallowed, tools list)
// - MCP servers (per-phase server configs)
// - Budget and limits (max_budget_usd, max_turns)
// - Environment variables and additional directories
// - Agent assignment (agent_ref, inline_agents - requires llmkit support)
// - Skill injection (skill_refs - resolved before passing to config)
func WithPhaseClaudeConfig(cfg *PhaseClaudeConfig) ClaudeExecutorOption {
	return func(e *ClaudeExecutor) { e.phaseConfig = cfg }
}

// NewClaudeExecutor creates a new Claude executor.
// If backend and taskID are provided, transcripts are stored automatically.
func NewClaudeExecutor(opts ...ClaudeExecutorOption) *ClaudeExecutor {
	e := &ClaudeExecutor{
		claudePath: "claude",
		logger:     slog.Default(),
	}
	for _, opt := range opts {
		opt(e)
	}

	// Create transcript handler if we have backend and taskID
	if e.backend != nil && e.taskID != "" {
		var captureHookEvents []string
		if e.phaseConfig != nil && len(e.phaseConfig.Hooks) > 0 {
			for event := range e.phaseConfig.Hooks {
				captureHookEvents = append(captureHookEvents, event)
			}
		}
		e.transcriptHandler = NewTranscriptStreamHandler(
			e.backend, e.logger,
			e.taskID, e.phaseID, e.sessionID, e.runID,
			e.model,
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

	// Store prompt before execution (if transcript handler is configured)
	if e.transcriptHandler != nil {
		e.transcriptHandler.StoreUserPrompt(prompt)
	}

	// Build CLI options using consolidated helper, then add JSON schema
	cliOpts := e.buildBaseCLIOptions()
	// Select schema based on phase and round - review/qa use specialized schemas
	schema := GetSchemaForPhaseWithRound(e.phaseID, e.reviewRound)
	cliOpts = append(cliOpts, claude.WithJSONSchema(schema))

	cli := claude.NewClaudeCLI(cliOpts...)

	// Build completion request with streaming callback for real-time transcript capture
	req := claude.CompletionRequest{
		Messages: []claude.Message{{Role: claude.RoleUser, Content: prompt}},
	}
	if e.transcriptHandler != nil {
		req.OnEvent = e.transcriptHandler.OnEvent
	}

	resp, err := cli.Complete(ctx, req)
	if err != nil {
		return &TurnResult{
			Duration:  time.Since(start),
			IsError:   true,
			ErrorText: err.Error(),
		}, fmt.Errorf("claude complete: %w", err)
	}

	// Build turn result
	result := &TurnResult{
		Content:   resp.Content,
		NumTurns:  resp.NumTurns,
		CostUSD:   resp.CostUSD,
		SessionID: resp.SessionID,
		Duration:  time.Since(start),
		Usage: &orcv1.TokenUsage{
			InputTokens:             int32(resp.Usage.InputTokens),
			OutputTokens:            int32(resp.Usage.OutputTokens),
			TotalTokens:             int32(resp.Usage.TotalTokens),
			CacheCreationInputTokens: int32(resp.Usage.CacheCreationInputTokens),
			CacheReadInputTokens:    int32(resp.Usage.CacheReadInputTokens),
		},
	}

	// Parse completion status from JSON response using phase-specific parser
	// Different phases use different schemas (review has findings/decision, QA has its own, etc.)
	// Error on parse failure - no silent continue
	status, reason, parseErr := ParsePhaseSpecificResponse(e.phaseID, e.reviewRound, resp.Content)
	result.Status = status
	result.Reason = reason

	if parseErr != nil {
		// JSON parse failed - this is a critical error, not a "continue" situation
		result.IsError = true
		result.ErrorText = parseErr.Error()
		return result, fmt.Errorf("phase completion JSON parse failed: %w", parseErr)
	}

	// Check for error response
	if resp.FinishReason == "error" {
		result.IsError = true
		result.ErrorText = resp.Content
	}

	return result, nil
}

// ExecuteTurnWithoutSchema sends a prompt without requiring structured output.
// Used for phases that don't need completion detection (e.g., conflict resolution).
// Transcripts are stored automatically if backend was configured.
func (e *ClaudeExecutor) ExecuteTurnWithoutSchema(ctx context.Context, prompt string) (*TurnResult, error) {
	start := time.Now()

	// Store prompt before execution (if transcript handler is configured)
	if e.transcriptHandler != nil {
		e.transcriptHandler.StoreUserPrompt(prompt)
	}

	// Build CLI options using consolidated helper (no JSON schema)
	cliOpts := e.buildBaseCLIOptions()

	cli := claude.NewClaudeCLI(cliOpts...)

	// Build completion request with streaming callback for real-time transcript capture
	req := claude.CompletionRequest{
		Messages: []claude.Message{{Role: claude.RoleUser, Content: prompt}},
	}
	if e.transcriptHandler != nil {
		req.OnEvent = e.transcriptHandler.OnEvent
	}

	resp, err := cli.Complete(ctx, req)
	if err != nil {
		return &TurnResult{
			Duration:  time.Since(start),
			IsError:   true,
			ErrorText: err.Error(),
		}, fmt.Errorf("claude complete: %w", err)
	}

	// Build turn result (no completion parsing - caller handles it)
	result := &TurnResult{
		Content:   resp.Content,
		NumTurns:  resp.NumTurns,
		CostUSD:   resp.CostUSD,
		SessionID: resp.SessionID,
		Duration:  time.Since(start),
		Status:    PhaseStatusContinue, // Default - caller determines actual status
		Usage: &orcv1.TokenUsage{
			InputTokens:             int32(resp.Usage.InputTokens),
			OutputTokens:            int32(resp.Usage.OutputTokens),
			TotalTokens:             int32(resp.Usage.TotalTokens),
			CacheCreationInputTokens: int32(resp.Usage.CacheCreationInputTokens),
			CacheReadInputTokens:    int32(resp.Usage.CacheReadInputTokens),
		},
	}

	if resp.FinishReason == "error" {
		result.IsError = true
		result.ErrorText = resp.Content
	}

	return result, nil
}

// UpdateSessionID updates the session ID for subsequent calls.
// Used after getting a session ID from the first response.
func (e *ClaudeExecutor) UpdateSessionID(id string) {
	e.logger.Debug("updating session ID", "old_id", e.sessionID, "new_id", id, "old_resume", e.resume)
	e.sessionID = id
	e.resume = true // Enable resume mode for subsequent calls
}

// SessionID returns the current session ID.
func (e *ClaudeExecutor) SessionID() string {
	return e.sessionID
}

// buildBaseCLIOptions builds the common set of CLI options shared by all execution methods.
// This consolidates the option building that was previously duplicated.
func (e *ClaudeExecutor) buildBaseCLIOptions() []claude.ClaudeOption {
	opts := []claude.ClaudeOption{
		claude.WithWorkdir(e.workdir),
		claude.WithOutputFormat(claude.OutputFormatJSON),
		claude.WithDangerouslySkipPermissions(),
		claude.WithSettingSources([]string{"project", "local", "user"}),
	}

	if e.claudePath != "" {
		opts = append(opts, claude.WithClaudePath(e.claudePath))
	}

	if e.model != "" {
		opts = append(opts, claude.WithModel(e.model))
	}

	// Session ID handling:
	// - First call: pass --session-id so Claude uses OUR UUID (not generate its own)
	// - Resume call: pass --resume to continue existing session
	if e.sessionID != "" {
		if e.resume {
			opts = append(opts, claude.WithResume(e.sessionID))
			e.logger.Debug("resuming existing session", "session_id", e.sessionID)
		} else {
			opts = append(opts, claude.WithSessionID(e.sessionID))
			e.logger.Debug("starting new session with ID", "session_id", e.sessionID)
		}
	}

	if e.maxTurns > 0 {
		opts = append(opts, claude.WithMaxTurns(e.maxTurns))
	}

	// Apply phase-specific Claude configuration
	// Priority: phaseConfig overrides executor-level settings
	if e.phaseConfig != nil {
		opts = e.applyPhaseConfig(opts)
	}

	return opts
}

// applyPhaseConfig applies PhaseClaudeConfig options to the CLI options.
// Returns the updated options slice.
func (e *ClaudeExecutor) applyPhaseConfig(opts []claude.ClaudeOption) []claude.ClaudeOption {
	cfg := e.phaseConfig
	if cfg == nil {
		return opts
	}

	// System prompts
	if cfg.SystemPrompt != "" {
		opts = append(opts, claude.WithSystemPrompt(cfg.SystemPrompt))
	}
	if cfg.AppendSystemPrompt != "" {
		opts = append(opts, claude.WithAppendSystemPrompt(cfg.AppendSystemPrompt))
	}
	// Note: SystemPromptFile and AppendSystemPromptFile are resolved to content
	// before being passed here (by skill_loader or workflow_phase.go)

	// Tool control
	if len(cfg.AllowedTools) > 0 {
		opts = append(opts, claude.WithAllowedTools(cfg.AllowedTools))
	}
	if len(cfg.DisallowedTools) > 0 {
		opts = append(opts, claude.WithDisallowedTools(cfg.DisallowedTools))
	}
	if len(cfg.Tools) > 0 {
		opts = append(opts, claude.WithTools(cfg.Tools))
	}

	// MCP servers
	if len(cfg.MCPServers) > 0 {
		opts = append(opts, claude.WithMCPServers(cfg.MCPServers))
	}
	if cfg.StrictMCPConfig {
		opts = append(opts, claude.WithStrictMCPConfig())
	}

	// Budget and limits - only apply if explicitly set in phase config
	// (0 means "not set", not "unlimited")
	if cfg.MaxBudgetUSD > 0 {
		opts = append(opts, claude.WithMaxBudgetUSD(cfg.MaxBudgetUSD))
	}
	if cfg.MaxTurns > 0 {
		// Phase config max_turns overrides executor-level maxTurns
		opts = append(opts, claude.WithMaxTurns(cfg.MaxTurns))
	}

	// Environment
	if len(cfg.Env) > 0 {
		opts = append(opts, claude.WithEnv(cfg.Env))
	}
	if len(cfg.AddDirs) > 0 {
		opts = append(opts, claude.WithAddDirs(cfg.AddDirs))
	}

	// Agent assignment (--agent and --agents)
	if cfg.AgentRef != "" {
		opts = append(opts, claude.WithAgent(cfg.AgentRef))
	}
	if len(cfg.InlineAgents) > 0 {
		opts = append(opts, claude.WithAgentsJSON(cfg.InlineAgentsJSON()))
	}

	// Skills are resolved before this point - content injected into AppendSystemPrompt
	// Hook events are handled by the transcript handler

	return opts
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

// NewMockTurnExecutorWithResponses creates a mock with a queue of responses.
func NewMockTurnExecutorWithResponses(responses ...string) *MockTurnExecutor {
	return &MockTurnExecutor{
		Responses:      responses,
		SessionIDValue: "mock-session-123",
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

	// Parse status from content using phase-specific parser (same as real executor)
	status, reason, parseErr := ParsePhaseSpecificResponse(m.PhaseID, m.ReviewRound, content)

	result := &TurnResult{
		Content:   content,
		Status:    status,
		Reason:    reason,
		NumTurns:  1,
		SessionID: m.SessionIDValue,
		Usage: &orcv1.TokenUsage{
			InputTokens:  100,
			OutputTokens: 50,
		},
	}

	if parseErr != nil {
		result.IsError = true
		result.ErrorText = parseErr.Error()
		return result, parseErr
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
