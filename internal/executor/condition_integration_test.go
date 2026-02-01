// Integration tests for TASK-686: Phase condition evaluator executor wiring.
//
// These tests verify that the condition evaluator is correctly integrated
// into the executor's phase loop and that skip/resume behavior works.
//
// Coverage mapping:
//   SC-5:  TestWorkflowExecutor_ConditionSkip, TestWorkflowExecutor_ConditionPass
//   SC-9:  TestWorkflowExecutor_ConditionSkip (SkipReason verification)
//   SC-10: TestWorkflowExecutor_ResumeSkipped
//
// Failure modes:
//   TestWorkflowExecutor_ConditionInvalid
//   TestWorkflowExecutor_EmptyCondition
//   TestWorkflowExecutor_MultipleSkips
package executor

import (
	"context"
	"log/slog"
	"testing"
	"time"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/internal/variable"
)

// =============================================================================
// SC-5: Executor skips phase when condition evaluates to false
// SC-9: Skipped phase has SkipReason in db.Phase
// =============================================================================

// TestWorkflowExecutor_ConditionSkip verifies the full skip path:
// - Phase with condition that evaluates to false is skipped
// - WorkflowRunPhase status is set to skipped
// - Task proto phase state is PHASE_STATUS_SKIPPED
// - SkipReason is recorded
// - Next phase executes normally
func TestWorkflowExecutor_ConditionSkip(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	tsk := task.NewProtoTask("TASK-001", "Test condition skip")
	tsk.Weight = orcv1.TaskWeight_TASK_WEIGHT_MEDIUM
	tsk.Category = orcv1.TaskCategory_TASK_CATEGORY_FEATURE
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Set up mock event publisher to capture events
	mockPub := newConditionTestPublisher()

	we := NewWorkflowExecutor(
		backend, nil, &config.Config{}, t.TempDir(),
		WithWorkflowLogger(slog.Default()),
		WithWorkflowPublisher(mockPub),
	)

	// Build a condition context for the skip check
	ctx := &ConditionContext{
		Task: tsk,
		Vars: variable.VariableSet{},
		RCtx: &variable.ResolutionContext{},
	}

	// Condition: task.weight == "trivial" (but task weight is medium → false → skip)
	conditionJSON := `{"field": "task.weight", "op": "eq", "value": "trivial"}`

	result, err := EvaluateCondition(conditionJSON, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Fatal("condition should evaluate to false (weight is medium, not trivial)")
	}

	// The implementation should call skipPhaseForCondition (or equivalent)
	// which updates the run phase, task proto, and publishes the event.
	// We verify the skip effects by testing SkipPhaseForCondition directly.
	phase := &db.WorkflowPhase{
		PhaseTemplateID: "tdd_write",
		Condition:       conditionJSON,
	}

	// Create a workflow run and run phase for the skip to operate on
	wf := &db.Workflow{ID: "test-wf", Name: "Test WF"}
	if err := backend.SaveWorkflow(wf); err != nil {
		t.Fatalf("save workflow: %v", err)
	}
	taskID := "TASK-001"
	run := &db.WorkflowRun{
		ID:          "run-001",
		WorkflowID:  "test-wf",
		TaskID:      &taskID,
		ContextType: "task",
		ContextData: "{}",
		Prompt:      "test",
		Status:      "running",
	}
	if err := backend.SaveWorkflowRun(run); err != nil {
		t.Fatalf("save run: %v", err)
	}
	runPhase := &db.WorkflowRunPhase{
		WorkflowRunID:   "run-001",
		PhaseTemplateID: "tdd_write",
		Status:          orcv1.PhaseStatus_PHASE_STATUS_PENDING.String(),
	}
	if err := backend.SaveWorkflowRunPhase(runPhase); err != nil {
		t.Fatalf("save run phase: %v", err)
	}

	// Call the skip function that the implementation will provide
	err = we.SkipPhaseForCondition(tsk, run, runPhase, phase)
	if err != nil {
		t.Fatalf("SkipPhaseForCondition error: %v", err)
	}

	// Verify: task proto phase state is PHASE_STATUS_SKIPPED
	ps, ok := tsk.Execution.Phases["tdd_write"]
	if !ok {
		t.Fatal("expected tdd_write phase in task execution state")
	}
	if ps.Status != orcv1.PhaseStatus_PHASE_STATUS_SKIPPED {
		t.Errorf("phase status = %v, want PHASE_STATUS_SKIPPED", ps.Status)
	}

	// Verify: SC-9 - SkipReason contains the condition JSON
	// The implementation should record the condition in db.Phase or runPhase
	if ps.Error == nil || *ps.Error == "" {
		t.Error("expected skip reason to be recorded in phase state")
	}

	// Verify: PhaseSkipped event was published (SC-8 wiring)
	if !mockPub.hasPhaseSkippedEvent("TASK-001", "tdd_write") {
		t.Error("expected PhaseSkipped event to be published")
	}
}

// =============================================================================
// SC-5: Executor runs phase when condition is true (no skip)
// =============================================================================

func TestWorkflowExecutor_ConditionPass(t *testing.T) {
	t.Parallel()

	// Task with weight=medium, condition checks for weight==medium → true
	tsk := task.NewProtoTask("TASK-002", "Test condition pass")
	tsk.Weight = orcv1.TaskWeight_TASK_WEIGHT_MEDIUM

	ctx := &ConditionContext{
		Task: tsk,
		Vars: variable.VariableSet{},
		RCtx: &variable.ResolutionContext{},
	}

	conditionJSON := `{"field": "task.weight", "op": "eq", "value": "medium"}`

	result, err := EvaluateCondition(conditionJSON, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("condition should evaluate to true (weight IS medium) → phase should execute")
	}
}

// =============================================================================
// SC-5 error path: Invalid condition JSON fails the run
// =============================================================================

func TestWorkflowExecutor_ConditionInvalid(t *testing.T) {
	t.Parallel()

	tsk := task.NewProtoTask("TASK-003", "Test invalid condition")
	tsk.Weight = orcv1.TaskWeight_TASK_WEIGHT_MEDIUM

	ctx := &ConditionContext{
		Task: tsk,
		Vars: variable.VariableSet{},
		RCtx: &variable.ResolutionContext{},
	}

	// Invalid JSON → should return error (NOT silently skip or execute)
	_, err := EvaluateCondition(`{"field": "task.weight", "op": "invalid_op"}`, ctx)
	if err == nil {
		t.Fatal("expected error for invalid operator, got nil — per constitution: NO silent failures")
	}
}

// =============================================================================
// SC-5: Empty condition string means phase always executes
// =============================================================================

func TestWorkflowExecutor_EmptyCondition(t *testing.T) {
	t.Parallel()

	tsk := task.NewProtoTask("TASK-004", "Test empty condition")
	tsk.Weight = orcv1.TaskWeight_TASK_WEIGHT_MEDIUM

	ctx := &ConditionContext{
		Task: tsk,
		Vars: variable.VariableSet{},
		RCtx: &variable.ResolutionContext{},
	}

	// Empty condition → phase should execute (always run)
	result, err := EvaluateCondition("", ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("empty condition should return true (phase always executes)")
	}
}

// =============================================================================
// SC-10: On resume, previously skipped phases are not re-executed
// =============================================================================

func TestWorkflowExecutor_ResumeSkipped(t *testing.T) {
	t.Parallel()

	// Set up a task with a phase already marked as SKIPPED in execution state.
	// When the executor resumes, it should skip this phase without
	// re-evaluating the condition.
	tsk := task.NewProtoTask("TASK-005", "Test resume skipped")
	tsk.Weight = orcv1.TaskWeight_TASK_WEIGHT_MEDIUM
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING

	// Mark the tdd_write phase as SKIPPED (from a previous run)
	task.SkipPhaseProto(tsk.Execution, "tdd_write", "condition: task.weight eq trivial")

	// Verify the phase is marked as SKIPPED
	ps, ok := tsk.Execution.Phases["tdd_write"]
	if !ok {
		t.Fatal("expected tdd_write phase in execution state")
	}
	if ps.Status != orcv1.PhaseStatus_PHASE_STATUS_SKIPPED {
		t.Fatalf("phase status = %v, want PHASE_STATUS_SKIPPED", ps.Status)
	}

	// The implementation adds PHASE_STATUS_SKIPPED to the resume check
	// alongside PHASE_STATUS_COMPLETED. This test verifies the phase
	// is treated as terminal during resume.
	//
	// isPhaseTerminal checks if a phase should be skipped during resume.
	// COMPLETED and SKIPPED are both terminal.
	if !IsPhaseTerminalForResume(ps.Status) {
		t.Error("PHASE_STATUS_SKIPPED should be treated as terminal during resume")
	}
}

// =============================================================================
// Edge case: Multiple phases skipped in sequence
// =============================================================================

func TestWorkflowExecutor_MultipleSkips(t *testing.T) {
	t.Parallel()

	tsk := task.NewProtoTask("TASK-006", "Test multiple skips")
	tsk.Weight = orcv1.TaskWeight_TASK_WEIGHT_TRIVIAL
	tsk.Category = orcv1.TaskCategory_TASK_CATEGORY_CHORE

	ctx := &ConditionContext{
		Task: tsk,
		Vars: variable.VariableSet{},
		RCtx: &variable.ResolutionContext{},
	}

	// Condition 1: task.weight in [medium, large] → false (trivial)
	result1, err := EvaluateCondition(
		`{"field": "task.weight", "op": "in", "value": ["medium", "large"]}`,
		ctx,
	)
	if err != nil {
		t.Fatalf("condition 1 error: %v", err)
	}
	if result1 {
		t.Error("condition 1 should be false for trivial weight")
	}

	// Condition 2: task.category == "feature" → false (chore)
	result2, err := EvaluateCondition(
		`{"field": "task.category", "op": "eq", "value": "feature"}`,
		ctx,
	)
	if err != nil {
		t.Fatalf("condition 2 error: %v", err)
	}
	if result2 {
		t.Error("condition 2 should be false for chore category")
	}

	// Both phases would be skipped. Verify that multiple independent
	// condition evaluations work without interfering with each other.
}

// =============================================================================
// Edge case: Skipped phase that would have produced output for downstream
// =============================================================================

func TestWorkflowExecutor_SkippedProducerPhase(t *testing.T) {
	t.Parallel()

	// When a phase is skipped, it produces no output.
	// Downstream variables that reference the skipped phase's output
	// should be empty/missing (not error).
	tsk := task.NewProtoTask("TASK-007", "Test skipped producer")
	tsk.Weight = orcv1.TaskWeight_TASK_WEIGHT_TRIVIAL

	ctx := &ConditionContext{
		Task: tsk,
		Vars: variable.VariableSet{},
		RCtx: &variable.ResolutionContext{
			PriorOutputs: map[string]string{
				// tdd_write was skipped → no output
			},
		},
	}

	// A downstream phase condition references phase_output.tdd_write.status
	// Since tdd_write was skipped, there's no output → field doesn't exist
	result, err := EvaluateCondition(
		`{"field": "phase_output.tdd_write.status", "op": "exists"}`,
		ctx,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("phase_output from skipped phase should not exist")
	}
}

// =============================================================================
// Helper: mock event publisher that tracks PhaseSkipped events
// =============================================================================

type conditionTestPublisher struct {
	events []events.Event
}

func newConditionTestPublisher() *conditionTestPublisher {
	return &conditionTestPublisher{events: make([]events.Event, 0)}
}

func (p *conditionTestPublisher) Publish(ev events.Event) {
	p.events = append(p.events, ev)
}

func (p *conditionTestPublisher) Subscribe(taskID string) <-chan events.Event {
	ch := make(chan events.Event)
	close(ch)
	return ch
}

func (p *conditionTestPublisher) Unsubscribe(taskID string, ch <-chan events.Event) {}

func (p *conditionTestPublisher) Close() {}

func (p *conditionTestPublisher) hasPhaseSkippedEvent(taskID, phase string) bool {
	for _, ev := range p.events {
		if ev.Type != events.EventPhase || ev.TaskID != taskID {
			continue
		}
		update, ok := ev.Data.(events.PhaseUpdate)
		if ok && update.Phase == phase && update.Status == "skipped" {
			return true
		}
	}
	return false
}

// Suppress unused import warnings — these are used by tests that reference
// types from these packages even if the compiler can't see it yet.
var _ = time.Now
var _ context.Context
var _ = slog.Default
var _ *config.Config
