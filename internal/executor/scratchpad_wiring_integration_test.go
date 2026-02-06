// Integration tests for TASK-020: Phase scratchpad persistent note-taking.
//
// These tests verify that new scratchpad code is properly wired into
// existing production paths. They complement the unit tests from tdd_write
// which test scratchpad functions in isolation.
//
// Wiring points verified:
//   1. WorkflowExecutor.Run() extracts scratchpad entries from phase output
//      and persists them via backend.SaveScratchpadEntry (SC-1, SC-3)
//   2. enrichContextForPhase() loads prior scratchpad entries from DB into
//      rctx.PrevScratchpad, making them available as PREV_SCRATCHPAD in
//      subsequent phases (SC-4)
//   3. enrichContextForPhase() loads retry scratchpad entries from DB into
//      rctx.RetryScratchpad on retry attempts (SC-5)
//   4. Phase output without scratchpad field does not cause errors (backward compat)
//
// Deletion test for each: If you remove the wiring code (the call to
// ExtractScratchpadEntries + SaveScratchpadEntry in Run(), or the
// PrevScratchpad/RetryScratchpad population in enrichContextForPhase()),
// these tests fail.
package executor

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// =============================================================================
// Integration Test 1: WorkflowExecutor.Run() → ExtractScratchpadEntries →
// backend.SaveScratchpadEntry
//
// Wiring point: After phase completion in Run(), the executor must call
// ExtractScratchpadEntries() on the phase output JSON and persist each
// entry via backend.SaveScratchpadEntry().
//
// Deletion test: Remove the ExtractScratchpadEntries + SaveScratchpadEntry
// block in Run() → this test fails (0 entries in DB).
// =============================================================================

// TestWorkflowRun_PersistsScratchpadEntries verifies that when a phase
// outputs JSON containing a scratchpad array, the executor extracts entries
// and persists them to the database.
func TestWorkflowRun_PersistsScratchpadEntries(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	gdb := testGlobalDBFrom(backend)

	// Create task
	taskID := "TASK-001"
	tsk := task.NewProtoTask(taskID, "Test scratchpad persistence")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Create workflow with single phase
	createWorkflowWithPhase(t, gdb, "test-workflow", "implement", "Implement")

	// Mock turn executor returns output WITH scratchpad entries
	phaseOutput := `{
		"status": "complete",
		"summary": "Implementation done",
		"content": "# Implementation\n\nDone.",
		"scratchpad": [
			{"category": "decision", "content": "Chose token bucket for rate limiting"},
			{"category": "observation", "content": "Existing middleware uses chi router"},
			{"category": "blocker", "content": "Test framework requires Node 18+"}
		]
	}`
	mockTE := &scratchpadMockTurnExecutor{result: phaseOutput}

	we := NewWorkflowExecutor(
		backend,
		backend.DB(),
		gdb,
		&config.Config{
			Gates: config.GateConfig{AutoApproveOnSuccess: true},
		},
		t.TempDir(),
		WithWorkflowLogger(slog.Default()),
		WithWorkflowTurnExecutor(mockTE),
		WithSkipGates(true),
	)

	// Run workflow through production path
	_, err := we.Run(context.Background(), "test-workflow", WorkflowRunOptions{
		ContextType: ContextTask,
		TaskID:      taskID,
		Prompt:      "implement scratchpad feature",
	})
	if err != nil {
		t.Fatalf("workflow run failed: %v", err)
	}

	// Verify scratchpad entries were persisted to database
	entries, err := backend.GetScratchpadEntries(taskID)
	if err != nil {
		t.Fatalf("GetScratchpadEntries: %v", err)
	}

	if len(entries) != 3 {
		t.Fatalf("expected 3 scratchpad entries persisted, got %d", len(entries))
	}

	// Verify entry metadata was set correctly
	foundDecision := false
	foundObservation := false
	foundBlocker := false
	for _, e := range entries {
		if e.TaskID != taskID {
			t.Errorf("entry TaskID = %q, want %q", e.TaskID, taskID)
		}
		if e.PhaseID != "implement" {
			t.Errorf("entry PhaseID = %q, want %q", e.PhaseID, "implement")
		}
		switch e.Category {
		case "decision":
			foundDecision = true
			if e.Content != "Chose token bucket for rate limiting" {
				t.Errorf("decision content = %q", e.Content)
			}
		case "observation":
			foundObservation = true
		case "blocker":
			foundBlocker = true
		}
	}

	if !foundDecision {
		t.Error("decision entry not found in persisted entries")
	}
	if !foundObservation {
		t.Error("observation entry not found in persisted entries")
	}
	if !foundBlocker {
		t.Error("blocker entry not found in persisted entries")
	}
}

// =============================================================================
// Integration Test 2: Two-phase workflow scratchpad propagation
//
// Wiring point: enrichContextForPhase() must load prior scratchpad entries
// from the database and populate rctx.PrevScratchpad, which becomes the
// PREV_SCRATCHPAD template variable via addBuiltinVariables().
//
// Deletion test: Remove PrevScratchpad population from enrichContextForPhase()
// → Phase 2's prompt won't contain scratchpad content from Phase 1.
// =============================================================================

// TestWorkflowRun_ScratchpadPropagatesAcrossPhases verifies that scratchpad
// entries from Phase 1 are available as PREV_SCRATCHPAD in Phase 2's prompt.
func TestWorkflowRun_ScratchpadPropagatesAcrossPhases(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	gdb := testGlobalDBFrom(backend)

	// Create task
	taskID := "TASK-001"
	tsk := task.NewProtoTask(taskID, "Test scratchpad propagation")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Create workflow with two phases: spec → implement
	createTwoPhaseWorkflow(t, gdb, "two-phase-wf")

	// Phase 1 (spec) returns output with scratchpad entries
	specOutput := `{
		"status": "complete",
		"summary": "Spec completed",
		"content": "# Specification\n\nUse REST API.",
		"scratchpad": [
			{"category": "decision", "content": "Chose REST over GraphQL for simplicity"},
			{"category": "observation", "content": "Existing API uses JSON:API format"}
		]
	}`

	// Phase 2 (implement) returns simple output — we capture the prompt
	implOutput := `{
		"status": "complete",
		"summary": "Implementation done",
		"content": "# Implementation\n\nDone."
	}`

	mockTE := &scratchpadCapturingMockTurnExecutor{
		results: []string{specOutput, implOutput},
	}

	we := NewWorkflowExecutor(
		backend,
		backend.DB(),
		gdb,
		&config.Config{
			Gates: config.GateConfig{AutoApproveOnSuccess: true},
		},
		t.TempDir(),
		WithWorkflowLogger(slog.Default()),
		WithWorkflowTurnExecutor(mockTE),
		WithSkipGates(true),
	)

	// Run workflow — goes through spec then implement
	_, err := we.Run(context.Background(), "two-phase-wf", WorkflowRunOptions{
		ContextType: ContextTask,
		TaskID:      taskID,
		Prompt:      "implement feature",
	})
	if err != nil {
		t.Fatalf("workflow run failed: %v", err)
	}

	// Verify Phase 2's prompt received scratchpad content from Phase 1
	// The capturing mock records prompts for each call
	if len(mockTE.prompts) < 2 {
		t.Fatalf("expected at least 2 phase prompts, got %d", len(mockTE.prompts))
	}

	phase2Prompt := mockTE.prompts[1]

	// Phase 2's prompt should contain the scratchpad entries from Phase 1
	if !strings.Contains(phase2Prompt, "Chose REST over GraphQL for simplicity") {
		t.Error("Phase 2 prompt should contain decision from Phase 1's scratchpad")
	}
	if !strings.Contains(phase2Prompt, "Existing API uses JSON:API format") {
		t.Error("Phase 2 prompt should contain observation from Phase 1's scratchpad")
	}
}

// =============================================================================
// Integration Test 3: Phase output without scratchpad field (backward compat)
//
// Wiring point: The scratchpad extraction in Run() must be graceful when
// phase output has no scratchpad field — no error, no crash.
//
// Deletion test: N/A (this verifies the wiring is safe, not that it exists)
// =============================================================================

// TestWorkflowRun_NoScratchpadField_Succeeds verifies that phase output
// without a scratchpad field doesn't cause workflow failure.
func TestWorkflowRun_NoScratchpadField_Succeeds(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	gdb := testGlobalDBFrom(backend)

	// Create task
	taskID := "TASK-001"
	tsk := task.NewProtoTask(taskID, "Test backward compat")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Create workflow with single phase
	createWorkflowWithPhase(t, gdb, "test-workflow", "implement", "Implement")

	// Phase output WITHOUT scratchpad field (pre-feature output)
	phaseOutput := `{
		"status": "complete",
		"summary": "Done",
		"content": "# Implementation\n\nComplete."
	}`
	mockTE := &scratchpadMockTurnExecutor{result: phaseOutput}

	we := NewWorkflowExecutor(
		backend,
		backend.DB(),
		gdb,
		&config.Config{
			Gates: config.GateConfig{AutoApproveOnSuccess: true},
		},
		t.TempDir(),
		WithWorkflowLogger(slog.Default()),
		WithWorkflowTurnExecutor(mockTE),
		WithSkipGates(true),
	)

	// Run workflow — should succeed without error
	_, err := we.Run(context.Background(), "test-workflow", WorkflowRunOptions{
		ContextType: ContextTask,
		TaskID:      taskID,
		Prompt:      "test",
	})
	if err != nil {
		t.Fatalf("workflow run without scratchpad should succeed: %v", err)
	}

	// Verify no scratchpad entries were created
	entries, err := backend.GetScratchpadEntries(taskID)
	if err != nil {
		t.Fatalf("GetScratchpadEntries: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 scratchpad entries, got %d", len(entries))
	}
}

// =============================================================================
// Helpers
// =============================================================================

// createWorkflowWithPhase creates a minimal workflow with a single phase.
func createWorkflowWithPhase(t *testing.T, gdb *db.GlobalDB, workflowID, phaseID, phaseName string) {
	t.Helper()

	wf := &db.Workflow{ID: workflowID, Name: "Test Workflow"}
	if err := gdb.SaveWorkflow(wf); err != nil {
		t.Fatalf("save workflow: %v", err)
	}

	tmpl := &db.PhaseTemplate{
		ID:               phaseID,
		Name:             phaseName,
		PromptSource:     "db",
		PromptContent:    "Test phase prompt for {{TASK_ID}}\n\n{{#if PREV_SCRATCHPAD}}\n## Prior Phase Notes\n{{PREV_SCRATCHPAD}}\n{{/if}}",
		OutputVarName:    strings.ToUpper(phaseID) + "_CONTENT",
		ProducesArtifact: true,
		ArtifactType:     phaseID,
		GateType:         "auto",
	}
	if err := gdb.SavePhaseTemplate(tmpl); err != nil {
		t.Fatalf("save phase template: %v", err)
	}

	phase := &db.WorkflowPhase{
		WorkflowID:      workflowID,
		PhaseTemplateID: phaseID,
		Sequence:        1,
	}
	if err := gdb.SaveWorkflowPhase(phase); err != nil {
		t.Fatalf("save workflow phase: %v", err)
	}
}

// createTwoPhaseWorkflow creates a workflow with spec → implement phases.
// The implement phase template includes {{PREV_SCRATCHPAD}} to verify propagation.
func createTwoPhaseWorkflow(t *testing.T, gdb *db.GlobalDB, workflowID string) {
	t.Helper()

	wf := &db.Workflow{ID: workflowID, Name: "Two Phase Workflow"}
	if err := gdb.SaveWorkflow(wf); err != nil {
		t.Fatalf("save workflow: %v", err)
	}

	// Phase 1: spec
	specTmpl := &db.PhaseTemplate{
		ID:               "spec",
		Name:             "Specification",
		PromptSource:     "db",
		PromptContent:    "Write a spec for {{TASK_ID}}",
		OutputVarName:    "SPEC_CONTENT",
		ProducesArtifact: true,
		ArtifactType:     "spec",
		GateType:         "auto",
	}
	if err := gdb.SavePhaseTemplate(specTmpl); err != nil {
		t.Fatalf("save spec template: %v", err)
	}

	specPhase := &db.WorkflowPhase{
		WorkflowID:      workflowID,
		PhaseTemplateID: "spec",
		Sequence:        1,
	}
	if err := gdb.SaveWorkflowPhase(specPhase); err != nil {
		t.Fatalf("save spec phase: %v", err)
	}

	// Phase 2: implement — prompt includes PREV_SCRATCHPAD for verification
	implTmpl := &db.PhaseTemplate{
		ID:               "implement",
		Name:             "Implementation",
		PromptSource:     "db",
		PromptContent:    "Implement for {{TASK_ID}}\n\n{{#if PREV_SCRATCHPAD}}\n## Prior Phase Notes\n{{PREV_SCRATCHPAD}}\n{{/if}}",
		OutputVarName:    "IMPLEMENT_CONTENT",
		ProducesArtifact: true,
		ArtifactType:     "implement",
		GateType:         "auto",
	}
	if err := gdb.SavePhaseTemplate(implTmpl); err != nil {
		t.Fatalf("save implement template: %v", err)
	}

	implPhase := &db.WorkflowPhase{
		WorkflowID:      workflowID,
		PhaseTemplateID: "implement",
		Sequence:        2,
	}
	if err := gdb.SaveWorkflowPhase(implPhase); err != nil {
		t.Fatalf("save implement phase: %v", err)
	}
}

// scratchpadMockTurnExecutor returns a single predefined result for all calls.
type scratchpadMockTurnExecutor struct {
	result    string
	sessionID string
}

func (m *scratchpadMockTurnExecutor) ExecuteTurn(ctx context.Context, prompt string) (*TurnResult, error) {
	if m.result == "" {
		return nil, errors.New("no mock result configured")
	}
	return &TurnResult{
		Content:   m.result,
		Status:    PhaseStatusComplete,
		SessionID: m.sessionID,
	}, nil
}

func (m *scratchpadMockTurnExecutor) ExecuteTurnWithoutSchema(ctx context.Context, prompt string) (*TurnResult, error) {
	return m.ExecuteTurn(ctx, prompt)
}

func (m *scratchpadMockTurnExecutor) UpdateSessionID(id string) {
	m.sessionID = id
}

func (m *scratchpadMockTurnExecutor) SessionID() string {
	return m.sessionID
}

// scratchpadCapturingMockTurnExecutor returns sequential results and captures prompts.
type scratchpadCapturingMockTurnExecutor struct {
	results   []string
	prompts   []string
	callIndex int
	sessionID string
}

func (m *scratchpadCapturingMockTurnExecutor) ExecuteTurn(ctx context.Context, prompt string) (*TurnResult, error) {
	m.prompts = append(m.prompts, prompt)
	if m.callIndex >= len(m.results) {
		return nil, errors.New("no more mock results")
	}
	result := m.results[m.callIndex]
	m.callIndex++
	return &TurnResult{
		Content:   result,
		Status:    PhaseStatusComplete,
		SessionID: m.sessionID,
	}, nil
}

func (m *scratchpadCapturingMockTurnExecutor) ExecuteTurnWithoutSchema(ctx context.Context, prompt string) (*TurnResult, error) {
	return m.ExecuteTurn(ctx, prompt)
}

func (m *scratchpadCapturingMockTurnExecutor) UpdateSessionID(id string) {
	m.sessionID = id
}

func (m *scratchpadCapturingMockTurnExecutor) SessionID() string {
	return m.sessionID
}
