package executor

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/randalmurphal/llmkit/claude"
	"github.com/randalmurphal/llmkit/claude/session"
)

// SessionAdapter wraps a llmkit session for phase execution.
// It provides methods for sending prompts and collecting responses
// with JSON-based completion detection.
type SessionAdapter struct {
	session session.Session
	manager session.SessionManager
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
				owns:    false,
			}, nil
		}
	}

	// Build session options
	var sessionOpts []session.SessionOption

	if opts.SessionID != "" {
		if opts.Resume {
			sessionOpts = append(sessionOpts, session.WithResume(opts.SessionID))
		} else if opts.Persistence {
			// Only use custom session ID when persistence is enabled.
			// Claude CLI expects session IDs to be UUIDs it generates.
			// For ephemeral sessions (Persistence: false), we skip the custom ID
			// to let Claude generate a valid UUID, avoiding "Invalid session ID" errors.
			sessionOpts = append(sessionOpts, session.WithSessionID(opts.SessionID))
		}
		// When Persistence is false and Resume is false, skip WithSessionID entirely.
		// The session is ephemeral and won't be persisted or resumed anyway.
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

	// Note: Thinking mode is NOT enabled via --append-system-prompt because
	// Claude Code's thinking triggers (ultrathink) only work in user messages.
	// Instead, inject "ultrathink" into the prompt text in the executor.

	// Create session
	s, err := mgr.Create(ctx, sessionOpts...)
	if err != nil {
		// If resume failed due to expired/invalid session, try fresh session
		if opts.Resume && isSessionExpiredError(err) {
			// Rebuild session options without resume flag
			var freshOpts []session.SessionOption
			if opts.Model != "" {
				freshOpts = append(freshOpts, session.WithModel(opts.Model))
			}
			if opts.Workdir != "" {
				freshOpts = append(freshOpts, session.WithWorkdir(opts.Workdir))
			}
			if opts.MaxTurns > 0 {
				freshOpts = append(freshOpts, session.WithMaxTurns(opts.MaxTurns))
			}
			if !opts.Persistence {
				freshOpts = append(freshOpts, session.WithNoSessionPersistence())
			}

			s, err = mgr.Create(ctx, freshOpts...)
			if err != nil {
				return nil, fmt.Errorf("create fresh session after resume failed: %w", err)
			}
			// Log that we fell back to fresh session
			// (caller will see fresh session ID instead of resumed one)
		} else {
			return nil, fmt.Errorf("create session: %w", err)
		}
	}

	return &SessionAdapter{
		session: s,
		manager: mgr,
		owns:    true,
	}, nil
}

// isSessionExpiredError checks if the error indicates a session that no longer exists.
func isSessionExpiredError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "session not found") ||
		strings.Contains(errStr, "invalid session") ||
		strings.Contains(errStr, "Session not found") ||
		strings.Contains(errStr, "Invalid session")
}

// SessionID returns the session identifier.
func (a *SessionAdapter) SessionID() string {
	return a.session.ID()
}

// JSONLPath returns the path to Claude's session JSONL file.
// Returns empty string if session has no persistent storage.
func (a *SessionAdapter) JSONLPath() string {
	return a.session.JSONLPath()
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

// EffectiveInputTokens returns the total input context size including cached tokens.
// This represents the actual prompt size that Claude processed.
func (u TokenUsage) EffectiveInputTokens() int {
	return u.InputTokens + u.CacheCreationInputTokens + u.CacheReadInputTokens
}

// EffectiveTotalTokens returns the total tokens including cached inputs.
func (u TokenUsage) EffectiveTotalTokens() int {
	return u.EffectiveInputTokens() + u.OutputTokens
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

	// Set up idle timeout (default 2 minutes) for workaround of Claude CLI bug #1920
	const idleTimeout = 2 * time.Minute
	idleTicker := time.NewTicker(idleTimeout / 4)
	defer idleTicker.Stop()
	lastActivity := time.Now()

	outputCh := a.session.Output()
collectLoop:
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-idleTicker.C:
			idleDuration := time.Since(lastActivity)
			// WORKAROUND for Claude Code CLI bug #1920: Missing result message
			// If we have accumulated content that indicates phase completion or blocking,
			// and we've been idle for longer than 2x the timeout, assume the turn is complete.
			// See: https://github.com/anthropics/claude-code/issues/1920
			accumulated := content.String()
			if accumulated != "" && idleDuration > idleTimeout*2 {
				if HasJSONCompletion(accumulated) {
					break collectLoop
				}
			}
		case output, ok := <-outputCh:
			if !ok {
				// Channel closed without result
				break collectLoop
			}

			lastActivity = time.Now()

			if output.IsAssistant() {
				content.WriteString(output.GetText())

				// WORKAROUND for Claude Code CLI bug #1920: Missing result message
				// Check for phase completion markers immediately - don't wait for result message.
				// See: https://github.com/anthropics/claude-code/issues/1920
				accumulated := content.String()
				if HasJSONCompletion(accumulated) {
					break collectLoop
				}
			}

			if output.IsResult() {
				result = output.Result
				break collectLoop
			}
		}
	}

	// Build turn result
	turnResult := &TurnResult{
		Content:  content.String(),
		Duration: time.Since(start),
	}

	// Check completion status
	turnResult.Status, turnResult.Reason = CheckPhaseCompletionJSON(turnResult.Content)

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

	outputCh := a.session.Output()
streamLoop:
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case output, ok := <-outputCh:
			if !ok {
				// Channel closed without result
				break streamLoop
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
				break streamLoop
			}
		}
	}

	// Build turn result
	turnResult := &TurnResult{
		Content:  content.String(),
		Duration: time.Since(start),
	}

	turnResult.Status, turnResult.Reason = CheckPhaseCompletionJSON(turnResult.Content)

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

// StreamTurnWithProgress sends a prompt and streams the response with activity tracking.
// It provides progress heartbeats and handles turn timeouts gracefully.
func (a *SessionAdapter) StreamTurnWithProgress(ctx context.Context, prompt string, opts StreamProgressOptions) (*TurnResult, error) {
	start := time.Now()

	// Create a cancellable context for turn timeout
	turnCtx := ctx
	var turnCancel context.CancelFunc
	if opts.TurnTimeout > 0 {
		turnCtx, turnCancel = context.WithTimeout(ctx, opts.TurnTimeout)
		defer turnCancel()
	}

	// Notify start of API call
	if opts.OnActivityChange != nil {
		opts.OnActivityChange(ActivityWaitingAPI)
	}

	// Send the prompt
	msg := session.NewUserMessage(prompt)
	if err := a.session.Send(turnCtx, msg); err != nil {
		if turnCtx.Err() == context.DeadlineExceeded && ctx.Err() == nil {
			// Turn timeout, not parent context cancellation
			if opts.OnTurnTimeout != nil {
				opts.OnTurnTimeout(time.Since(start))
			}
			return nil, fmt.Errorf("turn timeout after %s: %w", time.Since(start), turnCtx.Err())
		}
		return nil, fmt.Errorf("send message: %w", err)
	}

	// Set up heartbeat ticker if configured
	var heartbeatTicker *time.Ticker
	var heartbeatCh <-chan time.Time
	if opts.HeartbeatInterval > 0 {
		heartbeatTicker = time.NewTicker(opts.HeartbeatInterval)
		heartbeatCh = heartbeatTicker.C
		defer heartbeatTicker.Stop()
	}

	// Set up idle timeout ticker
	var idleTicker *time.Ticker
	var idleCh <-chan time.Time
	if opts.IdleTimeout > 0 {
		idleTicker = time.NewTicker(opts.IdleTimeout / 4) // Check more frequently than timeout
		idleCh = idleTicker.C
		defer idleTicker.Stop()
	}

	// Collect response with streaming callback
	var content strings.Builder
	var result *session.ResultMessage
	lastActivity := time.Now()
	hasWarned := false

	outputCh := a.session.Output()
streamLoop:
	for {
		select {
		case <-turnCtx.Done():
			if turnCtx.Err() == context.DeadlineExceeded && ctx.Err() == nil {
				// Turn timeout
				if opts.OnTurnTimeout != nil {
					opts.OnTurnTimeout(time.Since(start))
				}
				// Return partial result on timeout
				return &TurnResult{
					Content:   content.String(),
					Duration:  time.Since(start),
					Status:    PhaseStatusContinue,
					IsError:   true,
					ErrorText: fmt.Sprintf("turn timeout after %s", time.Since(start)),
				}, fmt.Errorf("turn timeout after %s", time.Since(start))
			}
			return nil, turnCtx.Err()

		case <-heartbeatCh:
			if opts.OnHeartbeat != nil {
				opts.OnHeartbeat()
			}

		case <-idleCh:
			idleDuration := time.Since(lastActivity)
			if idleDuration > opts.IdleTimeout {
				if !hasWarned {
					if opts.OnIdleWarning != nil {
						opts.OnIdleWarning(idleDuration)
					}
					hasWarned = true
				}

				// WORKAROUND for Claude Code CLI bug #1920: Missing result message
				// If we have accumulated content that indicates phase completion or blocking,
				// and we've been idle for longer than 2x the timeout, assume the turn is complete.
				// This prevents indefinite hangs when Claude CLI fails to send the result event.
				// See: https://github.com/anthropics/claude-code/issues/1920
				accumulated := content.String()
				if accumulated != "" && idleDuration > opts.IdleTimeout*2 {
					if HasJSONCompletion(accumulated) {
						break streamLoop
					}
				}
			}

		case output, ok := <-outputCh:
			if !ok {
				// Channel closed without result
				break streamLoop
			}

			lastActivity = time.Now()
			hasWarned = false // Reset warning on activity

			if output.IsAssistant() {
				text := output.GetText()
				content.WriteString(text)

				// WORKAROUND for Claude Code CLI bug #1920: Missing result message
				// Check for phase completion markers immediately - don't wait for result message.
				// When we see <phase_complete> or <phase_blocked>, the agent is done and won't
				// send more content, so we can safely exit even without a result message.
				// See: https://github.com/anthropics/claude-code/issues/1920
				accumulated := content.String()
				if HasJSONCompletion(accumulated) {
					break streamLoop
				}

				// Update activity state on first chunk
				if opts.OnActivityChange != nil && content.Len() == len(text) {
					opts.OnActivityChange(ActivityStreaming)
				}

				if opts.OnChunk != nil {
					opts.OnChunk(text)
				}
			}

			if output.IsResult() {
				result = output.Result
				break streamLoop
			}
		}
	}

	// Notify completion
	if opts.OnActivityChange != nil {
		opts.OnActivityChange(ActivityProcessing)
	}

	// Build turn result
	turnResult := &TurnResult{
		Content:  content.String(),
		Duration: time.Since(start),
	}

	turnResult.Status, turnResult.Reason = CheckPhaseCompletionJSON(turnResult.Content)

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

// StreamProgressOptions configures progress tracking for streaming.
type StreamProgressOptions struct {
	// TurnTimeout is the maximum duration for this turn (0 = no timeout)
	TurnTimeout time.Duration
	// HeartbeatInterval is how often to emit heartbeat callbacks (0 = disabled)
	HeartbeatInterval time.Duration
	// IdleTimeout is how long without activity before warning (0 = disabled)
	IdleTimeout time.Duration

	// Callbacks
	OnChunk          func(chunk string)
	OnActivityChange func(state ActivityState)
	OnHeartbeat      func()
	OnIdleWarning    func(idleDuration time.Duration)
	OnTurnTimeout    func(turnDuration time.Duration)
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
