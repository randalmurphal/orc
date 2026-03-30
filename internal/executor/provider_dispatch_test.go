// Integration tests for provider dispatch: verifies that the full chain
// resolvePhaseProvider → providerAdapterFor → executeWithProvider
// is actually wired together, not just unit-tested in isolation.
//
// Strategy: claudeAdapter pre-assigns a UUID session ID to the task before
// execution. codexAdapter does NOT (it captures thread_id from the response).
// This session ID presence/absence is the distinguishing signal.
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
	"github.com/randalmurphal/orc/internal/workflow"
)

// setupProviderDispatchTest creates the common test scaffolding for provider dispatch tests.
// Returns the executor, task, and all DB objects needed to call executePhase.
func setupProviderDispatchTest(t *testing.T, cfg *config.Config, phaseProvider string) (
	we *WorkflowExecutor,
	tmpl *db.PhaseTemplate,
	wfPhase *db.WorkflowPhase,
	run *db.WorkflowRun,
	runPhase *db.WorkflowRunPhase,
	tsk *orcv1.Task,
) {
	t.Helper()

	backend := storage.NewTestBackend(t)
	gdb := testGlobalDBFrom(backend)

	tmpl = &db.PhaseTemplate{
		ID:            "implement",
		Name:          "Implement",
		PromptSource:  "db",
		PromptContent: "Implement the feature",
	}
	if err := gdb.SavePhaseTemplate(tmpl); err != nil {
		t.Fatalf("save phase template: %v", err)
	}

	wf := &db.Workflow{ID: "test-wf", Name: "Test WF"}
	if err := gdb.SaveWorkflow(wf); err != nil {
		t.Fatalf("save workflow: %v", err)
	}

	wfPhase = &db.WorkflowPhase{
		WorkflowID:       "test-wf",
		PhaseTemplateID:  "implement",
		Sequence:         0,
		ProviderOverride: phaseProvider,
	}
	if err := gdb.SaveWorkflowPhase(wfPhase); err != nil {
		t.Fatalf("save workflow phase: %v", err)
	}

	// MockTurnExecutor with PhaseID set so codexAdapter gets a parsed Status
	mockTurns := &MockTurnExecutor{
		Responses: []string{
			`{"status": "complete", "summary": "done", "content": "implemented"}`,
		},
		PhaseID: "implement",
	}

	worktreeDir := t.TempDir()
	we = NewWorkflowExecutor(
		backend, nil, gdb, cfg, worktreeDir,
		WithWorkflowTurnExecutor(mockTurns),
		WithWorkflowLogger(slog.Default()),
		WithSkipGates(true),
	)
	// Set worktreePath so codex adapter path is exercised.
	// Nil out globalDB to skip llmkit runtime preparation (which needs hook_scripts
	// data from the real global schema). We're testing provider dispatch, not runtime prep.
	we.worktreePath = worktreeDir
	we.globalDB = nil

	tsk = task.NewProtoTask("TASK-001", "Test provider dispatch")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}
	// Wire task into executor so adapters can read/write session state
	we.task = tsk

	run = &db.WorkflowRun{
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

	runPhase = &db.WorkflowRunPhase{
		WorkflowRunID:   "run-001",
		PhaseTemplateID: "implement",
		Status:          orcv1.PhaseStatus_PHASE_STATUS_PENDING.String(),
	}
	if err := backend.SaveWorkflowRunPhase(runPhase); err != nil {
		t.Fatalf("save run phase: %v", err)
	}

	return we, tmpl, wfPhase, run, runPhase, tsk
}

// hasPreAssignedSessionID checks if the task's phase has a pre-assigned session ID.
// Claude adapter pre-assigns a UUID before execution; codex adapter does not.
func hasPreAssignedSessionID(tsk *orcv1.Task, phaseID string) bool {
	if tsk.Execution == nil || tsk.Execution.Phases == nil {
		return false
	}
	ps, ok := tsk.Execution.Phases[phaseID]
	if !ok {
		return false
	}
	return ps.SessionMetadata != nil && *ps.SessionMetadata != ""
}

// =============================================================================
// Provider="codex" routes through codexAdapter (no pre-assigned session ID)
// =============================================================================

func TestProviderDispatch_CodexPhaseOverride_TakesCodexRoute(t *testing.T) {
	t.Parallel()

	we, tmpl, wfPhase, run, runPhase, tsk := setupProviderDispatchTest(
		t, &config.Config{}, "codex",
	)

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

	if result.Status != orcv1.PhaseStatus_PHASE_STATUS_COMPLETED.String() {
		t.Errorf("status = %q, want COMPLETED", result.Status)
	}

	// codexAdapter does NOT pre-assign session ID (captures from response)
	if hasPreAssignedSessionID(tsk, "implement") {
		t.Fatal("session ID was pre-assigned — provider='codex' should NOT route through claudeAdapter")
	}
}

// =============================================================================
// Default provider routes through claudeAdapter (pre-assigns session ID)
// =============================================================================

func TestProviderDispatch_DefaultProvider_TakesClaudeRoute(t *testing.T) {
	t.Parallel()

	we, tmpl, wfPhase, run, runPhase, tsk := setupProviderDispatchTest(
		t, &config.Config{}, "", // Empty provider = default = claude
	)

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

	if result.Status != orcv1.PhaseStatus_PHASE_STATUS_COMPLETED.String() {
		t.Errorf("status = %q, want COMPLETED", result.Status)
	}

	// claudeAdapter pre-assigns a UUID session ID before execution
	if !hasPreAssignedSessionID(tsk, "implement") {
		t.Fatal("no session ID pre-assigned — default provider should route through claudeAdapter")
	}
}

// =============================================================================
// Config provider="codex" propagates through resolvePhaseProvider to codex path
// =============================================================================

func TestProviderDispatch_ConfigProvider_PropagatesCodexRoute(t *testing.T) {
	t.Parallel()

	// Provider set in config (lowest priority), no phase/workflow override
	we, tmpl, wfPhase, run, runPhase, tsk := setupProviderDispatchTest(
		t, &config.Config{Provider: "codex"}, "", // No phase override
	)

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

	if result.Status != orcv1.PhaseStatus_PHASE_STATUS_COMPLETED.String() {
		t.Errorf("status = %q, want COMPLETED", result.Status)
	}

	if hasPreAssignedSessionID(tsk, "implement") {
		t.Fatal("session ID pre-assigned — config provider='codex' did not propagate to codexAdapter")
	}
}

// =============================================================================
// Workflow DefaultProvider propagates to codex path
// =============================================================================

func TestProviderDispatch_WorkflowDefaultProvider_PropagatesCodexRoute(t *testing.T) {
	t.Parallel()

	// No config or phase override — workflow default is the only source
	we, tmpl, wfPhase, run, runPhase, tsk := setupProviderDispatchTest(
		t, &config.Config{}, "", // No phase override
	)

	// Set workflow with default provider (would normally be loaded from DB during Run)
	we.wf = &workflow.Workflow{DefaultProvider: "codex"}

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

	if result.Status != orcv1.PhaseStatus_PHASE_STATUS_COMPLETED.String() {
		t.Errorf("status = %q, want COMPLETED", result.Status)
	}

	if hasPreAssignedSessionID(tsk, "implement") {
		t.Fatal("session ID pre-assigned — workflow DefaultProvider='codex' did not propagate to codexAdapter")
	}
}

// =============================================================================
// Run-level provider override (--provider flag) overrides everything else
// =============================================================================

func TestProviderDispatch_RunProviderOverride_OverridesAll(t *testing.T) {
	t.Parallel()

	// Config says "claude", phase says nothing, but run-level override says "codex"
	we, tmpl, wfPhase, run, runPhase, tsk := setupProviderDispatchTest(
		t, &config.Config{}, "", // No phase override
	)

	// Simulate --provider codex flag
	we.runProvider = "codex"

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

	if result.Status != orcv1.PhaseStatus_PHASE_STATUS_COMPLETED.String() {
		t.Errorf("status = %q, want COMPLETED", result.Status)
	}

	if hasPreAssignedSessionID(tsk, "implement") {
		t.Fatal("session ID pre-assigned — runProvider='codex' override did not route to codexAdapter")
	}
}

// =============================================================================
// Provider priority: phase override beats workflow default
// =============================================================================

func TestProviderDispatch_PhaseOverrideBeatsWorkflowDefault(t *testing.T) {
	t.Parallel()

	// Phase override says "claude", workflow default says "codex" — phase wins
	we, tmpl, wfPhase, run, runPhase, tsk := setupProviderDispatchTest(
		t, &config.Config{}, "claude", // Phase explicitly overrides to claude
	)

	// Workflow says codex, but phase override should win
	we.wf = &workflow.Workflow{DefaultProvider: "codex"}

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

	if result.Status != orcv1.PhaseStatus_PHASE_STATUS_COMPLETED.String() {
		t.Errorf("status = %q, want COMPLETED", result.Status)
	}

	// Phase says "claude", so session ID SHOULD be pre-assigned
	if !hasPreAssignedSessionID(tsk, "implement") {
		t.Fatal("no session ID pre-assigned — phase override='claude' should have beaten workflow default='codex'")
	}
}
