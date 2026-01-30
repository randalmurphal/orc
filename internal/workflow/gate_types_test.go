package workflow

import (
	"testing"
)

// TestGateAI_Constant verifies the AI gate type constant exists.
// Covers SC-3: GateType enum includes GATE_TYPE_AI.
func TestGateAI_Constant(t *testing.T) {
	t.Parallel()

	if GateAI != GateType("ai") {
		t.Errorf("GateAI = %q, want %q", GateAI, "ai")
	}

	// Verify existing gate types are unchanged
	if GateAuto != GateType("auto") {
		t.Errorf("GateAuto = %q, want %q", GateAuto, "auto")
	}
	if GateHuman != GateType("human") {
		t.Errorf("GateHuman = %q, want %q", GateHuman, "human")
	}
	if GateSkip != GateType("skip") {
		t.Errorf("GateSkip = %q, want %q", GateSkip, "skip")
	}
}

// TestGateMode_Constants verifies GateMode type and constants.
// Covers SC-4: GateMode enum with GATE and REACTION values.
func TestGateMode_Constants(t *testing.T) {
	t.Parallel()

	if GateModeGate != GateMode("gate") {
		t.Errorf("GateModeGate = %q, want %q", GateModeGate, "gate")
	}
	if GateModeReaction != GateMode("reaction") {
		t.Errorf("GateModeReaction = %q, want %q", GateModeReaction, "reaction")
	}
}

// TestGateAction_Constants verifies GateAction type and all constants.
// Covers SC-2: GateOutputConfig on_approved/on_rejected use GateAction.
func TestGateAction_Constants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		got  GateAction
		want string
	}{
		{GateActionContinue, "continue"},
		{GateActionRetry, "retry"},
		{GateActionFail, "fail"},
		{GateActionSkipPhase, "skip_phase"},
		{GateActionRunScript, "run_script"},
	}

	for _, tc := range tests {
		if string(tc.got) != tc.want {
			t.Errorf("GateAction constant = %q, want %q", tc.got, tc.want)
		}
	}
}

// TestWorkflowTriggerEvent_Constants verifies lifecycle event types.
// Covers SC-10: WorkflowTriggerEvent with on_task_created, on_task_completed,
// on_task_failed, on_initiative_planned.
func TestWorkflowTriggerEvent_Constants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		got  WorkflowTriggerEvent
		want string
	}{
		{WorkflowTriggerEventOnTaskCreated, "on_task_created"},
		{WorkflowTriggerEventOnTaskCompleted, "on_task_completed"},
		{WorkflowTriggerEventOnTaskFailed, "on_task_failed"},
		{WorkflowTriggerEventOnInitiativePlanned, "on_initiative_planned"},
	}

	for _, tc := range tests {
		if string(tc.got) != tc.want {
			t.Errorf("WorkflowTriggerEvent = %q, want %q", tc.got, tc.want)
		}
	}
}

// TestGateInputConfig_Struct verifies GateInputConfig has required fields.
// Covers SC-1: include_phase_output, include_task, extra_vars fields.
func TestGateInputConfig_Struct(t *testing.T) {
	t.Parallel()

	cfg := GateInputConfig{
		IncludePhaseOutput: []string{"spec", "tdd_write"},
		IncludeTask:        true,
		ExtraVars:          []string{"CUSTOM_VAR_1", "CUSTOM_VAR_2"},
	}

	if len(cfg.IncludePhaseOutput) != 2 {
		t.Errorf("IncludePhaseOutput length = %d, want 2", len(cfg.IncludePhaseOutput))
	}
	if !cfg.IncludeTask {
		t.Error("IncludeTask = false, want true")
	}
	if len(cfg.ExtraVars) != 2 {
		t.Errorf("ExtraVars length = %d, want 2", len(cfg.ExtraVars))
	}
}

// TestGateOutputConfig_Struct verifies GateOutputConfig has required fields.
// Covers SC-2: variable_name, on_approved, on_rejected, retry_from, script fields.
func TestGateOutputConfig_Struct(t *testing.T) {
	t.Parallel()

	cfg := GateOutputConfig{
		VariableName: "GATE_RESULT",
		OnApproved:   GateActionContinue,
		OnRejected:   GateActionRetry,
		RetryFrom:    "implement",
		Script:       "scripts/post-gate.sh",
	}

	if cfg.VariableName != "GATE_RESULT" {
		t.Errorf("VariableName = %q, want GATE_RESULT", cfg.VariableName)
	}
	if cfg.OnApproved != GateActionContinue {
		t.Errorf("OnApproved = %q, want continue", cfg.OnApproved)
	}
	if cfg.OnRejected != GateActionRetry {
		t.Errorf("OnRejected = %q, want retry", cfg.OnRejected)
	}
	if cfg.RetryFrom != "implement" {
		t.Errorf("RetryFrom = %q, want implement", cfg.RetryFrom)
	}
	if cfg.Script != "scripts/post-gate.sh" {
		t.Errorf("Script = %q, want scripts/post-gate.sh", cfg.Script)
	}
}

// TestBeforePhaseTrigger_Struct verifies BeforePhaseTrigger has required fields.
// Covers SC-6: BeforePhaseTrigger with agent_id, input_config, output_config, mode.
func TestBeforePhaseTrigger_Struct(t *testing.T) {
	t.Parallel()

	trigger := BeforePhaseTrigger{
		AgentID: "dep-validator",
		InputConfig: &GateInputConfig{
			IncludeTask: true,
		},
		OutputConfig: &GateOutputConfig{
			VariableName: "DEP_CHECK",
			OnApproved:   GateActionContinue,
			OnRejected:   GateActionFail,
		},
		Mode: GateModeGate,
	}

	if trigger.AgentID != "dep-validator" {
		t.Errorf("AgentID = %q, want dep-validator", trigger.AgentID)
	}
	if trigger.InputConfig == nil {
		t.Fatal("InputConfig is nil")
	}
	if trigger.OutputConfig == nil {
		t.Fatal("OutputConfig is nil")
	}
	if trigger.Mode != GateModeGate {
		t.Errorf("Mode = %q, want gate", trigger.Mode)
	}
}

// TestWorkflowTrigger_Struct verifies WorkflowTrigger has required fields.
// Covers SC-10: WorkflowTrigger with event, agent_id, configs, mode, enabled.
func TestWorkflowTrigger_Struct(t *testing.T) {
	t.Parallel()

	trigger := WorkflowTrigger{
		Event:   WorkflowTriggerEventOnTaskCreated,
		AgentID: "init-checker",
		InputConfig: &GateInputConfig{
			IncludeTask: true,
		},
		OutputConfig: &GateOutputConfig{
			OnApproved: GateActionContinue,
			OnRejected: GateActionFail,
		},
		Mode:    GateModeReaction,
		Enabled: true,
	}

	if trigger.Event != WorkflowTriggerEventOnTaskCreated {
		t.Errorf("Event = %q, want on_task_created", trigger.Event)
	}
	if trigger.AgentID != "init-checker" {
		t.Errorf("AgentID = %q, want init-checker", trigger.AgentID)
	}
	if !trigger.Enabled {
		t.Error("Enabled = false, want true")
	}
	if trigger.Mode != GateModeReaction {
		t.Errorf("Mode = %q, want reaction", trigger.Mode)
	}
}

// TestPhaseTemplate_GateConfigFields verifies PhaseTemplate has new gate fields.
// Covers SC-5: PhaseTemplate includes gate_input_config, gate_output_config,
// gate_mode, gate_agent_id.
func TestPhaseTemplate_GateConfigFields(t *testing.T) {
	t.Parallel()

	pt := PhaseTemplate{
		ID:       "test-phase",
		Name:     "Test Phase",
		GateType: GateAI,
		GateMode: GateModeGate,
		GateAgentID: "review-agent",
		GateInputConfig: &GateInputConfig{
			IncludePhaseOutput: []string{"implement"},
			IncludeTask:        true,
		},
		GateOutputConfig: &GateOutputConfig{
			VariableName: "REVIEW_RESULT",
			OnApproved:   GateActionContinue,
			OnRejected:   GateActionRetry,
			RetryFrom:    "implement",
		},
	}

	if pt.GateMode != GateModeGate {
		t.Errorf("GateMode = %q, want gate", pt.GateMode)
	}
	if pt.GateAgentID != "review-agent" {
		t.Errorf("GateAgentID = %q, want review-agent", pt.GateAgentID)
	}
	if pt.GateInputConfig == nil {
		t.Fatal("GateInputConfig is nil")
	}
	if pt.GateOutputConfig == nil {
		t.Fatal("GateOutputConfig is nil")
	}
}

// TestWorkflowPhase_BeforeTriggers verifies WorkflowPhase has before_triggers field.
// Covers SC-6: WorkflowPhase includes before_triggers repeated field.
func TestWorkflowPhase_BeforeTriggers(t *testing.T) {
	t.Parallel()

	wp := WorkflowPhase{
		WorkflowID:      "test-wf",
		PhaseTemplateID: "implement",
		Sequence:        0,
		BeforeTriggers: []BeforePhaseTrigger{
			{
				AgentID: "dep-check",
				Mode:    GateModeGate,
			},
			{
				AgentID: "lint-check",
				Mode:    GateModeReaction,
			},
		},
	}

	if len(wp.BeforeTriggers) != 2 {
		t.Errorf("BeforeTriggers length = %d, want 2", len(wp.BeforeTriggers))
	}
	if wp.BeforeTriggers[0].AgentID != "dep-check" {
		t.Errorf("first trigger AgentID = %q, want dep-check", wp.BeforeTriggers[0].AgentID)
	}
}

// TestWorkflow_Triggers verifies Workflow has triggers field.
// Covers SC-10: Workflow has triggers repeated field.
func TestWorkflow_Triggers(t *testing.T) {
	t.Parallel()

	wf := Workflow{
		ID:   "test-wf",
		Name: "Test Workflow",
		Triggers: []WorkflowTrigger{
			{
				Event:   WorkflowTriggerEventOnTaskCreated,
				AgentID: "init-agent",
				Enabled: true,
			},
			{
				Event:   WorkflowTriggerEventOnTaskFailed,
				AgentID: "notify-agent",
				Mode:    GateModeReaction,
				Enabled: true,
			},
		},
	}

	if len(wf.Triggers) != 2 {
		t.Errorf("Triggers length = %d, want 2", len(wf.Triggers))
	}
	if wf.Triggers[0].Event != WorkflowTriggerEventOnTaskCreated {
		t.Errorf("first trigger Event = %q, want on_task_created", wf.Triggers[0].Event)
	}
}
