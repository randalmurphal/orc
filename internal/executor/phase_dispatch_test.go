// Integration tests for TASK-004: Phase type dispatch in executePhase().
//
// Coverage mapping:
//   SC-2: TestLLMPhaseBackwardCompat_EmptyType, TestLLMPhaseBackwardCompat_ExplicitLLM
//   SC-4: TestPhaseDispatch_NonLLMExecutor, TestPhaseDispatch_UnknownTypeFails
//
// These tests verify that the main executor loop dispatches to the correct
// phase type executor based on the phase template's type field, and that
// existing LLM phases continue to work identically.
package executor

import (
	"context"
	"log/slog"
	"testing"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/internal/variable"
)

// =============================================================================
// SC-4: executePhase dispatches to registered non-LLM executor
// =============================================================================

// TestPhaseDispatch_NonLLMExecutor verifies that executePhase() calls the
// registered executor for a non-LLM phase type instead of executeWithClaude().
// Uses a spy executor to verify the dispatch actually occurred.
func TestPhaseDispatch_NonLLMExecutor(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	gdb := testGlobalDBFrom(backend)

	// Create a spy executor that records when it's called
	spy := &spyPhaseTypeExecutor{
		name: "test-spy",
		result: PhaseResult{
			PhaseID: "test-phase",
			Status:  orcv1.PhaseStatus_PHASE_STATUS_COMPLETED.String(),
			Content: "spy executor output",
		},
	}

	// Save a phase template with a custom type
	tmpl := &db.PhaseTemplate{
		ID:           "test-phase",
		Name:         "Test Phase",
		PromptSource: "db",
		Type:         "test-spy", // Custom non-LLM type
	}
	if err := gdb.SavePhaseTemplate(tmpl); err != nil {
		t.Fatalf("save phase template: %v", err)
	}

	// Save workflow with the custom phase
	wf := &db.Workflow{ID: "test-wf", Name: "Test WF"}
	if err := gdb.SaveWorkflow(wf); err != nil {
		t.Fatalf("save workflow: %v", err)
	}
	wfPhase := &db.WorkflowPhase{
		WorkflowID:      "test-wf",
		PhaseTemplateID: "test-phase",
		Sequence:        0,
	}
	if err := gdb.SaveWorkflowPhase(wfPhase); err != nil {
		t.Fatalf("save workflow phase: %v", err)
	}

	// Create executor with the spy registered
	we := NewWorkflowExecutor(
		backend, nil, gdb, &config.Config{}, t.TempDir(),
		WithWorkflowLogger(slog.Default()),
		WithSkipGates(true),
		WithPhaseTypeExecutor("test-spy", spy), // Register the spy
	)

	// Create task
	tsk := task.NewProtoTask("TASK-001", "Test dispatch")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Create run and run phase
	run := &db.WorkflowRun{
		ID:          "run-001",
		WorkflowID:  "test-wf",
		TaskID:      &tsk.Id,
		ContextType: "task",
		Prompt:      "test",
		Status:      "running",
	}
	if err := backend.SaveWorkflowRun(run); err != nil {
		t.Fatalf("save run: %v", err)
	}
	runPhase := &db.WorkflowRunPhase{
		WorkflowRunID:   "run-001",
		PhaseTemplateID: "test-phase",
		Status:          orcv1.PhaseStatus_PHASE_STATUS_PENDING.String(),
	}
	if err := backend.SaveWorkflowRunPhase(runPhase); err != nil {
		t.Fatalf("save run phase: %v", err)
	}

	vars := variable.VariableSet{}
	rctx := &variable.ResolutionContext{
		PhaseOutputVars: make(map[string]string),
	}

	// Call executePhase — should dispatch to spy, NOT executeWithClaude
	result, err := we.executePhase(
		context.Background(), tmpl, wfPhase, vars, rctx, run, runPhase, tsk,
	)
	if err != nil {
		t.Fatalf("executePhase error: %v", err)
	}

	// Verify spy was called
	if !spy.called {
		t.Fatal("spy executor was not called — dispatch did not occur")
	}

	// Verify result came from spy
	if result.Content != "spy executor output" {
		t.Errorf("content = %q, want %q", result.Content, "spy executor output")
	}
	if result.Status != orcv1.PhaseStatus_PHASE_STATUS_COMPLETED.String() {
		t.Errorf("status = %q, want COMPLETED", result.Status)
	}
}

// =============================================================================
// SC-4 (error path): Unknown phase type fails with descriptive error
// =============================================================================

func TestPhaseDispatch_UnknownTypeFails(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	gdb := testGlobalDBFrom(backend)

	// Phase template with unknown type
	tmpl := &db.PhaseTemplate{
		ID:           "bad-phase",
		Name:         "Bad Phase",
		PromptSource: "db",
		Type:         "nonexistent_type",
	}
	if err := gdb.SavePhaseTemplate(tmpl); err != nil {
		t.Fatalf("save phase template: %v", err)
	}

	wf := &db.Workflow{ID: "test-wf", Name: "Test WF"}
	if err := gdb.SaveWorkflow(wf); err != nil {
		t.Fatalf("save workflow: %v", err)
	}
	wfPhase := &db.WorkflowPhase{
		WorkflowID:      "test-wf",
		PhaseTemplateID: "bad-phase",
		Sequence:        0,
	}
	if err := gdb.SaveWorkflowPhase(wfPhase); err != nil {
		t.Fatalf("save workflow phase: %v", err)
	}

	we := NewWorkflowExecutor(
		backend, nil, gdb, &config.Config{}, t.TempDir(),
		WithWorkflowLogger(slog.Default()),
		WithSkipGates(true),
	)

	tsk := task.NewProtoTask("TASK-002", "Test unknown type")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	run := &db.WorkflowRun{
		ID:          "run-002",
		WorkflowID:  "test-wf",
		TaskID:      &tsk.Id,
		ContextType: "task",
		Prompt:      "test",
		Status:      "running",
	}
	if err := backend.SaveWorkflowRun(run); err != nil {
		t.Fatalf("save run: %v", err)
	}
	runPhase := &db.WorkflowRunPhase{
		WorkflowRunID:   "run-002",
		PhaseTemplateID: "bad-phase",
		Status:          orcv1.PhaseStatus_PHASE_STATUS_PENDING.String(),
	}
	if err := backend.SaveWorkflowRunPhase(runPhase); err != nil {
		t.Fatalf("save run phase: %v", err)
	}

	vars := variable.VariableSet{}
	rctx := &variable.ResolutionContext{
		PhaseOutputVars: make(map[string]string),
	}

	_, err := we.executePhase(
		context.Background(), tmpl, wfPhase, vars, rctx, run, runPhase, tsk,
	)
	if err == nil {
		t.Fatal("expected error for unknown phase type")
	}

	// Error should include the unknown type
	errStr := err.Error()
	if !containsSubstring(errStr, "nonexistent_type") {
		t.Errorf("error should include unknown type, got: %q", errStr)
	}
}

// =============================================================================
// SC-2: LLM phases with empty type execute identically to current behavior
// =============================================================================

// TestLLMPhaseBackwardCompat_EmptyType verifies that a phase with type=""
// (the default for all existing phases) routes to the Claude execution path,
// same as before this change.
func TestLLMPhaseBackwardCompat_EmptyType(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	gdb := testGlobalDBFrom(backend)

	// Phase template with empty type (existing behavior)
	tmpl := &db.PhaseTemplate{
		ID:            "implement",
		Name:          "Implement",
		PromptSource:  "db",
		PromptContent: "Test prompt: implement the feature",
		// Type is empty — should default to LLM path
	}
	if err := gdb.SavePhaseTemplate(tmpl); err != nil {
		t.Fatalf("save phase template: %v", err)
	}

	// MockTurnExecutor returns JSON response string (matching existing pattern)
	mockTurns := &MockTurnExecutor{
		Responses: []string{
			`{"status": "complete", "summary": "done", "content": "implemented"}`,
		},
	}

	wf := &db.Workflow{ID: "test-wf", Name: "Test WF"}
	if err := gdb.SaveWorkflow(wf); err != nil {
		t.Fatalf("save workflow: %v", err)
	}
	wfPhase := &db.WorkflowPhase{
		WorkflowID:      "test-wf",
		PhaseTemplateID: "implement",
		Sequence:        0,
	}
	if err := gdb.SaveWorkflowPhase(wfPhase); err != nil {
		t.Fatalf("save workflow phase: %v", err)
	}

	we := NewWorkflowExecutor(
		backend, nil, gdb, &config.Config{}, t.TempDir(),
		WithWorkflowTurnExecutor(mockTurns),
		WithWorkflowLogger(slog.Default()),
		WithSkipGates(true),
	)

	tsk := task.NewProtoTask("TASK-003", "Test backward compat")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	run := &db.WorkflowRun{
		ID:          "run-003",
		WorkflowID:  "test-wf",
		TaskID:      &tsk.Id,
		ContextType: "task",
		Prompt:      "test",
		Status:      "running",
	}
	if err := backend.SaveWorkflowRun(run); err != nil {
		t.Fatalf("save run: %v", err)
	}
	runPhase := &db.WorkflowRunPhase{
		WorkflowRunID:   "run-003",
		PhaseTemplateID: "implement",
		Status:          orcv1.PhaseStatus_PHASE_STATUS_PENDING.String(),
	}
	if err := backend.SaveWorkflowRunPhase(runPhase); err != nil {
		t.Fatalf("save run phase: %v", err)
	}

	vars := variable.VariableSet{}
	rctx := &variable.ResolutionContext{
		PhaseOutputVars: make(map[string]string),
		PriorOutputs:    make(map[string]string),
	}

	result, err := we.executePhase(
		context.Background(), tmpl, wfPhase, vars, rctx, run, runPhase, tsk,
	)
	if err != nil {
		t.Fatalf("executePhase error: %v", err)
	}

	// Verify it went through Claude path (mockTurns was called)
	if mockTurns.callCount == 0 {
		t.Fatal("MockTurnExecutor was not called — empty type should route to LLM path")
	}

	// Verify completion
	if result.Status != orcv1.PhaseStatus_PHASE_STATUS_COMPLETED.String() {
		t.Errorf("status = %q, want COMPLETED", result.Status)
	}
}

// TestLLMPhaseBackwardCompat_ExplicitLLM verifies that type="llm" routes
// to the same Claude execution path as empty type.
func TestLLMPhaseBackwardCompat_ExplicitLLM(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	gdb := testGlobalDBFrom(backend)

	tmpl := &db.PhaseTemplate{
		ID:            "implement",
		Name:          "Implement",
		PromptSource:  "db",
		PromptContent: "Test prompt: implement the feature",
		Type:          "llm", // Explicit LLM type
	}
	if err := gdb.SavePhaseTemplate(tmpl); err != nil {
		t.Fatalf("save phase template: %v", err)
	}

	mockTurns := &MockTurnExecutor{
		Responses: []string{
			`{"status": "complete", "summary": "done", "content": "implemented"}`,
		},
	}

	wf := &db.Workflow{ID: "test-wf", Name: "Test WF"}
	if err := gdb.SaveWorkflow(wf); err != nil {
		t.Fatalf("save workflow: %v", err)
	}
	wfPhase := &db.WorkflowPhase{
		WorkflowID:      "test-wf",
		PhaseTemplateID: "implement",
		Sequence:        0,
	}
	if err := gdb.SaveWorkflowPhase(wfPhase); err != nil {
		t.Fatalf("save workflow phase: %v", err)
	}

	we := NewWorkflowExecutor(
		backend, nil, gdb, &config.Config{}, t.TempDir(),
		WithWorkflowTurnExecutor(mockTurns),
		WithWorkflowLogger(slog.Default()),
		WithSkipGates(true),
	)

	tsk := task.NewProtoTask("TASK-004", "Test explicit llm type")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	run := &db.WorkflowRun{
		ID:          "run-004",
		WorkflowID:  "test-wf",
		TaskID:      &tsk.Id,
		ContextType: "task",
		Prompt:      "test",
		Status:      "running",
	}
	if err := backend.SaveWorkflowRun(run); err != nil {
		t.Fatalf("save run: %v", err)
	}
	runPhase := &db.WorkflowRunPhase{
		WorkflowRunID:   "run-004",
		PhaseTemplateID: "implement",
		Status:          orcv1.PhaseStatus_PHASE_STATUS_PENDING.String(),
	}
	if err := backend.SaveWorkflowRunPhase(runPhase); err != nil {
		t.Fatalf("save run phase: %v", err)
	}

	vars := variable.VariableSet{}
	rctx := &variable.ResolutionContext{
		PhaseOutputVars: make(map[string]string),
		PriorOutputs:    make(map[string]string),
	}

	result, err := we.executePhase(
		context.Background(), tmpl, wfPhase, vars, rctx, run, runPhase, tsk,
	)
	if err != nil {
		t.Fatalf("executePhase error: %v", err)
	}

	// Verify Claude path was used
	if mockTurns.callCount == 0 {
		t.Fatal("MockTurnExecutor was not called — type='llm' should route to LLM path")
	}

	if result.Status != orcv1.PhaseStatus_PHASE_STATUS_COMPLETED.String() {
		t.Errorf("status = %q, want COMPLETED", result.Status)
	}
}

// =============================================================================
// Edge case: WorkflowPhase type override takes precedence over template type
// =============================================================================

func TestPhaseDispatch_TypeOverride(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	gdb := testGlobalDBFrom(backend)

	// Phase template has type="knowledge"
	tmpl := &db.PhaseTemplate{
		ID:            "gather-context",
		Name:          "Gather Context",
		PromptSource:  "db",
		PromptContent: "Gather context for the task: {{TASK_DESCRIPTION}}",
		Type:          "knowledge",
	}
	if err := gdb.SavePhaseTemplate(tmpl); err != nil {
		t.Fatalf("save phase template: %v", err)
	}

	// But workflow phase overrides type to "llm"
	wf := &db.Workflow{ID: "test-wf", Name: "Test WF"}
	if err := gdb.SaveWorkflow(wf); err != nil {
		t.Fatalf("save workflow: %v", err)
	}
	wfPhase := &db.WorkflowPhase{
		WorkflowID:      "test-wf",
		PhaseTemplateID: "gather-context",
		Sequence:        0,
		TypeOverride:    "llm", // Override template type
	}
	if err := gdb.SaveWorkflowPhase(wfPhase); err != nil {
		t.Fatalf("save workflow phase: %v", err)
	}

	mockTurns := &MockTurnExecutor{
		Responses: []string{
			`{"status": "complete", "summary": "done", "content": "override works"}`,
		},
	}

	we := NewWorkflowExecutor(
		backend, nil, gdb, &config.Config{}, t.TempDir(),
		WithWorkflowTurnExecutor(mockTurns),
		WithWorkflowLogger(slog.Default()),
		WithSkipGates(true),
	)

	tsk := task.NewProtoTask("TASK-005", "Test type override")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	run := &db.WorkflowRun{
		ID:          "run-005",
		WorkflowID:  "test-wf",
		TaskID:      &tsk.Id,
		ContextType: "task",
		Prompt:      "test",
		Status:      "running",
	}
	if err := backend.SaveWorkflowRun(run); err != nil {
		t.Fatalf("save run: %v", err)
	}
	runPhase := &db.WorkflowRunPhase{
		WorkflowRunID:   "run-005",
		PhaseTemplateID: "gather-context",
		Status:          orcv1.PhaseStatus_PHASE_STATUS_PENDING.String(),
	}
	if err := backend.SaveWorkflowRunPhase(runPhase); err != nil {
		t.Fatalf("save run phase: %v", err)
	}

	vars := variable.VariableSet{}
	rctx := &variable.ResolutionContext{
		PhaseOutputVars: make(map[string]string),
		PriorOutputs:    make(map[string]string),
	}

	result, err := we.executePhase(
		context.Background(), tmpl, wfPhase, vars, rctx, run, runPhase, tsk,
	)
	if err != nil {
		t.Fatalf("executePhase error: %v", err)
	}

	// WorkflowPhase override should route to LLM, not knowledge
	if mockTurns.callCount == 0 {
		t.Fatal("MockTurnExecutor was not called — type override to 'llm' should use LLM path")
	}

	if result.Status != orcv1.PhaseStatus_PHASE_STATUS_COMPLETED.String() {
		t.Errorf("status = %q, want COMPLETED", result.Status)
	}
}

// =============================================================================
// Helper: spy phase type executor that records calls
// =============================================================================

type spyPhaseTypeExecutor struct {
	name   string
	called bool
	params PhaseTypeParams
	result PhaseResult
	err    error
}

func (s *spyPhaseTypeExecutor) ExecutePhase(
	ctx context.Context,
	params PhaseTypeParams,
) (PhaseResult, error) {
	s.called = true
	s.params = params
	return s.result, s.err
}

func (s *spyPhaseTypeExecutor) Name() string {
	return s.name
}
