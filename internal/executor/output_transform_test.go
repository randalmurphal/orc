package executor

import (
	"strings"
	"testing"

	"github.com/randalmurphal/orc/internal/db"
)

// =============================================================================
// Tests for ApplyOutputTransform (TASK-757)
//
// SC-1: ApplyOutputTransform function exists and correctly transforms output
//       based on OutputTransformConfig type (format_findings, passthrough,
//       json_extract).
// =============================================================================

func TestApplyOutputTransform_FormatFindings(t *testing.T) {
	t.Parallel()

	cfg := &db.OutputTransformConfig{
		Type:      "format_findings",
		SourceVar: "REVIEW_OUTPUT",
		TargetVar: "REVIEW_FINDINGS",
	}

	// Input: raw review findings JSON
	input := `{"needs_changes":true,"summary":"Found bugs","issues":[{"severity":"high","description":"Memory leak in handler"}]}`

	result, err := ApplyOutputTransform(cfg, input)
	if err != nil {
		t.Fatalf("ApplyOutputTransform() error: %v", err)
	}

	// Should contain formatted summary
	if !strings.Contains(result, "Found bugs") {
		t.Errorf("result should contain summary 'Found bugs', got: %s", result)
	}
	// Should contain issue description
	if !strings.Contains(result, "Memory leak") {
		t.Errorf("result should contain issue 'Memory leak', got: %s", result)
	}
	// Should contain severity marker (FormatFindingsForRound2 uses uppercase)
	if !strings.Contains(result, "HIGH") {
		t.Errorf("result should contain severity 'HIGH', got: %s", result)
	}
}

func TestApplyOutputTransform_Passthrough(t *testing.T) {
	t.Parallel()

	cfg := &db.OutputTransformConfig{
		Type:      "passthrough",
		SourceVar: "RAW_OUTPUT",
		TargetVar: "PROCESSED_OUTPUT",
	}

	input := `{"arbitrary":"json","data":123}`

	result, err := ApplyOutputTransform(cfg, input)
	if err != nil {
		t.Fatalf("ApplyOutputTransform() error: %v", err)
	}

	if result != input {
		t.Errorf("passthrough should return input unchanged\ngot:  %s\nwant: %s", result, input)
	}
}

func TestApplyOutputTransform_JsonExtract(t *testing.T) {
	t.Parallel()

	cfg := &db.OutputTransformConfig{
		Type:        "json_extract",
		SourceVar:   "FULL_OUTPUT",
		TargetVar:   "EXTRACTED",
		ExtractPath: "summary",
	}

	input := `{"status":"complete","summary":"All tests passed","extra":"ignored"}`

	result, err := ApplyOutputTransform(cfg, input)
	if err != nil {
		t.Fatalf("ApplyOutputTransform() error: %v", err)
	}

	expected := "All tests passed"
	if result != expected {
		t.Errorf("json_extract should return extracted value\ngot:  %s\nwant: %s", result, expected)
	}
}

func TestApplyOutputTransform_NilConfig(t *testing.T) {
	t.Parallel()

	input := `{"any":"data"}`

	result, err := ApplyOutputTransform(nil, input)
	if err != nil {
		t.Fatalf("ApplyOutputTransform(nil, ...) error: %v", err)
	}

	// With nil config, should return input unchanged (passthrough behavior)
	if result != input {
		t.Errorf("nil config should passthrough input\ngot:  %s\nwant: %s", result, input)
	}
}

func TestApplyOutputTransform_FormatFindingsInvalidJSON(t *testing.T) {
	t.Parallel()

	cfg := &db.OutputTransformConfig{
		Type:      "format_findings",
		SourceVar: "REVIEW_OUTPUT",
		TargetVar: "REVIEW_FINDINGS",
	}

	input := `not valid json`

	_, err := ApplyOutputTransform(cfg, input)
	if err == nil {
		t.Error("ApplyOutputTransform() should error on invalid JSON for format_findings")
	}
}

func TestApplyOutputTransform_EmptyInput(t *testing.T) {
	t.Parallel()

	cfg := &db.OutputTransformConfig{
		Type:      "passthrough",
		SourceVar: "INPUT",
		TargetVar: "OUTPUT",
	}

	result, err := ApplyOutputTransform(cfg, "")
	if err != nil {
		t.Fatalf("ApplyOutputTransform() error: %v", err)
	}

	if result != "" {
		t.Errorf("empty input should return empty result, got: %s", result)
	}
}

// =============================================================================
// SC-2 is verified by existing tests in review_loop_generalization_test.go
//
// TestOutputTransform_UsesLoopConfigNotHardcoded (line 248) tests that the
// output transform is applied using LoopConfig, not hardcoded.
//
// After implementation, the test's local helper function should be updated
// to call the real ApplyOutputTransform, making the test pass.
// =============================================================================
