package executor

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/randalmurphal/llmkit/claude"
	"github.com/randalmurphal/llmkit/claude/session"
	"github.com/randalmurphal/llmkit/parser"
)

// SessionAdapter wraps a llmkit session for phase execution.
// It provides methods for sending prompts and collecting responses
// with completion marker detection.
type SessionAdapter struct {
	session session.Session
	manager session.SessionManager
	markers *parser.MarkerMatcher
	owns    bool // True if we created the session and should close it
}

// SessionAdapterOptions configures session creation.
type SessionAdapterOptions struct {
	SessionID   string
	Resume      bool
	Model       string
	Workdir     string
	MaxTurns    int
	Persistence bool
}

// NewSessionAdapter creates a new session adapter.
// If the session already exists in the manager, it will be reused.
func NewSessionAdapter(ctx context.Context, mgr session.SessionManager, opts SessionAdapterOptions) (*SessionAdapter, error) {
	// Check for existing session
	if opts.SessionID != "" && !opts.Resume {
		if existing, ok := mgr.Get(opts.SessionID); ok {
			return &SessionAdapter{
				session: existing,
				manager: mgr,
				markers: PhaseMarkers,
				owns:    false,
			}, nil
		}
	}

	// Build session options
	var sessionOpts []session.SessionOption

	if opts.SessionID != "" {
		if opts.Resume {
			sessionOpts = append(sessionOpts, session.WithResume(opts.SessionID))
		} else {
			sessionOpts = append(sessionOpts, session.WithSessionID(opts.SessionID))
		}
	}

	if opts.Model != "" {
		sessionOpts = append(sessionOpts, session.WithModel(opts.Model))
	}

	if opts.Workdir != "" {
		sessionOpts = append(sessionOpts, session.WithWorkdir(opts.Workdir))
	}

	if opts.MaxTurns > 0 {
		sessionOpts = append(sessionOpts, session.WithMaxTurns(opts.MaxTurns))
	}

	if !opts.Persistence {
		sessionOpts = append(sessionOpts, session.WithNoSessionPersistence())
	}

	// Create session
	s, err := mgr.Create(ctx, sessionOpts...)
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	return &SessionAdapter{
		session: s,
		manager: mgr,
		markers: PhaseMarkers,
		owns:    true,
	}, nil
}

// SessionID returns the session identifier.
func (a *SessionAdapter) SessionID() string {
	return a.session.ID()
}

// Status returns the current session status.
func (a *SessionAdapter) Status() session.SessionStatus {
	return a.session.Status()
}

// TurnResult contains the outcome of a single turn.
type TurnResult struct {
	Content   string
	Status    PhaseCompletionStatus
	Reason    string // For blocked status
	NumTurns  int
	CostUSD   float64
	Usage     TokenUsage
	Duration  time.Duration
	IsError   bool
	ErrorText string
}

// TokenUsage tracks token consumption.
type TokenUsage struct {
	InputTokens              int
	OutputTokens             int
	TotalTokens              int
	CacheCreationInputTokens int
	CacheReadInputTokens     int
}

// ExecuteTurn sends a prompt and waits for the response.
// It detects completion markers in the response.
func (a *SessionAdapter) ExecuteTurn(ctx context.Context, prompt string) (*TurnResult, error) {
	start := time.Now()

	// Send the prompt
	msg := session.NewUserMessage(prompt)
	if err := a.session.Send(ctx, msg); err != nil {
		return nil, fmt.Errorf("send message: %w", err)
	}

	// Collect response
	var content strings.Builder
	var result *session.ResultMessage

	for output := range a.session.Output() {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		if output.IsAssistant() {
			content.WriteString(output.GetText())
		}

		if output.IsResult() {
			result = output.Result
			break
		}
	}

	// Build turn result
	turnResult := &TurnResult{
		Content:  content.String(),
		Duration: time.Since(start),
	}

	// Check completion status
	turnResult.Status, turnResult.Reason = CheckPhaseCompletion(turnResult.Content)

	// Extract metadata from result message
	if result != nil {
		turnResult.NumTurns = result.NumTurns
		turnResult.CostUSD = result.TotalCostUSD
		turnResult.Usage = TokenUsage{
			InputTokens:              result.Usage.InputTokens,
			OutputTokens:             result.Usage.OutputTokens,
			TotalTokens:              result.Usage.InputTokens + result.Usage.OutputTokens,
			CacheCreationInputTokens: result.Usage.CacheCreationInputTokens,
			CacheReadInputTokens:     result.Usage.CacheReadInputTokens,
		}

		if result.IsError {
			turnResult.IsError = true
			turnResult.ErrorText = result.Result
		}
	}

	return turnResult, nil
}

// StreamTurn sends a prompt and streams the response with a callback.
// The callback receives each chunk and can return false to stop early.
func (a *SessionAdapter) StreamTurn(ctx context.Context, prompt string, callback func(chunk string)) (*TurnResult, error) {
	start := time.Now()

	// Send the prompt
	msg := session.NewUserMessage(prompt)
	if err := a.session.Send(ctx, msg); err != nil {
		return nil, fmt.Errorf("send message: %w", err)
	}

	// Collect response with streaming callback
	var content strings.Builder
	var result *session.ResultMessage

	for output := range a.session.Output() {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		if output.IsAssistant() {
			text := output.GetText()
			content.WriteString(text)
			if callback != nil {
				callback(text)
			}
		}

		if output.IsResult() {
			result = output.Result
			break
		}
	}

	// Build turn result
	turnResult := &TurnResult{
		Content:  content.String(),
		Duration: time.Since(start),
	}

	turnResult.Status, turnResult.Reason = CheckPhaseCompletion(turnResult.Content)

	if result != nil {
		turnResult.NumTurns = result.NumTurns
		turnResult.CostUSD = result.TotalCostUSD
		turnResult.Usage = TokenUsage{
			InputTokens:              result.Usage.InputTokens,
			OutputTokens:             result.Usage.OutputTokens,
			TotalTokens:              result.Usage.InputTokens + result.Usage.OutputTokens,
			CacheCreationInputTokens: result.Usage.CacheCreationInputTokens,
			CacheReadInputTokens:     result.Usage.CacheReadInputTokens,
		}

		if result.IsError {
			turnResult.IsError = true
			turnResult.ErrorText = result.Result
		}
	}

	return turnResult, nil
}

// Close closes the session if this adapter owns it.
func (a *SessionAdapter) Close() error {
	if a.owns && a.session != nil {
		return a.session.Close()
	}
	return nil
}

// ToClient returns a SessionClient that implements claude.Client.
// This allows using the session with code that expects the Client interface.
func (a *SessionAdapter) ToClient() *claude.SessionClient {
	return claude.NewSessionClient(a.session)
}
