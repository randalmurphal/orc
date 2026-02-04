// Package workflow provides workflow-related utilities.
//
// TDD Tests for weight to workflow_id mapping via string-based API.
//
// These tests verify the WeightToWorkflowIDString function returns the correct
// workflow ID for each task weight string.
//
// Success Criteria Coverage:
// - SC-1: weight="small" → workflow_id="implement-small"
// - SC-2: weight="medium" → workflow_id="implement-medium"
//
// Edge Cases:
// - Empty weight returns empty string
// - Trivial weight returns "implement-trivial"
// - Large weight returns "implement-large"
// - Unknown/invalid weight returns empty string
// - Enum-style weight strings (TASK_WEIGHT_*) also work
package workflow

import (
	"testing"

	"github.com/randalmurphal/orc/internal/config"
)

// TestWeightToWorkflowIDString_Small verifies SC-1:
// weight="small" maps to "implement-small"
func TestWeightToWorkflowIDString_Small(t *testing.T) {
	t.Parallel()

	wfID := WeightToWorkflowIDString("small")

	if wfID != "implement-small" {
		t.Errorf("WeightToWorkflowIDString(\"small\") = %q, want %q", wfID, "implement-small")
	}
}

// TestWeightToWorkflowIDString_Medium verifies SC-2:
// weight="medium" maps to "implement-medium"
func TestWeightToWorkflowIDString_Medium(t *testing.T) {
	t.Parallel()

	wfID := WeightToWorkflowIDString("medium")

	if wfID != "implement-medium" {
		t.Errorf("WeightToWorkflowIDString(\"medium\") = %q, want %q", wfID, "implement-medium")
	}
}

// TestWeightToWorkflowIDString_Trivial verifies trivial weight mapping
func TestWeightToWorkflowIDString_Trivial(t *testing.T) {
	t.Parallel()

	wfID := WeightToWorkflowIDString("trivial")

	if wfID != "implement-trivial" {
		t.Errorf("WeightToWorkflowIDString(\"trivial\") = %q, want %q", wfID, "implement-trivial")
	}
}

// TestWeightToWorkflowIDString_Large verifies large weight mapping
func TestWeightToWorkflowIDString_Large(t *testing.T) {
	t.Parallel()

	wfID := WeightToWorkflowIDString("large")

	if wfID != "implement-large" {
		t.Errorf("WeightToWorkflowIDString(\"large\") = %q, want %q", wfID, "implement-large")
	}
}

// TestWeightToWorkflowIDString_Empty verifies edge case:
// Empty weight returns empty string (no auto-assignment)
func TestWeightToWorkflowIDString_Empty(t *testing.T) {
	t.Parallel()

	wfID := WeightToWorkflowIDString("")

	if wfID != "" {
		t.Errorf("WeightToWorkflowIDString(\"\") = %q, want empty string", wfID)
	}
}

// TestWeightToWorkflowIDString_InvalidValue verifies edge case:
// Invalid/unknown weight values return empty string
func TestWeightToWorkflowIDString_InvalidValue(t *testing.T) {
	t.Parallel()

	wfID := WeightToWorkflowIDString("invalid")

	if wfID != "" {
		t.Errorf("WeightToWorkflowIDString(\"invalid\") = %q, want empty string", wfID)
	}
}

// TestWeightToWorkflowIDString_EnumStyleStrings verifies enum-style strings work
func TestWeightToWorkflowIDString_EnumStyleStrings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		weight string
		want   string
	}{
		{"TASK_WEIGHT_TRIVIAL", "implement-trivial"},
		{"TASK_WEIGHT_SMALL", "implement-small"},
		{"TASK_WEIGHT_MEDIUM", "implement-medium"},
		{"TASK_WEIGHT_LARGE", "implement-large"},
	}

	for _, tt := range tests {
		t.Run(tt.weight, func(t *testing.T) {
			t.Parallel()
			got := WeightToWorkflowIDString(tt.weight)
			if got != tt.want {
				t.Errorf("WeightToWorkflowIDString(%q) = %q, want %q", tt.weight, got, tt.want)
			}
		})
	}
}

// TestWeightToWorkflowIDString_AllWeights is a table-driven test covering all weight mappings
func TestWeightToWorkflowIDString_AllWeights(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		weight string
		want   string
	}{
		{
			name:   "empty returns empty",
			weight: "",
			want:   "",
		},
		{
			name:   "trivial maps to implement-trivial",
			weight: "trivial",
			want:   "implement-trivial",
		},
		{
			name:   "small maps to implement-small",
			weight: "small",
			want:   "implement-small",
		},
		{
			name:   "medium maps to implement-medium",
			weight: "medium",
			want:   "implement-medium",
		},
		{
			name:   "large maps to implement-large",
			weight: "large",
			want:   "implement-large",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := WeightToWorkflowIDString(tt.weight)
			if got != tt.want {
				t.Errorf("WeightToWorkflowIDString(%q) = %q, want %q", tt.weight, got, tt.want)
			}
		})
	}
}

// ============================================================================
// TASK-682: ResolveWorkflowID fallback logic
// ============================================================================

// TestResolveWorkflowID_Fallback verifies SC-2:
// When workflow_id is empty, ResolveWorkflowID falls back to config-based
// weight→workflow mapping instead of returning empty string.
func TestResolveWorkflowID_Fallback(t *testing.T) {
	t.Parallel()

	cfg := config.WeightsConfig{
		Small: "custom-small",
	}

	tests := []struct {
		name       string
		workflowID string
		weight     string
		want       string
	}{
		{
			name:       "existing workflow preserved",
			workflowID: "my-workflow",
			weight:     "small",
			want:       "my-workflow",
		},
		{
			name:       "empty falls back to config",
			workflowID: "",
			weight:     "small",
			want:       "custom-small",
		},
		{
			name:       "empty with medium falls back to default",
			workflowID: "",
			weight:     "medium",
			want:       "implement-medium",
		},
		{
			name:       "empty with empty weight returns empty",
			workflowID: "",
			weight:     "",
			want:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ResolveWorkflowIDFromString(tt.workflowID, tt.weight, cfg)
			if got != tt.want {
				t.Errorf("ResolveWorkflowIDFromString(%q, %q, cfg) = %q, want %q", tt.workflowID, tt.weight, got, tt.want)
			}
		})
	}
}
