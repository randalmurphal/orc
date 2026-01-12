// Package executor provides the flowgraph-based execution engine for orc.
package executor

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/internal/tokenpool"
)

// Sentinel errors for recovery decisions
var (
	ErrRateLimited    = errors.New("rate limited by API")
	ErrNetworkFailure = errors.New("network failure")
	ErrTimeout        = errors.New("execution timeout")
	ErrMaxRetries     = errors.New("max retries exceeded")
)

// RetryConfig controls retry behavior
type RetryConfig struct {
	MaxRetries     int           // Max attempts before giving up
	InitialBackoff time.Duration // Starting backoff duration
	MaxBackoff     time.Duration // Maximum backoff duration
	BackoffFactor  float64       // Multiplier for each retry
}

// DefaultRetryConfig returns sensible defaults
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 2 * time.Second,
		MaxBackoff:     60 * time.Second,
		BackoffFactor:  2.0,
	}
}

// ExecuteWithRetry wraps phase execution with retry logic
func (e *Executor) ExecuteWithRetry(ctx context.Context, t *task.Task, p *plan.Phase, s *state.State) (*Result, error) {
	cfg := DefaultRetryConfig()
	backoff := cfg.InitialBackoff

	var lastErr error
	var result *Result

	for attempt := 1; attempt <= cfg.MaxRetries; attempt++ {
		result, lastErr = e.ExecutePhase(ctx, t, p, s)
		if lastErr == nil {
			return result, nil
		}

		// Check if error is retryable
		if !isRetryable(lastErr) {
			return result, lastErr
		}

		// If rate limited and token pool is available, try switching accounts
		if IsRateLimitError(lastErr) && e.tokenPool != nil && e.tokenPool.SwitchOnRateLimit() {
			e.MarkCurrentAccountExhausted(lastErr.Error())
			if switchErr := e.SwitchToNextAccount(); switchErr != nil {
				e.logger.Warn("failed to switch to next account",
					"error", switchErr,
					"original_error", lastErr,
				)
				// If all accounts exhausted, don't retry - fail immediately
				if errors.Is(switchErr, tokenpool.ErrAllExhausted) {
					return result, fmt.Errorf("all accounts exhausted: %w", lastErr)
				}
			} else {
				e.logger.Info("switched account due to rate limit, retrying immediately")
				// Reset backoff since we have a fresh account
				backoff = cfg.InitialBackoff
			}
		}

		e.logger.Warn("phase execution failed, retrying",
			"phase", p.ID,
			"attempt", attempt,
			"max_attempts", cfg.MaxRetries,
			"backoff", backoff,
			"error", lastErr,
		)

		// Save recovery state before retry
		if s != nil {
			s.Error = lastErr.Error()
			taskDir := filepath.Join(e.config.WorkDir, task.OrcDir, task.TasksDir, s.TaskID)
			if saveErr := s.SaveTo(taskDir); saveErr != nil {
				e.logger.Error("failed to save recovery state", "error", saveErr)
			}
		}

		select {
		case <-ctx.Done():
			return result, ctx.Err()
		case <-time.After(backoff):
		}

		// Exponential backoff
		backoff = time.Duration(float64(backoff) * cfg.BackoffFactor)
		if backoff > cfg.MaxBackoff {
			backoff = cfg.MaxBackoff
		}
	}

	return result, ErrMaxRetries
}

// isRetryable determines if an error should trigger a retry
func isRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Check sentinel errors
	if errors.Is(err, ErrRateLimited) ||
		errors.Is(err, ErrNetworkFailure) ||
		errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	// Check error message patterns
	errStr := strings.ToLower(err.Error())
	retryablePatterns := []string{
		"connection refused",
		"connection reset",
		"rate limit",
		"usage limit",
		"timeout",
		"temporary failure",
		"service unavailable",
		"too many requests",
		"429",
		"503",
		"504",
	}

	for _, pattern := range retryablePatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}

	return false
}

// ClassifyError converts generic errors to typed errors for better handling
func ClassifyError(err error) error {
	if err == nil {
		return nil
	}

	errStr := strings.ToLower(err.Error())

	if strings.Contains(errStr, "rate limit") || strings.Contains(errStr, "429") || strings.Contains(errStr, "too many requests") {
		return ErrRateLimited
	}

	if strings.Contains(errStr, "connection") || strings.Contains(errStr, "network") {
		return ErrNetworkFailure
	}

	if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline") {
		return ErrTimeout
	}

	return err
}

// IsRateLimitError checks if an error indicates a rate limit has been hit.
// This is used by the token pool to decide when to switch accounts.
func IsRateLimitError(err error) bool {
	if err == nil {
		return false
	}

	// Check sentinel error
	if errors.Is(err, ErrRateLimited) {
		return true
	}

	errStr := strings.ToLower(err.Error())
	rateLimitPatterns := []string{
		"rate limit",
		"usage limit",
		"too many requests",
		"429",
		"limit reached",
		"limit exceeded",
		"quota exceeded",
	}

	for _, pattern := range rateLimitPatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}

	return false
}
