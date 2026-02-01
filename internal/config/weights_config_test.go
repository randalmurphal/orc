package config

import "testing"

// TestWeightsConfig_GetWorkflowID_CustomOverride verifies that custom weight→workflow
// mappings override the defaults. When weights config has small="my-custom-small",
// GetWorkflowID("small") returns "my-custom-small" not "implement-small".
func TestWeightsConfig_GetWorkflowID_CustomOverride(t *testing.T) {
	t.Parallel()

	cfg := WeightsConfig{
		Small: "my-custom-small",
		// Leave others at defaults (empty → fallback)
	}

	tests := []struct {
		name   string
		weight string
		want   string
	}{
		{"custom small", "small", "my-custom-small"},
		{"default medium", "medium", "implement-medium"},
		{"default trivial", "trivial", "implement-trivial"},
		{"default large", "large", "implement-large"},
		{"unknown weight", "unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := cfg.GetWorkflowID(tt.weight)
			if got != tt.want {
				t.Errorf("GetWorkflowID(%q) = %q, want %q", tt.weight, got, tt.want)
			}
		})
	}
}

// TestWeightsConfig_GetWorkflowID_AllCustom verifies all four weights can be
// overridden simultaneously.
func TestWeightsConfig_GetWorkflowID_AllCustom(t *testing.T) {
	t.Parallel()

	cfg := WeightsConfig{
		Trivial: "fast-fix",
		Small:   "quick-impl",
		Medium:  "standard-dev",
		Large:   "full-cycle",
	}

	tests := []struct {
		weight string
		want   string
	}{
		{"trivial", "fast-fix"},
		{"small", "quick-impl"},
		{"medium", "standard-dev"},
		{"large", "full-cycle"},
	}

	for _, tt := range tests {
		t.Run(tt.weight, func(t *testing.T) {
			t.Parallel()
			got := cfg.GetWorkflowID(tt.weight)
			if got != tt.want {
				t.Errorf("GetWorkflowID(%q) = %q, want %q", tt.weight, got, tt.want)
			}
		})
	}
}
