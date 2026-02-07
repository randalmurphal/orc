// Integration tests for provider dispatch: verifies that the full chain
// resolvePhaseProvider → isCodexFamilyProvider → executeWithCodex/executeWithClaude
// is actually wired together, not just unit-tested in isolation.
//
// Strategy: executeWithCodex writes AGENTS.md to the worktree (via ApplyCodexPhaseSettings),
// executeWithClaude does not. This side effect is the distinguishing signal.
package executor

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
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
	worktreeDir string,
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

	// MockTurnExecutor with PhaseID set so executeWithCodex gets a parsed Status
	mockTurns := &MockTurnExecutor{
		Responses: []string{
			`{"status": "complete", "summary": "done", "content": "implemented"}`,
		},
		PhaseID: "implement",
	}

	worktreeDir = t.TempDir()
	we = NewWorkflowExecutor(
		backend, nil, gdb, cfg, worktreeDir,
		WithWorkflowTurnExecutor(mockTurns),
		WithWorkflowLogger(slog.Default()),
		WithSkipGates(true),
	)
	// Set worktreePath directly (same package) so ApplyCodexPhaseSettings writes files.
	// Nil out globalDB to skip ApplyPhaseSettings (which needs hook_scripts table from
	// real global schema). We're testing provider dispatch, not phase settings.
	we.worktreePath = worktreeDir
	we.globalDB = nil

	tsk = task.NewProtoTask("TASK-001", "Test provider dispatch")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

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

	return we, tmpl, wfPhase, run, runPhase, tsk, worktreeDir
}

// =============================================================================
// Provider="codex" routes through executeWithCodex (writes AGENTS.md)
// =============================================================================

func TestProviderDispatch_CodexPhaseOverride_TakesCodexRoute(t *testing.T) {
	t.Parallel()

	we, tmpl, wfPhase, run, runPhase, tsk, worktreeDir := setupProviderDispatchTest(
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

	// AGENTS.md is the distinguishing side effect of executeWithCodex
	agentsMD := filepath.Join(worktreeDir, "AGENTS.md")
	if _, err := os.Stat(agentsMD); os.IsNotExist(err) {
		t.Fatal("AGENTS.md not created — provider='codex' did NOT route through executeWithCodex")
	}
}

// =============================================================================
// Default provider routes through executeWithClaude (no AGENTS.md)
// =============================================================================

func TestProviderDispatch_DefaultProvider_TakesClaudeRoute(t *testing.T) {
	t.Parallel()

	we, tmpl, wfPhase, run, runPhase, tsk, worktreeDir := setupProviderDispatchTest(
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

	// executeWithClaude does NOT write AGENTS.md
	agentsMD := filepath.Join(worktreeDir, "AGENTS.md")
	if _, err := os.Stat(agentsMD); err == nil {
		t.Fatal("AGENTS.md was created — default provider should NOT route through executeWithCodex")
	}
}

// =============================================================================
// Config provider="codex" propagates through resolvePhaseProvider to codex path
// =============================================================================

func TestProviderDispatch_ConfigProvider_PropagatesCodexRoute(t *testing.T) {
	t.Parallel()

	// Provider set in config (lowest priority), no phase/workflow override
	we, tmpl, wfPhase, run, runPhase, tsk, worktreeDir := setupProviderDispatchTest(
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

	agentsMD := filepath.Join(worktreeDir, "AGENTS.md")
	if _, err := os.Stat(agentsMD); os.IsNotExist(err) {
		t.Fatal("AGENTS.md not created — config provider='codex' did not propagate to executeWithCodex")
	}
}

// =============================================================================
// Workflow DefaultProvider propagates to codex path
// =============================================================================

func TestProviderDispatch_WorkflowDefaultProvider_PropagatesCodexRoute(t *testing.T) {
	t.Parallel()

	// No config or phase override — workflow default is the only source
	we, tmpl, wfPhase, run, runPhase, tsk, worktreeDir := setupProviderDispatchTest(
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

	agentsMD := filepath.Join(worktreeDir, "AGENTS.md")
	if _, err := os.Stat(agentsMD); os.IsNotExist(err) {
		t.Fatal("AGENTS.md not created — workflow DefaultProvider='codex' did not propagate to executeWithCodex")
	}
}

// =============================================================================
// Run-level provider override (--provider flag) overrides everything else
// =============================================================================

func TestProviderDispatch_RunProviderOverride_OverridesAll(t *testing.T) {
	t.Parallel()

	// Config says "claude", phase says nothing, but run-level override says "codex"
	we, tmpl, wfPhase, run, runPhase, tsk, worktreeDir := setupProviderDispatchTest(
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

	agentsMD := filepath.Join(worktreeDir, "AGENTS.md")
	if _, err := os.Stat(agentsMD); os.IsNotExist(err) {
		t.Fatal("AGENTS.md not created — runProvider='codex' override did not route to executeWithCodex")
	}
}

// =============================================================================
// Provider priority: phase override beats workflow default
// =============================================================================

func TestProviderDispatch_PhaseOverrideBeatsWorkflowDefault(t *testing.T) {
	t.Parallel()

	// Phase override says "claude", workflow default says "codex" — phase wins
	we, tmpl, wfPhase, run, runPhase, tsk, worktreeDir := setupProviderDispatchTest(
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

	// Phase says "claude", so AGENTS.md should NOT exist
	agentsMD := filepath.Join(worktreeDir, "AGENTS.md")
	if _, err := os.Stat(agentsMD); err == nil {
		t.Fatal("AGENTS.md was created — phase override='claude' should have beaten workflow default='codex'")
	}
}

// =============================================================================
// Ollama (codex-family) also routes through executeWithCodex
// =============================================================================

func TestProviderDispatch_Ollama_RoutesToCodex(t *testing.T) {
	t.Parallel()

	we, tmpl, wfPhase, run, runPhase, tsk, worktreeDir := setupProviderDispatchTest(
		t, &config.Config{}, "ollama",
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

	agentsMD := filepath.Join(worktreeDir, "AGENTS.md")
	if _, err := os.Stat(agentsMD); os.IsNotExist(err) {
		t.Fatal("AGENTS.md not created — provider='ollama' should route through executeWithCodex")
	}
}
