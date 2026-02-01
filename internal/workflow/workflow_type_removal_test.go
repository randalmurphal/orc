package workflow

import (
	"strings"
	"testing"
)

// SC-1: YAML round-trip serialization drops workflow_type field.
// After removal, parsing YAML with workflow_type and re-marshaling
// should NOT produce workflow_type in the output.
func TestWorkflowYAML_RoundTrip_DropsWorkflowType(t *testing.T) {
	t.Parallel()

	input := []byte(`
id: test-roundtrip
name: "Test Roundtrip"
workflow_type: task

phases:
  - template: implement
    sequence: 0
`)

	wf, err := parseWorkflowYAML(input)
	if err != nil {
		t.Fatalf("parseWorkflowYAML failed: %v", err)
	}

	if wf.ID != "test-roundtrip" {
		t.Fatalf("expected ID test-roundtrip, got %s", wf.ID)
	}

	out, err := marshalWorkflowYAML(wf)
	if err != nil {
		t.Fatalf("marshalWorkflowYAML failed: %v", err)
	}

	if strings.Contains(string(out), "workflow_type") {
		t.Errorf("marshaled YAML should not contain workflow_type, got:\n%s", out)
	}
}

// SC-2: Existing YAML files containing workflow_type parse without error.
// After removal, the field becomes unknown to the YAML struct and must be
// silently ignored (gopkg.in/yaml.v3 default behavior).
func TestWorkflowYAML_BackwardCompat_IgnoresWorkflowType(t *testing.T) {
	t.Parallel()

	input := []byte(`
id: legacy-workflow
name: "Legacy With Type"
workflow_type: task
default_model: claude-sonnet-4-20250514

phases:
  - template: spec
    sequence: 0
  - template: implement
    sequence: 1
`)

	wf, err := parseWorkflowYAML(input)
	if err != nil {
		t.Fatalf("YAML with workflow_type should parse without error: %v", err)
	}

	if wf.ID != "legacy-workflow" {
		t.Errorf("ID = %q, want %q", wf.ID, "legacy-workflow")
	}
	if wf.Name != "Legacy With Type" {
		t.Errorf("Name = %q, want %q", wf.Name, "Legacy With Type")
	}
	if wf.DefaultModel != "claude-sonnet-4-20250514" {
		t.Errorf("DefaultModel = %q, want %q", wf.DefaultModel, "claude-sonnet-4-20250514")
	}
	if len(wf.Phases) != 2 {
		t.Errorf("expected 2 phases, got %d", len(wf.Phases))
	}
}
