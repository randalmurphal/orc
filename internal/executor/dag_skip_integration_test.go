// Integration tests for TASK-715: DAG execution with skipped phases.
//
// These tests verify that when a phase is skipped due to condition=false,
// its dependent phases still execute (skipped is treated as "satisfied").
//
// Coverage mapping:
//   SC-7: TestDAG_SkippedPhaseDependentsStillRun
//
// This covers the specific scenario from the task description:
//   "Skipped phase in DAG: condition=false, dependents still run"
package executor

import (
	"context"
	"log/slog"
	"sync/atomic"
	"testing"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// =============================================================================
// SC-7: Skipped phase in DAG - dependents still run
//
// Scenario: A→B→C, where B has condition=false (skip)
// Expected: A runs, B is skipped, C runs (not blocked by skipped B)
// =============================================================================

// TestDAG_SkippedPhaseDependentsStillRun verifies that when a phase is skipped
// due to its condition evaluating to false, dependent phases still execute.
// This is critical for DAG execution: skipped != failed, so dependents proceed.
func TestDAG_SkippedPhaseDependentsStillRun(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	pdb := backend.DB()

	// Create phase templates
	templates := []string{"A", "B", "C"}
	for _, id := range templates {
		tmpl := &db.PhaseTemplate{
			ID:            id,
			Name:          id,
			PromptSource:  "db",
			PromptContent: "Test prompt for " + id,
		}
		if err := pdb.SavePhaseTemplate(tmpl); err != nil {
			t.Fatalf("save template %s: %v", id, err)
		}
	}

	// Create workflow with A→B→C, where B has a condition that evaluates to false
	wf := &db.Workflow{ID: "skip-dag-wf", Name: "Skip DAG Test"}
	if err := pdb.SaveWorkflow(wf); err != nil {
		t.Fatalf("save workflow: %v", err)
	}

	// Phase A: no deps, no condition
	phaseA := &db.WorkflowPhase{
		WorkflowID:      "skip-dag-wf",
		PhaseTemplateID: "A",
		Sequence:        1,
		DependsOn:       "[]",
	}
	if err := pdb.SaveWorkflowPhase(phaseA); err != nil {
		t.Fatalf("save phase A: %v", err)
	}

	// Phase B: depends on A, has condition that evaluates to false
	// Condition: task.category == "bug" (but task category will be feature → skip)
	phaseB := &db.WorkflowPhase{
		WorkflowID:      "skip-dag-wf",
		PhaseTemplateID: "B",
		Sequence:        2,
		DependsOn:       `["A"]`,
		Condition:       `{"field": "task.category", "op": "eq", "value": "bug"}`,
	}
	if err := pdb.SaveWorkflowPhase(phaseB); err != nil {
		t.Fatalf("save phase B: %v", err)
	}

	// Phase C: depends on B
	phaseC := &db.WorkflowPhase{
		WorkflowID:      "skip-dag-wf",
		PhaseTemplateID: "C",
		Sequence:        3,
		DependsOn:       `["B"]`,
	}
	if err := pdb.SaveWorkflowPhase(phaseC); err != nil {
		t.Fatalf("save phase C: %v", err)
	}

	// Create task with category=feature (so B's condition evaluates to false)
	tsk := task.NewProtoTask("TASK-SKIPDAG-001", "Test skip in DAG")
	tsk.Category = orcv1.TaskCategory_TASK_CATEGORY_FEATURE // NOT bug → B's condition is false
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	wfID := "skip-dag-wf"
	tsk.WorkflowId = &wfID
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Track which phases were executed (not skipped)
	var executedPhases []string
	var executedCount atomic.Int32

	mock := &phaseTrackingMock{
		onExecute: func(phase string) {
			executedPhases = append(executedPhases, phase)
			executedCount.Add(1)
		},
	}

	we := NewWorkflowExecutor(
		backend, pdb, &config.Config{}, t.TempDir(),
		WithWorkflowLogger(slog.Default()),
		WithWorkflowTurnExecutor(mock),
		WithSkipGates(true),
	)

	result, err := we.Run(context.Background(), "skip-dag-wf", WorkflowRunOptions{
		ContextType: ContextTask,
		TaskID:      tsk.Id,
		Prompt:      "test skip dag",
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// Verify results:
	// - A should have executed
	// - B should be skipped (condition=false)
	// - C should have executed (not blocked by skipped B)

	// Check phase results in workflow result
	phaseStatuses := make(map[string]string)
	for _, pr := range result.PhaseResults {
		phaseStatuses[pr.PhaseID] = pr.Status
	}

	// A should be completed
	if phaseStatuses["A"] != orcv1.PhaseStatus_PHASE_STATUS_COMPLETED.String() {
		t.Errorf("Phase A status = %s, want COMPLETED", phaseStatuses["A"])
	}

	// B should be skipped
	if phaseStatuses["B"] != orcv1.PhaseStatus_PHASE_STATUS_SKIPPED.String() {
		t.Errorf("Phase B status = %s, want SKIPPED", phaseStatuses["B"])
	}

	// C should be completed (critical: not blocked by skipped B)
	if phaseStatuses["C"] != orcv1.PhaseStatus_PHASE_STATUS_COMPLETED.String() {
		t.Errorf("SC-7 FAIL: Phase C status = %s, want COMPLETED. Skipped predecessor should not block dependent.", phaseStatuses["C"])
	}

	// Verify execution count: A and C executed, B was skipped
	if executedCount.Load() != 2 {
		t.Errorf("Expected 2 phases to execute (A and C), got %d", executedCount.Load())
	}

	// Verify A and C were in the execution list
	if !containsPhase(executedPhases, "A") {
		t.Error("Phase A should have been executed")
	}
	if containsPhase(executedPhases, "B") {
		t.Error("Phase B should NOT have been executed (should be skipped)")
	}
	if !containsPhase(executedPhases, "C") {
		t.Error("SC-7 FAIL: Phase C should have been executed despite B being skipped")
	}
}

// TestDAG_MultipleSkippedPhasesInParallel verifies that when multiple phases
// in a parallel level are skipped, their common dependent still runs.
// Scenario: A→[B,C]→D where B has condition=false
// Expected: A runs, B skipped, C runs, D runs (waits for both B(skip) and C(complete))
func TestDAG_MultipleSkippedPhasesInParallel(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	pdb := backend.DB()

	// Create phase templates
	templates := []string{"A", "B", "C", "D"}
	for _, id := range templates {
		tmpl := &db.PhaseTemplate{
			ID:            id,
			Name:          id,
			PromptSource:  "db",
			PromptContent: "Test prompt for " + id,
		}
		if err := pdb.SavePhaseTemplate(tmpl); err != nil {
			t.Fatalf("save template %s: %v", id, err)
		}
	}

	// Create workflow: A→[B,C]→D
	wf := &db.Workflow{ID: "parallel-skip-wf", Name: "Parallel Skip Test"}
	if err := pdb.SaveWorkflow(wf); err != nil {
		t.Fatalf("save workflow: %v", err)
	}

	phases := []struct {
		id        string
		seq       int
		deps      string
		condition string
	}{
		{"A", 1, "[]", ""},
		{"B", 2, `["A"]`, `{"field": "task.category", "op": "eq", "value": "bug"}`}, // Will skip
		{"C", 3, `["A"]`, ""},                                                        // Will run
		{"D", 4, `["B", "C"]`, ""},                                                   // Should run despite B being skipped
	}

	for _, p := range phases {
		phase := &db.WorkflowPhase{
			WorkflowID:      "parallel-skip-wf",
			PhaseTemplateID: p.id,
			Sequence:        p.seq,
			DependsOn:       p.deps,
			Condition:       p.condition,
		}
		if err := pdb.SaveWorkflowPhase(phase); err != nil {
			t.Fatalf("save phase %s: %v", p.id, err)
		}
	}

	// Create task with category=feature
	tsk := task.NewProtoTask("TASK-PARSKIP-001", "Test parallel skip")
	tsk.Category = orcv1.TaskCategory_TASK_CATEGORY_FEATURE
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	wfID := "parallel-skip-wf"
	tsk.WorkflowId = &wfID
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	var executedPhases []string
	mock := &phaseTrackingMock{
		onExecute: func(phase string) {
			executedPhases = append(executedPhases, phase)
		},
	}

	we := NewWorkflowExecutor(
		backend, pdb, &config.Config{}, t.TempDir(),
		WithWorkflowLogger(slog.Default()),
		WithWorkflowTurnExecutor(mock),
		WithSkipGates(true),
	)

	result, err := we.Run(context.Background(), "parallel-skip-wf", WorkflowRunOptions{
		ContextType: ContextTask,
		TaskID:      tsk.Id,
		Prompt:      "test parallel skip",
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// Build status map
	phaseStatuses := make(map[string]string)
	for _, pr := range result.PhaseResults {
		phaseStatuses[pr.PhaseID] = pr.Status
	}

	// Verify: A completed, B skipped, C completed, D completed
	expectations := map[string]string{
		"A": orcv1.PhaseStatus_PHASE_STATUS_COMPLETED.String(),
		"B": orcv1.PhaseStatus_PHASE_STATUS_SKIPPED.String(),
		"C": orcv1.PhaseStatus_PHASE_STATUS_COMPLETED.String(),
		"D": orcv1.PhaseStatus_PHASE_STATUS_COMPLETED.String(),
	}

	for phase, expected := range expectations {
		if phaseStatuses[phase] != expected {
			t.Errorf("Phase %s status = %s, want %s", phase, phaseStatuses[phase], expected)
		}
	}

	// Verify D was executed (this is the key assertion for SC-7)
	if !containsPhase(executedPhases, "D") {
		t.Error("SC-7 FAIL: Phase D should have executed despite predecessor B being skipped")
	}
}

// TestDAG_AllPredecessorsSkipped verifies that when ALL predecessors of a phase
// are skipped, the dependent phase still runs.
// Scenario: [A,B]→C where both A and B have condition=false
// Expected: A skipped, B skipped, C still runs
func TestDAG_AllPredecessorsSkipped(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	pdb := backend.DB()

	// Create phase templates
	templates := []string{"A", "B", "C"}
	for _, id := range templates {
		tmpl := &db.PhaseTemplate{
			ID:            id,
			Name:          id,
			PromptSource:  "db",
			PromptContent: "Test prompt for " + id,
		}
		if err := pdb.SavePhaseTemplate(tmpl); err != nil {
			t.Fatalf("save template %s: %v", id, err)
		}
	}

	// Create workflow: [A,B]→C, both A and B will skip
	wf := &db.Workflow{ID: "all-skip-wf", Name: "All Skip Test"}
	if err := pdb.SaveWorkflow(wf); err != nil {
		t.Fatalf("save workflow: %v", err)
	}

	// Both A and B have conditions that will evaluate to false
	phases := []struct {
		id        string
		seq       int
		deps      string
		condition string
	}{
		{"A", 1, "[]", `{"field": "task.category", "op": "eq", "value": "bug"}`},
		{"B", 2, "[]", `{"field": "task.category", "op": "eq", "value": "bug"}`},
		{"C", 3, `["A", "B"]`, ""},
	}

	for _, p := range phases {
		phase := &db.WorkflowPhase{
			WorkflowID:      "all-skip-wf",
			PhaseTemplateID: p.id,
			Sequence:        p.seq,
			DependsOn:       p.deps,
			Condition:       p.condition,
		}
		if err := pdb.SaveWorkflowPhase(phase); err != nil {
			t.Fatalf("save phase %s: %v", p.id, err)
		}
	}

	// Create task with category=feature (so A and B conditions are false)
	tsk := task.NewProtoTask("TASK-ALLSKIP-001", "Test all predecessors skip")
	tsk.Category = orcv1.TaskCategory_TASK_CATEGORY_FEATURE
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	wfID := "all-skip-wf"
	tsk.WorkflowId = &wfID
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	var executedPhases []string
	mock := &phaseTrackingMock{
		onExecute: func(phase string) {
			executedPhases = append(executedPhases, phase)
		},
	}

	we := NewWorkflowExecutor(
		backend, pdb, &config.Config{}, t.TempDir(),
		WithWorkflowLogger(slog.Default()),
		WithWorkflowTurnExecutor(mock),
		WithSkipGates(true),
	)

	result, err := we.Run(context.Background(), "all-skip-wf", WorkflowRunOptions{
		ContextType: ContextTask,
		TaskID:      tsk.Id,
		Prompt:      "test all skip",
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// Build status map
	phaseStatuses := make(map[string]string)
	for _, pr := range result.PhaseResults {
		phaseStatuses[pr.PhaseID] = pr.Status
	}

	// A and B should be skipped, C should still complete
	if phaseStatuses["A"] != orcv1.PhaseStatus_PHASE_STATUS_SKIPPED.String() {
		t.Errorf("Phase A status = %s, want SKIPPED", phaseStatuses["A"])
	}
	if phaseStatuses["B"] != orcv1.PhaseStatus_PHASE_STATUS_SKIPPED.String() {
		t.Errorf("Phase B status = %s, want SKIPPED", phaseStatuses["B"])
	}
	if phaseStatuses["C"] != orcv1.PhaseStatus_PHASE_STATUS_COMPLETED.String() {
		t.Errorf("SC-7 FAIL: Phase C status = %s, want COMPLETED. All predecessors skipped should not block dependent.", phaseStatuses["C"])
	}

	// Verify only C was executed
	if len(executedPhases) != 1 || executedPhases[0] != "C" {
		t.Errorf("Expected only C to execute, got %v", executedPhases)
	}
}

// =============================================================================
// Helper types and functions
// =============================================================================

// phaseTrackingMock is a TurnExecutor mock that tracks which phases are executed.
type phaseTrackingMock struct {
	onExecute func(phase string)
}

func (m *phaseTrackingMock) ExecuteTurn(ctx context.Context, prompt string) (*TurnResult, error) {
	phase := extractPhaseFromPrompt(prompt)
	if m.onExecute != nil {
		m.onExecute(phase)
	}
	return &TurnResult{
		Content:   `{"status": "complete", "summary": "Done"}`,
		Status:    PhaseStatusComplete,
		SessionID: "mock-session",
	}, nil
}

func (m *phaseTrackingMock) ExecuteTurnWithoutSchema(ctx context.Context, prompt string) (*TurnResult, error) {
	return m.ExecuteTurn(ctx, prompt)
}

func (m *phaseTrackingMock) UpdateSessionID(id string) {}
func (m *phaseTrackingMock) SessionID() string         { return "mock-session" }

// containsPhase checks if a string slice contains a specific phase ID.
func containsPhase(slice []string, val string) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
}
