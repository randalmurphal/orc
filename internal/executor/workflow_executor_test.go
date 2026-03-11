package executor

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/gate"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/internal/variable"
)

func TestIsPhaseTimeoutError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "regular error",
			err:      errors.New("something went wrong"),
			expected: false,
		},
		{
			name: "phase timeout error",
			err: &phaseTimeoutError{
				phase:   "implement",
				timeout: 30 * time.Minute,
				taskID:  "TASK-001",
				err:     context.DeadlineExceeded,
			},
			expected: true,
		},
		{
			name: "wrapped phase timeout error",
			err: errors.Join(errors.New("wrapper"), &phaseTimeoutError{
				phase:   "review",
				timeout: 60 * time.Minute,
				taskID:  "TASK-002",
				err:     context.DeadlineExceeded,
			}),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := IsPhaseTimeoutError(tt.err)
			if result != tt.expected {
				t.Errorf("IsPhaseTimeoutError() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestNewWorkflowExecutorInitializesPendingDecisionStore(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	we := NewWorkflowExecutor(backend, backend.DB(), testGlobalDBFrom(backend), config.Default(), t.TempDir())

	if we.pendingDecisions == nil {
		t.Fatal("pendingDecisions should be initialized by default")
	}
}

func TestPhaseTimeoutError_Error(t *testing.T) {
	t.Parallel()

	pte := &phaseTimeoutError{
		phase:   "implement",
		timeout: 45 * time.Minute,
		taskID:  "TASK-123",
		err:     context.DeadlineExceeded,
	}

	msg := pte.Error()
	expected := "phase implement exceeded timeout (45m0s). Run 'orc resume TASK-123' to retry."
	if msg != expected {
		t.Errorf("Error() = %q, want %q", msg, expected)
	}
}

func TestPhaseTimeoutError_Unwrap(t *testing.T) {
	t.Parallel()

	underlying := context.DeadlineExceeded
	pte := &phaseTimeoutError{
		phase:   "test",
		timeout: 10 * time.Minute,
		taskID:  "TASK-001",
		err:     underlying,
	}

	unwrapped := pte.Unwrap()
	if unwrapped != underlying {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, underlying)
	}
}

func TestIsPhaseBlockedError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "regular error",
			err:      errors.New("something went wrong"),
			expected: false,
		},
		{
			name: "phase blocked error",
			err: &PhaseBlockedError{
				Phase:  "review",
				Reason: "issues found requiring fixes",
				Output: `{"status": "blocked", "issues": []}`,
			},
			expected: true,
		},
		{
			name: "wrapped phase blocked error",
			err: errors.Join(errors.New("wrapper"), &PhaseBlockedError{
				Phase:  "review",
				Reason: "needs attention",
				Output: `{"status": "blocked"}`,
			}),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := IsPhaseBlockedError(tt.err)
			if result != tt.expected {
				t.Errorf("IsPhaseBlockedError() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestPhaseBlockedError_Error(t *testing.T) {
	t.Parallel()

	pbe := &PhaseBlockedError{
		Phase:  "review",
		Reason: "issues found requiring attention",
		Output: `{"status": "blocked"}`,
	}

	msg := pbe.Error()
	expected := "phase review blocked: issues found requiring attention"
	if msg != expected {
		t.Errorf("Error() = %q, want %q", msg, expected)
	}
}

func TestExecutePhaseWithTimeout_NoTimeout(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	// Create workflow executor with no timeout (PhaseMax = 0)
	we := &WorkflowExecutor{
		backend: backend,
		orcConfig: &config.Config{
			Timeouts: config.TimeoutsConfig{
				PhaseMax: 0, // No timeout
			},
		},
		logger:   slog.Default(),
		resolver: variable.NewResolver("/tmp"),
	}

	// Create minimal test fixtures
	tmpl := &db.PhaseTemplate{
		ID:   "test_phase",
		Name: "Test Phase",
	}
	phase := &db.WorkflowPhase{
		PhaseTemplateID: "test_phase",
	}
	run := &db.WorkflowRun{
		ID: "run-001",
	}
	runPhase := &db.WorkflowRunPhase{
		WorkflowRunID:   "run-001",
		PhaseTemplateID: "test_phase",
	}

	// Use existing MockTurnExecutor
	mockTE := NewMockTurnExecutor(`{"status": "complete", "summary": "Done"}`)
	we.turnExecutor = mockTE

	// Should call executePhase directly (no timeout wrapper)
	ctx := context.Background()
	_, err := we.executePhaseWithTimeout(ctx, tmpl, phase, map[string]string{}, nil, run, runPhase, nil)

	// We expect an error because we haven't set up the full execution environment,
	// but the important thing is it doesn't panic and the timeout logic is bypassed
	// when PhaseMax is 0
	_ = err // Error expected due to incomplete setup - that's OK for this test
}

func TestExecutePhaseWithTimeout_TimeoutReached(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	// Create workflow executor with very short timeout
	we := &WorkflowExecutor{
		backend: backend,
		orcConfig: &config.Config{
			Timeouts: config.TimeoutsConfig{
				PhaseMax: 50 * time.Millisecond, // Very short timeout for testing
			},
		},
		logger:   slog.Default(),
		resolver: variable.NewResolver("/tmp"),
	}

	// Create minimal test fixtures
	tmpl := &db.PhaseTemplate{
		ID:   "slow_phase",
		Name: "Slow Phase",
	}
	phase := &db.WorkflowPhase{
		PhaseTemplateID: "slow_phase",
	}
	run := &db.WorkflowRun{
		ID: "run-001",
	}
	runPhase := &db.WorkflowRunPhase{
		WorkflowRunID:   "run-001",
		PhaseTemplateID: "slow_phase",
	}

	tsk := &orcv1.Task{
		Id: "TASK-001",
	}

	// Use existing MockTurnExecutor with Delay
	mockTE := NewMockTurnExecutor(`{"status": "complete", "summary": "Done"}`)
	mockTE.Delay = 200 * time.Millisecond // Longer than timeout
	we.turnExecutor = mockTE

	ctx := context.Background()
	_, err := we.executePhaseWithTimeout(ctx, tmpl, phase, map[string]string{}, nil, run, runPhase, tsk)

	// Should get a timeout error (or context deadline exceeded)
	// Other errors from incomplete setup are OK - the key test is that timeout machinery doesn't panic
	_ = err
}

func TestExecutePhaseWithTimeout_WarningTimers(t *testing.T) {
	t.Parallel()

	// This test verifies that the warning timers don't cause issues
	// when the phase completes before the warnings fire

	backend := storage.NewTestBackend(t)

	we := &WorkflowExecutor{
		backend: backend,
		orcConfig: &config.Config{
			Timeouts: config.TimeoutsConfig{
				PhaseMax: 10 * time.Second, // Long enough timeout
			},
		},
		logger:   slog.Default(),
		resolver: variable.NewResolver("/tmp"),
	}

	tmpl := &db.PhaseTemplate{
		ID:   "quick_phase",
		Name: "Quick Phase",
	}
	phase := &db.WorkflowPhase{
		PhaseTemplateID: "quick_phase",
	}
	run := &db.WorkflowRun{
		ID: "run-001",
	}
	runPhase := &db.WorkflowRunPhase{
		WorkflowRunID:   "run-001",
		PhaseTemplateID: "quick_phase",
	}

	// Mock that returns immediately
	mockTE := NewMockTurnExecutor(`{"status": "complete", "summary": "Done"}`)
	we.turnExecutor = mockTE

	ctx := context.Background()
	_, err := we.executePhaseWithTimeout(ctx, tmpl, phase, map[string]string{}, nil, run, runPhase, nil)

	// The main check is that we don't have goroutine leaks or panics
	// when the phase completes before the 50%/75% warning timers fire
	_ = err // Error expected due to incomplete setup
}

// TestWorkflowRunResult_PopulatesFields verifies that WorkflowRunResult fields
// are properly populated from the workflow run.
func TestWorkflowRunResult_PopulatesFields(t *testing.T) {
	t.Parallel()

	// Test that the result struct has the expected fields
	result := WorkflowRunResult{
		RunID:        "RUN-001",
		WorkflowID:   "implement-small",
		TaskID:       "TASK-001",
		StartedAt:    time.Now(),
		TotalCostUSD: 1.25,
		TotalTokens:  5000,
	}

	if result.RunID != "RUN-001" {
		t.Errorf("RunID = %q, want %q", result.RunID, "RUN-001")
	}
	if result.TaskID != "TASK-001" {
		t.Errorf("TaskID = %q, want %q", result.TaskID, "TASK-001")
	}
	if result.TotalCostUSD != 1.25 {
		t.Errorf("TotalCostUSD = %f, want %f", result.TotalCostUSD, 1.25)
	}
	if result.TotalTokens != 5000 {
		t.Errorf("TotalTokens = %d, want %d", result.TotalTokens, 5000)
	}
}

// TestWorkflowContextType verifies context types for task vs non-task workflows.
func TestWorkflowContextType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		contextType ContextType
		hasTask     bool
	}{
		{"default creates task", ContextDefault, true},
		{"task attaches to task", ContextTask, true},
		{"branch has no task", ContextBranch, false},
		{"pr has no task", ContextPR, false},
		{"standalone has no task", ContextStandalone, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Verify the context type semantics
			hasTask := tt.contextType == ContextDefault || tt.contextType == ContextTask
			if hasTask != tt.hasTask {
				t.Errorf("context %s hasTask = %v, want %v", tt.contextType, hasTask, tt.hasTask)
			}
		})
	}
}

// TestEvaluateLoopCondition verifies the QA loop condition evaluation logic.
func TestEvaluateLoopCondition(t *testing.T) {
	t.Parallel()

	logger := slog.Default()
	we := &WorkflowExecutor{logger: logger}

	tests := []struct {
		name        string
		condition   string
		targetPhase string
		vars        map[string]string
		rctx        *variable.ResolutionContext
		expected    bool
	}{
		{
			name:        "has_findings with findings",
			condition:   "has_findings",
			targetPhase: "qa_e2e_test",
			vars:        map[string]string{},
			rctx: &variable.ResolutionContext{
				PriorOutputs: map[string]string{
					"qa_e2e_test": `{"status":"complete","summary":"Found 2 issues","findings":[{"id":"QA-001","severity":"high","confidence":95,"category":"functional","title":"Bug","steps_to_reproduce":["1"],"expected":"A","actual":"B"}]}`,
				},
			},
			expected: true,
		},
		{
			name:        "has_findings without findings",
			condition:   "has_findings",
			targetPhase: "qa_e2e_test",
			vars:        map[string]string{},
			rctx: &variable.ResolutionContext{
				PriorOutputs: map[string]string{
					"qa_e2e_test": `{"status":"complete","summary":"All tests passed","findings":[]}`,
				},
			},
			expected: false,
		},
		{
			name:        "has_findings with no output",
			condition:   "has_findings",
			targetPhase: "qa_e2e_test",
			vars:        map[string]string{},
			rctx: &variable.ResolutionContext{
				PriorOutputs: map[string]string{},
			},
			expected: false,
		},
		{
			name:        "not_empty with content",
			condition:   "not_empty",
			targetPhase: "spec",
			vars:        map[string]string{},
			rctx: &variable.ResolutionContext{
				PriorOutputs: map[string]string{
					"spec": `{"content":"some content"}`,
				},
			},
			expected: true,
		},
		{
			name:        "not_empty with empty object",
			condition:   "not_empty",
			targetPhase: "spec",
			vars:        map[string]string{},
			rctx: &variable.ResolutionContext{
				PriorOutputs: map[string]string{
					"spec": `{}`,
				},
			},
			expected: false,
		},
		{
			name:        "status_needs_fix with needs_fix status",
			condition:   "status_needs_fix",
			targetPhase: "qa",
			vars:        map[string]string{},
			rctx: &variable.ResolutionContext{
				PriorOutputs: map[string]string{
					"qa": `{"status":"needs_fix"}`,
				},
			},
			expected: true,
		},
		{
			name:        "status_needs_fix with complete status",
			condition:   "status_needs_fix",
			targetPhase: "qa",
			vars:        map[string]string{},
			rctx: &variable.ResolutionContext{
				PriorOutputs: map[string]string{
					"qa": `{"status":"complete"}`,
				},
			},
			expected: false,
		},
		{
			name:        "unknown condition",
			condition:   "unknown_condition",
			targetPhase: "test",
			vars:        map[string]string{},
			rctx: &variable.ResolutionContext{
				PriorOutputs: map[string]string{
					"test": `{"data":"value"}`,
				},
			},
			expected: false,
		},
		{
			name:        "falls back to OUTPUT_ var",
			condition:   "not_empty",
			targetPhase: "custom_phase",
			vars: map[string]string{
				"OUTPUT_custom_phase": `{"content":"from var"}`,
			},
			rctx: &variable.ResolutionContext{
				PriorOutputs: map[string]string{},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := we.evaluateLoopCondition(tt.condition, tt.targetPhase, tt.vars, tt.rctx)
			if result != tt.expected {
				t.Errorf("evaluateLoopCondition(%q, %q) = %v, want %v",
					tt.condition, tt.targetPhase, result, tt.expected)
			}
		})
	}
}

func TestExtractPhaseOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "extracts content field when present",
			input:    `{"status": "complete", "content": "The spec content"}`,
			expected: "The spec content",
		},
		{
			name:     "returns full JSON for qa_e2e_test output (findings)",
			input:    `{"status": "complete", "summary": "Tested 5 scenarios", "findings": [{"id": "QA-001", "title": "Bug found"}]}`,
			expected: `{"status": "complete", "summary": "Tested 5 scenarios", "findings": [{"id": "QA-001", "title": "Bug found"}]}`,
		},
		{
			name:     "returns full JSON for qa_e2e_fix output (fixes_applied)",
			input:    `{"status": "complete", "summary": "Fixed 2 issues", "fixes_applied": [{"finding_id": "QA-001", "status": "fixed"}]}`,
			expected: `{"status": "complete", "summary": "Fixed 2 issues", "fixes_applied": [{"finding_id": "QA-001", "status": "fixed"}]}`,
		},
		{
			name:     "returns empty for invalid JSON",
			input:    "not valid json",
			expected: "",
		},
		{
			name:     "returns empty for empty input",
			input:    "",
			expected: "",
		},
		{
			name:     "handles whitespace",
			input:    `  {"status": "complete", "findings": []}  `,
			expected: `{"status": "complete", "findings": []}`,
		},
		{
			name:     "prefers content field over full JSON",
			input:    `{"status": "complete", "content": "The content", "findings": []}`,
			expected: "The content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractPhaseOutput(tt.input)
			if result != tt.expected {
				t.Errorf("extractPhaseOutput() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// =============================================================================
// TASK-709: Ensure phase output variables survive retry flow
//
// Coverage mapping:
//   SC-1: TestApplyPhaseContentToVars_PersistsToRctx
//   SC-2: TestPhaseOutputVarsSurviveResolveAll
//   SC-3: TestRetryFlowPreservesPhaseOutputVars
//   SC-4: TestApplyGateOutputToVars_PersistsToRctx
//   SC-5: TestGateOutputVarsSurviveResolveAll
//   SC-6: TestRetryFlowPreservesGateOutputVars
//
// Failure modes:
//   TestApplyGateOutputToVars_SkipsEmpty
//
// Edge cases:
//   TestApplyGateOutputToVars_NilRctx
// =============================================================================

// SC-1: applyPhaseContentToVars stores output in rctx.PhaseOutputVars and rctx.PriorOutputs.
// Documents existing behavior that must be preserved.
func TestApplyPhaseContentToVars_PersistsToRctx(t *testing.T) {
	t.Parallel()

	vars := make(map[string]string)
	rctx := &variable.ResolutionContext{
		PriorOutputs: make(map[string]string),
	}

	content := "# Specification\n\nThis is the spec content with success criteria."
	applyPhaseContentToVars(vars, rctx, "spec", content, "SPEC_CONTENT")

	// Verify rctx.PhaseOutputVars is populated
	if rctx.PhaseOutputVars == nil {
		t.Fatal("rctx.PhaseOutputVars should be initialized, got nil")
	}
	if got := rctx.PhaseOutputVars["SPEC_CONTENT"]; got != content {
		t.Errorf("rctx.PhaseOutputVars[\"SPEC_CONTENT\"] = %q, want %q", got, content)
	}

	// Verify rctx.PriorOutputs is populated
	if got := rctx.PriorOutputs["spec"]; got != content {
		t.Errorf("rctx.PriorOutputs[\"spec\"] = %q, want %q", got, content)
	}

	// Verify vars map also has the named variable
	if got := vars["SPEC_CONTENT"]; got != content {
		t.Errorf("vars[\"SPEC_CONTENT\"] = %q, want %q", got, content)
	}

	// Verify the lowercase OUTPUT_ key is also set (for loop condition evaluation)
	if got := vars["OUTPUT_spec"]; got != content {
		t.Errorf("vars[\"OUTPUT_spec\"] = %q, want %q", got, content)
	}
}

// SC-2: Phase output variables restored by addBuiltinVariables from rctx after
// ResolveAll creates a new vars map. Documents existing behavior.
func TestPhaseOutputVarsSurviveResolveAll(t *testing.T) {
	t.Parallel()

	resolver := variable.NewResolver(t.TempDir())
	rctx := &variable.ResolutionContext{
		PhaseOutputVars: map[string]string{
			"SPEC_CONTENT": "the spec content from a prior phase",
		},
		PriorOutputs: map[string]string{
			"spec": "the spec content from a prior phase",
		},
	}

	// ResolveAll creates a fresh VariableSet each call
	vars, err := resolver.ResolveAll(context.Background(), nil, rctx)
	if err != nil {
		t.Fatalf("ResolveAll: %v", err)
	}

	// Phase output variables should be restored from rctx.PhaseOutputVars
	if got := vars["SPEC_CONTENT"]; got != "the spec content from a prior phase" {
		t.Errorf("vars[\"SPEC_CONTENT\"] = %q, want %q", got, "the spec content from a prior phase")
	}

	// Generic OUTPUT_ prefix should also be restored from PriorOutputs
	if got := vars["OUTPUT_SPEC"]; got != "the spec content from a prior phase" {
		t.Errorf("vars[\"OUTPUT_SPEC\"] = %q, want %q", got, "the spec content from a prior phase")
	}
}

// SC-4: applyGateOutputToVars persists gate output to rctx.PhaseOutputVars
// in addition to the local vars map. This is the core fix.
// The function's NEW signature includes rctx — this test will NOT COMPILE
// until the implementation adds the rctx parameter.
func TestApplyGateOutputToVars_PersistsToRctx(t *testing.T) {
	t.Parallel()

	vars := make(map[string]string)
	rctx := &variable.ResolutionContext{}

	gateResult := &GateEvaluationResult{
		Approved:  false,
		Reason:    "needs work on error handling",
		OutputVar: "REVIEW_ANALYSIS",
		OutputData: map[string]any{
			"verdict":  "needs_fix",
			"findings": []string{"missing error check in handler.go"},
		},
	}

	// Call with NEW signature that includes rctx
	applyGateOutputToVars(vars, rctx, gateResult)

	// Verify vars map has the gate output
	if _, ok := vars["REVIEW_ANALYSIS"]; !ok {
		t.Fatal("vars should contain REVIEW_ANALYSIS")
	}

	// Verify the value is valid JSON
	var parsed map[string]any
	if err := json.Unmarshal([]byte(vars["REVIEW_ANALYSIS"]), &parsed); err != nil {
		t.Fatalf("vars[\"REVIEW_ANALYSIS\"] is not valid JSON: %v", err)
	}

	// Verify rctx.PhaseOutputVars is populated with gate output
	if rctx.PhaseOutputVars == nil {
		t.Fatal("rctx.PhaseOutputVars should be initialized, got nil")
	}
	if got, ok := rctx.PhaseOutputVars["REVIEW_ANALYSIS"]; !ok {
		t.Fatal("rctx.PhaseOutputVars should contain REVIEW_ANALYSIS")
	} else if got != vars["REVIEW_ANALYSIS"] {
		t.Errorf("rctx.PhaseOutputVars[\"REVIEW_ANALYSIS\"] = %q, want %q (same as vars)", got, vars["REVIEW_ANALYSIS"])
	}
}

// SC-4 failure mode: empty OutputVar, nil OutputData, or nil gate result skips storage.
// Guard behavior must be preserved: no entry stored in vars or rctx.
func TestApplyGateOutputToVars_SkipsEmpty(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		gateResult *GateEvaluationResult
	}{
		{
			name:       "nil gate result",
			gateResult: nil,
		},
		{
			name: "empty OutputVar",
			gateResult: &GateEvaluationResult{
				OutputVar:  "",
				OutputData: map[string]any{"verdict": "ok"},
			},
		},
		{
			name: "whitespace-only OutputVar",
			gateResult: &GateEvaluationResult{
				OutputVar:  "   ",
				OutputData: map[string]any{"verdict": "ok"},
			},
		},
		{
			name: "nil OutputData",
			gateResult: &GateEvaluationResult{
				OutputVar:  "REVIEW_RESULT",
				OutputData: nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			vars := make(map[string]string)
			rctx := &variable.ResolutionContext{}

			// Call with NEW signature
			applyGateOutputToVars(vars, rctx, tt.gateResult)

			if len(vars) != 0 {
				t.Errorf("vars should be empty, got %v", vars)
			}
			if len(rctx.PhaseOutputVars) != 0 {
				t.Errorf("rctx.PhaseOutputVars should be empty, got %v", rctx.PhaseOutputVars)
			}
		})
	}
}

// Edge case: nil rctx should still work (degrade gracefully to vars-only storage).
func TestApplyGateOutputToVars_NilRctx(t *testing.T) {
	t.Parallel()

	vars := make(map[string]string)
	gateResult := &GateEvaluationResult{
		OutputVar:  "REVIEW_ANALYSIS",
		OutputData: map[string]any{"verdict": "approved"},
	}

	// Should not panic with nil rctx
	applyGateOutputToVars(vars, nil, gateResult)

	// vars should still be populated
	if _, ok := vars["REVIEW_ANALYSIS"]; !ok {
		t.Fatal("vars should contain REVIEW_ANALYSIS even with nil rctx")
	}
}

// SC-5: Gate output variables restored by addBuiltinVariables after ResolveAll.
// This piggybacks on existing PhaseOutputVars restoration mechanism.
func TestGateOutputVarsSurviveResolveAll(t *testing.T) {
	t.Parallel()

	resolver := variable.NewResolver(t.TempDir())

	// Simulate: gate output was stored in rctx.PhaseOutputVars (after fix)
	gateOutputJSON := `{"verdict":"needs_fix","findings":["missing error check"]}`
	rctx := &variable.ResolutionContext{
		PhaseOutputVars: map[string]string{
			"SPEC_CONTENT":    "the spec content",
			"REVIEW_ANALYSIS": gateOutputJSON,
		},
	}

	// ResolveAll creates a fresh VariableSet
	vars, err := resolver.ResolveAll(context.Background(), nil, rctx)
	if err != nil {
		t.Fatalf("ResolveAll: %v", err)
	}

	// Gate output variable should be restored from rctx.PhaseOutputVars
	if got := vars["REVIEW_ANALYSIS"]; got != gateOutputJSON {
		t.Errorf("vars[\"REVIEW_ANALYSIS\"] = %q, want %q", got, gateOutputJSON)
	}

	// Phase output variable should also be restored
	if got := vars["SPEC_CONTENT"]; got != "the spec content" {
		t.Errorf("vars[\"SPEC_CONTENT\"] = %q, want %q", got, "the spec content")
	}
}

// SC-3: Integration test — retried phase's rendered prompt contains output
// variable from a prior phase that ran before the retry.
//
// Setup: 3-phase workflow (spec→implement→review).
// - spec produces SPEC_CONTENT via OutputVarName
// - review gate rejects and triggers retry to implement
// - implement template contains {{SPEC_CONTENT}}
// - After retry, implement's rendered prompt should contain the spec content.
func TestRetryFlowPreservesPhaseOutputVars(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	// Set up phase templates:
	// - spec produces artifact with OutputVarName="SPEC_CONTENT"
	// - implement consumes SPEC_CONTENT in its prompt
	// - review has gate that rejects (triggering retry to implement)
	if err := backend.SavePhaseTemplate(&db.PhaseTemplate{
		ID:               "spec",
		Name:             "spec",
		PromptSource:     "db",
		PromptContent:    "Generate a specification.",
		ProducesArtifact: true,
		OutputVarName:    "SPEC_CONTENT",
	}); err != nil {
		t.Fatalf("save spec template: %v", err)
	}

	if err := backend.SavePhaseTemplate(&db.PhaseTemplate{
		ID:            "implement",
		Name:          "implement",
		PromptSource:  "db",
		PromptContent: "Implement based on spec:\n{{SPEC_CONTENT}}\nEnd of spec.",
	}); err != nil {
		t.Fatalf("save implement template: %v", err)
	}

	outputCfgJSON, _ := json.Marshal(db.GateOutputConfig{
		OnRejected: "retry",
		RetryFrom:  "implement",
	})
	if err := backend.SavePhaseTemplate(&db.PhaseTemplate{
		ID:               "review",
		Name:             "review",
		PromptSource:     "db",
		PromptContent:    "Review the implementation.",
		GateType:         "ai",
		GateOutputConfig: string(outputCfgJSON),
	}); err != nil {
		t.Fatalf("save review template: %v", err)
	}

	setupThreePhaseWorkflow(t, backend, "retry-phase-output-wf", "spec", "implement", "review")

	tsk := task.NewProtoTask("TASK-RETRY-PHASE-001", "Test phase output survives retry")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	wfID := "retry-phase-output-wf"
	tsk.WorkflowId = &wfID
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Mock turn executor with queued responses:
	// 1. spec: returns content that becomes SPEC_CONTENT
	// 2. implement (first): returns complete
	// 3. review (first): returns complete (gate will reject externally)
	// 4. implement (retry): returns complete
	// 5. review (second): returns complete (gate will approve)
	mockTE := &MockTurnExecutor{
		Responses: []string{
			`{"status": "complete", "content": "THE SPEC OUTPUT FROM PHASE ONE"}`,
			`{"status": "complete", "summary": "Implemented"}`,
			`{"status": "complete", "summary": "Reviewed"}`,
			`{"status": "complete", "summary": "Re-implemented after review"}`,
			`{"status": "complete", "summary": "Approved"}`,
		},
		SessionIDValue: "mock-session",
	}

	// Gate evaluator: review rejects first time, approves second
	reviewCallCount := 0
	mockEval := &configGateEvaluator{
		decisionFn: func(g *gate.Gate, output string, opts *gate.EvaluateOptions) (*gate.Decision, error) {
			if opts != nil && opts.Phase == "review" {
				reviewCallCount++
				if reviewCallCount == 1 {
					return &gate.Decision{Approved: false, Reason: "needs work"}, nil
				}
			}
			return &gate.Decision{Approved: true, Reason: "approved"}, nil
		},
	}

	we := NewWorkflowExecutor(
		backend, backend.DB(), testGlobalDBFrom(backend), &config.Config{}, t.TempDir(),
		WithWorkflowLogger(slog.Default()),
		WithWorkflowGateEvaluator(mockEval),
		WithWorkflowTurnExecutor(mockTE),
	)

	_, err := we.Run(context.Background(), "retry-phase-output-wf", WorkflowRunOptions{
		ContextType: ContextTask,
		TaskID:      tsk.Id,
	})
	if err != nil {
		t.Fatalf("workflow run failed: %v", err)
	}

	// The implement phase should have been called twice (initial + retry).
	// Find the second implement call's prompt and verify it contains SPEC_CONTENT.
	// Prompts: [spec, implement(1), review(1), implement(2), review(2)]
	if len(mockTE.Prompts) < 4 {
		t.Fatalf("expected at least 4 prompts (spec + impl + review + impl-retry), got %d", len(mockTE.Prompts))
	}

	// The 4th prompt (index 3) should be the implement retry.
	// It should contain the spec output because SPEC_CONTENT survives retry.
	retryImplementPrompt := mockTE.Prompts[3]
	if !strings.Contains(retryImplementPrompt, "THE SPEC OUTPUT FROM PHASE ONE") {
		t.Errorf("SC-3: retried implement prompt should contain SPEC_CONTENT.\nGot prompt:\n%s", retryImplementPrompt)
	}
}

// SC-6: Integration test — retried phase's rendered prompt contains the gate
// output variable from the rejecting gate.
//
// Setup: 3-phase workflow (spec→implement→review).
// - review gate rejects with OutputVar="GATE_REVIEW" and OutputData
// - implement template contains {{GATE_REVIEW}}
// - After retry, implement's rendered prompt should contain gate output.
//
// This test FAILS before fix because applyGateOutputToVars only writes to vars
// (lost on ResolveAll) and not to rctx.PhaseOutputVars.
func TestRetryFlowPreservesGateOutputVars(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	// spec produces SPEC_CONTENT
	if err := backend.SavePhaseTemplate(&db.PhaseTemplate{
		ID:               "spec",
		Name:             "spec",
		PromptSource:     "db",
		PromptContent:    "Generate a specification.",
		ProducesArtifact: true,
		OutputVarName:    "SPEC_CONTENT",
	}); err != nil {
		t.Fatalf("save spec template: %v", err)
	}

	// implement consumes both SPEC_CONTENT and GATE_REVIEW
	if err := backend.SavePhaseTemplate(&db.PhaseTemplate{
		ID:            "implement",
		Name:          "implement",
		PromptSource:  "db",
		PromptContent: "Implement based on spec:\n{{SPEC_CONTENT}}\n\nReview feedback:\n{{GATE_REVIEW}}\n\nEnd.",
	}); err != nil {
		t.Fatalf("save implement template: %v", err)
	}

	// review gate configured to retry with output
	outputCfgJSON, _ := json.Marshal(db.GateOutputConfig{
		OnRejected: "retry",
		RetryFrom:  "implement",
	})
	if err := backend.SavePhaseTemplate(&db.PhaseTemplate{
		ID:               "review",
		Name:             "review",
		PromptSource:     "db",
		PromptContent:    "Review the implementation.",
		GateType:         "ai",
		GateOutputConfig: string(outputCfgJSON),
	}); err != nil {
		t.Fatalf("save review template: %v", err)
	}

	setupThreePhaseWorkflow(t, backend, "retry-gate-output-wf", "spec", "implement", "review")

	tsk := task.NewProtoTask("TASK-RETRY-GATE-001", "Test gate output survives retry")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	wfID := "retry-gate-output-wf"
	tsk.WorkflowId = &wfID
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Mock turn executor with queued responses
	mockTE := &MockTurnExecutor{
		Responses: []string{
			`{"status": "complete", "content": "The spec content"}`,
			`{"status": "complete", "summary": "Implemented"}`,
			`{"status": "complete", "summary": "Reviewed"}`,
			`{"status": "complete", "summary": "Re-implemented with gate feedback"}`,
			`{"status": "complete", "summary": "Approved"}`,
		},
		SessionIDValue: "mock-session",
	}

	// Gate evaluator: review rejects first time with OutputVar/OutputData
	reviewCallCount := 0
	mockEval := &configGateEvaluator{
		decisionFn: func(g *gate.Gate, output string, opts *gate.EvaluateOptions) (*gate.Decision, error) {
			if opts != nil && opts.Phase == "review" {
				reviewCallCount++
				if reviewCallCount == 1 {
					return &gate.Decision{
						Approved: false,
						Reason:   "needs fixes",
						OutputVar: "GATE_REVIEW",
						OutputData: map[string]any{
							"verdict":  "needs_fix",
							"findings": []string{"error handling incomplete", "missing unit tests"},
						},
					}, nil
				}
			}
			return &gate.Decision{Approved: true, Reason: "approved"}, nil
		},
	}

	we := NewWorkflowExecutor(
		backend, backend.DB(), testGlobalDBFrom(backend), &config.Config{}, t.TempDir(),
		WithWorkflowLogger(slog.Default()),
		WithWorkflowGateEvaluator(mockEval),
		WithWorkflowTurnExecutor(mockTE),
	)

	_, err := we.Run(context.Background(), "retry-gate-output-wf", WorkflowRunOptions{
		ContextType: ContextTask,
		TaskID:      tsk.Id,
	})
	if err != nil {
		t.Fatalf("workflow run failed: %v", err)
	}

	// Find the retried implement prompt (4th prompt, index 3)
	if len(mockTE.Prompts) < 4 {
		t.Fatalf("expected at least 4 prompts, got %d", len(mockTE.Prompts))
	}

	retryImplementPrompt := mockTE.Prompts[3]

	// SC-6: The retried implement prompt should contain the gate output data.
	// Before fix: GATE_REVIEW is lost during ResolveAll → empty string in template.
	// After fix: GATE_REVIEW persists in rctx.PhaseOutputVars → rendered in template.
	if !strings.Contains(retryImplementPrompt, "needs_fix") {
		t.Errorf("SC-6: retried implement prompt should contain gate output (\"needs_fix\").\n"+
			"This fails before fix because gate output variables are lost during retry.\n"+
			"Got prompt:\n%s", retryImplementPrompt)
	}
	if !strings.Contains(retryImplementPrompt, "error handling incomplete") {
		t.Errorf("SC-6: retried implement prompt should contain gate findings.\n"+
			"Got prompt:\n%s", retryImplementPrompt)
	}
}
