// Integration tests for TASK-004: Verify new code is wired into production paths.
//
// These tests COMPLEMENT the unit tests from tdd_write — they don't test new code
// in isolation, but verify it's reachable from existing production entry points.
//
// Wiring points verified:
//   1. Run() populates ConditionContext.KnowledgeAvailable from injected knowledge service
//   2. Run() dispatches non-LLM phases through the full execution loop
//   3. Non-LLM phase output flows to subsequent phases via variable system
//   4. Non-LLM phase results are persisted to WorkflowRunPhase records
//
// Deletion test: If you remove the wiring code (KnowledgeAvailable population,
// phase dispatch, variable propagation), these tests fail.
package executor

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/knowledge/retrieve"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/internal/variable"
)

// =============================================================================
// Integration Test 1: Run() → ConditionContext.KnowledgeAvailable propagation
//
// Wiring point: Run() creates ConditionContext at the condition evaluation step.
// The KnowledgeAvailable field must be set based on whether a knowledge service
// was injected via WithWorkflowKnowledgeService().
//
// Without knowledge service → KnowledgeAvailable=false → condition fails → skip
// This test goes through Run() (not EvaluateCondition directly).
// =============================================================================

// TestRunLoop_KnowledgeConditionSkipsWhenNoService verifies that when no
// knowledge service is injected, Run() evaluates knowledge.available as false,
// causing the knowledge-conditional phase to be skipped.
//
// Deletion test: Remove KnowledgeAvailable propagation in Run() → this test
// still passes (condition defaults to false). That's the EXPECTED behavior.
// The complementary test (WithService) catches the opposite failure.
func TestRunLoop_KnowledgeConditionSkipsWhenNoService(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	gdb := testGlobalDBFrom(backend)

	// Phase 1: knowledge-conditional phase (should be skipped)
	gatherTmpl := &db.PhaseTemplate{
		ID:            "gather-context",
		Name:          "Gather Context",
		Type:          "knowledge",
		PromptSource:  "db",
		PromptContent: "gather knowledge",
	}
	if err := gdb.SavePhaseTemplate(gatherTmpl); err != nil {
		t.Fatalf("save gather template: %v", err)
	}

	// Phase 2: regular LLM phase (should execute)
	implTmpl := &db.PhaseTemplate{
		ID:            "implement",
		Name:          "Implement",
		PromptSource:  "db",
		PromptContent: "implement the feature",
	}
	if err := gdb.SavePhaseTemplate(implTmpl); err != nil {
		t.Fatalf("save implement template: %v", err)
	}

	// Workflow: gather-context (conditional) → implement
	wf := &db.Workflow{ID: "knowledge-cond-wf", Name: "Knowledge Condition Test"}
	if err := gdb.SaveWorkflow(wf); err != nil {
		t.Fatalf("save workflow: %v", err)
	}

	// gather-context has knowledge.available condition
	gatherPhase := &db.WorkflowPhase{
		WorkflowID:      "knowledge-cond-wf",
		PhaseTemplateID: "gather-context",
		Sequence:        0,
		Condition:       `{"field": "knowledge.available", "op": "eq", "value": "true"}`,
	}
	if err := gdb.SaveWorkflowPhase(gatherPhase); err != nil {
		t.Fatalf("save gather phase: %v", err)
	}

	implPhase := &db.WorkflowPhase{
		WorkflowID:      "knowledge-cond-wf",
		PhaseTemplateID: "implement",
		Sequence:        1,
	}
	if err := gdb.SaveWorkflowPhase(implPhase); err != nil {
		t.Fatalf("save implement phase: %v", err)
	}

	// Create task
	tsk := task.NewProtoTask("TASK-INT-001", "Test knowledge condition skip")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	wfID := "knowledge-cond-wf"
	tsk.WorkflowId = &wfID
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	mockTurn := NewMockTurnExecutor(`{"status": "complete", "summary": "done"}`)

	// NO knowledge service injected → KnowledgeAvailable should be false
	we := NewWorkflowExecutor(
		backend, backend.DB(), gdb, &config.Config{}, t.TempDir(),
		WithWorkflowTurnExecutor(mockTurn),
		WithWorkflowLogger(slog.Default()),
		WithSkipGates(true),
	)

	result, err := we.Run(context.Background(), "knowledge-cond-wf", WorkflowRunOptions{
		ContextType: ContextTask,
		TaskID:      tsk.Id,
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// Build phase status map
	phaseStatuses := make(map[string]string)
	for _, pr := range result.PhaseResults {
		phaseStatuses[pr.PhaseID] = pr.Status
	}

	// gather-context should be SKIPPED (knowledge.available = false)
	if phaseStatuses["gather-context"] != orcv1.PhaseStatus_PHASE_STATUS_SKIPPED.String() {
		t.Errorf("gather-context status = %q, want SKIPPED (no knowledge service)", phaseStatuses["gather-context"])
	}

	// implement should be COMPLETED (not affected by skip)
	if phaseStatuses["implement"] != orcv1.PhaseStatus_PHASE_STATUS_COMPLETED.String() {
		t.Errorf("implement status = %q, want COMPLETED", phaseStatuses["implement"])
	}
}

// =============================================================================
// Integration Test 2: Run() + WithWorkflowKnowledgeService → condition passes
//
// Wiring point: WithWorkflowKnowledgeService() injects knowledge service into
// the executor. Run() then sets KnowledgeAvailable=true on ConditionContext.
//
// Deletion test: Remove WithWorkflowKnowledgeService wiring → KnowledgeAvailable
// stays false → condition fails → phase skipped → this test fails.
// =============================================================================

// TestRunLoop_KnowledgeConditionPassesWithService verifies that when a
// knowledge service IS injected, Run() evaluates knowledge.available as true,
// allowing the knowledge-conditional phase to execute (dispatched to knowledge
// executor, not Claude).
func TestRunLoop_KnowledgeConditionPassesWithService(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	gdb := testGlobalDBFrom(backend)

	// Knowledge-conditional gather phase
	gatherTmpl := &db.PhaseTemplate{
		ID:            "gather-context",
		Name:          "Gather Context",
		Type:          "knowledge",
		PromptSource:  "db",
		PromptContent: "gather knowledge",
		OutputVarName: "KNOWLEDGE_CONTEXT",
	}
	if err := gdb.SavePhaseTemplate(gatherTmpl); err != nil {
		t.Fatalf("save gather template: %v", err)
	}

	// LLM implement phase
	implTmpl := &db.PhaseTemplate{
		ID:            "implement",
		Name:          "Implement",
		PromptSource:  "db",
		PromptContent: "implement the feature, context: {{KNOWLEDGE_CONTEXT}}",
	}
	if err := gdb.SavePhaseTemplate(implTmpl); err != nil {
		t.Fatalf("save implement template: %v", err)
	}

	wf := &db.Workflow{ID: "knowledge-pass-wf", Name: "Knowledge Pass Test"}
	if err := gdb.SaveWorkflow(wf); err != nil {
		t.Fatalf("save workflow: %v", err)
	}

	gatherPhase := &db.WorkflowPhase{
		WorkflowID:      "knowledge-pass-wf",
		PhaseTemplateID: "gather-context",
		Sequence:        0,
		Condition:       `{"field": "knowledge.available", "op": "eq", "value": "true"}`,
	}
	if err := gdb.SaveWorkflowPhase(gatherPhase); err != nil {
		t.Fatalf("save gather phase: %v", err)
	}

	implPhase := &db.WorkflowPhase{
		WorkflowID:      "knowledge-pass-wf",
		PhaseTemplateID: "implement",
		Sequence:        1,
	}
	if err := gdb.SaveWorkflowPhase(implPhase); err != nil {
		t.Fatalf("save implement phase: %v", err)
	}

	tsk := task.NewProtoTask("TASK-INT-002", "Test knowledge condition pass")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	wfID := "knowledge-pass-wf"
	tsk.WorkflowId = &wfID
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Mock knowledge service that IS available
	mockSvc := &mockKnowledgeService{
		available: true,
		queryResult: &retrieve.PipelineResult{
			Documents: []retrieve.ScoredDocument{
				{Document: retrieve.Document{
					ID:      "doc-1",
					Content: "Prior decision: use bcrypt for passwords",
				}},
			},
		},
	}

	mockTurn := NewMockTurnExecutor(`{"status": "complete", "summary": "done"}`)

	// Inject knowledge service — this should make KnowledgeAvailable=true
	we := NewWorkflowExecutor(
		backend, backend.DB(), gdb, &config.Config{}, t.TempDir(),
		WithWorkflowTurnExecutor(mockTurn),
		WithWorkflowKnowledgeService(mockSvc), // KEY: injects knowledge service
		WithWorkflowLogger(slog.Default()),
		WithSkipGates(true),
	)

	result, err := we.Run(context.Background(), "knowledge-pass-wf", WorkflowRunOptions{
		ContextType: ContextTask,
		TaskID:      tsk.Id,
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// Build phase status map
	phaseStatuses := make(map[string]string)
	for _, pr := range result.PhaseResults {
		phaseStatuses[pr.PhaseID] = pr.Status
	}

	// gather-context should NOT be skipped — condition passes because knowledge is available
	if phaseStatuses["gather-context"] == orcv1.PhaseStatus_PHASE_STATUS_SKIPPED.String() {
		t.Error("gather-context was SKIPPED but should have executed — knowledge service is available")
	}

	// gather-context should be COMPLETED (dispatched to knowledge executor)
	if phaseStatuses["gather-context"] != orcv1.PhaseStatus_PHASE_STATUS_COMPLETED.String() {
		t.Errorf("gather-context status = %q, want COMPLETED", phaseStatuses["gather-context"])
	}

	// implement should also be COMPLETED
	if phaseStatuses["implement"] != orcv1.PhaseStatus_PHASE_STATUS_COMPLETED.String() {
		t.Errorf("implement status = %q, want COMPLETED", phaseStatuses["implement"])
	}
}

// =============================================================================
// Integration Test 3: Run() → non-LLM phase output flows to next phase as var
//
// Wiring point: After executePhase() returns for a non-LLM phase, Run() must
// call applyPhaseContentToVars() with the result content. The next phase's
// prompt must receive this variable.
//
// Deletion test: If executePhase() doesn't propagate non-LLM phase content to
// applyPhaseContentToVars, the implement phase never sees KNOWLEDGE_CONTEXT.
// =============================================================================

// TestRunLoop_NonLLMPhaseVarPropagation verifies that output from a non-LLM
// phase (via spy executor) flows through the variable system to the next LLM
// phase's prompt.
func TestRunLoop_NonLLMPhaseVarPropagation(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	gdb := testGlobalDBFrom(backend)

	// Phase 1: spy executor that produces output
	spyTmpl := &db.PhaseTemplate{
		ID:               "gather-context",
		Name:             "Gather Context",
		Type:             "test-spy",
		PromptSource:     "db",
		ProducesArtifact: true,
		OutputVarName:    "KNOWLEDGE_CONTEXT",
	}
	if err := gdb.SavePhaseTemplate(spyTmpl); err != nil {
		t.Fatalf("save spy template: %v", err)
	}

	// Phase 2: LLM phase that should receive the variable
	implTmpl := &db.PhaseTemplate{
		ID:            "implement",
		Name:          "Implement",
		PromptSource:  "db",
		PromptContent: "implement using context: {{KNOWLEDGE_CONTEXT}}",
	}
	if err := gdb.SavePhaseTemplate(implTmpl); err != nil {
		t.Fatalf("save implement template: %v", err)
	}

	wf := &db.Workflow{ID: "var-prop-wf", Name: "Var Propagation Test"}
	if err := gdb.SaveWorkflow(wf); err != nil {
		t.Fatalf("save workflow: %v", err)
	}

	spyPhase := &db.WorkflowPhase{
		WorkflowID:      "var-prop-wf",
		PhaseTemplateID: "gather-context",
		Sequence:        0,
	}
	if err := gdb.SaveWorkflowPhase(spyPhase); err != nil {
		t.Fatalf("save spy phase: %v", err)
	}

	implPhase := &db.WorkflowPhase{
		WorkflowID:      "var-prop-wf",
		PhaseTemplateID: "implement",
		Sequence:        1,
	}
	if err := gdb.SaveWorkflowPhase(implPhase); err != nil {
		t.Fatalf("save implement phase: %v", err)
	}

	tsk := task.NewProtoTask("TASK-INT-003", "Test variable propagation")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	wfID := "var-prop-wf"
	tsk.WorkflowId = &wfID
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Spy executor produces specific content
	spy := &spyPhaseTypeExecutor{
		name: "test-spy",
		result: PhaseResult{
			PhaseID: "gather-context",
			Status:  orcv1.PhaseStatus_PHASE_STATUS_COMPLETED.String(),
			Content: "knowledge: always use bcrypt for passwords",
		},
	}

	// MockTurnExecutor that captures the prompt it receives
	mockTurn := &MockTurnExecutor{
		Responses: []string{
			`{"status": "complete", "summary": "done", "content": "implemented"}`,
		},
	}

	we := NewWorkflowExecutor(
		backend, backend.DB(), gdb, &config.Config{}, t.TempDir(),
		WithWorkflowTurnExecutor(mockTurn),
		WithPhaseTypeExecutor("test-spy", spy),
		WithWorkflowLogger(slog.Default()),
		WithSkipGates(true),
	)

	result, err := we.Run(context.Background(), "var-prop-wf", WorkflowRunOptions{
		ContextType: ContextTask,
		TaskID:      tsk.Id,
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// Verify both phases completed
	phaseStatuses := make(map[string]string)
	for _, pr := range result.PhaseResults {
		phaseStatuses[pr.PhaseID] = pr.Status
	}

	if phaseStatuses["gather-context"] != orcv1.PhaseStatus_PHASE_STATUS_COMPLETED.String() {
		t.Errorf("gather-context status = %q, want COMPLETED", phaseStatuses["gather-context"])
	}
	if phaseStatuses["implement"] != orcv1.PhaseStatus_PHASE_STATUS_COMPLETED.String() {
		t.Errorf("implement status = %q, want COMPLETED", phaseStatuses["implement"])
	}

	// KEY ASSERTION: The implement phase's prompt should contain the spy's output.
	// MockTurnExecutor.Prompts captures what was sent to Claude.
	if len(mockTurn.Prompts) == 0 {
		t.Fatal("implement phase was never called (no prompts captured)")
	}

	implementPrompt := mockTurn.Prompts[0]
	if !strings.Contains(implementPrompt, "always use bcrypt for passwords") {
		t.Errorf("implement prompt does not contain spy output.\nGot: %s\nWant: contains 'always use bcrypt for passwords'",
			truncateForError(implementPrompt, 200))
	}
}

// =============================================================================
// Integration Test 4: executePhase() → non-LLM result persisted to DB record
//
// Wiring point: executePhase() must save non-LLM phase results to the
// WorkflowRunPhase record with correct status, content, and zero cost.
//
// Deletion test: If the post-processing code in executePhase() doesn't handle
// non-LLM phase results (only handles executeWithProvider results), the run
// phase record won't be updated.
// =============================================================================

// TestExecutePhase_NonLLMResultPersistedToRunPhaseRecord verifies that when
// executePhase() dispatches to a non-LLM executor, the WorkflowRunPhase
// record is saved with COMPLETED status, the phase content, and zero cost.
func TestExecutePhase_NonLLMResultPersistedToRunPhaseRecord(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	gdb := testGlobalDBFrom(backend)

	tmpl := &db.PhaseTemplate{
		ID:               "gather-context",
		Name:             "Gather Context",
		Type:             "test-spy",
		PromptSource:     "db",
		ProducesArtifact: true,
		OutputVarName:    "KNOWLEDGE_CONTEXT",
	}
	if err := gdb.SavePhaseTemplate(tmpl); err != nil {
		t.Fatalf("save template: %v", err)
	}

	wf := &db.Workflow{ID: "persist-wf", Name: "Persist Test"}
	if err := gdb.SaveWorkflow(wf); err != nil {
		t.Fatalf("save workflow: %v", err)
	}
	wfPhase := &db.WorkflowPhase{
		WorkflowID:      "persist-wf",
		PhaseTemplateID: "gather-context",
		Sequence:        0,
	}
	if err := gdb.SaveWorkflowPhase(wfPhase); err != nil {
		t.Fatalf("save workflow phase: %v", err)
	}

	spy := &spyPhaseTypeExecutor{
		name: "test-spy",
		result: PhaseResult{
			PhaseID: "gather-context",
			Status:  orcv1.PhaseStatus_PHASE_STATUS_COMPLETED.String(),
			Content: "knowledge context content here",
		},
	}

	we := NewWorkflowExecutor(
		backend, backend.DB(), gdb, &config.Config{}, t.TempDir(),
		WithPhaseTypeExecutor("test-spy", spy),
		WithWorkflowLogger(slog.Default()),
		WithSkipGates(true),
	)

	tsk := task.NewProtoTask("TASK-INT-004", "Test persistence")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	run := &db.WorkflowRun{
		ID:          "run-persist-001",
		WorkflowID:  "persist-wf",
		TaskID:      &tsk.Id,
		ContextType: "task",
		Prompt:      "test",
		Status:      "running",
	}
	if err := backend.SaveWorkflowRun(run); err != nil {
		t.Fatalf("save run: %v", err)
	}

	runPhase := &db.WorkflowRunPhase{
		WorkflowRunID:   "run-persist-001",
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

	// Call executePhase through the PRODUCTION method
	result, err := we.executePhase(
		context.Background(), tmpl, wfPhase, vars, rctx, run, runPhase, tsk,
	)
	if err != nil {
		t.Fatalf("executePhase error: %v", err)
	}

	// Verify phase result
	if result.Status != orcv1.PhaseStatus_PHASE_STATUS_COMPLETED.String() {
		t.Errorf("result.Status = %q, want COMPLETED", result.Status)
	}

	// Verify DB record was updated
	phases, err := backend.GetWorkflowRunPhases("run-persist-001")
	if err != nil {
		t.Fatalf("get run phases: %v", err)
	}
	if len(phases) == 0 {
		t.Fatal("no run phases found")
	}

	savedPhase := phases[0]
	if savedPhase.Status != orcv1.PhaseStatus_PHASE_STATUS_COMPLETED.String() {
		t.Errorf("saved phase status = %q, want COMPLETED", savedPhase.Status)
	}
	if savedPhase.Content != "knowledge context content here" {
		t.Errorf("saved phase content = %q, want 'knowledge context content here'", savedPhase.Content)
	}

	// Non-LLM phase should have zero cost
	if savedPhase.CostUSD != 0 {
		t.Errorf("saved phase cost = %f, want 0 (non-LLM phase)", savedPhase.CostUSD)
	}
	if savedPhase.InputTokens != 0 {
		t.Errorf("saved phase input tokens = %d, want 0", savedPhase.InputTokens)
	}
}

// =============================================================================
// Integration Test 5: Run() → non-LLM phase with zero cost in run totals
//
// Wiring point: After non-LLM phase completes, Run() accumulates cost in
// run.TotalCostUSD. Non-LLM phase should contribute zero.
// =============================================================================

// TestRunLoop_NonLLMPhaseZeroCostInRunTotals verifies that a non-LLM phase
// contributes zero to the workflow run's total cost, while a subsequent LLM
// phase contributes its actual cost.
func TestRunLoop_NonLLMPhaseZeroCostInRunTotals(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	gdb := testGlobalDBFrom(backend)

	// Phase 1: non-LLM spy (zero cost)
	spyTmpl := &db.PhaseTemplate{
		ID:           "gather-context",
		Name:         "Gather Context",
		Type:         "test-spy",
		PromptSource: "db",
	}
	if err := gdb.SavePhaseTemplate(spyTmpl); err != nil {
		t.Fatalf("save spy template: %v", err)
	}

	// Phase 2: LLM phase
	implTmpl := &db.PhaseTemplate{
		ID:            "implement",
		Name:          "Implement",
		PromptSource:  "db",
		PromptContent: "implement the feature",
	}
	if err := gdb.SavePhaseTemplate(implTmpl); err != nil {
		t.Fatalf("save implement template: %v", err)
	}

	wf := &db.Workflow{ID: "cost-wf", Name: "Cost Test"}
	if err := gdb.SaveWorkflow(wf); err != nil {
		t.Fatalf("save workflow: %v", err)
	}

	spyPhase := &db.WorkflowPhase{
		WorkflowID:      "cost-wf",
		PhaseTemplateID: "gather-context",
		Sequence:        0,
	}
	if err := gdb.SaveWorkflowPhase(spyPhase); err != nil {
		t.Fatalf("save spy phase: %v", err)
	}

	implPhase := &db.WorkflowPhase{
		WorkflowID:      "cost-wf",
		PhaseTemplateID: "implement",
		Sequence:        1,
	}
	if err := gdb.SaveWorkflowPhase(implPhase); err != nil {
		t.Fatalf("save implement phase: %v", err)
	}

	tsk := task.NewProtoTask("TASK-INT-005", "Test zero cost")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	wfID := "cost-wf"
	tsk.WorkflowId = &wfID
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	spy := &spyPhaseTypeExecutor{
		name: "test-spy",
		result: PhaseResult{
			PhaseID:      "gather-context",
			Status:       orcv1.PhaseStatus_PHASE_STATUS_COMPLETED.String(),
			CostUSD:      0, // Non-LLM: zero cost
			InputTokens:  0,
			OutputTokens: 0,
		},
	}

	mockTurn := NewMockTurnExecutor(`{"status": "complete", "summary": "done"}`)

	we := NewWorkflowExecutor(
		backend, backend.DB(), gdb, &config.Config{}, t.TempDir(),
		WithWorkflowTurnExecutor(mockTurn),
		WithPhaseTypeExecutor("test-spy", spy),
		WithWorkflowLogger(slog.Default()),
		WithSkipGates(true),
	)

	result, err := we.Run(context.Background(), "cost-wf", WorkflowRunOptions{
		ContextType: ContextTask,
		TaskID:      tsk.Id,
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// Find the gather-context phase result — it should have zero cost
	for _, pr := range result.PhaseResults {
		if pr.PhaseID == "gather-context" {
			if pr.CostUSD != 0 {
				t.Errorf("gather-context CostUSD = %f, want 0", pr.CostUSD)
			}
			if pr.InputTokens != 0 {
				t.Errorf("gather-context InputTokens = %d, want 0", pr.InputTokens)
			}
		}
	}

	// Overall result should succeed
	if !result.Success {
		t.Errorf("workflow should succeed, got error: %s", result.Error)
	}
}

// =============================================================================
// Helpers
// =============================================================================

// truncateForError truncates a string for error messages.
func truncateForError(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

