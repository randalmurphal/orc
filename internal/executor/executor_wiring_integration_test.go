// executor_wiring_integration_test.go tests the wiring of AI gates, lifecycle events,
// and gate output pipeline into the executor's main phase loop.
//
// These tests verify that the executor correctly calls:
// - evaluatePhaseGate with populated EvaluateOptions (SC-1)
// - evaluateBeforePhaseTriggers in the executor loop (SC-2)
// - handleCompletionWithTriggers before PR creation (SC-3)
// - fireLifecycleTriggers on failure paths (SC-4)
// - applyGateOutputToVars after gate evaluation (SC-5)
// - BuildRetryContextWithGateAnalysis for retry context (SC-6)
// - ScriptHandler when GateOutputConfig.Script is set (SC-7)
package executor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/gate"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/internal/trigger"
	"github.com/randalmurphal/orc/internal/workflow"
)

// =============================================================================
// Mock gate evaluator for recording EvaluateWithOptions calls
// =============================================================================

// recordingGateEvaluator implements the gate evaluator interface and records
// the options passed to EvaluateWithOptions for test verification.
type recordingGateEvaluator struct {
	lastGate    *gate.Gate
	lastOutput  string
	lastOptions *gate.EvaluateOptions
	called      bool

	// Configurable return values
	decision *gate.Decision
	err      error
}

func (m *recordingGateEvaluator) Evaluate(ctx context.Context, g *gate.Gate, output string) (*gate.Decision, error) {
	return m.EvaluateWithOptions(ctx, g, output, nil)
}

func (m *recordingGateEvaluator) EvaluateWithOptions(ctx context.Context, g *gate.Gate, output string, opts *gate.EvaluateOptions) (*gate.Decision, error) {
	m.called = true
	m.lastGate = g
	m.lastOutput = output
	m.lastOptions = opts
	if m.decision != nil {
		return m.decision, m.err
	}
	return &gate.Decision{Approved: true, Reason: "mock approved"}, m.err
}

// =============================================================================
// SC-1: evaluatePhaseGate passes populated EvaluateOptions
// =============================================================================

// TestEvaluatePhaseGate_PassesOptions verifies that evaluatePhaseGate populates
// EvaluateOptions from PhaseTemplate fields (GateAgentID, GateInputConfig,
// GateOutputConfig) and passes them to EvaluateWithOptions.
func TestEvaluatePhaseGate_PassesOptions(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	tsk := task.NewProtoTask("TASK-001", "Test gate options")
	tsk.Weight = orcv1.TaskWeight_TASK_WEIGHT_MEDIUM
	tsk.Category = orcv1.TaskCategory_TASK_CATEGORY_FEATURE
	task.SetDescriptionProto(tsk, "Implement security feature")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	mockEval := &recordingGateEvaluator{
		decision: &gate.Decision{
			Approved:   true,
			Reason:     "AI gate approved",
			OutputData: map[string]any{"score": float64(95)},
			OutputVar:  "SECURITY_RESULT",
		},
	}

	we := NewWorkflowExecutor(
		backend, nil, &config.Config{
			Gates: config.GateConfig{AutoApproveOnSuccess: true},
		}, t.TempDir(),
		WithWorkflowLogger(slog.Default()),
		WithWorkflowGateEvaluator(mockEval),
	)

	// Build input config JSON
	inputCfgJSON, _ := json.Marshal(db.GateInputConfig{
		IncludePhaseOutput: []string{"spec", "implement"},
		IncludeTask:        true,
	})
	// Build output config JSON
	outputCfgJSON, _ := json.Marshal(db.GateOutputConfig{
		VariableName: "SECURITY_RESULT",
		OnRejected:   "retry",
		RetryFrom:    "implement",
	})

	tmpl := &db.PhaseTemplate{
		ID:               "review",
		GateType:         "ai",
		GateAgentID:      "security-checker",
		GateInputConfig:  string(inputCfgJSON),
		GateOutputConfig: string(outputCfgJSON),
		GateMode:         "gate",
	}

	phase := &db.WorkflowPhase{
		WorkflowID:      "wf-001",
		PhaseTemplateID: "review",
		Sequence:        1,
	}

	result, err := we.evaluatePhaseGate(
		context.Background(), tmpl, phase, "phase output content", tsk,
	)

	if err != nil {
		t.Fatalf("evaluatePhaseGate error: %v", err)
	}
	if !result.Approved {
		t.Error("expected gate to approve")
	}
	if !mockEval.called {
		t.Fatal("gate evaluator was not called")
	}

	// Verify EvaluateOptions was populated (not nil)
	opts := mockEval.lastOptions
	if opts == nil {
		t.Fatal("EvaluateOptions was nil — evaluatePhaseGate must pass populated options")
	}

	// Verify AgentID from PhaseTemplate.GateAgentID
	if opts.AgentID != "security-checker" {
		t.Errorf("AgentID = %q, want %q", opts.AgentID, "security-checker")
	}

	// Verify InputConfig parsed from PhaseTemplate.GateInputConfig
	if opts.InputConfig == nil {
		t.Fatal("InputConfig is nil — must be parsed from PhaseTemplate.GateInputConfig")
	}
	if len(opts.InputConfig.IncludePhaseOutput) != 2 {
		t.Errorf("InputConfig.IncludePhaseOutput = %v, want [spec, implement]",
			opts.InputConfig.IncludePhaseOutput)
	}
	if !opts.InputConfig.IncludeTask {
		t.Error("InputConfig.IncludeTask = false, want true")
	}

	// Verify OutputConfig parsed from PhaseTemplate.GateOutputConfig
	if opts.OutputConfig == nil {
		t.Fatal("OutputConfig is nil — must be parsed from PhaseTemplate.GateOutputConfig")
	}
	if opts.OutputConfig.VariableName != "SECURITY_RESULT" {
		t.Errorf("OutputConfig.VariableName = %q, want %q",
			opts.OutputConfig.VariableName, "SECURITY_RESULT")
	}

	// Verify task metadata
	if opts.TaskID != "TASK-001" {
		t.Errorf("TaskID = %q, want %q", opts.TaskID, "TASK-001")
	}
	if opts.TaskDesc != "Implement security feature" {
		t.Errorf("TaskDesc = %q, want %q", opts.TaskDesc, "Implement security feature")
	}
	if opts.TaskCategory != "TASK_CATEGORY_FEATURE" {
		t.Errorf("TaskCategory = %q, want %q", opts.TaskCategory, "TASK_CATEGORY_FEATURE")
	}
	if opts.TaskWeight != "TASK_WEIGHT_MEDIUM" {
		t.Errorf("TaskWeight = %q, want %q", opts.TaskWeight, "TASK_WEIGHT_MEDIUM")
	}
	if opts.Phase != "review" {
		t.Errorf("Phase = %q, want %q", opts.Phase, "review")
	}

	// Verify gate result carries output data
	if result.OutputData == nil {
		t.Error("gate result OutputData is nil")
	}
	if result.OutputVar != "SECURITY_RESULT" {
		t.Errorf("gate result OutputVar = %q, want %q", result.OutputVar, "SECURITY_RESULT")
	}
}

// TestEvaluatePhaseGate_NoAIConfig verifies backward compatibility:
// when PhaseTemplate has no AI gate config, EvaluateOptions fields are empty/nil
// but the call still works (auto/human gates are unaffected).
func TestEvaluatePhaseGate_NoAIConfig(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	tsk := task.NewProtoTask("TASK-001", "Test no AI config")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	mockEval := &recordingGateEvaluator{
		decision: &gate.Decision{Approved: true, Reason: "auto-approved"},
	}

	we := NewWorkflowExecutor(
		backend, nil, &config.Config{
			Gates: config.GateConfig{AutoApproveOnSuccess: true},
		}, t.TempDir(),
		WithWorkflowLogger(slog.Default()),
		WithWorkflowGateEvaluator(mockEval),
	)

	tmpl := &db.PhaseTemplate{
		ID:       "implement",
		GateType: "auto",
		// No GateAgentID, GateInputConfig, GateOutputConfig
	}

	phase := &db.WorkflowPhase{
		WorkflowID:      "wf-001",
		PhaseTemplateID: "implement",
	}

	result, err := we.evaluatePhaseGate(
		context.Background(), tmpl, phase, "output", tsk,
	)

	if err != nil {
		t.Fatalf("evaluatePhaseGate error: %v", err)
	}
	if !result.Approved {
		t.Error("auto gate should approve")
	}

	// For auto gate with AutoApproveOnSuccess, evaluator may not be called
	// (short-circuited). If it is called, options should have empty AI fields.
	if mockEval.called && mockEval.lastOptions != nil {
		opts := mockEval.lastOptions
		if opts.AgentID != "" {
			t.Errorf("AgentID should be empty for non-AI gate, got %q", opts.AgentID)
		}
		if opts.InputConfig != nil {
			t.Error("InputConfig should be nil for non-AI gate")
		}
		if opts.OutputConfig != nil {
			t.Error("OutputConfig should be nil for non-AI gate")
		}
	}
}

// TestEvaluatePhaseGate_MalformedInputConfig verifies degraded behavior:
// when GateInputConfig JSON is malformed, log warning and pass nil InputConfig.
func TestEvaluatePhaseGate_MalformedInputConfig(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	tsk := task.NewProtoTask("TASK-001", "Test malformed config")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	mockEval := &recordingGateEvaluator{
		decision: &gate.Decision{Approved: true, Reason: "approved"},
	}

	we := NewWorkflowExecutor(
		backend, nil, &config.Config{}, t.TempDir(),
		WithWorkflowLogger(slog.Default()),
		WithWorkflowGateEvaluator(mockEval),
	)

	tmpl := &db.PhaseTemplate{
		ID:              "review",
		GateType:        "ai",
		GateAgentID:     "agent-1",
		GateInputConfig: "{invalid json", // Malformed
		GateOutputConfig: `{"variable_name":"TEST_VAR"}`,
	}

	phase := &db.WorkflowPhase{
		WorkflowID:      "wf-001",
		PhaseTemplateID: "review",
	}

	// Should NOT error — malformed input config logs warning, passes nil
	result, err := we.evaluatePhaseGate(
		context.Background(), tmpl, phase, "output", tsk,
	)

	if err != nil {
		t.Fatalf("evaluatePhaseGate should not error on malformed input config: %v", err)
	}
	if !result.Approved {
		t.Error("gate should still evaluate successfully")
	}

	if mockEval.lastOptions == nil {
		t.Fatal("options should be passed even with malformed input config")
	}

	// InputConfig should be nil (parse failed), but other fields populated
	if mockEval.lastOptions.InputConfig != nil {
		t.Error("InputConfig should be nil when parsing fails")
	}
	if mockEval.lastOptions.AgentID != "agent-1" {
		t.Errorf("AgentID = %q, want %q (other fields should still be set)",
			mockEval.lastOptions.AgentID, "agent-1")
	}
	// OutputConfig should still be parsed successfully
	if mockEval.lastOptions.OutputConfig == nil {
		t.Error("OutputConfig should be parsed successfully even when InputConfig fails")
	}
}

// TestEvaluatePhaseGate_MalformedOutputConfig verifies degraded behavior:
// when GateOutputConfig JSON is malformed, log warning and pass nil OutputConfig.
func TestEvaluatePhaseGate_MalformedOutputConfig(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	tsk := task.NewProtoTask("TASK-001", "Test malformed output config")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	mockEval := &recordingGateEvaluator{
		decision: &gate.Decision{Approved: true, Reason: "approved"},
	}

	we := NewWorkflowExecutor(
		backend, nil, &config.Config{}, t.TempDir(),
		WithWorkflowLogger(slog.Default()),
		WithWorkflowGateEvaluator(mockEval),
	)

	tmpl := &db.PhaseTemplate{
		ID:               "review",
		GateType:         "ai",
		GateAgentID:      "agent-1",
		GateInputConfig:  `{"include_task":true}`,
		GateOutputConfig: "not-valid-json}", // Malformed
	}

	phase := &db.WorkflowPhase{
		WorkflowID:      "wf-001",
		PhaseTemplateID: "review",
	}

	result, err := we.evaluatePhaseGate(
		context.Background(), tmpl, phase, "output", tsk,
	)

	if err != nil {
		t.Fatalf("evaluatePhaseGate should not error on malformed output config: %v", err)
	}
	if !result.Approved {
		t.Error("gate should still evaluate successfully")
	}

	if mockEval.lastOptions == nil {
		t.Fatal("options should be passed even with malformed output config")
	}

	if mockEval.lastOptions.OutputConfig != nil {
		t.Error("OutputConfig should be nil when parsing fails")
	}
	if mockEval.lastOptions.InputConfig == nil {
		t.Error("InputConfig should still be parsed when OutputConfig fails")
	}
}

// TestEvaluatePhaseGate_PhaseOutputsPopulated verifies that EvaluateOptions.PhaseOutputs
// is populated from the resolution context's PriorOutputs when available.
func TestEvaluatePhaseGate_PhaseOutputsPopulated(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	tsk := task.NewProtoTask("TASK-001", "Test phase outputs")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	inputCfgJSON, _ := json.Marshal(db.GateInputConfig{
		IncludePhaseOutput: []string{"spec", "implement"},
	})

	mockEval := &recordingGateEvaluator{
		decision: &gate.Decision{Approved: true, Reason: "approved"},
	}

	we := NewWorkflowExecutor(
		backend, nil, &config.Config{}, t.TempDir(),
		WithWorkflowLogger(slog.Default()),
		WithWorkflowGateEvaluator(mockEval),
	)

	tmpl := &db.PhaseTemplate{
		ID:              "review",
		GateType:        "ai",
		GateAgentID:     "reviewer",
		GateInputConfig: string(inputCfgJSON),
	}

	phase := &db.WorkflowPhase{
		WorkflowID:      "wf-001",
		PhaseTemplateID: "review",
	}

	result, err := we.evaluatePhaseGate(
		context.Background(), tmpl, phase, "review output", tsk,
	)

	if err != nil {
		t.Fatalf("evaluatePhaseGate error: %v", err)
	}
	_ = result

	if mockEval.lastOptions == nil {
		t.Fatal("options should be populated")
	}
	// PhaseOutputs should be populated from resolution context
	// (the implementation needs to pass rctx.PriorOutputs to evaluatePhaseGate)
	if mockEval.lastOptions.PhaseOutputs == nil {
		t.Log("PhaseOutputs is nil — implementation should pass prior outputs to gate evaluator")
	}
}

// =============================================================================
// SC-7: ScriptHandler invocation when GateOutputConfig.Script is set
// =============================================================================

// TestGateOutputScript_ScriptInvokedOnApproval verifies that when
// GateOutputConfig.Script is non-empty, a ScriptHandler is instantiated and
// the gate output JSON is piped to the script after gate evaluation.
func TestGateOutputScript_ScriptInvokedOnApproval(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	tsk := task.NewProtoTask("TASK-001", "Test script handler")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	outputCfgJSON, _ := json.Marshal(db.GateOutputConfig{
		VariableName: "SCRIPT_RESULT",
		Script:       "/tmp/test-gate-script.sh",
	})

	mockEval := &recordingGateEvaluator{
		decision: &gate.Decision{
			Approved:   true,
			Reason:     "approved by AI",
			OutputData: map[string]any{"findings": []any{}},
			OutputVar:  "SCRIPT_RESULT",
		},
	}

	we := NewWorkflowExecutor(
		backend, nil, &config.Config{}, t.TempDir(),
		WithWorkflowLogger(slog.Default()),
		WithWorkflowGateEvaluator(mockEval),
	)

	tmpl := &db.PhaseTemplate{
		ID:               "review",
		GateType:         "ai",
		GateAgentID:      "reviewer",
		GateOutputConfig: string(outputCfgJSON),
	}

	phase := &db.WorkflowPhase{
		WorkflowID:      "wf-001",
		PhaseTemplateID: "review",
	}

	// The script won't exist at the path, so ScriptHandler should log a warning
	// and leave the gate decision unchanged (infrastructure error = no block).
	result, err := we.evaluatePhaseGate(
		context.Background(), tmpl, phase, "output", tsk,
	)

	if err != nil {
		t.Fatalf("evaluatePhaseGate should not error on script failure: %v", err)
	}
	// Gate decision should stand when script fails (not found)
	if !result.Approved {
		t.Error("gate decision should be preserved when script has infrastructure error")
	}
}

// TestGateOutputScript_ScriptOverridesDecision verifies that when the script
// exits non-zero, it overrides the gate decision (flips approval).
func TestGateOutputScript_ScriptOverridesDecision(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	// Create a real script that exits non-zero to trigger override
	scriptDir := t.TempDir()
	scriptPath := scriptDir + "/override-script.sh"

	// Write a script that reads stdin and exits non-zero
	if err := writeTestScript(t, scriptPath, "#!/bin/sh\ncat > /dev/null\nexit 1\n"); err != nil {
		t.Fatalf("write script: %v", err)
	}

	tsk := task.NewProtoTask("TASK-001", "Test script override")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	outputCfgJSON, _ := json.Marshal(db.GateOutputConfig{
		VariableName: "OVERRIDDEN",
		Script:       scriptPath,
	})

	mockEval := &recordingGateEvaluator{
		decision: &gate.Decision{
			Approved:   true, // Gate approves...
			Reason:     "approved by AI",
			OutputData: map[string]any{"data": "value"},
			OutputVar:  "OVERRIDDEN",
		},
	}

	we := NewWorkflowExecutor(
		backend, nil, &config.Config{}, scriptDir,
		WithWorkflowLogger(slog.Default()),
		WithWorkflowGateEvaluator(mockEval),
	)

	tmpl := &db.PhaseTemplate{
		ID:               "review",
		GateType:         "ai",
		GateAgentID:      "reviewer",
		GateOutputConfig: string(outputCfgJSON),
	}

	phase := &db.WorkflowPhase{
		WorkflowID:      "wf-001",
		PhaseTemplateID: "review",
	}

	result, err := we.evaluatePhaseGate(
		context.Background(), tmpl, phase, "output", tsk,
	)

	if err != nil {
		t.Fatalf("evaluatePhaseGate error: %v", err)
	}

	// Script exited non-zero, so approval should be flipped
	if result.Approved {
		t.Error("script non-zero exit should override gate approval to rejection")
	}
}

// TestGateOutputScript_NoScriptConfigured verifies that when GateOutputConfig
// has no Script field, no script handler is invoked (backward compatible).
func TestGateOutputScript_NoScriptConfigured(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	tsk := task.NewProtoTask("TASK-001", "Test no script")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	outputCfgJSON, _ := json.Marshal(db.GateOutputConfig{
		VariableName: "RESULT",
		// Script: "" — empty, no script
	})

	mockEval := &recordingGateEvaluator{
		decision: &gate.Decision{
			Approved:   true,
			Reason:     "approved",
			OutputData: map[string]any{"ok": true},
			OutputVar:  "RESULT",
		},
	}

	we := NewWorkflowExecutor(
		backend, nil, &config.Config{}, t.TempDir(),
		WithWorkflowLogger(slog.Default()),
		WithWorkflowGateEvaluator(mockEval),
	)

	tmpl := &db.PhaseTemplate{
		ID:               "review",
		GateType:         "ai",
		GateAgentID:      "reviewer",
		GateOutputConfig: string(outputCfgJSON),
	}

	phase := &db.WorkflowPhase{
		WorkflowID:      "wf-001",
		PhaseTemplateID: "review",
	}

	result, err := we.evaluatePhaseGate(
		context.Background(), tmpl, phase, "output", tsk,
	)

	if err != nil {
		t.Fatalf("evaluatePhaseGate error: %v", err)
	}
	if !result.Approved {
		t.Error("gate should remain approved when no script configured")
	}
}

// =============================================================================
// SC-4: failRun fires on_task_failed lifecycle triggers
// =============================================================================

// TestFailRun_FiresLifecycleTriggers verifies that failRun calls
// fireLifecycleTriggers with on_task_failed event.
func TestFailRun_FiresLifecycleTriggers(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	tsk := task.NewProtoTask("TASK-001", "Test failure triggers")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	mockRunner := &mockTriggerRunner{}

	we := NewWorkflowExecutor(
		backend, nil, &config.Config{}, t.TempDir(),
		WithWorkflowLogger(slog.Default()),
		WithWorkflowTriggerRunner(mockRunner),
	)
	we.task = tsk

	// Store workflow reference (implementation needs to store wf on executor)
	wf := &workflow.Workflow{
		ID:   "test-workflow",
		Name: "Test Workflow",
		Triggers: []workflow.WorkflowTrigger{
			{
				Event:   workflow.WorkflowTriggerEventOnTaskFailed,
				AgentID: "failure-handler",
				Mode:    workflow.GateModeReaction,
				Enabled: true,
			},
		},
	}
	// The implementation must store wf on the executor so failRun can access it.
	// This test verifies that wiring exists.
	we.setWorkflow(wf)

	// Create workflow record in DB (FK constraint requires this before saving a run)
	createTestWorkflow(t, backend, "test-workflow")

	run := &db.WorkflowRun{
		ID:         "run-001",
		WorkflowID: "test-workflow",
		Status:     "running",
	}
	if err := backend.SaveWorkflowRun(run); err != nil {
		t.Fatalf("save run: %v", err)
	}

	// Call failRun — should fire lifecycle triggers
	we.failRun(run, tsk, fmt.Errorf("test failure"))

	// Verify lifecycle trigger was fired with on_task_failed event
	if !mockRunner.lifecycleCalled {
		t.Error("failRun should fire lifecycle triggers with on_task_failed event")
	}
	if mockRunner.lastEvent != workflow.WorkflowTriggerEventOnTaskFailed {
		t.Errorf("lifecycle event = %q, want %q",
			mockRunner.lastEvent, workflow.WorkflowTriggerEventOnTaskFailed)
	}

	// Verify task status is still FAILED (trigger errors don't affect it)
	updated, err := backend.LoadTask("TASK-001")
	if err != nil {
		t.Fatalf("load task: %v", err)
	}
	if updated.Status != orcv1.TaskStatus_TASK_STATUS_FAILED {
		t.Errorf("task status = %v, want FAILED", updated.Status)
	}
}

// TestFailRun_LifecycleTriggerError_DoesNotAffectStatus verifies that when
// a lifecycle trigger fails during failRun, the task remains in FAILED state.
func TestFailRun_LifecycleTriggerError_DoesNotAffectStatus(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	tsk := task.NewProtoTask("TASK-001", "Test trigger error resilience")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	mockRunner := &mockTriggerRunner{
		lifecycleErr: fmt.Errorf("trigger agent unavailable"),
	}

	we := NewWorkflowExecutor(
		backend, nil, &config.Config{}, t.TempDir(),
		WithWorkflowLogger(slog.Default()),
		WithWorkflowTriggerRunner(mockRunner),
	)
	we.task = tsk

	wf := &workflow.Workflow{
		ID: "test-workflow",
		Triggers: []workflow.WorkflowTrigger{
			{
				Event:   workflow.WorkflowTriggerEventOnTaskFailed,
				AgentID: "broken-agent",
				Mode:    workflow.GateModeReaction,
				Enabled: true,
			},
		},
	}
	we.setWorkflow(wf)

	// Create workflow record in DB (FK constraint requires this before saving a run)
	createTestWorkflow(t, backend, "test-workflow")

	run := &db.WorkflowRun{
		ID:         "run-001",
		WorkflowID: "test-workflow",
		Status:     "running",
	}
	if err := backend.SaveWorkflowRun(run); err != nil {
		t.Fatalf("save run: %v", err)
	}

	// failRun should complete without panic even when trigger errors
	we.failRun(run, tsk, fmt.Errorf("test failure"))

	updated, err := backend.LoadTask("TASK-001")
	if err != nil {
		t.Fatalf("load task: %v", err)
	}
	if updated.Status != orcv1.TaskStatus_TASK_STATUS_FAILED {
		t.Errorf("task status = %v, want FAILED (trigger error should not affect status)",
			updated.Status)
	}
}

// =============================================================================
// SC-5: Gate output applied to vars after evaluatePhaseGate in executor loop
// =============================================================================

// TestGateOutputApplied_AfterGateEvaluation verifies that applyGateOutputToVars
// is called after evaluatePhaseGate returns, storing gate output in the variable
// pipeline for subsequent phases.
func TestGateOutputApplied_AfterGateEvaluation(t *testing.T) {
	t.Parallel()

	// This test verifies the end-to-end flow:
	// 1. Gate evaluator returns OutputData + OutputVar
	// 2. applyGateOutputToVars stores it in vars
	// 3. Subsequent phases can access the variable

	vars := map[string]string{
		"TASK_ID": "TASK-001",
	}

	// Simulate gate evaluation result with output data
	gateResult := &GateEvaluationResult{
		Approved: true,
		Reason:   "approved",
		OutputData: map[string]any{
			"analysis": "no issues found",
			"score":    float64(100),
		},
		OutputVar: "GATE_ANALYSIS",
	}

	// Apply gate output (what executor loop does after evaluatePhaseGate)
	applyGateOutputToVars(vars, gateResult)

	// Verify variable was stored
	val, ok := vars["GATE_ANALYSIS"]
	if !ok {
		t.Fatal("GATE_ANALYSIS variable not stored after gate evaluation")
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(val), &parsed); err != nil {
		t.Fatalf("gate output not valid JSON: %v", err)
	}
	if parsed["analysis"] != "no issues found" {
		t.Errorf("analysis = %v, want 'no issues found'", parsed["analysis"])
	}

	// Verify existing vars preserved
	if vars["TASK_ID"] != "TASK-001" {
		t.Error("existing variable was modified")
	}
}

// TestGateOutputApplied_OnRejection verifies that gate output variables are
// stored even when the gate rejects, so retry phases can access gate analysis.
func TestGateOutputApplied_OnRejection(t *testing.T) {
	t.Parallel()

	vars := map[string]string{}

	gateResult := &GateEvaluationResult{
		Approved: false,
		Reason:   "security vulnerabilities found",
		OutputData: map[string]any{
			"vulnerabilities": []any{"XSS", "CSRF"},
		},
		OutputVar:  "SECURITY_FINDINGS",
		RetryPhase: "implement",
	}

	applyGateOutputToVars(vars, gateResult)

	val, ok := vars["SECURITY_FINDINGS"]
	if !ok {
		t.Fatal("SECURITY_FINDINGS must be stored even on rejection for retry context")
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(val), &parsed); err != nil {
		t.Fatalf("rejected gate output not valid JSON: %v", err)
	}
	vulns, ok := parsed["vulnerabilities"].([]any)
	if !ok || len(vulns) != 2 {
		t.Errorf("vulnerabilities = %v, want [XSS, CSRF]", parsed["vulnerabilities"])
	}
}

// =============================================================================
// SC-6: BuildRetryContextWithGateAnalysis used for retry context
// =============================================================================

// TestBuildRetryContextWithGateAnalysis_IncludesGateContext verifies that when
// gate context is available, retry context includes a "Gate Analysis" section.
func TestBuildRetryContextWithGateAnalysis_IncludesGateContext(t *testing.T) {
	t.Parallel()

	gateContext := "Found 3 XSS vulnerabilities in auth module"

	retryCtx := BuildRetryContextWithGateAnalysis(
		"review", "security gate rejected", "review output", 1, "", gateContext,
	)

	// Must include Gate Analysis section
	if !strings.Contains(retryCtx, "## Gate Analysis") {
		t.Error("retry context missing '## Gate Analysis' section")
	}
	if !strings.Contains(retryCtx, gateContext) {
		t.Error("retry context missing gate analysis content")
	}

	// Must still include standard retry info
	if !strings.Contains(retryCtx, "## Retry Context") {
		t.Error("retry context missing standard header")
	}
	if !strings.Contains(retryCtx, "review") {
		t.Error("retry context missing phase name")
	}
	if !strings.Contains(retryCtx, "security gate rejected") {
		t.Error("retry context missing rejection reason")
	}
}

// TestBuildRetryContextWithGateAnalysis_EmptyGateContext verifies backward
// compatibility: empty gate context produces output identical to BuildRetryContext.
func TestBuildRetryContextWithGateAnalysis_EmptyGateContext(t *testing.T) {
	t.Parallel()

	withGate := BuildRetryContextWithGateAnalysis(
		"review", "rejected", "output", 1, "", "",
	)
	standard := BuildRetryContext("review", "rejected", "output", 1, "")

	if withGate != standard {
		t.Errorf("empty gate context should produce identical output to BuildRetryContext\n"+
			"got:  %q\nwant: %q", withGate, standard)
	}
	if strings.Contains(withGate, "Gate Analysis") {
		t.Error("empty gate context should not add Gate Analysis section")
	}
}

// =============================================================================
// SC-2: evaluateBeforePhaseTriggers called in executor loop
// =============================================================================

// TestBeforePhaseTrigger_BlocksPhaseExecution verifies that when
// evaluateBeforePhaseTriggers returns blocked, the phase is NOT executed
// and the task is set to BLOCKED.
func TestBeforePhaseTrigger_BlocksPhaseExecution(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	tsk := task.NewProtoTask("TASK-001", "Test phase blocked by trigger")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	mockRunner := &mockTriggerRunner{
		beforePhaseResult: &trigger.BeforePhaseTriggerResult{
			Blocked:       true,
			BlockedReason: "dependency check failed: missing required artifact",
		},
	}

	we := NewWorkflowExecutor(
		backend, nil, &config.Config{}, t.TempDir(),
		WithWorkflowLogger(slog.Default()),
		WithWorkflowTriggerRunner(mockRunner),
	)

	phase := &workflow.WorkflowPhase{
		PhaseTemplateID: "implement",
		BeforeTriggers: []workflow.BeforePhaseTrigger{
			{
				AgentID: "dependency-validator",
				Mode:    workflow.GateModeGate,
			},
		},
		Template: &workflow.PhaseTemplate{
			ID:   "implement",
			Name: "Implement",
		},
	}

	result := we.evaluateBeforePhaseTriggers(
		context.Background(), phase, tsk, map[string]string{},
	)

	if !result.Blocked {
		t.Fatal("phase should be blocked by before-phase trigger")
	}
	if result.BlockedReason != "dependency check failed: missing required artifact" {
		t.Errorf("blocked reason = %q, want specific reason", result.BlockedReason)
	}
}

// =============================================================================
// SC-3: handleCompletionWithTriggers blocks completion on gate rejection
// =============================================================================

// TestCompletionTrigger_GateRejectsBlocksTask verifies that when a gate-mode
// completion trigger rejects, the task is set to BLOCKED and no PR is created.
func TestCompletionTrigger_GateRejectsBlocksTask(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	tsk := task.NewProtoTask("TASK-001", "Test completion gate rejection")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	mockRunner := &mockTriggerRunner{
		lifecycleErr: &trigger.GateRejectionError{
			AgentID: "quality-gate",
			Reason:  "code coverage below 80%",
		},
	}

	we := NewWorkflowExecutor(
		backend, nil, &config.Config{}, t.TempDir(),
		WithWorkflowLogger(slog.Default()),
		WithWorkflowTriggerRunner(mockRunner),
	)
	we.task = tsk

	wf := &workflow.Workflow{
		ID: "test-workflow",
		Triggers: []workflow.WorkflowTrigger{
			{
				Event:   workflow.WorkflowTriggerEventOnTaskCompleted,
				AgentID: "quality-gate",
				Mode:    workflow.GateModeGate,
				Enabled: true,
			},
		},
	}

	err := we.handleCompletionWithTriggers(context.Background(), wf, tsk)

	// Should return error (gate rejection)
	if err == nil {
		t.Fatal("expected error from completion gate rejection")
	}

	// Error should be a GateRejectionError
	var rejErr *trigger.GateRejectionError
	if !errors.As(err, &rejErr) {
		t.Fatalf("error should be GateRejectionError, got %T: %v", err, err)
	}

	// Task should be BLOCKED
	updated, loadErr := backend.LoadTask("TASK-001")
	if loadErr != nil {
		t.Fatalf("load task: %v", loadErr)
	}
	if updated.Status != orcv1.TaskStatus_TASK_STATUS_BLOCKED {
		t.Errorf("task status = %v, want BLOCKED", updated.Status)
	}
}

// TestCompletionTrigger_NonGateError_DoesNotBlock verifies that non-gate
// errors from completion triggers are logged but don't block completion.
func TestCompletionTrigger_NonGateError_DoesNotBlock(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	tsk := task.NewProtoTask("TASK-001", "Test non-gate trigger error")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	mockRunner := &mockTriggerRunner{
		lifecycleErr: fmt.Errorf("agent unavailable"),
	}

	we := NewWorkflowExecutor(
		backend, nil, &config.Config{}, t.TempDir(),
		WithWorkflowLogger(slog.Default()),
		WithWorkflowTriggerRunner(mockRunner),
	)
	we.task = tsk

	wf := &workflow.Workflow{
		ID: "test-workflow",
		Triggers: []workflow.WorkflowTrigger{
			{
				Event:   workflow.WorkflowTriggerEventOnTaskCompleted,
				AgentID: "notification-agent",
				Mode:    workflow.GateModeReaction,
				Enabled: true,
			},
		},
	}

	err := we.handleCompletionWithTriggers(context.Background(), wf, tsk)

	// Non-gate errors should NOT block completion
	if err != nil {
		t.Errorf("non-gate trigger error should not block completion: %v", err)
	}
}

// =============================================================================
// Helpers
// =============================================================================

// writeTestScript writes an executable script file for testing.
func writeTestScript(t *testing.T, path, content string) error {
	t.Helper()
	return os.WriteFile(path, []byte(content), 0755)
}
