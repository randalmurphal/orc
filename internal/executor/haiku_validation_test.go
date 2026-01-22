package executor

import (
	"context"
	"encoding/json"
	"strings"
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

func TestValidateTaskReadiness_JSONParsing(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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

	t.Run("empty content returns error", func(t *testing.T) {
		client := &mockValidationClient{response: ""}
		_, _, err := ValidateTaskReadiness(
			context.Background(),
			client,
			"desc",
			"spec",
			"medium",
		)
		if err == nil {
			t.Error("expected error for empty content")
		}
		if !strings.Contains(err.Error(), "empty response content") {
			t.Errorf("error should mention empty response content, got: %v", err)
		}
	})
}

func TestValidateSuccessCriteria_JSONParsing(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		response       criteriaCompletionResponse
		wantAllMet     bool
		wantCritCount  int
		wantMissing    string
	}{
		{
			name: "all criteria met",
			response: criteriaCompletionResponse{
				AllMet: true,
				Criteria: []CriterionStatus{
					{ID: "SC-1", Status: "MET", Reason: "Implemented correctly"},
					{ID: "SC-2", Status: "MET", Reason: "Tests pass"},
				},
				MissingSummary: "",
			},
			wantAllMet:    true,
			wantCritCount: 2,
			wantMissing:   "",
		},
		{
			name: "some criteria not met",
			response: criteriaCompletionResponse{
				AllMet: false,
				Criteria: []CriterionStatus{
					{ID: "SC-1", Status: "MET", Reason: "Done"},
					{ID: "SC-2", Status: "NOT_MET", Reason: "Missing implementation"},
				},
				MissingSummary: "SC-2 is not implemented",
			},
			wantAllMet:    false,
			wantCritCount: 2,
			wantMissing:   "SC-2 is not implemented",
		},
		{
			name: "partial status",
			response: criteriaCompletionResponse{
				AllMet: false,
				Criteria: []CriterionStatus{
					{ID: "SC-1", Status: "PARTIAL", Reason: "Partially done"},
				},
				MissingSummary: "SC-1 needs more work",
			},
			wantAllMet:    false,
			wantCritCount: 1,
			wantMissing:   "SC-1 needs more work",
		},
		{
			name: "lowercase status normalized",
			response: criteriaCompletionResponse{
				AllMet: true,
				Criteria: []CriterionStatus{
					{ID: "SC-1", Status: "met", Reason: "done"},
				},
				MissingSummary: "",
			},
			wantAllMet:    true,
			wantCritCount: 1,
			wantMissing:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonBytes, _ := json.Marshal(tt.response)
			client := &mockValidationClient{response: string(jsonBytes)}

			result, err := ValidateSuccessCriteria(
				context.Background(),
				client,
				"spec with SC-1 and SC-2",
				"implementation summary",
			)

			if err != nil {
				t.Fatalf("ValidateSuccessCriteria() error = %v", err)
			}
			if result.AllMet != tt.wantAllMet {
				t.Errorf("AllMet = %v, want %v", result.AllMet, tt.wantAllMet)
			}
			if len(result.Criteria) != tt.wantCritCount {
				t.Errorf("criteria count = %d, want %d", len(result.Criteria), tt.wantCritCount)
			}
			if result.MissingSummary != tt.wantMissing {
				t.Errorf("MissingSummary = %q, want %q", result.MissingSummary, tt.wantMissing)
			}
		})
	}
}

func TestValidateSuccessCriteria_EdgeCases(t *testing.T) {
	t.Parallel()
	t.Run("nil client returns AllMet=true", func(t *testing.T) {
		result, err := ValidateSuccessCriteria(
			context.Background(),
			nil,
			"spec",
			"impl",
		)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !result.AllMet {
			t.Error("expected AllMet=true for nil client")
		}
	})

	t.Run("empty spec returns AllMet=true", func(t *testing.T) {
		client := &mockValidationClient{response: "{}"}
		result, err := ValidateSuccessCriteria(
			context.Background(),
			client,
			"",
			"impl",
		)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !result.AllMet {
			t.Error("expected AllMet=true for empty spec")
		}
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		client := &mockValidationClient{response: "not json"}
		_, err := ValidateSuccessCriteria(
			context.Background(),
			client,
			"spec",
			"impl",
		)
		if err == nil {
			t.Error("expected error for invalid JSON")
		}
	})

	t.Run("empty content returns error", func(t *testing.T) {
		client := &mockValidationClient{response: ""}
		_, err := ValidateSuccessCriteria(
			context.Background(),
			client,
			"spec with criteria",
			"impl",
		)
		if err == nil {
			t.Error("expected error for empty content")
		}
		if !strings.Contains(err.Error(), "empty response content") {
			t.Errorf("error should mention empty response content, got: %v", err)
		}
	})
}

func TestFormatCriteriaFeedback(t *testing.T) {
	t.Parallel()
	t.Run("nil result returns empty string", func(t *testing.T) {
		got := FormatCriteriaFeedback(nil)
		if got != "" {
			t.Errorf("expected empty string, got %q", got)
		}
	})

	t.Run("all met returns empty string", func(t *testing.T) {
		result := &CriteriaValidationResult{AllMet: true}
		got := FormatCriteriaFeedback(result)
		if got != "" {
			t.Errorf("expected empty string for AllMet=true, got %q", got)
		}
	})

	t.Run("formats missing criteria", func(t *testing.T) {
		result := &CriteriaValidationResult{
			AllMet: false,
			Criteria: []CriterionStatus{
				{ID: "SC-1", Status: "MET", Reason: "Done"},
				{ID: "SC-2", Status: "NOT_MET", Reason: "Missing tests"},
				{ID: "SC-3", Status: "PARTIAL", Reason: "Incomplete"},
			},
			MissingSummary: "2 criteria need work",
		}
		got := FormatCriteriaFeedback(result)

		// Should contain header
		if !strings.Contains(got, "Criteria Validation Failed") {
			t.Error("expected header in output")
		}
		// Should contain summary
		if !strings.Contains(got, "2 criteria need work") {
			t.Error("expected summary in output")
		}
		// Should list NOT_MET criteria
		if !strings.Contains(got, "SC-2") || !strings.Contains(got, "NOT_MET") {
			t.Error("expected NOT_MET criterion in output")
		}
		// Should list PARTIAL criteria
		if !strings.Contains(got, "SC-3") || !strings.Contains(got, "PARTIAL") {
			t.Error("expected PARTIAL criterion in output")
		}
	})

	t.Run("includes description when present", func(t *testing.T) {
		result := &CriteriaValidationResult{
			AllMet: false,
			Criteria: []CriterionStatus{
				{ID: "SC-1", Description: "User can log in", Status: "NOT_MET", Reason: "No login form"},
			},
			MissingSummary: "Login not implemented",
		}
		got := FormatCriteriaFeedback(result)

		if !strings.Contains(got, "User can log in") {
			t.Error("expected description in output")
		}
	})
}
