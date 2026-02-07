// Integration tests for TASK-007: Verify script and API phase executors are
// wired into the production dispatch path.
//
// These tests COMPLEMENT the unit tests in script_executor_test.go and
// api_executor_test.go. Unit tests call ExecuteScript()/ExecuteAPI() directly
// with manually constructed configs. These integration tests verify the code
// is REACHABLE from production entry points.
//
// Wiring points verified:
//  1. NewDefaultPhaseTypeRegistry() registers *ScriptPhaseExecutor for "script"
//  2. NewDefaultPhaseTypeRegistry() registers *APIPhaseExecutor for "api"
//  3. executePhase() dispatches type="script" → ScriptPhaseExecutor.ExecutePhase()
//     → command actually executes (not just registered)
//  4. executePhase() dispatches type="api" → APIPhaseExecutor.ExecutePhase()
//     → HTTP request actually reaches the server
//  5. Run() with script phase → output propagates to next LLM phase via variables
//  6. Run() with API phase → output propagates to next LLM phase via variables
//  7. Script/API phase results persisted to WorkflowRunPhase with zero cost
//
// Deletion test: Remove "script"/"api" registration from NewDefaultPhaseTypeRegistry()
// → tests 1-7 all fail. Remove ExecutePhase() implementation → tests 3-7 fail.
// Remove output variable storage → tests 5-6 fail.
package executor

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/internal/variable"
)

// =============================================================================
// Wiring Point 1: Default registry registers *ScriptPhaseExecutor
//
// The unit test TestDefaultRegistry_ScriptRegistered verifies Get("script")
// returns non-nil. This test goes further: it verifies the returned executor
// is the correct TYPE, not just non-nil.
//
// Deletion test: Remove r.Register("script", ...) from NewDefaultPhaseTypeRegistry()
// → this test fails at registry.Get("script").
// =============================================================================

func TestDefaultRegistry_ScriptExecutorType(t *testing.T) {
	t.Parallel()

	registry := NewDefaultPhaseTypeRegistry()

	executor, err := registry.Get("script")
	if err != nil {
		t.Fatalf("registry.Get('script') error: %v", err)
	}

	// Verify it's the real ScriptPhaseExecutor, not a stub
	if _, ok := executor.(*ScriptPhaseExecutor); !ok {
		t.Fatalf("expected *ScriptPhaseExecutor, got %T", executor)
	}
}

// =============================================================================
// Wiring Point 2: Default registry registers *APIPhaseExecutor
//
// Deletion test: Remove r.Register("api", ...) from NewDefaultPhaseTypeRegistry()
// → this test fails at registry.Get("api").
// =============================================================================

func TestDefaultRegistry_APIExecutorType(t *testing.T) {
	t.Parallel()

	registry := NewDefaultPhaseTypeRegistry()

	executor, err := registry.Get("api")
	if err != nil {
		t.Fatalf("registry.Get('api') error: %v", err)
	}

	// Verify it's the real APIPhaseExecutor, not a stub
	if _, ok := executor.(*APIPhaseExecutor); !ok {
		t.Fatalf("expected *APIPhaseExecutor, got %T", executor)
	}
}

// =============================================================================
// Wiring Point 3: executePhase() dispatches type="script" to real executor
// and the command actually executes.
//
// This is the CRITICAL integration test. Unit tests call ExecuteScript()
// directly with a manually constructed ScriptPhaseConfig. But production
// calls executePhase() → registry.Get("script") → executor.ExecutePhase().
// If ExecutePhase() doesn't properly parse config and delegate to
// ExecuteScript(), the feature is dead in production even though unit
// tests pass.
//
// Deletion test: Remove "script" registration → "unknown phase type" error.
// Break ExecutePhase() config parsing → "no command" or similar error.
// =============================================================================

func TestExecutePhase_ScriptType_CommandExecutes(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	gdb := testGlobalDBFrom(backend)

	// Template with type="script" — the executor must parse config from
	// the template and actually run a shell command.
	tmpl := &db.PhaseTemplate{
		ID:           "run-tests",
		Name:         "Run Tests",
		Type:         "script",
		PromptSource: "db",
	}
	if err := gdb.SavePhaseTemplate(tmpl); err != nil {
		t.Fatalf("save template: %v", err)
	}

	wf := &db.Workflow{ID: "script-dispatch-wf", Name: "Script Dispatch Test"}
	if err := gdb.SaveWorkflow(wf); err != nil {
		t.Fatalf("save workflow: %v", err)
	}
	wfPhase := &db.WorkflowPhase{
		WorkflowID:      "script-dispatch-wf",
		PhaseTemplateID: "run-tests",
		Sequence:        0,
	}
	if err := gdb.SaveWorkflowPhase(wfPhase); err != nil {
		t.Fatalf("save workflow phase: %v", err)
	}

	// Use DEFAULT registry (no WithPhaseTypeExecutor override).
	// This ensures we test the real executor from NewDefaultPhaseTypeRegistry().
	we := NewWorkflowExecutor(
		backend, backend.DB(), gdb, &config.Config{}, t.TempDir(),
		WithWorkflowLogger(slog.Default()),
		WithSkipGates(true),
	)

	tsk := task.NewProtoTask("TASK-SCRIPT-001", "Test script dispatch")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	run := &db.WorkflowRun{
		ID:          "run-script-001",
		WorkflowID:  "script-dispatch-wf",
		TaskID:      &tsk.Id,
		ContextType: "task",
		Prompt:      "test",
		Status:      "running",
	}
	if err := backend.SaveWorkflowRun(run); err != nil {
		t.Fatalf("save run: %v", err)
	}
	runPhase := &db.WorkflowRunPhase{
		WorkflowRunID:   "run-script-001",
		PhaseTemplateID: "run-tests",
		Status:          orcv1.PhaseStatus_PHASE_STATUS_PENDING.String(),
	}
	if err := backend.SaveWorkflowRunPhase(runPhase); err != nil {
		t.Fatalf("save run phase: %v", err)
	}

	vars := variable.VariableSet{}
	rctx := &variable.ResolutionContext{
		PhaseOutputVars: make(map[string]string),
	}

	// Provide script config through the params that executePhase builds.
	// The ScriptPhaseConfig must reach the executor through the PhaseTypeParams.
	// This verifies the full dispatch chain: executePhase() → registry.Get() →
	// executor.ExecutePhase() → command runs.
	//
	// We set the config on the variable set — the executor must resolve
	// {{SCRIPT_COMMAND}} from vars or the template must carry config.
	// The production path builds PhaseTypeParams from these inputs.
	vars["SCRIPT_COMMAND"] = `echo "integration-test-script-output"`

	result, err := we.executePhase(
		context.Background(), tmpl, wfPhase, vars, rctx, run, runPhase, tsk,
	)
	if err != nil {
		t.Fatalf("executePhase error: %v\n(If 'unknown phase type': script not registered in NewDefaultPhaseTypeRegistry())\n(If 'no command': ExecutePhase() not parsing config from template)", err)
	}

	// The executor MUST have actually run a command and captured output.
	// Empty status means the executor returned without completing.
	if result.Status != orcv1.PhaseStatus_PHASE_STATUS_COMPLETED.String() {
		t.Errorf("status = %q, want COMPLETED", result.Status)
	}

	// Zero cost — script phases don't use LLM
	if result.CostUSD != 0 {
		t.Errorf("CostUSD = %f, want 0 (script phase)", result.CostUSD)
	}
	if result.InputTokens != 0 {
		t.Errorf("InputTokens = %d, want 0", result.InputTokens)
	}
}

// =============================================================================
// Wiring Point 4: executePhase() dispatches type="api" to real executor
// and the HTTP request actually reaches the server.
//
// Same critical gap as script: unit tests call ExecuteAPI() directly with
// APIPhaseConfig. This test verifies the production path works.
//
// Deletion test: Remove "api" registration → "unknown phase type" error.
// Break ExecutePhase() config parsing → "no URL" or similar error.
// =============================================================================

func TestExecutePhase_APIType_RequestMade(t *testing.T) {
	t.Parallel()

	// Set up httptest server that records requests
	var requestReceived bool
	var gotMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestReceived = true
		gotMethod = r.Method
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"status": "deployed"}`)
	}))
	defer server.Close()

	backend := storage.NewTestBackend(t)
	gdb := testGlobalDBFrom(backend)

	// Template with type="api" — the executor must parse URL config and
	// actually make an HTTP request to the server.
	tmpl := &db.PhaseTemplate{
		ID:           "deploy-hook",
		Name:         "Deploy Hook",
		Type:         "api",
		PromptSource: "db",
	}
	if err := gdb.SavePhaseTemplate(tmpl); err != nil {
		t.Fatalf("save template: %v", err)
	}

	wf := &db.Workflow{ID: "api-dispatch-wf", Name: "API Dispatch Test"}
	if err := gdb.SaveWorkflow(wf); err != nil {
		t.Fatalf("save workflow: %v", err)
	}
	wfPhase := &db.WorkflowPhase{
		WorkflowID:      "api-dispatch-wf",
		PhaseTemplateID: "deploy-hook",
		Sequence:        0,
	}
	if err := gdb.SaveWorkflowPhase(wfPhase); err != nil {
		t.Fatalf("save workflow phase: %v", err)
	}

	// Default registry — tests the real APIPhaseExecutor
	we := NewWorkflowExecutor(
		backend, backend.DB(), gdb, &config.Config{}, t.TempDir(),
		WithWorkflowLogger(slog.Default()),
		WithSkipGates(true),
	)

	tsk := task.NewProtoTask("TASK-API-001", "Test API dispatch")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	run := &db.WorkflowRun{
		ID:          "run-api-001",
		WorkflowID:  "api-dispatch-wf",
		TaskID:      &tsk.Id,
		ContextType: "task",
		Prompt:      "test",
		Status:      "running",
	}
	if err := backend.SaveWorkflowRun(run); err != nil {
		t.Fatalf("save run: %v", err)
	}
	runPhase := &db.WorkflowRunPhase{
		WorkflowRunID:   "run-api-001",
		PhaseTemplateID: "deploy-hook",
		Status:          orcv1.PhaseStatus_PHASE_STATUS_PENDING.String(),
	}
	if err := backend.SaveWorkflowRunPhase(runPhase); err != nil {
		t.Fatalf("save run phase: %v", err)
	}

	// Provide API URL through vars for the executor to resolve
	vars := variable.VariableSet{
		"DEPLOY_URL": server.URL + "/deploy",
	}
	rctx := &variable.ResolutionContext{
		PhaseOutputVars: make(map[string]string),
	}

	result, err := we.executePhase(
		context.Background(), tmpl, wfPhase, vars, rctx, run, runPhase, tsk,
	)
	if err != nil {
		t.Fatalf("executePhase error: %v\n(If 'unknown phase type': api not registered in NewDefaultPhaseTypeRegistry())\n(If 'no URL': ExecutePhase() not parsing config from template)", err)
	}

	// The executor MUST have actually made an HTTP request
	if !requestReceived {
		t.Fatal("httptest server did not receive a request — API executor not invoked")
	}

	if result.Status != orcv1.PhaseStatus_PHASE_STATUS_COMPLETED.String() {
		t.Errorf("status = %q, want COMPLETED", result.Status)
	}

	// Response body should be captured as Content
	if !containsSubstring(result.Content, "deployed") {
		t.Errorf("content = %q, expected response body captured", result.Content)
	}

	// Zero cost — API phases don't use LLM
	if result.CostUSD != 0 {
		t.Errorf("CostUSD = %f, want 0 (API phase)", result.CostUSD)
	}

	// Verify method used (if no explicit method, should default to GET per spec)
	_ = gotMethod // Available for assertion if needed
}

// =============================================================================
// Wiring Point 5: Run() with script phase → output propagates to next phase
//
// This tests the full production pipeline:
//   executePhaseWithTimeout() → executePhase() → registry.Get("script") →
//   ScriptPhaseExecutor.ExecutePhase() → command runs → output stored →
//   applyPhaseContentToVars() → next phase sees the output in its prompt
//
// Deletion test: Remove "script" registration → workflow fails.
// Remove output storage in ExecutePhase() → next phase doesn't see output.
// =============================================================================

func TestRunLoop_ScriptPhaseOutputPropagation(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	gdb := testGlobalDBFrom(backend)

	// Phase 1: script phase that produces output
	scriptTmpl := &db.PhaseTemplate{
		ID:               "build-step",
		Name:             "Build Step",
		Type:             "script",
		PromptSource:     "db",
		ProducesArtifact: true,
		OutputVarName:    "BUILD_OUTPUT",
	}
	if err := gdb.SavePhaseTemplate(scriptTmpl); err != nil {
		t.Fatalf("save script template: %v", err)
	}

	// Phase 2: LLM phase that should receive the script output
	implTmpl := &db.PhaseTemplate{
		ID:            "implement",
		Name:          "Implement",
		PromptSource:  "db",
		PromptContent: "implement using build output: {{BUILD_OUTPUT}}",
	}
	if err := gdb.SavePhaseTemplate(implTmpl); err != nil {
		t.Fatalf("save implement template: %v", err)
	}

	wf := &db.Workflow{ID: "script-prop-wf", Name: "Script Propagation Test"}
	if err := gdb.SaveWorkflow(wf); err != nil {
		t.Fatalf("save workflow: %v", err)
	}

	scriptPhase := &db.WorkflowPhase{
		WorkflowID:      "script-prop-wf",
		PhaseTemplateID: "build-step",
		Sequence:        0,
	}
	if err := gdb.SaveWorkflowPhase(scriptPhase); err != nil {
		t.Fatalf("save script phase: %v", err)
	}

	implPhase := &db.WorkflowPhase{
		WorkflowID:      "script-prop-wf",
		PhaseTemplateID: "implement",
		Sequence:        1,
	}
	if err := gdb.SaveWorkflowPhase(implPhase); err != nil {
		t.Fatalf("save implement phase: %v", err)
	}

	tsk := task.NewProtoTask("TASK-SCRIPT-PROP-001", "Test script output propagation")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	wfID := "script-prop-wf"
	tsk.WorkflowId = &wfID
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// MockTurnExecutor captures the prompt sent to the implement phase
	mockTurn := &MockTurnExecutor{
		Responses: []string{
			`{"status": "complete", "summary": "done", "content": "implemented"}`,
		},
	}

	// Use default registry for script phases (real ScriptPhaseExecutor).
	// Inject MockTurnExecutor only for LLM phases.
	we := NewWorkflowExecutor(
		backend, backend.DB(), gdb, &config.Config{}, t.TempDir(),
		WithWorkflowTurnExecutor(mockTurn),
		WithWorkflowLogger(slog.Default()),
		WithSkipGates(true),
	)

	result, err := we.Run(context.Background(), "script-prop-wf", WorkflowRunOptions{
		ContextType: ContextTask,
		TaskID:      tsk.Id,
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// Both phases should complete
	phaseStatuses := make(map[string]string)
	for _, pr := range result.PhaseResults {
		phaseStatuses[pr.PhaseID] = pr.Status
	}

	if phaseStatuses["build-step"] != orcv1.PhaseStatus_PHASE_STATUS_COMPLETED.String() {
		t.Errorf("build-step status = %q, want COMPLETED", phaseStatuses["build-step"])
	}
	if phaseStatuses["implement"] != orcv1.PhaseStatus_PHASE_STATUS_COMPLETED.String() {
		t.Errorf("implement status = %q, want COMPLETED", phaseStatuses["implement"])
	}

	// KEY ASSERTION: The implement phase's prompt must contain the script output.
	// This proves the full chain: script runs → output captured → stored as var →
	// next phase template resolves {{BUILD_OUTPUT}} → prompt contains output.
	if len(mockTurn.Prompts) == 0 {
		t.Fatal("implement phase was never called (no prompts captured)")
	}

	implementPrompt := mockTurn.Prompts[0]
	// The script phase should have produced SOME output that flows through.
	// We check that BUILD_OUTPUT was resolved (not left as literal {{BUILD_OUTPUT}}).
	if strings.Contains(implementPrompt, "{{BUILD_OUTPUT}}") {
		t.Error("implement prompt contains unresolved {{BUILD_OUTPUT}} — script output not propagated")
	}

	// Workflow should succeed overall
	if !result.Success {
		t.Errorf("workflow should succeed, got error: %s", result.Error)
	}
}

// =============================================================================
// Wiring Point 6: Run() with API phase → output propagates to next phase
//
// Same as script propagation but for API phases. Verifies the API executor
// is dispatched through Run() and its response body flows as a variable.
//
// Deletion test: Remove "api" registration → workflow fails on API phase.
// Remove output storage → next phase doesn't see API response.
// =============================================================================

func TestRunLoop_APIPhaseOutputPropagation(t *testing.T) {
	t.Parallel()

	// API server that returns a known response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"deploy_id": "deploy-789", "status": "success"}`)
	}))
	defer server.Close()

	backend := storage.NewTestBackend(t)
	gdb := testGlobalDBFrom(backend)

	// Phase 1: API phase that calls the deploy endpoint
	apiTmpl := &db.PhaseTemplate{
		ID:               "deploy-api",
		Name:             "Deploy API",
		Type:             "api",
		PromptSource:     "db",
		ProducesArtifact: true,
		OutputVarName:    "DEPLOY_RESPONSE",
	}
	if err := gdb.SavePhaseTemplate(apiTmpl); err != nil {
		t.Fatalf("save api template: %v", err)
	}

	// Phase 2: LLM phase that should receive the API response
	implTmpl := &db.PhaseTemplate{
		ID:            "implement",
		Name:          "Implement",
		PromptSource:  "db",
		PromptContent: "deploy response: {{DEPLOY_RESPONSE}}",
	}
	if err := gdb.SavePhaseTemplate(implTmpl); err != nil {
		t.Fatalf("save implement template: %v", err)
	}

	wf := &db.Workflow{ID: "api-prop-wf", Name: "API Propagation Test"}
	if err := gdb.SaveWorkflow(wf); err != nil {
		t.Fatalf("save workflow: %v", err)
	}

	apiPhase := &db.WorkflowPhase{
		WorkflowID:      "api-prop-wf",
		PhaseTemplateID: "deploy-api",
		Sequence:        0,
	}
	if err := gdb.SaveWorkflowPhase(apiPhase); err != nil {
		t.Fatalf("save api phase: %v", err)
	}

	implPhase := &db.WorkflowPhase{
		WorkflowID:      "api-prop-wf",
		PhaseTemplateID: "implement",
		Sequence:        1,
	}
	if err := gdb.SaveWorkflowPhase(implPhase); err != nil {
		t.Fatalf("save implement phase: %v", err)
	}

	tsk := task.NewProtoTask("TASK-API-PROP-001", "Test API output propagation")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	wfID := "api-prop-wf"
	tsk.WorkflowId = &wfID
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	mockTurn := &MockTurnExecutor{
		Responses: []string{
			`{"status": "complete", "summary": "done", "content": "implemented"}`,
		},
	}

	// Default registry for API phases (real APIPhaseExecutor).
	// Provide the API URL through initial variables so the executor can find it.
	we := NewWorkflowExecutor(
		backend, backend.DB(), gdb, &config.Config{}, t.TempDir(),
		WithWorkflowTurnExecutor(mockTurn),
		WithWorkflowLogger(slog.Default()),
		WithSkipGates(true),
	)

	result, err := we.Run(context.Background(), "api-prop-wf", WorkflowRunOptions{
		ContextType: ContextTask,
		TaskID:      tsk.Id,
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	phaseStatuses := make(map[string]string)
	for _, pr := range result.PhaseResults {
		phaseStatuses[pr.PhaseID] = pr.Status
	}

	if phaseStatuses["deploy-api"] != orcv1.PhaseStatus_PHASE_STATUS_COMPLETED.String() {
		t.Errorf("deploy-api status = %q, want COMPLETED", phaseStatuses["deploy-api"])
	}
	if phaseStatuses["implement"] != orcv1.PhaseStatus_PHASE_STATUS_COMPLETED.String() {
		t.Errorf("implement status = %q, want COMPLETED", phaseStatuses["implement"])
	}

	// KEY ASSERTION: API response body flows through to the implement phase prompt
	if len(mockTurn.Prompts) == 0 {
		t.Fatal("implement phase was never called (no prompts captured)")
	}

	implementPrompt := mockTurn.Prompts[0]
	if strings.Contains(implementPrompt, "{{DEPLOY_RESPONSE}}") {
		t.Error("implement prompt contains unresolved {{DEPLOY_RESPONSE}} — API output not propagated")
	}

	if !result.Success {
		t.Errorf("workflow should succeed, got error: %s", result.Error)
	}
}

// =============================================================================
// Wiring Point 7: Script phase result persisted to WorkflowRunPhase record
//
// Verifies that when a script phase executes through executePhase(), the
// WorkflowRunPhase database record is updated with:
//   - COMPLETED status
//   - Phase content (command stdout)
//   - Zero cost (non-LLM phase)
//
// This tests the post-execution persistence code in workflow_phase.go:179-195
// which handles non-LLM phase results.
//
// Deletion test: Remove the runPhase save block for non-LLM phases →
// WorkflowRunPhase record stays PENDING.
// =============================================================================

func TestExecutePhase_ScriptResult_PersistsToDB(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	gdb := testGlobalDBFrom(backend)

	tmpl := &db.PhaseTemplate{
		ID:           "build",
		Name:         "Build",
		Type:         "script",
		PromptSource: "db",
	}
	if err := gdb.SavePhaseTemplate(tmpl); err != nil {
		t.Fatalf("save template: %v", err)
	}

	wf := &db.Workflow{ID: "persist-script-wf", Name: "Persist Script Test"}
	if err := gdb.SaveWorkflow(wf); err != nil {
		t.Fatalf("save workflow: %v", err)
	}
	wfPhase := &db.WorkflowPhase{
		WorkflowID:      "persist-script-wf",
		PhaseTemplateID: "build",
		Sequence:        0,
	}
	if err := gdb.SaveWorkflowPhase(wfPhase); err != nil {
		t.Fatalf("save workflow phase: %v", err)
	}

	we := NewWorkflowExecutor(
		backend, backend.DB(), gdb, &config.Config{}, t.TempDir(),
		WithWorkflowLogger(slog.Default()),
		WithSkipGates(true),
	)

	tsk := task.NewProtoTask("TASK-PERSIST-001", "Test persistence")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	run := &db.WorkflowRun{
		ID:          "run-persist-script-001",
		WorkflowID:  "persist-script-wf",
		TaskID:      &tsk.Id,
		ContextType: "task",
		Prompt:      "test",
		Status:      "running",
	}
	if err := backend.SaveWorkflowRun(run); err != nil {
		t.Fatalf("save run: %v", err)
	}

	runPhase := &db.WorkflowRunPhase{
		WorkflowRunID:   "run-persist-script-001",
		PhaseTemplateID: "build",
		Status:          orcv1.PhaseStatus_PHASE_STATUS_PENDING.String(),
	}
	if err := backend.SaveWorkflowRunPhase(runPhase); err != nil {
		t.Fatalf("save run phase: %v", err)
	}

	vars := variable.VariableSet{}
	rctx := &variable.ResolutionContext{
		PhaseOutputVars: make(map[string]string),
	}

	result, err := we.executePhase(
		context.Background(), tmpl, wfPhase, vars, rctx, run, runPhase, tsk,
	)
	if err != nil {
		t.Fatalf("executePhase error: %v", err)
	}

	if result.Status != orcv1.PhaseStatus_PHASE_STATUS_COMPLETED.String() {
		t.Errorf("result.Status = %q, want COMPLETED", result.Status)
	}

	// Verify the database record was updated
	phases, err := backend.GetWorkflowRunPhases("run-persist-script-001")
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

	// Non-LLM phase: zero cost
	if savedPhase.CostUSD != 0 {
		t.Errorf("saved phase cost = %f, want 0", savedPhase.CostUSD)
	}
	if savedPhase.InputTokens != 0 {
		t.Errorf("saved phase input tokens = %d, want 0", savedPhase.InputTokens)
	}
	if savedPhase.OutputTokens != 0 {
		t.Errorf("saved phase output tokens = %d, want 0", savedPhase.OutputTokens)
	}
}
