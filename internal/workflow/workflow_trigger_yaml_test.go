package workflow

import (
	"encoding/json"
	"testing"

	"github.com/randalmurphal/orc/templates"
)

// --- SC-6: Workflow YAML files include on_initiative_planned trigger ---

func TestImplementMediumWorkflow_HasInitiativePlannedTrigger(t *testing.T) {
	t.Parallel()

	data, err := templates.Workflows.ReadFile("workflows/implement-medium.yaml")
	if err != nil {
		t.Fatalf("failed to read implement-medium.yaml: %v", err)
	}

	wf, err := parseWorkflowYAML(data)
	if err != nil {
		t.Fatalf("failed to parse implement-medium.yaml: %v", err)
	}

	// SC-6: must have triggers section
	if len(wf.Triggers) == 0 {
		t.Fatal("implement-medium workflow has no triggers")
	}

	// Find the on_initiative_planned trigger
	found := false
	for _, trig := range wf.Triggers {
		if trig.Event == WorkflowTriggerEventOnInitiativePlanned {
			found = true

			// SC-6: agent_id must be dependency-validator
			if trig.AgentID != "dependency-validator" {
				t.Errorf("trigger agent_id = %q, want %q", trig.AgentID, "dependency-validator")
			}

			// SC-6: mode must be gate
			if trig.Mode != GateModeGate {
				t.Errorf("trigger mode = %q, want %q", trig.Mode, GateModeGate)
			}

			// SC-6: enabled must be true
			if !trig.Enabled {
				t.Error("trigger enabled = false, want true")
			}

			break
		}
	}
	if !found {
		t.Error("implement-medium workflow has no on_initiative_planned trigger")
	}
}

func TestImplementLargeWorkflow_HasInitiativePlannedTrigger(t *testing.T) {
	t.Parallel()

	data, err := templates.Workflows.ReadFile("workflows/implement-large.yaml")
	if err != nil {
		t.Fatalf("failed to read implement-large.yaml: %v", err)
	}

	wf, err := parseWorkflowYAML(data)
	if err != nil {
		t.Fatalf("failed to parse implement-large.yaml: %v", err)
	}

	// SC-6: must have triggers section
	if len(wf.Triggers) == 0 {
		t.Fatal("implement-large workflow has no triggers")
	}

	// Find the on_initiative_planned trigger
	found := false
	for _, trig := range wf.Triggers {
		if trig.Event == WorkflowTriggerEventOnInitiativePlanned {
			found = true

			if trig.AgentID != "dependency-validator" {
				t.Errorf("trigger agent_id = %q, want %q", trig.AgentID, "dependency-validator")
			}
			if trig.Mode != GateModeGate {
				t.Errorf("trigger mode = %q, want %q", trig.Mode, GateModeGate)
			}
			if !trig.Enabled {
				t.Error("trigger enabled = false, want true")
			}

			break
		}
	}
	if !found {
		t.Error("implement-large workflow has no on_initiative_planned trigger")
	}
}

// SC-6: Workflow YAML with triggers section parses correctly

func TestParseWorkflowYAML_WithTriggers(t *testing.T) {
	t.Parallel()

	yamlData := []byte(`
id: test-workflow
name: "Test Workflow"
workflow_type: task

phases:
  - template: implement
    sequence: 0

triggers:
  - event: on_initiative_planned
    agent_id: dependency-validator
    mode: gate
    enabled: true
  - event: on_task_created
    agent_id: some-agent
    mode: reaction
    enabled: false
`)

	wf, err := parseWorkflowYAML(yamlData)
	if err != nil {
		t.Fatalf("parseWorkflowYAML failed: %v", err)
	}

	if len(wf.Triggers) != 2 {
		t.Fatalf("expected 2 triggers, got %d", len(wf.Triggers))
	}

	// First trigger
	trig := wf.Triggers[0]
	if trig.Event != WorkflowTriggerEventOnInitiativePlanned {
		t.Errorf("trigger[0].Event = %q, want %q", trig.Event, WorkflowTriggerEventOnInitiativePlanned)
	}
	if trig.AgentID != "dependency-validator" {
		t.Errorf("trigger[0].AgentID = %q, want %q", trig.AgentID, "dependency-validator")
	}
	if trig.Mode != GateModeGate {
		t.Errorf("trigger[0].Mode = %q, want %q", trig.Mode, GateModeGate)
	}
	if !trig.Enabled {
		t.Error("trigger[0].Enabled = false, want true")
	}

	// Second trigger
	trig2 := wf.Triggers[1]
	if trig2.Event != WorkflowTriggerEventOnTaskCreated {
		t.Errorf("trigger[1].Event = %q, want %q", trig2.Event, WorkflowTriggerEventOnTaskCreated)
	}
	if trig2.Enabled {
		t.Error("trigger[1].Enabled = true, want false")
	}
}

// Preservation: Existing workflows without triggers still parse

func TestParseWorkflowYAML_WithoutTriggers(t *testing.T) {
	t.Parallel()

	yamlData := []byte(`
id: no-triggers
name: "No Triggers"
workflow_type: task

phases:
  - template: implement
    sequence: 0
`)

	wf, err := parseWorkflowYAML(yamlData)
	if err != nil {
		t.Fatalf("parseWorkflowYAML failed: %v", err)
	}

	if len(wf.Triggers) != 0 {
		t.Errorf("expected 0 triggers, got %d", len(wf.Triggers))
	}

	if len(wf.Phases) != 1 {
		t.Errorf("expected 1 phase, got %d", len(wf.Phases))
	}
}

// SC-6 (integration): Seeded workflows include triggers

func TestSeedBuiltins_WorkflowsHaveTriggers(t *testing.T) {
	t.Parallel()

	gdb := openTestGlobalDB(t)

	_, err := SeedBuiltins(gdb)
	if err != nil {
		t.Fatalf("SeedBuiltins failed: %v", err)
	}

	// Helper to check a workflow has the dependency-validator trigger
	checkWorkflowTrigger := func(workflowID string) {
		t.Helper()

		wf, err := gdb.GetWorkflow(workflowID)
		if err != nil {
			t.Fatalf("GetWorkflow(%s) failed: %v", workflowID, err)
		}
		if wf == nil {
			t.Fatalf("%s workflow not found", workflowID)
		}

		// DB stores triggers as JSON string
		if wf.Triggers == "" {
			t.Fatalf("%s workflow has empty triggers string", workflowID)
		}

		var triggers []WorkflowTrigger
		if err := json.Unmarshal([]byte(wf.Triggers), &triggers); err != nil {
			t.Fatalf("%s: failed to parse triggers JSON %q: %v", workflowID, wf.Triggers, err)
		}

		hasTrigger := false
		for _, trig := range triggers {
			if trig.Event == WorkflowTriggerEventOnInitiativePlanned &&
				trig.AgentID == "dependency-validator" {
				hasTrigger = true
				break
			}
		}
		if !hasTrigger {
			t.Errorf("%s workflow missing on_initiative_planned trigger for dependency-validator; triggers: %s",
				workflowID, wf.Triggers)
		}
	}

	checkWorkflowTrigger("implement-medium")
	checkWorkflowTrigger("implement-large")
}
