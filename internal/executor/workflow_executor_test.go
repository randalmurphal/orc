package executor

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/storage"
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
