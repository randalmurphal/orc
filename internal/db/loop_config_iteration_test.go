// Tests for TASK-710: LoopConfig iteration-specific configuration.
//
// These tests define the contract for new LoopConfig fields that enable
// iteration-specific templates, schemas, and output transforms.
//
// Coverage mapping:
//
//	SC-1: TestLoopConfig_ParseLoopTemplates
//	SC-2: TestLoopConfig_ParseLoopSchemas
//	SC-3: TestLoopConfig_ParseOutputTransform
//	SC-3: TestLoopConfig_OutputTransformTypes
//
// Failure modes:
//
//	TestLoopConfig_InvalidOutputTransformType
//	TestLoopConfig_MissingTransformFields
package db

import (
	"encoding/json"
	"testing"
)

// =============================================================================
// SC-1: LoopConfig parses loop_templates field
// =============================================================================

func TestLoopConfig_ParseLoopTemplates(t *testing.T) {
	t.Parallel()

	input := `{
		"loop_to_phase": "implement",
		"condition": {"field": "phase_output.review.status", "op": "eq", "value": "needs_changes"},
		"max_loops": 3,
		"loop_templates": {
			"1": "review.md",
			"2": "review_round2.md",
			"default": "review_round2.md"
		}
	}`

	cfg, err := ParseLoopConfig(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.LoopTemplates == nil {
		t.Fatal("LoopTemplates should not be nil")
	}
	if len(cfg.LoopTemplates) != 3 {
		t.Errorf("LoopTemplates length = %d, want 3", len(cfg.LoopTemplates))
	}
	if cfg.LoopTemplates["1"] != "review.md" {
		t.Errorf("LoopTemplates[\"1\"] = %q, want %q", cfg.LoopTemplates["1"], "review.md")
	}
	if cfg.LoopTemplates["2"] != "review_round2.md" {
		t.Errorf("LoopTemplates[\"2\"] = %q, want %q", cfg.LoopTemplates["2"], "review_round2.md")
	}
	if cfg.LoopTemplates["default"] != "review_round2.md" {
		t.Errorf("LoopTemplates[\"default\"] = %q, want %q", cfg.LoopTemplates["default"], "review_round2.md")
	}
}

func TestLoopConfig_ParseLoopTemplatesEmpty(t *testing.T) {
	t.Parallel()

	input := `{
		"loop_to_phase": "implement",
		"max_loops": 3
	}`

	cfg, err := ParseLoopConfig(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Empty loop_templates is valid (means no iteration-specific templates)
	if len(cfg.LoopTemplates) != 0 {
		t.Errorf("LoopTemplates should be nil or empty, got %v", cfg.LoopTemplates)
	}
}

// =============================================================================
// SC-2: LoopConfig parses loop_schemas field
// =============================================================================

func TestLoopConfig_ParseLoopSchemas(t *testing.T) {
	t.Parallel()

	input := `{
		"loop_to_phase": "implement",
		"condition": {"field": "phase_output.review.status", "op": "eq", "value": "needs_changes"},
		"max_loops": 3,
		"loop_schemas": {
			"1": "findings",
			"default": "decision"
		}
	}`

	cfg, err := ParseLoopConfig(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.LoopSchemas == nil {
		t.Fatal("LoopSchemas should not be nil")
	}
	if len(cfg.LoopSchemas) != 2 {
		t.Errorf("LoopSchemas length = %d, want 2", len(cfg.LoopSchemas))
	}
	if cfg.LoopSchemas["1"] != "findings" {
		t.Errorf("LoopSchemas[\"1\"] = %q, want %q", cfg.LoopSchemas["1"], "findings")
	}
	if cfg.LoopSchemas["default"] != "decision" {
		t.Errorf("LoopSchemas[\"default\"] = %q, want %q", cfg.LoopSchemas["default"], "decision")
	}
}

func TestLoopConfig_ParseLoopSchemasEmpty(t *testing.T) {
	t.Parallel()

	input := `{
		"loop_to_phase": "implement",
		"max_loops": 3
	}`

	cfg, err := ParseLoopConfig(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Empty loop_schemas is valid
	if len(cfg.LoopSchemas) != 0 {
		t.Errorf("LoopSchemas should be nil or empty, got %v", cfg.LoopSchemas)
	}
}

// =============================================================================
// SC-3: LoopConfig parses output_transform field
// =============================================================================

func TestLoopConfig_ParseOutputTransform(t *testing.T) {
	t.Parallel()

	input := `{
		"loop_to_phase": "implement",
		"max_loops": 3,
		"output_transform": {
			"type": "format_findings",
			"source_var": "REVIEW_OUTPUT",
			"target_var": "REVIEW_FINDINGS"
		}
	}`

	cfg, err := ParseLoopConfig(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.OutputTransform == nil {
		t.Fatal("OutputTransform should not be nil")
	}
	if cfg.OutputTransform.Type != "format_findings" {
		t.Errorf("OutputTransform.Type = %q, want %q", cfg.OutputTransform.Type, "format_findings")
	}
	if cfg.OutputTransform.SourceVar != "REVIEW_OUTPUT" {
		t.Errorf("OutputTransform.SourceVar = %q, want %q", cfg.OutputTransform.SourceVar, "REVIEW_OUTPUT")
	}
	if cfg.OutputTransform.TargetVar != "REVIEW_FINDINGS" {
		t.Errorf("OutputTransform.TargetVar = %q, want %q", cfg.OutputTransform.TargetVar, "REVIEW_FINDINGS")
	}
}

func TestLoopConfig_ParseOutputTransformEmpty(t *testing.T) {
	t.Parallel()

	input := `{
		"loop_to_phase": "implement",
		"max_loops": 3
	}`

	cfg, err := ParseLoopConfig(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Nil output_transform is valid (means no transform between iterations)
	if cfg.OutputTransform != nil {
		t.Errorf("OutputTransform should be nil, got %+v", cfg.OutputTransform)
	}
}

// =============================================================================
// SC-3: OutputTransform supports multiple types
// =============================================================================

func TestLoopConfig_OutputTransformTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		transformCfg string
		wantType     string
	}{
		{
			name: "format_findings",
			transformCfg: `{
				"type": "format_findings",
				"source_var": "REVIEW_OUTPUT",
				"target_var": "REVIEW_FINDINGS"
			}`,
			wantType: "format_findings",
		},
		{
			name: "json_extract",
			transformCfg: `{
				"type": "json_extract",
				"source_var": "PHASE_OUTPUT",
				"target_var": "EXTRACTED_FIELD",
				"extract_path": ".issues"
			}`,
			wantType: "json_extract",
		},
		{
			name: "passthrough",
			transformCfg: `{
				"type": "passthrough",
				"source_var": "INPUT",
				"target_var": "OUTPUT"
			}`,
			wantType: "passthrough",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := `{
				"loop_to_phase": "implement",
				"max_loops": 3,
				"output_transform": ` + tt.transformCfg + `
			}`

			cfg, err := ParseLoopConfig(input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if cfg.OutputTransform == nil {
				t.Fatal("OutputTransform should not be nil")
			}
			if cfg.OutputTransform.Type != tt.wantType {
				t.Errorf("OutputTransform.Type = %q, want %q", cfg.OutputTransform.Type, tt.wantType)
			}
		})
	}
}

// =============================================================================
// Failure Mode: Invalid output transform type
// =============================================================================

func TestLoopConfig_InvalidOutputTransformType(t *testing.T) {
	t.Parallel()

	// Parsing should succeed, but validation should fail
	input := `{
		"loop_to_phase": "implement",
		"max_loops": 3,
		"output_transform": {
			"type": "invalid_type",
			"source_var": "INPUT"
		}
	}`

	cfg, err := ParseLoopConfig(input)
	if err != nil {
		t.Fatalf("parsing should succeed: %v", err)
	}

	// Validation should fail for invalid type
	err = cfg.OutputTransform.Validate()
	if err == nil {
		t.Error("expected validation error for invalid transform type")
	}
}

// =============================================================================
// Failure Mode: Missing required transform fields
// =============================================================================

func TestLoopConfig_MissingTransformFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			name: "missing_type",
			input: `{
				"loop_to_phase": "implement",
				"max_loops": 3,
				"output_transform": {
					"source_var": "INPUT",
					"target_var": "OUTPUT"
				}
			}`,
			wantErr: "type",
		},
		{
			name: "missing_source_var",
			input: `{
				"loop_to_phase": "implement",
				"max_loops": 3,
				"output_transform": {
					"type": "format_findings",
					"target_var": "OUTPUT"
				}
			}`,
			wantErr: "source_var",
		},
		{
			name: "missing_target_var",
			input: `{
				"loop_to_phase": "implement",
				"max_loops": 3,
				"output_transform": {
					"type": "format_findings",
					"source_var": "INPUT"
				}
			}`,
			wantErr: "target_var",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := ParseLoopConfig(tt.input)
			if err != nil {
				t.Fatalf("parsing should succeed: %v", err)
			}

			if cfg.OutputTransform == nil {
				t.Fatal("OutputTransform should not be nil")
			}

			err = cfg.OutputTransform.Validate()
			if err == nil {
				t.Errorf("expected validation error for %s", tt.wantErr)
			}
		})
	}
}

// =============================================================================
// LoopConfig.GetTemplateForIteration helper
// =============================================================================

func TestLoopConfig_GetTemplateForIteration(t *testing.T) {
	t.Parallel()

	cfg := &LoopConfig{
		LoopToPhase: "implement",
		MaxLoops:    3,
		LoopTemplates: map[string]string{
			"1":       "review.md",
			"default": "review_round2.md",
		},
	}

	// Iteration 1 should return "review.md"
	tmpl1 := cfg.GetTemplateForIteration(1, "base.md")
	if tmpl1 != "review.md" {
		t.Errorf("GetTemplateForIteration(1) = %q, want %q", tmpl1, "review.md")
	}

	// Iteration 2 should return default "review_round2.md"
	tmpl2 := cfg.GetTemplateForIteration(2, "base.md")
	if tmpl2 != "review_round2.md" {
		t.Errorf("GetTemplateForIteration(2) = %q, want %q", tmpl2, "review_round2.md")
	}

	// Iteration 3 should also return default
	tmpl3 := cfg.GetTemplateForIteration(3, "base.md")
	if tmpl3 != "review_round2.md" {
		t.Errorf("GetTemplateForIteration(3) = %q, want %q", tmpl3, "review_round2.md")
	}
}

func TestLoopConfig_GetTemplateForIterationNoTemplates(t *testing.T) {
	t.Parallel()

	cfg := &LoopConfig{
		LoopToPhase: "implement",
		MaxLoops:    3,
		// No LoopTemplates configured
	}

	// Should return the base template when no loop_templates configured
	tmpl := cfg.GetTemplateForIteration(2, "base.md")
	if tmpl != "base.md" {
		t.Errorf("GetTemplateForIteration(2) = %q, want %q (base)", tmpl, "base.md")
	}
}

// =============================================================================
// LoopConfig.GetSchemaForIteration helper
// =============================================================================

func TestLoopConfig_GetSchemaForIteration(t *testing.T) {
	t.Parallel()

	cfg := &LoopConfig{
		LoopToPhase: "implement",
		MaxLoops:    3,
		LoopSchemas: map[string]string{
			"1":       "findings",
			"default": "decision",
		},
	}

	// Iteration 1 should return "findings"
	schema1 := cfg.GetSchemaForIteration(1)
	if schema1 != "findings" {
		t.Errorf("GetSchemaForIteration(1) = %q, want %q", schema1, "findings")
	}

	// Iteration 2 should return default "decision"
	schema2 := cfg.GetSchemaForIteration(2)
	if schema2 != "decision" {
		t.Errorf("GetSchemaForIteration(2) = %q, want %q", schema2, "decision")
	}
}

func TestLoopConfig_GetSchemaForIterationNoSchemas(t *testing.T) {
	t.Parallel()

	cfg := &LoopConfig{
		LoopToPhase: "implement",
		MaxLoops:    3,
		// No LoopSchemas configured
	}

	// Should return empty string when no loop_schemas configured
	schema := cfg.GetSchemaForIteration(1)
	if schema != "" {
		t.Errorf("GetSchemaForIteration(1) = %q, want empty string", schema)
	}
}

// =============================================================================
// LoopConfig JSON roundtrip
// =============================================================================

func TestLoopConfig_JSONRoundtrip(t *testing.T) {
	t.Parallel()

	original := &LoopConfig{
		LoopToPhase:   "implement",
		MaxLoops:      3,
		MaxIterations: 0,
		Condition:     json.RawMessage(`{"field":"phase_output.review.status","op":"eq","value":"needs_changes"}`),
		LoopTemplates: map[string]string{
			"1":       "review.md",
			"default": "review_round2.md",
		},
		LoopSchemas: map[string]string{
			"1":       "findings",
			"default": "decision",
		},
		OutputTransform: &OutputTransformConfig{
			Type:      "format_findings",
			SourceVar: "REVIEW_OUTPUT",
			TargetVar: "REVIEW_FINDINGS",
		},
	}

	// Marshal to JSON
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	// Parse back
	parsed, err := ParseLoopConfig(string(data))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	// Verify fields
	if parsed.LoopToPhase != original.LoopToPhase {
		t.Errorf("LoopToPhase = %q, want %q", parsed.LoopToPhase, original.LoopToPhase)
	}
	if parsed.MaxLoops != original.MaxLoops {
		t.Errorf("MaxLoops = %d, want %d", parsed.MaxLoops, original.MaxLoops)
	}
	if len(parsed.LoopTemplates) != len(original.LoopTemplates) {
		t.Errorf("LoopTemplates length = %d, want %d", len(parsed.LoopTemplates), len(original.LoopTemplates))
	}
	if len(parsed.LoopSchemas) != len(original.LoopSchemas) {
		t.Errorf("LoopSchemas length = %d, want %d", len(parsed.LoopSchemas), len(original.LoopSchemas))
	}
	if parsed.OutputTransform == nil {
		t.Error("OutputTransform should not be nil after roundtrip")
	}
}
