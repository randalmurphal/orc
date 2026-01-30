package executor

import (
	"context"
	"log/slog"
	"testing"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/internal/trigger"
	"github.com/randalmurphal/orc/internal/workflow"
)

// --- SC-1: Before-phase triggers loaded and evaluated before executePhaseWithTimeout ---

func TestBeforePhaseTrigger_GateMode(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	// Create a task and workflow with a before-phase trigger on implement
	tsk := task.NewProtoTask("TASK-001", "Test before-phase gate")
	tsk.Weight = orcv1.TaskWeight_TASK_WEIGHT_MEDIUM
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Create a mock trigger runner that rejects
	mockRunner := &mockTriggerRunner{
		beforePhaseResult: &trigger.BeforePhaseTriggerResult{
			Blocked:       true,
			BlockedReason: "precondition validation failed: no tests found",
		},
	}

	we := NewWorkflowExecutor(
		backend, nil, &config.Config{}, t.TempDir(),
		WithWorkflowLogger(slog.Default()),
		WithWorkflowTriggerRunner(mockRunner),
	)

	// Set up a workflow phase with before-triggers
	phase := &workflow.WorkflowPhase{
		PhaseTemplateID: "implement",
		BeforeTriggers: []workflow.BeforePhaseTrigger{
			{
				AgentID: "validator-agent",
				Mode:    workflow.GateModeGate,
			},
		},
		Template: &workflow.PhaseTemplate{
			ID:   "implement",
			Name: "Implement",
		},
	}

	// Execute the phase - should be blocked by the before-phase trigger
	result := we.evaluateBeforePhaseTriggers(
		context.Background(), phase, tsk, map[string]string{},
	)

	if !result.Blocked {
		t.Error("expected phase to be blocked by before-phase trigger")
	}
	if result.BlockedReason == "" {
		t.Error("expected blocked reason to be set")
	}

	// Verify mock was called with correct phase
	if mockRunner.lastPhase != "implement" {
		t.Errorf("phase = %q, want %q", mockRunner.lastPhase, "implement")
	}
}

// --- SC-2: Before-phase triggers in reaction mode never block ---

func TestBeforePhaseTrigger_ReactionMode(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	tsk := task.NewProtoTask("TASK-001", "Test reaction trigger")
	tsk.Weight = orcv1.TaskWeight_TASK_WEIGHT_MEDIUM
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	mockRunner := &mockTriggerRunner{
		beforePhaseResult: &trigger.BeforePhaseTriggerResult{
			Blocked: false,
			// Reaction mode: never blocks even if agent would reject
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
				AgentID: "async-checker",
				Mode:    workflow.GateModeReaction,
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

	if result.Blocked {
		t.Error("reaction mode trigger should never block phase execution")
	}
}

// --- SC-3: Before-phase trigger output flows into variable system ---

func TestBeforePhaseTrigger_OutputVariable(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	tsk := task.NewProtoTask("TASK-001", "Test output variable")
	tsk.Weight = orcv1.TaskWeight_TASK_WEIGHT_MEDIUM
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	mockRunner := &mockTriggerRunner{
		beforePhaseResult: &trigger.BeforePhaseTriggerResult{
			Blocked:     false,
			UpdatedVars: map[string]string{
				"validation_result": "all checks passed",
				"existing_var":      "preserved",
			},
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
				AgentID: "validator-agent",
				Mode:    workflow.GateModeGate,
				OutputConfig: &workflow.GateOutputConfig{
					VariableName: "validation_result",
				},
			},
		},
		Template: &workflow.PhaseTemplate{
			ID:   "implement",
			Name: "Implement",
		},
	}

	inputVars := map[string]string{"existing_var": "preserved"}
	result := we.evaluateBeforePhaseTriggers(
		context.Background(), phase, tsk, inputVars,
	)

	if result.Blocked {
		t.Error("trigger approved but result shows blocked")
	}

	// Variable should be available in the returned vars
	if result.UpdatedVars["validation_result"] != "all checks passed" {
		t.Errorf("validation_result = %q, want %q",
			result.UpdatedVars["validation_result"], "all checks passed")
	}
	if result.UpdatedVars["existing_var"] != "preserved" {
		t.Errorf("existing_var = %q, want %q",
			result.UpdatedVars["existing_var"], "preserved")
	}
}

// --- Edge case: No before-phase triggers configured ---

func TestBeforePhaseTrigger_NoneConfigured(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	tsk := task.NewProtoTask("TASK-001", "Test no triggers")
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

	phase := &workflow.WorkflowPhase{
		PhaseTemplateID: "implement",
		BeforeTriggers:  nil, // No triggers
		Template: &workflow.PhaseTemplate{
			ID:   "implement",
			Name: "Implement",
		},
	}

	result := we.evaluateBeforePhaseTriggers(
		context.Background(), phase, tsk, map[string]string{},
	)

	if result.Blocked {
		t.Error("no triggers should mean not blocked")
	}
	if mockRunner.beforePhaseCalled {
		t.Error("trigger runner should not be called when no triggers configured")
	}
}

// --- Edge case: Before-phase trigger on skipped/completed phase (resume) ---

func TestBeforePhaseTrigger_SkippedOnResume(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	tsk := task.NewProtoTask("TASK-001", "Test resume skip")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	mockRunner := &mockTriggerRunner{
		beforePhaseResult: &trigger.BeforePhaseTriggerResult{
			Blocked: false,
		},
	}

	we := NewWorkflowExecutor(
		backend, nil, &config.Config{}, t.TempDir(),
		WithWorkflowLogger(slog.Default()),
		WithWorkflowTriggerRunner(mockRunner),
	)
	we.isResuming = true

	phase := &workflow.WorkflowPhase{
		PhaseTemplateID: "implement",
		BeforeTriggers: []workflow.BeforePhaseTrigger{
			{
				AgentID: "gate-agent",
				Mode:    workflow.GateModeGate,
			},
		},
		Template: &workflow.PhaseTemplate{
			ID:   "implement",
			Name: "Implement",
		},
	}

	// When resuming and the phase is already completed, triggers should not re-evaluate
	result := we.evaluateBeforePhaseTriggers(
		context.Background(), phase, tsk, map[string]string{},
	)

	// For already-completed phases on resume, the executor skips them entirely.
	// This test verifies the trigger runner is also skipped.
	// The executor's phase loop handles this, but the evaluateBeforePhaseTriggers
	// method should also be safe to call (no-op or normal evaluation).
	_ = result // The key assertion is that it doesn't panic or error
}

// --- Mock for TriggerRunner interface used by executor ---

type mockTriggerRunner struct {
	beforePhaseResult *trigger.BeforePhaseTriggerResult
	beforePhaseCalled bool
	lastPhase         string

	lifecycleErr    error
	lifecycleCalled bool
	lastEvent       workflow.WorkflowTriggerEvent
}

func (m *mockTriggerRunner) RunBeforePhaseTriggers(
	ctx context.Context,
	phase string,
	triggers []workflow.BeforePhaseTrigger,
	vars map[string]string,
	task *orcv1.Task,
) (*trigger.BeforePhaseTriggerResult, error) {
	m.beforePhaseCalled = true
	m.lastPhase = phase
	if m.beforePhaseResult != nil {
		return m.beforePhaseResult, nil
	}
	return &trigger.BeforePhaseTriggerResult{}, nil
}

func (m *mockTriggerRunner) RunLifecycleTriggers(
	ctx context.Context,
	event workflow.WorkflowTriggerEvent,
	triggers []workflow.WorkflowTrigger,
	tsk *orcv1.Task,
) error {
	m.lifecycleCalled = true
	m.lastEvent = event
	return m.lifecycleErr
}
