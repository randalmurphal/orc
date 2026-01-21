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
)

// ClaudeExecutor wraps ClaudeCLI for phase execution with proper
// structured output via --json-schema. This replaces the old
// session-based approach that used stream-json mode.
type ClaudeExecutor struct {
	claudePath string
	workdir    string
	model      string
	logger     *slog.Logger

	// Session management for multi-turn
	sessionID string
	resume    bool

	// MCP config path (optional)
	mcpConfigPath string

	// Max turns (budget control)
	maxTurns int
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

// WithClaudeMCPConfig sets the MCP config path.
func WithClaudeMCPConfig(path string) ClaudeExecutorOption {
	return func(e *ClaudeExecutor) { e.mcpConfigPath = path }
}

// WithClaudeMaxTurns sets the maximum turns for budget control.
func WithClaudeMaxTurns(maxTurns int) ClaudeExecutorOption {
	return func(e *ClaudeExecutor) { e.maxTurns = maxTurns }
}

// WithClaudeLogger sets the logger.
func WithClaudeLogger(l *slog.Logger) ClaudeExecutorOption {
	return func(e *ClaudeExecutor) { e.logger = l }
}

// NewClaudeExecutor creates a new Claude executor.
func NewClaudeExecutor(opts ...ClaudeExecutorOption) *ClaudeExecutor {
	e := &ClaudeExecutor{
		claudePath: "claude",
		logger:     slog.Default(),
	}
	for _, opt := range opts {
		opt(e)
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
	Usage     TokenUsage
	Duration  time.Duration
	IsError   bool
	ErrorText string
	SessionID string // Session ID from response (for tracking)
}

// TokenUsage tracks token consumption.
type TokenUsage struct {
	InputTokens              int
	OutputTokens             int
	TotalTokens              int
	CacheCreationInputTokens int
	CacheReadInputTokens     int
}

// EffectiveInputTokens returns the total input context size including cached tokens.
func (u TokenUsage) EffectiveInputTokens() int {
	return u.InputTokens + u.CacheCreationInputTokens + u.CacheReadInputTokens
}

// EffectiveTotalTokens returns the total tokens including cached inputs.
func (u TokenUsage) EffectiveTotalTokens() int {
	return u.EffectiveInputTokens() + u.OutputTokens
}

// ExecuteTurn sends a prompt to Claude and waits for the response.
// Uses --json-schema to force structured output for completion detection.
func (e *ClaudeExecutor) ExecuteTurn(ctx context.Context, prompt string) (*TurnResult, error) {
	start := time.Now()

	// Build ClaudeCLI with proper options
	cliOpts := []claude.ClaudeOption{
		claude.WithWorkdir(e.workdir),
		claude.WithOutputFormat(claude.OutputFormatJSON),
		claude.WithJSONSchema(PhaseCompletionSchema),
		claude.WithDangerouslySkipPermissions(),
		claude.WithSettingSources([]string{"project", "local", "user"}),
	}

	if e.claudePath != "" {
		cliOpts = append(cliOpts, claude.WithClaudePath(e.claudePath))
	}

	if e.model != "" {
		cliOpts = append(cliOpts, claude.WithModel(e.model))
	}

	if e.sessionID != "" {
		if e.resume {
			cliOpts = append(cliOpts, claude.WithResume(e.sessionID))
		} else {
			cliOpts = append(cliOpts, claude.WithSessionID(e.sessionID))
		}
	}

	if e.mcpConfigPath != "" {
		cliOpts = append(cliOpts, claude.WithMCPConfig(e.mcpConfigPath))
	}

	if e.maxTurns > 0 {
		cliOpts = append(cliOpts, claude.WithMaxTurns(e.maxTurns))
	}

	cli := claude.NewClaudeCLI(cliOpts...)

	// Execute the request
	resp, err := cli.Complete(ctx, claude.CompletionRequest{
		Messages: []claude.Message{{Role: claude.RoleUser, Content: prompt}},
	})
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
		Usage: TokenUsage{
			InputTokens:              resp.Usage.InputTokens,
			OutputTokens:             resp.Usage.OutputTokens,
			TotalTokens:              resp.Usage.TotalTokens,
			CacheCreationInputTokens: resp.Usage.CacheCreationInputTokens,
			CacheReadInputTokens:     resp.Usage.CacheReadInputTokens,
		},
	}

	// Parse completion status from JSON response
	// Since we use --json-schema, response should be pure JSON matching PhaseCompletionSchema
	result.Status, result.Reason = CheckPhaseCompletionJSON(resp.Content)

	// Check for error response
	if resp.FinishReason == "error" {
		result.IsError = true
		result.ErrorText = resp.Content
	}

	return result, nil
}

// ExecuteTurnWithoutSchema sends a prompt without requiring structured output.
// Used for phases that don't need completion detection (e.g., conflict resolution).
func (e *ClaudeExecutor) ExecuteTurnWithoutSchema(ctx context.Context, prompt string) (*TurnResult, error) {
	start := time.Now()

	// Build ClaudeCLI without JSON schema
	cliOpts := []claude.ClaudeOption{
		claude.WithWorkdir(e.workdir),
		claude.WithOutputFormat(claude.OutputFormatJSON), // Still use JSON for metadata
		claude.WithDangerouslySkipPermissions(),
		claude.WithSettingSources([]string{"project", "local", "user"}),
	}

	if e.claudePath != "" {
		cliOpts = append(cliOpts, claude.WithClaudePath(e.claudePath))
	}

	if e.model != "" {
		cliOpts = append(cliOpts, claude.WithModel(e.model))
	}

	if e.sessionID != "" {
		if e.resume {
			cliOpts = append(cliOpts, claude.WithResume(e.sessionID))
		} else {
			cliOpts = append(cliOpts, claude.WithSessionID(e.sessionID))
		}
	}

	if e.mcpConfigPath != "" {
		cliOpts = append(cliOpts, claude.WithMCPConfig(e.mcpConfigPath))
	}

	if e.maxTurns > 0 {
		cliOpts = append(cliOpts, claude.WithMaxTurns(e.maxTurns))
	}

	cli := claude.NewClaudeCLI(cliOpts...)

	// Execute the request
	resp, err := cli.Complete(ctx, claude.CompletionRequest{
		Messages: []claude.Message{{Role: claude.RoleUser, Content: prompt}},
	})
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
		Usage: TokenUsage{
			InputTokens:              resp.Usage.InputTokens,
			OutputTokens:             resp.Usage.OutputTokens,
			TotalTokens:              resp.Usage.TotalTokens,
			CacheCreationInputTokens: resp.Usage.CacheCreationInputTokens,
			CacheReadInputTokens:     resp.Usage.CacheReadInputTokens,
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
	e.sessionID = id
	e.resume = true // Enable resume mode for subsequent calls
}

// SessionID returns the current session ID.
func (e *ClaudeExecutor) SessionID() string {
	return e.sessionID
}
