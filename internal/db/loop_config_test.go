// Tests for TASK-687: LoopConfig parsing and EffectiveMaxLoops.
//
// These tests define the contract for the updated LoopConfig struct:
//   - Condition field changes from string to json.RawMessage
//   - New MaxLoops field alongside legacy MaxIterations
//   - EffectiveMaxLoops() resolver method with defined precedence
//
// Coverage mapping:
//   SC-4:  TestParseLoopConfig_NewJSONConditionFormat, TestParseLoopConfig_LegacyStringCondition
//   SC-7:  TestLoopConfig_EffectiveMaxLoops_*
//
// Failure modes:
//   TestParseLoopConfig_MalformedJSON
//   TestParseLoopConfig_EmptyInput
package db

import (
	"encoding/json"
	"testing"
)

// =============================================================================
// SC-4: LoopConfig parses new JSON condition format
// =============================================================================

func TestParseLoopConfig_NewJSONConditionFormat(t *testing.T) {
	t.Parallel()

	input := `{
		"loop_to_phase": "implement",
		"condition": {"field": "phase_output.review.status", "op": "eq", "value": "needs_changes"},
		"max_loops": 3
	}`

	cfg, err := ParseLoopConfig(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil LoopConfig")
	}

	if cfg.LoopToPhase != "implement" {
		t.Errorf("LoopToPhase = %q, want %q", cfg.LoopToPhase, "implement")
	}
	if cfg.MaxLoops != 3 {
		t.Errorf("MaxLoops = %d, want 3", cfg.MaxLoops)
	}

	// Verify Condition is valid JSON that can be unmarshaled
	if cfg.Condition == nil {
		t.Fatal("Condition should not be nil for JSON condition format")
	}

	// Verify it's a JSON object (not a string)
	var condMap map[string]interface{}
	if err := json.Unmarshal(cfg.Condition, &condMap); err != nil {
		t.Fatalf("Condition is not a valid JSON object: %v", err)
	}
	if condMap["field"] != "phase_output.review.status" {
		t.Errorf("condition field = %v, want %q", condMap["field"], "phase_output.review.status")
	}
	if condMap["op"] != "eq" {
		t.Errorf("condition op = %v, want %q", condMap["op"], "eq")
	}
}

// =============================================================================
// SC-4: LoopConfig parses legacy string condition (backward compat)
// =============================================================================

func TestParseLoopConfig_LegacyStringCondition(t *testing.T) {
	t.Parallel()

	input := `{
		"loop_to_phase": "qa_e2e_fix",
		"condition": "has_findings",
		"max_iterations": 3
	}`

	cfg, err := ParseLoopConfig(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil LoopConfig")
	}

	if cfg.LoopToPhase != "qa_e2e_fix" {
		t.Errorf("LoopToPhase = %q, want %q", cfg.LoopToPhase, "qa_e2e_fix")
	}
	if cfg.MaxIterations != 3 {
		t.Errorf("MaxIterations = %d, want 3", cfg.MaxIterations)
	}

	// Verify Condition is a JSON string (legacy format)
	if cfg.Condition == nil {
		t.Fatal("Condition should not be nil for legacy string condition")
	}

	var condStr string
	if err := json.Unmarshal(cfg.Condition, &condStr); err != nil {
		t.Fatalf("legacy Condition should unmarshal as string: %v", err)
	}
	if condStr != "has_findings" {
		t.Errorf("condition string = %q, want %q", condStr, "has_findings")
	}
}

// =============================================================================
// SC-4: LoopConfig with compound condition (all/any)
// =============================================================================

func TestParseLoopConfig_CompoundCondition(t *testing.T) {
	t.Parallel()

	input := `{
		"loop_to_phase": "implement",
		"condition": {
			"all": [
				{"field": "phase_output.review.status", "op": "eq", "value": "needs_changes"},
				{"field": "task.weight", "op": "in", "value": ["medium", "large"]}
			]
		},
		"max_loops": 2
	}`

	cfg, err := ParseLoopConfig(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil LoopConfig")
	}

	// Verify compound condition parses
	var condMap map[string]interface{}
	if err := json.Unmarshal(cfg.Condition, &condMap); err != nil {
		t.Fatalf("Condition is not a valid JSON object: %v", err)
	}
	if _, ok := condMap["all"]; !ok {
		t.Error("expected 'all' key in compound condition")
	}
}

// =============================================================================
// SC-7: EffectiveMaxLoops precedence: max_loops > max_iterations > 3
// =============================================================================

func TestLoopConfig_EffectiveMaxLoops_MaxLoopsSet(t *testing.T) {
	t.Parallel()

	cfg := &LoopConfig{
		LoopToPhase:   "implement",
		Condition:     json.RawMessage(`"has_findings"`),
		MaxLoops:      5,
		MaxIterations: 3, // Should be ignored when MaxLoops is set
	}

	got := cfg.EffectiveMaxLoops()
	if got != 5 {
		t.Errorf("EffectiveMaxLoops() = %d, want 5 (MaxLoops takes precedence)", got)
	}
}

func TestLoopConfig_EffectiveMaxLoops_FallsBackToMaxIterations(t *testing.T) {
	t.Parallel()

	cfg := &LoopConfig{
		LoopToPhase:   "implement",
		Condition:     json.RawMessage(`"has_findings"`),
		MaxLoops:      0, // Not set
		MaxIterations: 4,
	}

	got := cfg.EffectiveMaxLoops()
	if got != 4 {
		t.Errorf("EffectiveMaxLoops() = %d, want 4 (falls back to MaxIterations)", got)
	}
}

func TestLoopConfig_EffectiveMaxLoops_Default(t *testing.T) {
	t.Parallel()

	cfg := &LoopConfig{
		LoopToPhase:   "implement",
		Condition:     json.RawMessage(`"has_findings"`),
		MaxLoops:      0,
		MaxIterations: 0, // Both unset → default to 3
	}

	got := cfg.EffectiveMaxLoops()
	if got != 3 {
		t.Errorf("EffectiveMaxLoops() = %d, want 3 (default when both zero)", got)
	}
}

func TestLoopConfig_EffectiveMaxLoops_NegativeValues(t *testing.T) {
	t.Parallel()

	cfg := &LoopConfig{
		LoopToPhase:   "implement",
		Condition:     json.RawMessage(`"has_findings"`),
		MaxLoops:      -1,
		MaxIterations: -2,
	}

	got := cfg.EffectiveMaxLoops()
	if got != 3 {
		t.Errorf("EffectiveMaxLoops() = %d, want 3 (default when negative)", got)
	}
}

func TestLoopConfig_EffectiveMaxLoops_MaxLoopsOne(t *testing.T) {
	t.Parallel()

	cfg := &LoopConfig{
		LoopToPhase: "implement",
		Condition:   json.RawMessage(`"has_findings"`),
		MaxLoops:    1,
	}

	got := cfg.EffectiveMaxLoops()
	if got != 1 {
		t.Errorf("EffectiveMaxLoops() = %d, want 1", got)
	}
}

// =============================================================================
// Failure mode: Malformed JSON → error returned
// =============================================================================

func TestParseLoopConfig_MalformedJSON(t *testing.T) {
	t.Parallel()

	_, err := ParseLoopConfig(`{not valid json}`)
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

// =============================================================================
// Edge case: Empty/null input → nil config, no error
// =============================================================================

func TestParseLoopConfig_EmptyInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
	}{
		{"empty string", ""},
		{"null string", "null"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg, err := ParseLoopConfig(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg != nil {
				t.Error("expected nil config for empty/null input")
			}
		})
	}
}

// =============================================================================
// SC-4: Both max_loops and max_iterations present in JSON
// =============================================================================

func TestParseLoopConfig_BothMaxFields(t *testing.T) {
	t.Parallel()

	input := `{
		"loop_to_phase": "implement",
		"condition": "status_needs_fix",
		"max_loops": 5,
		"max_iterations": 3
	}`

	cfg, err := ParseLoopConfig(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil LoopConfig")
	}

	// Both should be preserved
	if cfg.MaxLoops != 5 {
		t.Errorf("MaxLoops = %d, want 5", cfg.MaxLoops)
	}
	if cfg.MaxIterations != 3 {
		t.Errorf("MaxIterations = %d, want 3", cfg.MaxIterations)
	}

	// EffectiveMaxLoops should prefer MaxLoops
	if cfg.EffectiveMaxLoops() != 5 {
		t.Errorf("EffectiveMaxLoops() = %d, want 5 (MaxLoops takes precedence)", cfg.EffectiveMaxLoops())
	}
}

// =============================================================================
// Edge case: LoopConfig with empty condition
// =============================================================================

func TestParseLoopConfig_EmptyCondition(t *testing.T) {
	t.Parallel()

	input := `{
		"loop_to_phase": "implement",
		"max_loops": 2
	}`

	cfg, err := ParseLoopConfig(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil LoopConfig")
	}

	// Condition should be nil/empty when not set
	if cfg.LoopToPhase != "implement" {
		t.Errorf("LoopToPhase = %q, want %q", cfg.LoopToPhase, "implement")
	}
}

// =============================================================================
// SC-4: IsLegacyCondition helper distinguishes string vs JSON object
// =============================================================================

func TestLoopConfig_IsLegacyCondition(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		condition json.RawMessage
		isLegacy  bool
	}{
		{
			name:      "JSON string is legacy",
			condition: json.RawMessage(`"has_findings"`),
			isLegacy:  true,
		},
		{
			name:      "JSON object is new format",
			condition: json.RawMessage(`{"field":"phase_output.review.status","op":"eq","value":"needs_changes"}`),
			isLegacy:  false,
		},
		{
			name:      "empty is not legacy",
			condition: nil,
			isLegacy:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := &LoopConfig{
				LoopToPhase: "implement",
				Condition:   tt.condition,
			}

			got := cfg.IsLegacyCondition()
			if got != tt.isLegacy {
				t.Errorf("IsLegacyCondition() = %v, want %v", got, tt.isLegacy)
			}
		})
	}
}
