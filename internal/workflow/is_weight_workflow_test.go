package workflow

import "testing"

func TestIsWeightBasedWorkflow(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		workflowID string
		want       bool
	}{
		{"implement-trivial is weight-based", "implement-trivial", true},
		{"implement-small is weight-based", "implement-small", true},
		{"implement-medium is weight-based", "implement-medium", true},
		{"implement-large is weight-based", "implement-large", true},
		{"qa-e2e is not weight-based", "qa-e2e", false},
		{"custom-workflow is not weight-based", "custom-workflow", false},
		{"empty string is not weight-based", "", false},
		{"review is not weight-based", "review", false},
		{"spec is not weight-based", "spec", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := IsWeightBasedWorkflow(tt.workflowID)
			if got != tt.want {
				t.Errorf("IsWeightBasedWorkflow(%q) = %v, want %v", tt.workflowID, got, tt.want)
			}
		})
	}
}
