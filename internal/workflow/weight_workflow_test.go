// Package workflow provides workflow-related utilities.
//
// TDD Tests for TASK-590: Weight to workflow_id auto-assignment
//
// These tests verify the WeightToWorkflowID function returns the correct
// workflow ID for each task weight.
//
// Success Criteria Coverage:
// - SC-1: weight=small → workflow_id="implement-small"
// - SC-2: weight=medium → workflow_id="implement-medium"
//
// Edge Cases:
// - Unspecified weight returns empty string
// - Trivial weight returns "implement-trivial"
// - Large weight returns "implement-large"
// - Unknown/invalid weight returns empty string
package workflow

import (
	"testing"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
)

// TestWeightToWorkflowID_Small verifies SC-1:
// TaskWeight_TASK_WEIGHT_SMALL maps to "implement-small"
func TestWeightToWorkflowID_Small(t *testing.T) {
	t.Parallel()

	wfID := WeightToWorkflowID(orcv1.TaskWeight_TASK_WEIGHT_SMALL)

	if wfID != "implement-small" {
		t.Errorf("WeightToWorkflowID(SMALL) = %q, want %q", wfID, "implement-small")
	}
}

// TestWeightToWorkflowID_Medium verifies SC-2:
// TaskWeight_TASK_WEIGHT_MEDIUM maps to "implement-medium"
func TestWeightToWorkflowID_Medium(t *testing.T) {
	t.Parallel()

	wfID := WeightToWorkflowID(orcv1.TaskWeight_TASK_WEIGHT_MEDIUM)

	if wfID != "implement-medium" {
		t.Errorf("WeightToWorkflowID(MEDIUM) = %q, want %q", wfID, "implement-medium")
	}
}

// TestWeightToWorkflowID_Trivial verifies trivial weight mapping
func TestWeightToWorkflowID_Trivial(t *testing.T) {
	t.Parallel()

	wfID := WeightToWorkflowID(orcv1.TaskWeight_TASK_WEIGHT_TRIVIAL)

	if wfID != "implement-trivial" {
		t.Errorf("WeightToWorkflowID(TRIVIAL) = %q, want %q", wfID, "implement-trivial")
	}
}

// TestWeightToWorkflowID_Large verifies large weight mapping
func TestWeightToWorkflowID_Large(t *testing.T) {
	t.Parallel()

	wfID := WeightToWorkflowID(orcv1.TaskWeight_TASK_WEIGHT_LARGE)

	if wfID != "implement-large" {
		t.Errorf("WeightToWorkflowID(LARGE) = %q, want %q", wfID, "implement-large")
	}
}

// TestWeightToWorkflowID_Unspecified verifies edge case:
// Unspecified weight returns empty string (no auto-assignment)
func TestWeightToWorkflowID_Unspecified(t *testing.T) {
	t.Parallel()

	wfID := WeightToWorkflowID(orcv1.TaskWeight_TASK_WEIGHT_UNSPECIFIED)

	if wfID != "" {
		t.Errorf("WeightToWorkflowID(UNSPECIFIED) = %q, want empty string", wfID)
	}
}

// TestWeightToWorkflowID_InvalidValue verifies edge case:
// Invalid/unknown weight values return empty string
func TestWeightToWorkflowID_InvalidValue(t *testing.T) {
	t.Parallel()

	// Test with an invalid enum value (outside defined range)
	invalidWeight := orcv1.TaskWeight(999)
	wfID := WeightToWorkflowID(invalidWeight)

	if wfID != "" {
		t.Errorf("WeightToWorkflowID(invalid) = %q, want empty string", wfID)
	}
}

// TestWeightToWorkflowID_AllWeights is a table-driven test covering all weight mappings
func TestWeightToWorkflowID_AllWeights(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		weight orcv1.TaskWeight
		want   string
	}{
		{
			name:   "unspecified returns empty",
			weight: orcv1.TaskWeight_TASK_WEIGHT_UNSPECIFIED,
			want:   "",
		},
		{
			name:   "trivial maps to implement-trivial",
			weight: orcv1.TaskWeight_TASK_WEIGHT_TRIVIAL,
			want:   "implement-trivial",
		},
		{
			name:   "small maps to implement-small",
			weight: orcv1.TaskWeight_TASK_WEIGHT_SMALL,
			want:   "implement-small",
		},
		{
			name:   "medium maps to implement-medium",
			weight: orcv1.TaskWeight_TASK_WEIGHT_MEDIUM,
			want:   "implement-medium",
		},
		{
			name:   "large maps to implement-large",
			weight: orcv1.TaskWeight_TASK_WEIGHT_LARGE,
			want:   "implement-large",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := WeightToWorkflowID(tt.weight)
			if got != tt.want {
				t.Errorf("WeightToWorkflowID(%v) = %q, want %q", tt.weight, got, tt.want)
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
		weight     orcv1.TaskWeight
		want       string
	}{
		{
			name:       "existing workflow preserved",
			workflowID: "my-workflow",
			weight:     orcv1.TaskWeight_TASK_WEIGHT_SMALL,
			want:       "my-workflow",
		},
		{
			name:       "empty falls back to config",
			workflowID: "",
			weight:     orcv1.TaskWeight_TASK_WEIGHT_SMALL,
			want:       "custom-small",
		},
		{
			name:       "empty with medium falls back to default",
			workflowID: "",
			weight:     orcv1.TaskWeight_TASK_WEIGHT_MEDIUM,
			want:       "implement-medium",
		},
		{
			name:       "empty with unspecified weight returns empty",
			workflowID: "",
			weight:     orcv1.TaskWeight_TASK_WEIGHT_UNSPECIFIED,
			want:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ResolveWorkflowID(tt.workflowID, tt.weight, cfg)
			if got != tt.want {
				t.Errorf("ResolveWorkflowID(%q, %v, cfg) = %q, want %q", tt.workflowID, tt.weight, got, tt.want)
			}
		})
	}
}
