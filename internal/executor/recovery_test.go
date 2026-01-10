package executor

import (
	"context"
	"errors"
	"testing"
)

func TestDefaultRetryConfig(t *testing.T) {
	cfg := DefaultRetryConfig()

	if cfg.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", cfg.MaxRetries)
	}

	if cfg.BackoffFactor != 2.0 {
		t.Errorf("BackoffFactor = %f, want 2.0", cfg.BackoffFactor)
	}
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		retryable bool
	}{
		{"nil error", nil, false},
		{"rate limited", ErrRateLimited, true},
		{"network failure", ErrNetworkFailure, true},
		{"timeout sentinel", ErrTimeout, true}, // Contains "timeout" in message, so retryable
		{"context deadline", context.DeadlineExceeded, true},
		{"connection refused", errors.New("connection refused"), true},
		{"rate limit message", errors.New("rate limit exceeded"), true},
		{"timeout message", errors.New("request timeout"), true},
		{"429 status", errors.New("HTTP 429 Too Many Requests"), true},
		{"503 status", errors.New("HTTP 503 Service Unavailable"), true},
		{"random error", errors.New("something went wrong"), false},
		{"validation error", errors.New("invalid input"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRetryable(tt.err); got != tt.retryable {
				t.Errorf("isRetryable(%v) = %v, want %v", tt.err, got, tt.retryable)
			}
		})
	}
}

func TestClassifyError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected error
	}{
		{"nil", nil, nil},
		{"rate limit", errors.New("rate limit exceeded"), ErrRateLimited},
		{"429", errors.New("HTTP 429"), ErrRateLimited},
		{"too many requests", errors.New("too many requests"), ErrRateLimited},
		{"connection error", errors.New("connection refused"), ErrNetworkFailure},
		{"network error", errors.New("network unreachable"), ErrNetworkFailure},
		{"timeout", errors.New("request timeout"), ErrTimeout},
		{"deadline", errors.New("deadline exceeded"), ErrTimeout},
		{"unknown", errors.New("something else"), errors.New("something else")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyError(tt.err)
			if tt.expected == nil {
				if got != nil {
					t.Errorf("ClassifyError() = %v, want nil", got)
				}
				return
			}

			// For sentinel errors, check equality
			if errors.Is(tt.expected, ErrRateLimited) || errors.Is(tt.expected, ErrNetworkFailure) || errors.Is(tt.expected, ErrTimeout) {
				if !errors.Is(got, tt.expected) {
					t.Errorf("ClassifyError() = %v, want %v", got, tt.expected)
				}
			}
		})
	}
}
