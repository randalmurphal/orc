package executor

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/internal/trigger"
	"github.com/randalmurphal/orc/internal/workflow"
)

// --- SC-4: on_task_completed triggers fire after task marked complete ---

func TestLifecycleTrigger_TaskCompleted(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	tsk := task.NewProtoTask("TASK-001", "Test task completed trigger")
	tsk.Weight = orcv1.TaskWeight_TASK_WEIGHT_MEDIUM
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

	// Simulate workflow triggers on the workflow
	wf := &workflow.Workflow{
		ID:   "medium-workflow",
		Name: "Medium Workflow",
		Triggers: []workflow.WorkflowTrigger{
			{
				Event:   workflow.WorkflowTriggerEventOnTaskCompleted,
				AgentID: "completion-notifier",
				Mode:    workflow.GateModeReaction,
				Enabled: true,
			},
		},
	}

	// Fire completion triggers (method that should be called after task marked complete)
	we.fireLifecycleTriggers(context.Background(), workflow.WorkflowTriggerEventOnTaskCompleted, wf, tsk)

	if !mockRunner.lifecycleCalled {
		t.Error("lifecycle trigger should have been called on task completion")
	}
	if mockRunner.lastEvent != workflow.WorkflowTriggerEventOnTaskCompleted {
		t.Errorf("event = %q, want %q", mockRunner.lastEvent, workflow.WorkflowTriggerEventOnTaskCompleted)
	}
}

// --- SC-5: on_task_failed triggers fire after task fails ---

func TestLifecycleTrigger_TaskFailed(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	tsk := task.NewProtoTask("TASK-001", "Test task failed trigger")
	tsk.Weight = orcv1.TaskWeight_TASK_WEIGHT_MEDIUM
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

	wf := &workflow.Workflow{
		ID:   "medium-workflow",
		Name: "Medium Workflow",
		Triggers: []workflow.WorkflowTrigger{
			{
				Event:   workflow.WorkflowTriggerEventOnTaskFailed,
				AgentID: "failure-handler",
				Mode:    workflow.GateModeReaction,
				Enabled: true,
			},
		},
	}

	we.fireLifecycleTriggers(context.Background(), workflow.WorkflowTriggerEventOnTaskFailed, wf, tsk)

	if !mockRunner.lifecycleCalled {
		t.Error("lifecycle trigger should have been called on task failure")
	}
	if mockRunner.lastEvent != workflow.WorkflowTriggerEventOnTaskFailed {
		t.Errorf("event = %q, want %q", mockRunner.lastEvent, workflow.WorkflowTriggerEventOnTaskFailed)
	}
}

// --- SC-6: Gate-mode lifecycle trigger can block task completion ---

func TestLifecycleTrigger_GateBlocks(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	tsk := task.NewProtoTask("TASK-001", "Test gate blocks completion")
	tsk.Weight = orcv1.TaskWeight_TASK_WEIGHT_MEDIUM
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	mockRunner := &mockTriggerRunner{
		lifecycleErr: &trigger.GateRejectionError{
			AgentID: "quality-gate",
			Reason:  "test coverage below threshold",
		},
	}

	we := NewWorkflowExecutor(
		backend, nil, &config.Config{}, t.TempDir(),
		WithWorkflowLogger(slog.Default()),
		WithWorkflowTriggerRunner(mockRunner),
	)
	we.task = tsk

	wf := &workflow.Workflow{
		ID:   "medium-workflow",
		Name: "Medium Workflow",
		Triggers: []workflow.WorkflowTrigger{
			{
				Event:   workflow.WorkflowTriggerEventOnTaskCompleted,
				AgentID: "quality-gate",
				Mode:    workflow.GateModeGate,
				Enabled: true,
			},
		},
	}

	// When a gate-mode lifecycle trigger rejects, the task should be BLOCKED
	err := we.handleCompletionWithTriggers(context.Background(), wf, tsk)

	if err == nil {
		t.Fatal("expected error from rejected gate, got nil")
	}

	// Reload task to check status
	updated, loadErr := backend.LoadTask("TASK-001")
	if loadErr != nil {
		t.Fatalf("load task: %v", loadErr)
	}

	if updated.Status != orcv1.TaskStatus_TASK_STATUS_BLOCKED {
		t.Errorf("task status = %v, want BLOCKED", updated.Status)
	}
}

// --- Edge case: on_task_failed trigger itself fails ---

func TestLifecycleTrigger_FailedTriggerOnFail(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	tsk := task.NewProtoTask("TASK-001", "Test double failure")
	tsk.Weight = orcv1.TaskWeight_TASK_WEIGHT_MEDIUM
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_FAILED
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	mockRunner := &mockTriggerRunner{
		lifecycleErr: errors.New("trigger agent crashed"),
	}

	we := NewWorkflowExecutor(
		backend, nil, &config.Config{}, t.TempDir(),
		WithWorkflowLogger(slog.Default()),
		WithWorkflowTriggerRunner(mockRunner),
	)
	we.task = tsk

	wf := &workflow.Workflow{
		ID:   "medium-workflow",
		Name: "Medium Workflow",
		Triggers: []workflow.WorkflowTrigger{
			{
				Event:   workflow.WorkflowTriggerEventOnTaskFailed,
				AgentID: "crash-agent",
				Mode:    workflow.GateModeReaction,
				Enabled: true,
			},
		},
	}

	// on_task_failed trigger itself fails - task should remain FAILED, not change
	we.fireLifecycleTriggers(context.Background(), workflow.WorkflowTriggerEventOnTaskFailed, wf, tsk)

	// Task should still be in FAILED state (reaction mode failure only logs warning)
	updated, err := backend.LoadTask("TASK-001")
	if err != nil {
		t.Fatalf("load task: %v", err)
	}
	if updated.Status != orcv1.TaskStatus_TASK_STATUS_FAILED {
		t.Errorf("task status = %v, want FAILED (trigger failure should not change status)", updated.Status)
	}
}

// --- Edge case: No workflow triggers configured ---

func TestLifecycleTrigger_NoWorkflowTriggers(t *testing.T) {
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
	we.task = tsk

	wf := &workflow.Workflow{
		ID:       "simple-workflow",
		Name:     "Simple Workflow",
		Triggers: nil, // No triggers
	}

	we.fireLifecycleTriggers(context.Background(), workflow.WorkflowTriggerEventOnTaskCompleted, wf, tsk)

	if mockRunner.lifecycleCalled {
		t.Error("trigger runner should not be called when workflow has no triggers")
	}
}

// --- Edge case: Nil workflow ---

func TestLifecycleTrigger_NilWorkflow(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	tsk := task.NewProtoTask("TASK-001", "Test nil workflow")
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

	// Nil workflow should not panic
	we.fireLifecycleTriggers(context.Background(), workflow.WorkflowTriggerEventOnTaskCompleted, nil, tsk)

	if mockRunner.lifecycleCalled {
		t.Error("trigger runner should not be called with nil workflow")
	}
}

// --- Integration test: Full workflow with triggers ---

func TestWorkflowWithTriggers_Integration(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	tsk := task.NewProtoTask("TASK-001", "Integration test task")
	tsk.Weight = orcv1.TaskWeight_TASK_WEIGHT_MEDIUM
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Track all trigger invocations
	var calls []string
	mockRunner := &recordingTriggerRunner{
		calls: &calls,
		beforePhaseResult: &trigger.BeforePhaseTriggerResult{
			Blocked: false,
			UpdatedVars: map[string]string{
				"gate_result": "approved",
			},
		},
	}

	we := NewWorkflowExecutor(
		backend, nil, &config.Config{}, t.TempDir(),
		WithWorkflowLogger(slog.Default()),
		WithWorkflowTriggerRunner(mockRunner),
	)
	we.task = tsk

	// Full workflow with both before-phase and lifecycle triggers
	wf := &workflow.Workflow{
		ID:   "test-workflow",
		Name: "Test Workflow",
		Triggers: []workflow.WorkflowTrigger{
			{
				Event:   workflow.WorkflowTriggerEventOnTaskCompleted,
				AgentID: "completion-agent",
				Mode:    workflow.GateModeReaction,
				Enabled: true,
			},
		},
		Phases: []workflow.WorkflowPhase{
			{
				PhaseTemplateID: "implement",
				Sequence:        1,
				BeforeTriggers: []workflow.BeforePhaseTrigger{
					{
						AgentID: "pre-implement-gate",
						Mode:    workflow.GateModeGate,
						OutputConfig: &workflow.GateOutputConfig{
							VariableName: "gate_result",
						},
					},
				},
				Template: &workflow.PhaseTemplate{
					ID:   "implement",
					Name: "Implement",
				},
			},
		},
	}

	// Evaluate before-phase triggers for the implement phase
	vars := map[string]string{}
	result := we.evaluateBeforePhaseTriggers(
		context.Background(), &wf.Phases[0], tsk, vars,
	)

	if result.Blocked {
		t.Error("before-phase trigger should approve")
	}
	if result.UpdatedVars["gate_result"] != "approved" {
		t.Errorf("gate_result = %q, want %q", result.UpdatedVars["gate_result"], "approved")
	}

	// Fire completion triggers
	we.fireLifecycleTriggers(context.Background(), workflow.WorkflowTriggerEventOnTaskCompleted, wf, tsk)

	// Both before-phase and lifecycle triggers should have been called
	if len(calls) < 2 {
		t.Errorf("expected at least 2 trigger calls, got %d: %v", len(calls), calls)
	}
}

// --- Recording mock that tracks all calls ---

type recordingTriggerRunner struct {
	calls             *[]string
	beforePhaseResult *trigger.BeforePhaseTriggerResult
	lifecycleErr      error
}

func (r *recordingTriggerRunner) RunBeforePhaseTriggers(
	ctx context.Context,
	phase string,
	triggers []workflow.BeforePhaseTrigger,
	vars map[string]string,
	tsk *orcv1.Task,
) (*trigger.BeforePhaseTriggerResult, error) {
	*r.calls = append(*r.calls, "before_phase:"+phase)
	if r.beforePhaseResult != nil {
		return r.beforePhaseResult, nil
	}
	return &trigger.BeforePhaseTriggerResult{}, nil
}

func (r *recordingTriggerRunner) RunLifecycleTriggers(
	ctx context.Context,
	event workflow.WorkflowTriggerEvent,
	triggers []workflow.WorkflowTrigger,
	tsk *orcv1.Task,
) error {
	*r.calls = append(*r.calls, "lifecycle:"+string(event))
	return r.lifecycleErr
}
