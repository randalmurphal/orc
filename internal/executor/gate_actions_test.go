// Tests for TASK-690: Wire gate output actions in executor (OnApproved/OnRejected).
//
// These tests define the contract for gate output action resolution and dispatch.
// The implementation will:
//  1. Add OutputConfig field to GateEvaluationResult (SC-7)
//  2. Create resolveApprovedAction / resolveRejectedAction functions (gate_actions.go)
//  3. Replace hardcoded gate handling in executor loop with action dispatch
//
// Coverage mapping:
//
//	SC-1:  TestResolveRejectedAction (fail case) + TestGateAction_OnRejectedFail_FailsTask
//	SC-2:  TestResolveRejectedAction (retry case) + TestGateAction_OnRejectedRetry_RetriesFromPhase
//	SC-3:  TestGateAction_OnRejectedRetry_MaxRetriesFallsToFail
//	SC-4:  TestResolveRejectedAction (skip_phase case) + TestGateAction_OnRejectedSkipPhase_SkipsCurrent
//	SC-5:  TestResolveApprovedAction (skip_phase case) + TestGateAction_OnApprovedSkipPhase_SkipsNextPhase
//	SC-6:  TestResolveApprovedAction (empty/continue cases) + TestGateAction_OnApprovedContinue_Backward
//	SC-7:  TestGateEvaluationResult_OutputConfigField + TestEvaluatePhaseGate_PopulatesOutputConfig
//	SC-8:  TestResolveRejectedAction (run_script case) + TestGateAction_OnRejectedRunScript_ThenFail
//	SC-9:  TestResolveApprovedAction (run_script case) + TestGateAction_OnApprovedRunScript_ThenContinue
//	SC-10: TestRetryFrom_OutputCfgWinsOverTemplate + TestEvaluatePhaseGate_OutputCfgRetryFromPrecedence
//
// Edge cases:
//
//	TestResolveApprovedAction / TestResolveRejectedAction (nil, invalid action)
//	TestGateAction_SkipPhaseOnLastPhase_WarnsAndContinues
//	TestGateAction_SkipGates_NoOutputConfig
//	TestGateAction_RunScriptOverride_FlipsToFail
//
// Failure modes:
//
//	TestGateAction_RetryNoRetryFrom_FailsWithError
//	TestGateAction_RunScriptEmptyPath_WarnsAndAppliesSecondary
//	TestResolveRejectedAction (invalid action → legacy)
package executor

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"testing"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/gate"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/internal/workflow"
)

// =============================================================================
// SC-7: GateEvaluationResult carries OutputConfig from parsed GateOutputConfig
// =============================================================================

// TestGateEvaluationResult_OutputConfigField verifies that GateEvaluationResult
// has an OutputConfig field that can carry the parsed GateOutputConfig.
func TestGateEvaluationResult_OutputConfigField(t *testing.T) {
	t.Parallel()

	outputCfg := &db.GateOutputConfig{
		VariableName: "REVIEW_RESULT",
		OnApproved:   "continue",
		OnRejected:   "fail",
		RetryFrom:    "implement",
		Script:       "/tmp/gate-script.sh",
	}

	result := &GateEvaluationResult{
		Approved:     true,
		Reason:       "approved",
		OutputConfig: outputCfg,
	}

	if result.OutputConfig == nil {
		t.Fatal("OutputConfig must be settable on GateEvaluationResult")
	}
	if result.OutputConfig.OnApproved != "continue" {
		t.Errorf("OutputConfig.OnApproved = %q, want %q", result.OutputConfig.OnApproved, "continue")
	}
	if result.OutputConfig.OnRejected != "fail" {
		t.Errorf("OutputConfig.OnRejected = %q, want %q", result.OutputConfig.OnRejected, "fail")
	}
	if result.OutputConfig.RetryFrom != "implement" {
		t.Errorf("OutputConfig.RetryFrom = %q, want %q", result.OutputConfig.RetryFrom, "implement")
	}
	if result.OutputConfig.Script != "/tmp/gate-script.sh" {
		t.Errorf("OutputConfig.Script = %q, want %q", result.OutputConfig.Script, "/tmp/gate-script.sh")
	}
}

// TestGateEvaluationResult_OutputConfigNilByDefault verifies backward compatibility:
// when no output config is parsed, OutputConfig is nil.
func TestGateEvaluationResult_OutputConfigNilByDefault(t *testing.T) {
	t.Parallel()

	result := &GateEvaluationResult{
		Approved: true,
		Reason:   "auto-approved",
	}

	if result.OutputConfig != nil {
		t.Errorf("OutputConfig should be nil by default, got %+v", result.OutputConfig)
	}
}

// TestEvaluatePhaseGate_PopulatesOutputConfig verifies that evaluatePhaseGate
// populates the OutputConfig field on GateEvaluationResult from the parsed
// GateOutputConfig in PhaseTemplate.
func TestEvaluatePhaseGate_PopulatesOutputConfig(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	tsk := task.NewProtoTask("TASK-OCF-001", "Test output config population")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	outputCfgJSON, _ := json.Marshal(db.GateOutputConfig{
		VariableName: "GATE_VAR",
		OnApproved:   "continue",
		OnRejected:   "fail",
		RetryFrom:    "implement",
	})

	mockEval := &recordingGateEvaluator{
		decision: &gate.Decision{Approved: true, Reason: "approved"},
	}

	we := NewWorkflowExecutor(
		backend, nil, testGlobalDBFrom(backend), &config.Config{}, t.TempDir(),
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

	// OutputConfig MUST be populated from the parsed GateOutputConfig
	if result.OutputConfig == nil {
		t.Fatal("OutputConfig is nil — evaluatePhaseGate must populate it from PhaseTemplate.GateOutputConfig")
	}
	if result.OutputConfig.OnApproved != "continue" {
		t.Errorf("OutputConfig.OnApproved = %q, want %q", result.OutputConfig.OnApproved, "continue")
	}
	if result.OutputConfig.OnRejected != "fail" {
		t.Errorf("OutputConfig.OnRejected = %q, want %q", result.OutputConfig.OnRejected, "fail")
	}
	if result.OutputConfig.RetryFrom != "implement" {
		t.Errorf("OutputConfig.RetryFrom = %q, want %q", result.OutputConfig.RetryFrom, "implement")
	}
}

// TestEvaluatePhaseGate_NoOutputConfig_NilOutputConfig verifies that when
// PhaseTemplate has no GateOutputConfig, OutputConfig is nil (backward compat).
func TestEvaluatePhaseGate_NoOutputConfig_NilOutputConfig(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	tsk := task.NewProtoTask("TASK-OCF-002", "Test no output config")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	mockEval := &recordingGateEvaluator{
		decision: &gate.Decision{Approved: true, Reason: "auto-approved"},
	}

	we := NewWorkflowExecutor(
		backend, nil, testGlobalDBFrom(backend), &config.Config{
			Gates: config.GateConfig{AutoApproveOnSuccess: true},
		}, t.TempDir(),
		WithWorkflowLogger(slog.Default()),
		WithWorkflowGateEvaluator(mockEval),
	)

	tmpl := &db.PhaseTemplate{
		ID:       "implement",
		GateType: "auto",
		// No GateOutputConfig
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
	if result.OutputConfig != nil {
		t.Errorf("OutputConfig should be nil when no GateOutputConfig configured, got %+v", result.OutputConfig)
	}
}

func TestEvaluatePhaseGate_InvalidOutputConfig_ReturnsError(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	tsk := task.NewProtoTask("TASK-OCF-003", "Test invalid output config")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	we := NewWorkflowExecutor(
		backend, nil, testGlobalDBFrom(backend), &config.Config{}, t.TempDir(),
		WithWorkflowLogger(slog.Default()),
		WithWorkflowGateEvaluator(&recordingGateEvaluator{decision: &gate.Decision{Approved: true, Reason: "approved"}}),
	)

	tmpl := &db.PhaseTemplate{
		ID:               "review",
		GateType:         "ai",
		GateOutputConfig: "{not-json",
	}
	phase := &db.WorkflowPhase{WorkflowID: "wf-001", PhaseTemplateID: "review"}

	_, err := we.evaluatePhaseGate(context.Background(), tmpl, phase, "output", tsk)
	if err == nil {
		t.Fatal("evaluatePhaseGate should fail on invalid GateOutputConfig")
	}
}

func TestEvaluatePhaseGate_InvalidInputConfig_ReturnsError(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	tsk := task.NewProtoTask("TASK-OCF-004", "Test invalid input config")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	we := NewWorkflowExecutor(
		backend, nil, testGlobalDBFrom(backend), &config.Config{}, t.TempDir(),
		WithWorkflowLogger(slog.Default()),
		WithWorkflowGateEvaluator(&recordingGateEvaluator{decision: &gate.Decision{Approved: true, Reason: "approved"}}),
	)

	tmpl := &db.PhaseTemplate{
		ID:              "review",
		GateType:        "ai",
		GateInputConfig: "{not-json",
	}
	phase := &db.WorkflowPhase{WorkflowID: "wf-001", PhaseTemplateID: "review"}

	_, err := we.evaluatePhaseGate(context.Background(), tmpl, phase, "output", tsk)
	if err == nil {
		t.Fatal("evaluatePhaseGate should fail on invalid GateInputConfig")
	}
}

func TestEvaluatePhaseGate_InvalidScriptPath_ReturnsError(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	tsk := task.NewProtoTask("TASK-OCF-005", "Test invalid gate script path")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	outputCfgJSON, _ := json.Marshal(db.GateOutputConfig{
		Script: "../bad-script.sh",
	})

	we := NewWorkflowExecutor(
		backend, nil, testGlobalDBFrom(backend), &config.Config{}, t.TempDir(),
		WithWorkflowLogger(slog.Default()),
		WithWorkflowGateEvaluator(&recordingGateEvaluator{decision: &gate.Decision{Approved: true, Reason: "approved"}}),
	)

	tmpl := &db.PhaseTemplate{
		ID:               "review",
		GateType:         "ai",
		GateOutputConfig: string(outputCfgJSON),
	}
	phase := &db.WorkflowPhase{WorkflowID: "wf-001", PhaseTemplateID: "review"}

	_, err := we.evaluatePhaseGate(context.Background(), tmpl, phase, "output", tsk)
	if err == nil {
		t.Fatal("evaluatePhaseGate should fail on invalid gate script path")
	}
}

func TestEvaluatePhaseGate_HumanGateUsesPendingDecisionStore(t *testing.T) {
	t.Parallel()
	projectDir := t.TempDir()
	backend, err := storage.NewDatabaseBackend(projectDir, nil)
	if err != nil {
		t.Fatalf("create backend: %v", err)
	}
	t.Cleanup(func() {
		_ = backend.Close()
	})

	tsk := task.NewProtoTask("TASK-HUMAN-GATE-001", "Test human gate headless execution")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	mockPub := newLoopTestPublisher()
	we := NewWorkflowExecutor(
		backend, nil, testGlobalDBFrom(backend), &config.Config{}, t.TempDir(),
		WithWorkflowLogger(slog.Default()),
		WithWorkflowPublisher(mockPub),
	)

	tmpl := &db.PhaseTemplate{
		ID:       "plan",
		GateType: "human",
	}
	phase := &db.WorkflowPhase{WorkflowID: "wf-001", PhaseTemplateID: "plan"}

	result, err := we.evaluatePhaseGate(context.Background(), tmpl, phase, `{"status":"complete"}`, tsk)
	if err != nil {
		t.Fatalf("evaluatePhaseGate error: %v", err)
	}
	if !result.Pending {
		t.Fatalf("gate result should be pending, got %+v", result)
	}
	if result.Approved {
		t.Fatalf("pending human gate should not be approved: %+v", result)
	}

	pending := we.pendingDecisions.List(we.projectIDForEvents())
	if len(pending) != 1 {
		t.Fatalf("pending decisions = %d, want 1", len(pending))
	}
	if pending[0].TaskID != tsk.Id {
		t.Fatalf("pending decision task_id = %q, want %q", pending[0].TaskID, tsk.Id)
	}

	foundDecisionEvent := false
	for _, ev := range mockPub.events {
		if ev.Type == events.EventDecisionRequired && ev.TaskID == tsk.Id {
			foundDecisionEvent = true
			break
		}
	}
	if !foundDecisionEvent {
		t.Fatal("expected decision_required event for headless human gate")
	}
}

// =============================================================================
// Action Resolution: resolveApprovedAction (solitary, pure function tests)
// =============================================================================

// TestResolveApprovedAction verifies that resolveApprovedAction maps
// GateOutputConfig.OnApproved to the correct workflow.GateAction.
func TestResolveApprovedAction(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		cfg      *db.GateOutputConfig
		expected workflow.GateAction
	}{
		{
			name:     "nil config → continue (backward compat)",
			cfg:      nil,
			expected: workflow.GateActionContinue,
		},
		{
			name:     "empty OnApproved → continue (backward compat)",
			cfg:      &db.GateOutputConfig{OnApproved: ""},
			expected: workflow.GateActionContinue,
		},
		{
			name:     "continue → continue",
			cfg:      &db.GateOutputConfig{OnApproved: "continue"},
			expected: workflow.GateActionContinue,
		},
		{
			name:     "skip_phase → skip_phase",
			cfg:      &db.GateOutputConfig{OnApproved: "skip_phase"},
			expected: workflow.GateActionSkipPhase,
		},
		{
			name:     "run_script → run_script",
			cfg:      &db.GateOutputConfig{OnApproved: "run_script"},
			expected: workflow.GateActionRunScript,
		},
		{
			name:     "invalid action → continue (legacy behavior)",
			cfg:      &db.GateOutputConfig{OnApproved: "foobar"},
			expected: workflow.GateActionContinue,
		},
		{
			name:     "fail is valid for approved (unusual but valid)",
			cfg:      &db.GateOutputConfig{OnApproved: "fail"},
			expected: workflow.GateActionFail,
		},
		{
			name:     "retry is valid for approved (unusual but valid)",
			cfg:      &db.GateOutputConfig{OnApproved: "retry"},
			expected: workflow.GateActionRetry,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := resolveApprovedAction(tt.cfg)
			if got != tt.expected {
				t.Errorf("resolveApprovedAction() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// =============================================================================
// Action Resolution: resolveRejectedAction (solitary, pure function tests)
// =============================================================================

// TestResolveRejectedAction verifies that resolveRejectedAction maps
// GateOutputConfig.OnRejected to the correct workflow.GateAction.
// Empty/nil returns empty string (legacy behavior: phase-specific dispatch).
func TestResolveRejectedAction(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		cfg      *db.GateOutputConfig
		expected workflow.GateAction
	}{
		{
			name:     "nil config → empty (legacy behavior)",
			cfg:      nil,
			expected: "",
		},
		{
			name:     "empty OnRejected → empty (legacy behavior)",
			cfg:      &db.GateOutputConfig{OnRejected: ""},
			expected: "",
		},
		{
			name:     "fail → fail (SC-1)",
			cfg:      &db.GateOutputConfig{OnRejected: "fail"},
			expected: workflow.GateActionFail,
		},
		{
			name:     "retry → retry (SC-2)",
			cfg:      &db.GateOutputConfig{OnRejected: "retry"},
			expected: workflow.GateActionRetry,
		},
		{
			name:     "skip_phase → skip_phase (SC-4)",
			cfg:      &db.GateOutputConfig{OnRejected: "skip_phase"},
			expected: workflow.GateActionSkipPhase,
		},
		{
			name:     "run_script → run_script (SC-8)",
			cfg:      &db.GateOutputConfig{OnRejected: "run_script"},
			expected: workflow.GateActionRunScript,
		},
		{
			name:     "continue → continue",
			cfg:      &db.GateOutputConfig{OnRejected: "continue"},
			expected: workflow.GateActionContinue,
		},
		{
			name:     "invalid action → empty (legacy behavior)",
			cfg:      &db.GateOutputConfig{OnRejected: "foobar"},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := resolveRejectedAction(tt.cfg)
			if got != tt.expected {
				t.Errorf("resolveRejectedAction() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// =============================================================================
// SC-10: outputCfg.RetryFrom takes precedence over tmpl.RetryFromPhase
// =============================================================================

// TestRetryFrom_OutputCfgWinsOverTemplate verifies that when both
// outputCfg.RetryFrom and tmpl.RetryFromPhase are set, outputCfg.RetryFrom
// is used for determining the retry target.
func TestRetryFrom_OutputCfgWinsOverTemplate(t *testing.T) {
	t.Parallel()

	// Scenario: tmpl says retry from "spec", outputCfg says retry from "tdd_write".
	// outputCfg.RetryFrom should win.
	outputCfg := &db.GateOutputConfig{
		OnRejected: "retry",
		RetryFrom:  "tdd_write",
	}

	tmplRetryFrom := "spec" // This should be overridden

	// The implementation should prefer outputCfg.RetryFrom over tmpl.RetryFromPhase
	retryTarget := resolveRetryFrom(outputCfg, tmplRetryFrom)
	if retryTarget != "tdd_write" {
		t.Errorf("resolveRetryFrom() = %q, want %q (outputCfg.RetryFrom should win)", retryTarget, "tdd_write")
	}
}

// TestRetryFrom_FallsBackToTemplate verifies that when outputCfg.RetryFrom
// is empty, tmpl.RetryFromPhase is used.
func TestRetryFrom_FallsBackToTemplate(t *testing.T) {
	t.Parallel()

	outputCfg := &db.GateOutputConfig{
		OnRejected: "retry",
		RetryFrom:  "", // Empty — fall back to template
	}

	tmplRetryFrom := "implement"

	retryTarget := resolveRetryFrom(outputCfg, tmplRetryFrom)
	if retryTarget != "implement" {
		t.Errorf("resolveRetryFrom() = %q, want %q (should fall back to template)", retryTarget, "implement")
	}
}

// TestRetryFrom_NilConfig_FallsBackToTemplate verifies that when outputCfg
// is nil, tmpl.RetryFromPhase is used.
func TestRetryFrom_NilConfig_FallsBackToTemplate(t *testing.T) {
	t.Parallel()

	retryTarget := resolveRetryFrom(nil, "implement")
	if retryTarget != "implement" {
		t.Errorf("resolveRetryFrom(nil, ...) = %q, want %q", retryTarget, "implement")
	}
}

// TestEvaluatePhaseGate_OutputCfgRetryFromPrecedence is an integration test
// verifying that when a gate rejects and outputCfg.RetryFrom is set,
// GateEvaluationResult.RetryPhase uses outputCfg.RetryFrom, NOT tmpl.RetryFromPhase.
func TestEvaluatePhaseGate_OutputCfgRetryFromPrecedence(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	tsk := task.NewProtoTask("TASK-RFP-001", "Test retry from precedence")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	outputCfgJSON, _ := json.Marshal(db.GateOutputConfig{
		OnRejected: "retry",
		RetryFrom:  "tdd_write", // OutputConfig says retry from tdd_write
	})

	mockEval := &recordingGateEvaluator{
		decision: &gate.Decision{Approved: false, Reason: "needs work"},
	}

	we := NewWorkflowExecutor(
		backend, nil, testGlobalDBFrom(backend), &config.Config{}, t.TempDir(),
		WithWorkflowLogger(slog.Default()),
		WithWorkflowGateEvaluator(mockEval),
	)

	tmpl := &db.PhaseTemplate{
		ID:               "review",
		GateType:         "ai",
		GateOutputConfig: string(outputCfgJSON),
		RetryFromPhase:   "spec", // Template says retry from spec
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

	// RetryPhase should come from outputCfg.RetryFrom (tdd_write), not tmpl.RetryFromPhase (spec)
	if result.RetryPhase != "tdd_write" {
		t.Errorf("RetryPhase = %q, want %q (outputCfg.RetryFrom should take precedence over tmpl.RetryFromPhase)",
			result.RetryPhase, "tdd_write")
	}
}

// =============================================================================
// SC-1: on_rejected: fail immediately fails the task regardless of phase type
// =============================================================================

// TestGateAction_OnRejectedFail_FailsTask verifies that when a non-review phase
// gate rejects with on_rejected=fail, the task fails (unlike legacy behavior which
// continues for non-review phases).
func TestGateAction_OnRejectedFail_FailsTask(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	// Configure implement phase with on_rejected: fail
	outputCfgJSON, _ := json.Marshal(db.GateOutputConfig{
		OnRejected: "fail",
	})
	if err := backend.SavePhaseTemplate(&db.PhaseTemplate{
		ID:               "implement",
		Name:             "implement",
		PromptSource:     "db",
		PromptContent:    "Test prompt for implement",
		GateType:         "ai",
		GateOutputConfig: string(outputCfgJSON),
	}); err != nil {
		t.Fatalf("save phase template: %v", err)
	}

	setupSinglePhaseWorkflow(t, backend, "fail-test-wf", "implement")

	tsk := task.NewProtoTask("TASK-FAIL-001", "Test on_rejected: fail")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	wfID := "fail-test-wf"
	tsk.WorkflowId = &wfID
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	mockEval := &recordingGateEvaluator{
		decision: &gate.Decision{Approved: false, Reason: "quality gate failed"},
	}

	mockTE := NewMockTurnExecutor(`{"status": "complete", "summary": "Done"}`)

	we := NewWorkflowExecutor(
		backend, backend.DB(), testGlobalDBFrom(backend), &config.Config{}, t.TempDir(),
		WithWorkflowLogger(slog.Default()),
		WithWorkflowGateEvaluator(mockEval),
		WithWorkflowTurnExecutor(mockTE),
	)

	_, err := we.Run(context.Background(), "fail-test-wf", WorkflowRunOptions{
		ContextType: ContextTask,
		TaskID:      tsk.Id,
	})

	// Task should fail (on_rejected: fail), NOT continue (legacy behavior for non-review)
	if err == nil {
		t.Fatal("expected error from on_rejected: fail, but got nil")
	}

	updated, loadErr := backend.LoadTask(tsk.Id)
	if loadErr != nil {
		t.Fatalf("load task: %v", loadErr)
	}
	if updated.Status != orcv1.TaskStatus_TASK_STATUS_FAILED {
		t.Errorf("task status = %v, want FAILED (on_rejected: fail should fail regardless of phase type)",
			updated.Status)
	}
}

// =============================================================================
// SC-2: on_rejected: retry retries from retry_from phase using loop counter
// =============================================================================

// TestGateAction_OnRejectedRetry_RetriesFromPhase verifies that when a gate
// rejects with on_rejected=retry, execution retries from the configured retry_from
// phase and uses the loop counter.
func TestGateAction_OnRejectedRetry_RetriesFromPhase(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	// Configure review phase with on_rejected: retry + retry_from: implement
	outputCfgJSON, _ := json.Marshal(db.GateOutputConfig{
		OnRejected: "retry",
		RetryFrom:  "implement",
	})
	if err := backend.SavePhaseTemplate(&db.PhaseTemplate{
		ID:               "review",
		Name:             "review",
		PromptSource:     "db",
		PromptContent:    "Test prompt for review",
		GateType:         "ai",
		GateOutputConfig: string(outputCfgJSON),
	}); err != nil {
		t.Fatalf("save review template: %v", err)
	}
	// implement phase has no special gate config
	if err := backend.SavePhaseTemplate(&db.PhaseTemplate{
		ID:            "implement",
		Name:          "implement",
		PromptSource:  "db",
		PromptContent: "Test prompt for implement",
	}); err != nil {
		t.Fatalf("save implement template: %v", err)
	}

	setupTwoPhaseWorkflow(t, backend, "retry-test-wf", "implement", "review")

	tsk := task.NewProtoTask("TASK-RETRY-001", "Test on_rejected: retry")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	wfID := "retry-test-wf"
	tsk.WorkflowId = &wfID
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// First review rejects, second review approves
	callCount := 0
	mockEval := &configGateEvaluator{
		decisionFn: func(g *gate.Gate, output string, opts *gate.EvaluateOptions) (*gate.Decision, error) {
			callCount++
			if opts.Phase == "review" && callCount == 1 {
				return &gate.Decision{Approved: false, Reason: "needs work"}, nil
			}
			return &gate.Decision{Approved: true, Reason: "approved"}, nil
		},
	}

	mockTE := NewMockTurnExecutor(`{"status": "complete", "summary": "Done"}`)

	we := NewWorkflowExecutor(
		backend, backend.DB(), testGlobalDBFrom(backend), &config.Config{}, t.TempDir(),
		WithWorkflowLogger(slog.Default()),
		WithWorkflowGateEvaluator(mockEval),
		WithWorkflowTurnExecutor(mockTE),
	)

	_, err := we.Run(context.Background(), "retry-test-wf", WorkflowRunOptions{
		ContextType: ContextTask,
		TaskID:      tsk.Id,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Implement should have been called twice (initial + retry)
	if mockTE.CallCount() < 3 {
		t.Errorf("expected at least 3 turn executor calls (implement + review + implement retry), got %d",
			mockTE.CallCount())
	}

	// Task should complete successfully after retry
	updated, loadErr := backend.LoadTask(tsk.Id)
	if loadErr != nil {
		t.Fatalf("load task: %v", loadErr)
	}
	if updated.Status == orcv1.TaskStatus_TASK_STATUS_FAILED {
		t.Error("task should not be FAILED — retry should recover after second review passes")
	}
}

// =============================================================================
// SC-3: on_rejected: retry at max retries falls through to fail
// =============================================================================

// TestGateAction_OnRejectedRetry_MaxRetriesFallsToFail verifies that when
// on_rejected=retry but max retries are exhausted, the task fails.
func TestGateAction_OnRejectedRetry_MaxRetriesFallsToFail(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	outputCfgJSON, _ := json.Marshal(db.GateOutputConfig{
		OnRejected: "retry",
		RetryFrom:  "implement",
	})
	if err := backend.SavePhaseTemplate(&db.PhaseTemplate{
		ID:               "review",
		Name:             "review",
		PromptSource:     "db",
		PromptContent:    "Test prompt for review",
		GateType:         "ai",
		GateOutputConfig: string(outputCfgJSON),
	}); err != nil {
		t.Fatalf("save review template: %v", err)
	}
	if err := backend.SavePhaseTemplate(&db.PhaseTemplate{
		ID:            "implement",
		Name:          "implement",
		PromptSource:  "db",
		PromptContent: "Test prompt for implement",
	}); err != nil {
		t.Fatalf("save implement template: %v", err)
	}

	setupTwoPhaseWorkflow(t, backend, "maxretry-wf", "implement", "review")

	tsk := task.NewProtoTask("TASK-MAXRETRY-001", "Test retry max exceeded")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	wfID := "maxretry-wf"
	tsk.WorkflowId = &wfID
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Gate always rejects — retry should exhaust
	mockEval := &configGateEvaluator{
		decisionFn: func(g *gate.Gate, output string, opts *gate.EvaluateOptions) (*gate.Decision, error) {
			if opts.Phase == "review" {
				return &gate.Decision{Approved: false, Reason: "still bad"}, nil
			}
			return &gate.Decision{Approved: true, Reason: "ok"}, nil
		},
	}

	mockTE := NewMockTurnExecutor(`{"status": "complete", "summary": "Done"}`)

	we := NewWorkflowExecutor(
		backend, backend.DB(), testGlobalDBFrom(backend), &config.Config{
			Retry: config.RetryConfig{MaxRetries: 2},
		}, t.TempDir(),
		WithWorkflowLogger(slog.Default()),
		WithWorkflowGateEvaluator(mockEval),
		WithWorkflowTurnExecutor(mockTE),
	)

	_, err := we.Run(context.Background(), "maxretry-wf", WorkflowRunOptions{
		ContextType: ContextTask,
		TaskID:      tsk.Id,
	})

	// Should fail after exhausting retries
	if err == nil {
		t.Fatal("expected error after max retries exhausted")
	}

	updated, loadErr := backend.LoadTask(tsk.Id)
	if loadErr != nil {
		t.Fatalf("load task: %v", loadErr)
	}
	if updated.Status != orcv1.TaskStatus_TASK_STATUS_FAILED {
		t.Errorf("task status = %v, want FAILED after retry exhaustion", updated.Status)
	}
}

func TestGateAction_PendingDecision_BlocksAndClearsExecutor(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	if err := backend.SavePhaseTemplate(&db.PhaseTemplate{
		ID:            "review",
		Name:          "review",
		PromptSource:  "db",
		PromptContent: "Test prompt for review",
		GateType:      "ai",
	}); err != nil {
		t.Fatalf("save review template: %v", err)
	}

	setupSinglePhaseWorkflow(t, backend, "pending-gate-wf", "review")

	tsk := task.NewProtoTask("TASK-PENDING-001", "Test pending gate")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	wfID := "pending-gate-wf"
	tsk.WorkflowId = &wfID
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	mockEval := &configGateEvaluator{
		decisionFn: func(g *gate.Gate, output string, opts *gate.EvaluateOptions) (*gate.Decision, error) {
			return &gate.Decision{Pending: true, Reason: "awaiting human decision"}, nil
		},
	}

	mockTE := NewMockTurnExecutor(`{"status": "complete", "summary": "Done"}`)

	we := NewWorkflowExecutor(
		backend, backend.DB(), testGlobalDBFrom(backend), &config.Config{}, t.TempDir(),
		WithWorkflowLogger(slog.Default()),
		WithWorkflowGateEvaluator(mockEval),
		WithWorkflowTurnExecutor(mockTE),
	)

	_, err := we.Run(context.Background(), "pending-gate-wf", WorkflowRunOptions{
		ContextType: ContextTask,
		TaskID:      tsk.Id,
	})
	if err == nil {
		t.Fatal("expected blocked error for pending gate")
	}
	if !errors.Is(err, ErrTaskBlocked) {
		t.Fatalf("Run() error = %v, want ErrTaskBlocked", err)
	}

	updated, loadErr := backend.LoadTask(tsk.Id)
	if loadErr != nil {
		t.Fatalf("load task: %v", loadErr)
	}
	if updated.Status != orcv1.TaskStatus_TASK_STATUS_BLOCKED {
		t.Errorf("task status = %v, want BLOCKED", updated.Status)
	}
	if updated.ExecutorPid != 0 {
		t.Errorf("executor pid = %d, want 0 after pending gate", updated.ExecutorPid)
	}

	runs, loadErr := backend.ListWorkflowRuns(db.WorkflowRunListOpts{TaskID: tsk.Id})
	if loadErr != nil {
		t.Fatalf("list workflow runs: %v", loadErr)
	}
	if len(runs) != 1 {
		t.Fatalf("workflow runs = %d, want 1", len(runs))
	}
	if runs[0].Status != string(workflow.RunStatusCompleted) {
		t.Errorf("run status = %q, want %q", runs[0].Status, workflow.RunStatusCompleted)
	}
}

func TestGateAction_ReviewRequirePass_BlocksContinuationOnRejectedReview(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	outputCfgJSON, _ := json.Marshal(db.GateOutputConfig{
		OnRejected: "continue",
	})
	if err := backend.SavePhaseTemplate(&db.PhaseTemplate{
		ID:               "review",
		Name:             "review",
		PromptSource:     "db",
		PromptContent:    "Test prompt for review",
		GateType:         "ai",
		GateOutputConfig: string(outputCfgJSON),
	}); err != nil {
		t.Fatalf("save review template: %v", err)
	}
	if err := backend.SavePhaseTemplate(&db.PhaseTemplate{
		ID:            "docs",
		Name:          "docs",
		PromptSource:  "db",
		PromptContent: "Test prompt for docs",
	}); err != nil {
		t.Fatalf("save docs template: %v", err)
	}

	setupTwoPhaseWorkflow(t, backend, "review-require-pass-wf", "review", "docs")

	tsk := task.NewProtoTask("TASK-REVIEW-PASS-001", "Review must pass")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	wfID := "review-require-pass-wf"
	tsk.WorkflowId = &wfID
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	mockEval := &configGateEvaluator{
		decisionFn: func(g *gate.Gate, output string, opts *gate.EvaluateOptions) (*gate.Decision, error) {
			if opts.Phase == "review" {
				return &gate.Decision{Approved: false, Reason: "still has issues"}, nil
			}
			return &gate.Decision{Approved: true, Reason: "approved"}, nil
		},
	}

	mockTE := NewMockTurnExecutor(`{"status": "complete", "summary": "Done"}`)

	we := NewWorkflowExecutor(
		backend, backend.DB(), testGlobalDBFrom(backend), &config.Config{
			Review: config.ReviewConfig{RequirePass: true},
		}, t.TempDir(),
		WithWorkflowLogger(slog.Default()),
		WithWorkflowGateEvaluator(mockEval),
		WithWorkflowTurnExecutor(mockTE),
	)

	_, err := we.Run(context.Background(), "review-require-pass-wf", WorkflowRunOptions{
		ContextType: ContextTask,
		TaskID:      tsk.Id,
	})
	if err == nil {
		t.Fatal("expected rejected review to fail when review.require_pass is true")
	}

	updated, loadErr := backend.LoadTask(tsk.Id)
	if loadErr != nil {
		t.Fatalf("load task: %v", loadErr)
	}
	if updated.Status != orcv1.TaskStatus_TASK_STATUS_FAILED {
		t.Errorf("task status = %v, want FAILED", updated.Status)
	}
}

// =============================================================================
// SC-4: on_rejected: skip_phase skips current phase and continues to next
// =============================================================================

// TestGateAction_OnRejectedSkipPhase_SkipsCurrent verifies that when a gate
// rejects with on_rejected=skip_phase, the current phase is skipped (its
// rejection doesn't cause retry or failure) and execution continues to the next.
func TestGateAction_OnRejectedSkipPhase_SkipsCurrent(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	// Configure implement gate with on_rejected: skip_phase
	outputCfgJSON, _ := json.Marshal(db.GateOutputConfig{
		OnRejected: "skip_phase",
	})
	if err := backend.SavePhaseTemplate(&db.PhaseTemplate{
		ID:               "implement",
		Name:             "implement",
		PromptSource:     "db",
		PromptContent:    "Test prompt",
		GateType:         "ai",
		GateOutputConfig: string(outputCfgJSON),
	}); err != nil {
		t.Fatalf("save template: %v", err)
	}
	if err := backend.SavePhaseTemplate(&db.PhaseTemplate{
		ID:            "review",
		Name:          "review",
		PromptSource:  "db",
		PromptContent: "Test prompt",
	}); err != nil {
		t.Fatalf("save template: %v", err)
	}

	setupTwoPhaseWorkflow(t, backend, "skip-rejected-wf", "implement", "review")

	tsk := task.NewProtoTask("TASK-SKIP-R-001", "Test on_rejected: skip_phase")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	wfID := "skip-rejected-wf"
	tsk.WorkflowId = &wfID
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	mockEval := &configGateEvaluator{
		decisionFn: func(g *gate.Gate, output string, opts *gate.EvaluateOptions) (*gate.Decision, error) {
			if opts.Phase == "implement" {
				return &gate.Decision{Approved: false, Reason: "rejected"}, nil
			}
			return &gate.Decision{Approved: true, Reason: "approved"}, nil
		},
	}

	mockTE := NewMockTurnExecutor(`{"status": "complete", "summary": "Done"}`)

	we := NewWorkflowExecutor(
		backend, backend.DB(), testGlobalDBFrom(backend), &config.Config{}, t.TempDir(),
		WithWorkflowLogger(slog.Default()),
		WithWorkflowGateEvaluator(mockEval),
		WithWorkflowTurnExecutor(mockTE),
		WithSkipGates(false),
	)

	_, err := we.Run(context.Background(), "skip-rejected-wf", WorkflowRunOptions{
		ContextType: ContextTask,
		TaskID:      tsk.Id,
	})

	// Should NOT fail — rejection with skip_phase continues to next
	if err != nil {
		t.Fatalf("on_rejected: skip_phase should not fail, got: %v", err)
	}

	// Review phase should have executed (implement gate rejected → skipped, then review runs)
	if mockTE.CallCount() < 2 {
		t.Errorf("expected at least 2 phases executed (implement + review), got %d calls",
			mockTE.CallCount())
	}
}

// =============================================================================
// SC-5: on_approved: skip_phase skips the NEXT phase in sequence
// =============================================================================

// TestGateAction_OnApprovedSkipPhase_SkipsNextPhase verifies that when a gate
// approves with on_approved=skip_phase, the NEXT phase in the sequence is skipped.
func TestGateAction_OnApprovedSkipPhase_SkipsNextPhase(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	// 3-phase workflow: implement → review → docs
	// implement gate approves with on_approved: skip_phase → review should be skipped
	outputCfgJSON, _ := json.Marshal(db.GateOutputConfig{
		OnApproved: "skip_phase",
	})
	if err := backend.SavePhaseTemplate(&db.PhaseTemplate{
		ID:               "implement",
		Name:             "implement",
		PromptSource:     "db",
		PromptContent:    "Test prompt",
		GateType:         "ai",
		GateOutputConfig: string(outputCfgJSON),
	}); err != nil {
		t.Fatalf("save template: %v", err)
	}
	if err := backend.SavePhaseTemplate(&db.PhaseTemplate{
		ID:            "review",
		Name:          "review",
		PromptSource:  "db",
		PromptContent: "Test prompt",
	}); err != nil {
		t.Fatalf("save template: %v", err)
	}
	if err := backend.SavePhaseTemplate(&db.PhaseTemplate{
		ID:            "docs",
		Name:          "docs",
		PromptSource:  "db",
		PromptContent: "Test prompt",
	}); err != nil {
		t.Fatalf("save template: %v", err)
	}

	setupThreePhaseWorkflow(t, backend, "skip-next-wf", "implement", "review", "docs")

	tsk := task.NewProtoTask("TASK-SKIPNEXT-001", "Test on_approved: skip_phase")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	wfID := "skip-next-wf"
	tsk.WorkflowId = &wfID
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	mockEval := &recordingGateEvaluator{
		decision: &gate.Decision{Approved: true, Reason: "approved"},
	}

	// Track which phases execute
	executedPhases := make([]string, 0)
	mockTE := &phaseTrackingTurnExecutor{
		response:       `{"status": "complete", "summary": "Done"}`,
		executedPhases: &executedPhases,
	}

	we := NewWorkflowExecutor(
		backend, backend.DB(), testGlobalDBFrom(backend), &config.Config{}, t.TempDir(),
		WithWorkflowLogger(slog.Default()),
		WithWorkflowGateEvaluator(mockEval),
		WithWorkflowTurnExecutor(mockTE),
	)

	_, err := we.Run(context.Background(), "skip-next-wf", WorkflowRunOptions{
		ContextType: ContextTask,
		TaskID:      tsk.Id,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// implement and docs should execute; review should be skipped
	// We check via task's phase states
	updated, loadErr := backend.LoadTask(tsk.Id)
	if loadErr != nil {
		t.Fatalf("load task: %v", loadErr)
	}

	task.EnsureExecutionProto(updated)
	if ps, ok := updated.Execution.Phases["review"]; ok {
		if ps.Status != orcv1.PhaseStatus_PHASE_STATUS_SKIPPED {
			t.Errorf("review phase status = %v, want SKIPPED (on_approved: skip_phase should skip next phase)",
				ps.Status)
		}
	}
}

// =============================================================================
// SC-6: on_approved: continue (or empty/unset) preserves current behavior
// =============================================================================

// TestGateAction_OnApprovedContinue_BackwardCompat verifies that when
// on_approved is empty, "continue", or unset, the executor preserves exact
// current behavior (approved → continue to next phase).
func TestGateAction_OnApprovedContinue_BackwardCompat(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	// No GateOutputConfig → legacy behavior
	if err := backend.SavePhaseTemplate(&db.PhaseTemplate{
		ID:            "implement",
		Name:          "implement",
		PromptSource:  "db",
		PromptContent: "Test prompt",
	}); err != nil {
		t.Fatalf("save template: %v", err)
	}
	if err := backend.SavePhaseTemplate(&db.PhaseTemplate{
		ID:            "review",
		Name:          "review",
		PromptSource:  "db",
		PromptContent: "Test prompt",
	}); err != nil {
		t.Fatalf("save template: %v", err)
	}

	setupTwoPhaseWorkflow(t, backend, "compat-wf", "implement", "review")

	tsk := task.NewProtoTask("TASK-COMPAT-001", "Test backward compat")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	wfID := "compat-wf"
	tsk.WorkflowId = &wfID
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	mockEval := &recordingGateEvaluator{
		decision: &gate.Decision{Approved: true, Reason: "approved"},
	}
	mockTE := NewMockTurnExecutor(`{"status": "complete", "summary": "Done"}`)

	we := NewWorkflowExecutor(
		backend, backend.DB(), testGlobalDBFrom(backend), &config.Config{}, t.TempDir(),
		WithWorkflowLogger(slog.Default()),
		WithWorkflowGateEvaluator(mockEval),
		WithWorkflowTurnExecutor(mockTE),
	)

	_, err := we.Run(context.Background(), "compat-wf", WorkflowRunOptions{
		ContextType: ContextTask,
		TaskID:      tsk.Id,
	})

	// Both phases should execute normally (no skip, no fail)
	if err != nil {
		t.Fatalf("backward compat test should succeed: %v", err)
	}
	if mockTE.CallCount() != 2 {
		t.Errorf("expected 2 phase executions (implement + review), got %d", mockTE.CallCount())
	}
}

// =============================================================================
// SC-8: on_rejected: run_script executes script then applies fail
// =============================================================================

// TestGateAction_OnRejectedRunScript_ThenFail verifies that when a gate rejects
// with on_rejected=run_script, the script is executed and then the task fails
// (fail is the secondary action for rejected run_script).
func TestGateAction_OnRejectedRunScript_ThenFail(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	// Create a real script that writes to a marker file to prove it ran
	scriptDir := t.TempDir()
	markerFile := scriptDir + "/script-ran.marker"
	scriptPath := scriptDir + "/rejected-script.sh"
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\ncat > /dev/null\ntouch "+markerFile+"\nexit 0\n"), 0755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	outputCfgJSON, _ := json.Marshal(db.GateOutputConfig{
		OnRejected: "run_script",
		Script:     scriptPath,
	})
	if err := backend.SavePhaseTemplate(&db.PhaseTemplate{
		ID:               "implement",
		Name:             "implement",
		PromptSource:     "db",
		PromptContent:    "Test prompt",
		GateType:         "ai",
		GateOutputConfig: string(outputCfgJSON),
	}); err != nil {
		t.Fatalf("save template: %v", err)
	}

	setupSinglePhaseWorkflow(t, backend, "run-script-rej-wf", "implement")

	tsk := task.NewProtoTask("TASK-RSREJ-001", "Test on_rejected: run_script")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	wfID := "run-script-rej-wf"
	tsk.WorkflowId = &wfID
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	mockEval := &recordingGateEvaluator{
		decision: &gate.Decision{Approved: false, Reason: "rejected"},
	}
	mockTE := NewMockTurnExecutor(`{"status": "complete", "summary": "Done"}`)

	we := NewWorkflowExecutor(
		backend, backend.DB(), testGlobalDBFrom(backend), &config.Config{}, scriptDir,
		WithWorkflowLogger(slog.Default()),
		WithWorkflowGateEvaluator(mockEval),
		WithWorkflowTurnExecutor(mockTE),
	)

	_, err := we.Run(context.Background(), "run-script-rej-wf", WorkflowRunOptions{
		ContextType: ContextTask,
		TaskID:      tsk.Id,
	})

	// Task should fail (secondary action = fail for rejected run_script)
	if err == nil {
		t.Fatal("expected error from on_rejected: run_script (secondary action is fail)")
	}

	// Script should have been executed (marker file created)
	if _, statErr := os.Stat(markerFile); os.IsNotExist(statErr) {
		t.Error("script was not executed — marker file does not exist")
	}

	updated, loadErr := backend.LoadTask(tsk.Id)
	if loadErr != nil {
		t.Fatalf("load task: %v", loadErr)
	}
	if updated.Status != orcv1.TaskStatus_TASK_STATUS_FAILED {
		t.Errorf("task status = %v, want FAILED (secondary action for rejected run_script)", updated.Status)
	}
}

// =============================================================================
// SC-9: on_approved: run_script executes script then continues
// =============================================================================

// TestGateAction_OnApprovedRunScript_ThenContinue verifies that when a gate
// approves with on_approved=run_script, the script is executed and then
// execution continues to the next phase (continue is the secondary action).
func TestGateAction_OnApprovedRunScript_ThenContinue(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	scriptDir := t.TempDir()
	markerFile := scriptDir + "/approved-script-ran.marker"
	scriptPath := scriptDir + "/approved-script.sh"
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\ncat > /dev/null\ntouch "+markerFile+"\nexit 0\n"), 0755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	outputCfgJSON, _ := json.Marshal(db.GateOutputConfig{
		OnApproved: "run_script",
		Script:     scriptPath,
	})
	if err := backend.SavePhaseTemplate(&db.PhaseTemplate{
		ID:               "implement",
		Name:             "implement",
		PromptSource:     "db",
		PromptContent:    "Test prompt",
		GateType:         "ai",
		GateOutputConfig: string(outputCfgJSON),
	}); err != nil {
		t.Fatalf("save implement template: %v", err)
	}
	if err := backend.SavePhaseTemplate(&db.PhaseTemplate{
		ID:            "review",
		Name:          "review",
		PromptSource:  "db",
		PromptContent: "Test prompt",
	}); err != nil {
		t.Fatalf("save review template: %v", err)
	}

	setupTwoPhaseWorkflow(t, backend, "run-script-app-wf", "implement", "review")

	tsk := task.NewProtoTask("TASK-RSAPP-001", "Test on_approved: run_script")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	wfID := "run-script-app-wf"
	tsk.WorkflowId = &wfID
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	mockEval := &recordingGateEvaluator{
		decision: &gate.Decision{Approved: true, Reason: "approved"},
	}
	mockTE := NewMockTurnExecutor(`{"status": "complete", "summary": "Done"}`)

	we := NewWorkflowExecutor(
		backend, backend.DB(), testGlobalDBFrom(backend), &config.Config{}, scriptDir,
		WithWorkflowLogger(slog.Default()),
		WithWorkflowGateEvaluator(mockEval),
		WithWorkflowTurnExecutor(mockTE),
	)

	_, err := we.Run(context.Background(), "run-script-app-wf", WorkflowRunOptions{
		ContextType: ContextTask,
		TaskID:      tsk.Id,
	})

	// Should succeed — run_script on approval continues
	if err != nil {
		t.Fatalf("on_approved: run_script should continue execution: %v", err)
	}

	// Script should have been executed
	if _, statErr := os.Stat(markerFile); os.IsNotExist(statErr) {
		t.Error("approved script was not executed — marker file does not exist")
	}

	// Both phases should have executed
	if mockTE.CallCount() != 2 {
		t.Errorf("expected 2 phase executions, got %d", mockTE.CallCount())
	}
}

// =============================================================================
// Edge Cases
// =============================================================================

// TestGateAction_SkipPhaseOnLastPhase_WarnsAndContinues verifies that when
// on_approved: skip_phase is set on the last phase, it logs a warning and
// continues normally (no panic, no skip).
func TestGateAction_SkipPhaseOnLastPhase_WarnsAndContinues(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	outputCfgJSON, _ := json.Marshal(db.GateOutputConfig{
		OnApproved: "skip_phase",
	})
	if err := backend.SavePhaseTemplate(&db.PhaseTemplate{
		ID:               "implement",
		Name:             "implement",
		PromptSource:     "db",
		PromptContent:    "Test prompt",
		GateType:         "ai",
		GateOutputConfig: string(outputCfgJSON),
	}); err != nil {
		t.Fatalf("save template: %v", err)
	}

	// Single-phase workflow — there's no "next phase" to skip
	setupSinglePhaseWorkflow(t, backend, "last-phase-wf", "implement")

	tsk := task.NewProtoTask("TASK-LAST-001", "Test skip on last phase")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	wfID := "last-phase-wf"
	tsk.WorkflowId = &wfID
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	mockEval := &recordingGateEvaluator{
		decision: &gate.Decision{Approved: true, Reason: "approved"},
	}
	mockTE := NewMockTurnExecutor(`{"status": "complete", "summary": "Done"}`)

	we := NewWorkflowExecutor(
		backend, backend.DB(), testGlobalDBFrom(backend), &config.Config{}, t.TempDir(),
		WithWorkflowLogger(slog.Default()),
		WithWorkflowGateEvaluator(mockEval),
		WithWorkflowTurnExecutor(mockTE),
	)

	// Should NOT panic or fail — just log warning and continue
	_, err := we.Run(context.Background(), "last-phase-wf", WorkflowRunOptions{
		ContextType: ContextTask,
		TaskID:      tsk.Id,
	})

	if err != nil {
		t.Fatalf("skip_phase on last phase should not fail: %v", err)
	}
}

// TestGateAction_SkipGates_NoOutputConfig verifies that when --skip-gates
// is active, GateEvaluationResult has nil OutputConfig (gates are bypassed).
func TestGateAction_SkipGates_NoOutputConfig(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	tsk := task.NewProtoTask("TASK-SKIPG-001", "Test skip gates")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	outputCfgJSON, _ := json.Marshal(db.GateOutputConfig{
		OnApproved: "skip_phase",
		OnRejected: "fail",
	})

	we := NewWorkflowExecutor(
		backend, nil, testGlobalDBFrom(backend), &config.Config{}, t.TempDir(),
		WithWorkflowLogger(slog.Default()),
		WithSkipGates(true),
	)

	tmpl := &db.PhaseTemplate{
		ID:               "review",
		GateType:         "ai",
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
		t.Error("skip-gates should auto-approve")
	}
	if result.OutputConfig != nil {
		t.Errorf("OutputConfig should be nil when gates are skipped, got %+v", result.OutputConfig)
	}
}

// TestGateAction_RunScriptOverride_FlipsToFail verifies edge case: when
// on_approved: run_script and the script exits non-zero, the approval is
// overridden and the task should be treated as failed/rejected.
func TestGateAction_RunScriptOverride_FlipsToFail(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	scriptDir := t.TempDir()
	scriptPath := scriptDir + "/override-script.sh"
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\ncat > /dev/null\nexit 1\n"), 0755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	outputCfgJSON, _ := json.Marshal(db.GateOutputConfig{
		OnApproved: "run_script",
		Script:     scriptPath,
	})

	mockEval := &recordingGateEvaluator{
		decision: &gate.Decision{Approved: true, Reason: "approved by AI"},
	}

	we := NewWorkflowExecutor(
		backend, nil, testGlobalDBFrom(backend), &config.Config{}, scriptDir,
		WithWorkflowLogger(slog.Default()),
		WithWorkflowGateEvaluator(mockEval),
	)

	tsk := task.NewProtoTask("TASK-OVERRIDE-001", "Test script override")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	tmpl := &db.PhaseTemplate{
		ID:               "review",
		GateType:         "ai",
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

	// Script override: non-zero exit should flip approval
	if result.Approved {
		t.Error("script non-zero exit should override approval to rejection")
	}
}

// =============================================================================
// Failure Modes
// =============================================================================

// TestGateAction_RetryNoRetryFrom_FailsWithError verifies that when
// on_rejected=retry but there's no retry_from (neither in outputCfg nor template),
// the task fails with a meaningful error.
func TestGateAction_RetryNoRetryFrom_FailsWithError(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	// retry with NO retry_from anywhere
	outputCfgJSON, _ := json.Marshal(db.GateOutputConfig{
		OnRejected: "retry",
		// RetryFrom: "" — intentionally missing
	})
	if err := backend.SavePhaseTemplate(&db.PhaseTemplate{
		ID:               "implement",
		Name:             "implement",
		PromptSource:     "db",
		PromptContent:    "Test prompt",
		GateType:         "ai",
		GateOutputConfig: string(outputCfgJSON),
		// RetryFromPhase: "" — also missing
	}); err != nil {
		t.Fatalf("save template: %v", err)
	}

	setupSinglePhaseWorkflow(t, backend, "no-retry-from-wf", "implement")

	tsk := task.NewProtoTask("TASK-NORF-001", "Test retry with no retry_from")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	wfID := "no-retry-from-wf"
	tsk.WorkflowId = &wfID
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	mockEval := &recordingGateEvaluator{
		decision: &gate.Decision{Approved: false, Reason: "rejected"},
	}
	mockTE := NewMockTurnExecutor(`{"status": "complete", "summary": "Done"}`)

	we := NewWorkflowExecutor(
		backend, backend.DB(), testGlobalDBFrom(backend), &config.Config{}, t.TempDir(),
		WithWorkflowLogger(slog.Default()),
		WithWorkflowGateEvaluator(mockEval),
		WithWorkflowTurnExecutor(mockTE),
	)

	_, err := we.Run(context.Background(), "no-retry-from-wf", WorkflowRunOptions{
		ContextType: ContextTask,
		TaskID:      tsk.Id,
	})

	// Should fail — retry with no retry_from is an error
	if err == nil {
		t.Fatal("expected error when on_rejected=retry but no retry_from is configured")
	}

	updated, loadErr := backend.LoadTask(tsk.Id)
	if loadErr != nil {
		t.Fatalf("load task: %v", loadErr)
	}
	if updated.Status != orcv1.TaskStatus_TASK_STATUS_FAILED {
		t.Errorf("task status = %v, want FAILED (retry with no retry_from)", updated.Status)
	}
}

// TestGateAction_RunScriptEmptyPath_WarnsAndAppliesSecondary verifies that when
// on_rejected=run_script but Script path is empty, the script is skipped (with
// warning) and the secondary action (fail) is applied.
func TestGateAction_RunScriptEmptyPath_WarnsAndAppliesSecondary(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	// run_script with empty Script path
	outputCfgJSON, _ := json.Marshal(db.GateOutputConfig{
		OnRejected: "run_script",
		Script:     "", // Empty — should warn and fall through to fail
	})
	if err := backend.SavePhaseTemplate(&db.PhaseTemplate{
		ID:               "implement",
		Name:             "implement",
		PromptSource:     "db",
		PromptContent:    "Test prompt",
		GateType:         "ai",
		GateOutputConfig: string(outputCfgJSON),
	}); err != nil {
		t.Fatalf("save template: %v", err)
	}

	setupSinglePhaseWorkflow(t, backend, "empty-script-wf", "implement")

	tsk := task.NewProtoTask("TASK-EMPTYS-001", "Test run_script with empty path")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	wfID := "empty-script-wf"
	tsk.WorkflowId = &wfID
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	mockEval := &recordingGateEvaluator{
		decision: &gate.Decision{Approved: false, Reason: "rejected"},
	}
	mockTE := NewMockTurnExecutor(`{"status": "complete", "summary": "Done"}`)

	we := NewWorkflowExecutor(
		backend, backend.DB(), testGlobalDBFrom(backend), &config.Config{}, t.TempDir(),
		WithWorkflowLogger(slog.Default()),
		WithWorkflowGateEvaluator(mockEval),
		WithWorkflowTurnExecutor(mockTE),
	)

	_, err := we.Run(context.Background(), "empty-script-wf", WorkflowRunOptions{
		ContextType: ContextTask,
		TaskID:      tsk.Id,
	})

	// Should fail — empty script path means skip script, apply secondary action (fail)
	if err == nil {
		t.Fatal("expected error when run_script has empty script path (secondary action = fail)")
	}

	updated, loadErr := backend.LoadTask(tsk.Id)
	if loadErr != nil {
		t.Fatalf("load task: %v", loadErr)
	}
	if updated.Status != orcv1.TaskStatus_TASK_STATUS_FAILED {
		t.Errorf("task status = %v, want FAILED (empty script path → secondary action)", updated.Status)
	}
}

// =============================================================================
// Test Helpers
// =============================================================================

// configGateEvaluator is a gate evaluator that delegates to a function
// for dynamic decision-making in tests. More flexible than recordingGateEvaluator.
type configGateEvaluator struct {
	decisionFn func(g *gate.Gate, output string, opts *gate.EvaluateOptions) (*gate.Decision, error)
}

func (m *configGateEvaluator) Evaluate(ctx context.Context, g *gate.Gate, output string) (*gate.Decision, error) {
	return m.EvaluateWithOptions(ctx, g, output, nil)
}

func (m *configGateEvaluator) EvaluateWithOptions(ctx context.Context, g *gate.Gate, output string, opts *gate.EvaluateOptions) (*gate.Decision, error) {
	if m.decisionFn != nil {
		return m.decisionFn(g, output, opts)
	}
	return &gate.Decision{Approved: true, Reason: "default approved"}, nil
}

// phaseTrackingTurnExecutor tracks which phases execute via their prompts.
type phaseTrackingTurnExecutor struct {
	response       string
	executedPhases *[]string
	callCount      int
}

func (m *phaseTrackingTurnExecutor) ExecuteTurn(ctx context.Context, prompt string) (*TurnResult, error) {
	m.callCount++
	return &TurnResult{
		Content:   m.response,
		SessionID: "mock-session",
	}, nil
}

func (m *phaseTrackingTurnExecutor) ExecuteTurnWithoutSchema(ctx context.Context, prompt string) (*TurnResult, error) {
	return m.ExecuteTurn(ctx, prompt)
}

func (m *phaseTrackingTurnExecutor) UpdateSessionID(id string) {}

func (m *phaseTrackingTurnExecutor) SessionID() string { return "mock-session" }

// setupSinglePhaseWorkflow creates a workflow with one phase for integration tests.
func setupSinglePhaseWorkflow(t *testing.T, backend *storage.DatabaseBackend, workflowID, phaseID string) {
	t.Helper()
	pdb := backend.DB()

	wf := &db.Workflow{ID: workflowID, Name: workflowID}
	if err := pdb.SaveWorkflow(wf); err != nil {
		t.Fatalf("save workflow: %v", err)
	}

	phase := &db.WorkflowPhase{
		WorkflowID:      workflowID,
		PhaseTemplateID: phaseID,
		Sequence:        1,
	}
	if err := pdb.SaveWorkflowPhase(phase); err != nil {
		t.Fatalf("save phase %s: %v", phaseID, err)
	}
}

// setupTwoPhaseWorkflow creates a workflow with two sequential phases.
func setupTwoPhaseWorkflow(t *testing.T, backend *storage.DatabaseBackend, workflowID, phase1ID, phase2ID string) {
	t.Helper()
	pdb := backend.DB()

	wf := &db.Workflow{ID: workflowID, Name: workflowID}
	if err := pdb.SaveWorkflow(wf); err != nil {
		t.Fatalf("save workflow: %v", err)
	}

	for i, id := range []string{phase1ID, phase2ID} {
		phase := &db.WorkflowPhase{
			WorkflowID:      workflowID,
			PhaseTemplateID: id,
			Sequence:        i + 1,
		}
		if err := pdb.SaveWorkflowPhase(phase); err != nil {
			t.Fatalf("save phase %s: %v", id, err)
		}
	}
}

// setupThreePhaseWorkflow creates a workflow with three sequential phases.
func setupThreePhaseWorkflow(t *testing.T, backend *storage.DatabaseBackend, workflowID, phase1ID, phase2ID, phase3ID string) {
	t.Helper()
	pdb := backend.DB()

	wf := &db.Workflow{ID: workflowID, Name: workflowID}
	if err := pdb.SaveWorkflow(wf); err != nil {
		t.Fatalf("save workflow: %v", err)
	}

	for i, id := range []string{phase1ID, phase2ID, phase3ID} {
		phase := &db.WorkflowPhase{
			WorkflowID:      workflowID,
			PhaseTemplateID: id,
			Sequence:        i + 1,
		}
		if err := pdb.SaveWorkflowPhase(phase); err != nil {
			t.Fatalf("save phase %s: %v", id, err)
		}
	}
}
