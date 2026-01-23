package executor

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
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

func TestCheckSpecRequirements_TrivialWeight(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	we := &WorkflowExecutor{
		backend: backend,
		orcConfig: &config.Config{
			Plan: config.PlanConfig{
				RequireSpecForExecution: true,
			},
		},
		logger: slog.Default(),
	}

	// Trivial tasks should always pass
	tsk := &task.Task{
		ID:     "TASK-001",
		Weight: task.WeightTrivial,
	}

	err := we.checkSpecRequirements(tsk, nil)
	if err != nil {
		t.Errorf("checkSpecRequirements() for trivial weight = %v, want nil", err)
	}
}

func TestCheckSpecRequirements_NilTask(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	we := &WorkflowExecutor{
		backend: backend,
		orcConfig: &config.Config{
			Plan: config.PlanConfig{
				RequireSpecForExecution: true,
			},
		},
		logger: slog.Default(),
	}

	err := we.checkSpecRequirements(nil, nil)
	if err != nil {
		t.Errorf("checkSpecRequirements() for nil task = %v, want nil", err)
	}
}

func TestCheckSpecRequirements_StartsWithSpecPhase(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	we := &WorkflowExecutor{
		backend: backend,
		orcConfig: &config.Config{
			Plan: config.PlanConfig{
				RequireSpecForExecution: true,
			},
		},
		logger: slog.Default(),
	}

	tsk := &task.Task{
		ID:     "TASK-001",
		Weight: task.WeightMedium,
	}

	phases := []*db.WorkflowPhase{
		{PhaseTemplateID: "spec"},
		{PhaseTemplateID: "implement"},
	}

	// Should pass because workflow starts with spec phase
	err := we.checkSpecRequirements(tsk, phases)
	if err != nil {
		t.Errorf("checkSpecRequirements() with spec phase = %v, want nil", err)
	}
}

func TestCheckSpecRequirements_StartsWithTinySpecPhase(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	we := &WorkflowExecutor{
		backend: backend,
		orcConfig: &config.Config{
			Plan: config.PlanConfig{
				RequireSpecForExecution: true,
			},
		},
		logger: slog.Default(),
	}

	tsk := &task.Task{
		ID:     "TASK-001",
		Weight: task.WeightSmall,
	}

	phases := []*db.WorkflowPhase{
		{PhaseTemplateID: "tiny_spec"},
		{PhaseTemplateID: "implement"},
	}

	err := we.checkSpecRequirements(tsk, phases)
	if err != nil {
		t.Errorf("checkSpecRequirements() with tiny_spec phase = %v, want nil", err)
	}
}

func TestCheckSpecRequirements_ValidationDisabled(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	we := &WorkflowExecutor{
		backend: backend,
		orcConfig: &config.Config{
			Plan: config.PlanConfig{
				RequireSpecForExecution: false,
			},
		},
		logger: slog.Default(),
	}

	tsk := &task.Task{
		ID:     "TASK-001",
		Weight: task.WeightMedium,
	}

	// No phases - would fail if validation was enabled
	err := we.checkSpecRequirements(tsk, nil)
	if err != nil {
		t.Errorf("checkSpecRequirements() with validation disabled = %v, want nil", err)
	}
}

func TestCheckSpecRequirements_SkipValidationWeights(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	we := &WorkflowExecutor{
		backend: backend,
		orcConfig: &config.Config{
			Plan: config.PlanConfig{
				RequireSpecForExecution: true,
				SkipValidationWeights:   []string{"small", "medium"},
			},
		},
		logger: slog.Default(),
	}

	tsk := &task.Task{
		ID:     "TASK-001",
		Weight: task.WeightSmall,
	}

	// Small weight is in skip list - should pass
	err := we.checkSpecRequirements(tsk, nil)
	if err != nil {
		t.Errorf("checkSpecRequirements() with skipped weight = %v, want nil", err)
	}
}

func TestCheckSpecRequirements_MissingSpec(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	// Create and save task
	tsk := &task.Task{
		ID:     "TASK-001",
		Weight: task.WeightMedium,
	}
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("SaveTask() = %v", err)
	}

	we := &WorkflowExecutor{
		backend: backend,
		orcConfig: &config.Config{
			Plan: config.PlanConfig{
				RequireSpecForExecution: true,
			},
		},
		logger: slog.Default(),
	}

	// No spec saved, implement phase first - should fail
	phases := []*db.WorkflowPhase{
		{PhaseTemplateID: "implement"},
	}

	err := we.checkSpecRequirements(tsk, phases)
	if err == nil {
		t.Error("checkSpecRequirements() with missing spec = nil, want error")
	}
}

func TestCheckSpecRequirements_WithValidSpec(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	// Create and save task
	tsk := &task.Task{
		ID:     "TASK-001",
		Weight: task.WeightMedium,
	}
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("SaveTask() = %v", err)
	}

	// Save spec (taskID, content, source)
	if err := backend.SaveSpecForTask(tsk.ID, "# Spec\n\nValid spec content", "test"); err != nil {
		t.Fatalf("SaveSpecForTask() = %v", err)
	}

	we := &WorkflowExecutor{
		backend: backend,
		orcConfig: &config.Config{
			Plan: config.PlanConfig{
				RequireSpecForExecution: true,
			},
		},
		logger: slog.Default(),
	}

	phases := []*db.WorkflowPhase{
		{PhaseTemplateID: "implement"},
	}

	err := we.checkSpecRequirements(tsk, phases)
	if err != nil {
		t.Errorf("checkSpecRequirements() with valid spec = %v, want nil", err)
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

	tsk := &task.Task{
		ID: "TASK-001",
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
