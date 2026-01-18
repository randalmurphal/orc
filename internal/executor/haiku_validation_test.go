package executor

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/randalmurphal/llmkit/claude"
)

// mockValidationClient implements claude.Client for testing.
type mockValidationClient struct {
	response string
	err      error
}

func (m *mockValidationClient) Complete(_ context.Context, _ claude.CompletionRequest) (*claude.CompletionResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &claude.CompletionResponse{Content: m.response}, nil
}

func (m *mockValidationClient) Stream(_ context.Context, _ claude.CompletionRequest) (<-chan claude.StreamChunk, error) {
	return nil, nil
}

func TestValidateIterationProgress_JSONParsing(t *testing.T) {
	tests := []struct {
		name         string
		response     progressResponse
		wantDecision ValidationDecision
		wantReason   string
	}{
		{
			name:         "continue decision",
			response:     progressResponse{Decision: "CONTINUE", Reason: "Making good progress"},
			wantDecision: ValidationContinue,
			wantReason:   "Making good progress",
		},
		{
			name:         "retry decision",
			response:     progressResponse{Decision: "RETRY", Reason: "Wrong approach"},
			wantDecision: ValidationRetry,
			wantReason:   "Wrong approach",
		},
		{
			name:         "stop decision",
			response:     progressResponse{Decision: "STOP", Reason: "Blocked by dependency"},
			wantDecision: ValidationStop,
			wantReason:   "Blocked by dependency",
		},
		{
			name:         "lowercase decision normalized",
			response:     progressResponse{Decision: "continue", Reason: "All good"},
			wantDecision: ValidationContinue,
			wantReason:   "All good",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonBytes, _ := json.Marshal(tt.response)
			client := &mockValidationClient{response: string(jsonBytes)}

			decision, reason, err := ValidateIterationProgress(
				context.Background(),
				client,
				"spec content",
				"iteration output",
			)

			if err != nil {
				t.Fatalf("ValidateIterationProgress() error = %v", err)
			}
			if decision != tt.wantDecision {
				t.Errorf("decision = %v, want %v", decision, tt.wantDecision)
			}
			if reason != tt.wantReason {
				t.Errorf("reason = %q, want %q", reason, tt.wantReason)
			}
		})
	}
}

func TestValidateIterationProgress_EdgeCases(t *testing.T) {
	t.Run("nil client returns continue", func(t *testing.T) {
		decision, reason, err := ValidateIterationProgress(
			context.Background(),
			nil,
			"spec",
			"output",
		)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if decision != ValidationContinue {
			t.Errorf("decision = %v, want ValidationContinue", decision)
		}
		if reason != "" {
			t.Errorf("reason = %q, want empty", reason)
		}
	})

	t.Run("empty spec returns continue", func(t *testing.T) {
		client := &mockValidationClient{response: "should not be called"}
		decision, _, err := ValidateIterationProgress(
			context.Background(),
			client,
			"", // empty spec
			"output",
		)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if decision != ValidationContinue {
			t.Errorf("decision = %v, want ValidationContinue", decision)
		}
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		client := &mockValidationClient{response: "not json"}
		_, _, err := ValidateIterationProgress(
			context.Background(),
			client,
			"spec",
			"output",
		)
		if err == nil {
			t.Error("expected error for invalid JSON")
		}
	})
}

func TestValidateTaskReadiness_JSONParsing(t *testing.T) {
	tests := []struct {
		name          string
		response      readinessResponse
		wantReady     bool
		wantSuggCount int
	}{
		{
			name:          "ready with empty suggestions",
			response:      readinessResponse{Ready: true, Suggestions: []string{}},
			wantReady:     true,
			wantSuggCount: 0,
		},
		{
			name:          "not ready with suggestions",
			response:      readinessResponse{Ready: false, Suggestions: []string{"Add success criteria", "Define testing"}},
			wantReady:     false,
			wantSuggCount: 2,
		},
		{
			name:          "ready with nil suggestions",
			response:      readinessResponse{Ready: true, Suggestions: nil},
			wantReady:     true,
			wantSuggCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonBytes, _ := json.Marshal(tt.response)
			client := &mockValidationClient{response: string(jsonBytes)}

			ready, suggestions, err := ValidateTaskReadiness(
				context.Background(),
				client,
				"task description",
				"spec content",
				"medium", // weight that triggers validation
			)

			if err != nil {
				t.Fatalf("ValidateTaskReadiness() error = %v", err)
			}
			if ready != tt.wantReady {
				t.Errorf("ready = %v, want %v", ready, tt.wantReady)
			}
			if len(suggestions) != tt.wantSuggCount {
				t.Errorf("suggestion count = %d, want %d", len(suggestions), tt.wantSuggCount)
			}
		})
	}
}

func TestValidateTaskReadiness_EdgeCases(t *testing.T) {
	t.Run("nil client returns ready", func(t *testing.T) {
		ready, suggestions, err := ValidateTaskReadiness(
			context.Background(),
			nil,
			"desc",
			"spec",
			"medium",
		)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !ready {
			t.Error("expected ready=true for nil client")
		}
		if len(suggestions) != 0 {
			t.Errorf("suggestions = %v, want empty", suggestions)
		}
	})

	t.Run("trivial weight skips validation", func(t *testing.T) {
		client := &mockValidationClient{response: "should not be called"}
		ready, _, err := ValidateTaskReadiness(
			context.Background(),
			client,
			"desc",
			"spec",
			"trivial",
		)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !ready {
			t.Error("expected ready=true for trivial weight")
		}
	})

	t.Run("small weight skips validation", func(t *testing.T) {
		client := &mockValidationClient{response: "should not be called"}
		ready, _, err := ValidateTaskReadiness(
			context.Background(),
			client,
			"desc",
			"spec",
			"small",
		)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !ready {
			t.Error("expected ready=true for small weight")
		}
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		client := &mockValidationClient{response: "not json"}
		_, _, err := ValidateTaskReadiness(
			context.Background(),
			client,
			"desc",
			"spec",
			"large",
		)
		if err == nil {
			t.Error("expected error for invalid JSON")
		}
	})
}

func TestValidationDecision_String(t *testing.T) {
	tests := []struct {
		decision ValidationDecision
		want     string
	}{
		{ValidationContinue, "continue"},
		{ValidationRetry, "retry"},
		{ValidationStop, "stop"},
		{ValidationDecision(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.decision.String(); got != tt.want {
				t.Errorf("ValidationDecision.String() = %v, want %v", got, tt.want)
			}
		})
	}
}
