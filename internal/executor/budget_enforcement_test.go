package executor

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// ============================================================================
// SC-1 + SC-7: Executor blocks when over budget, before any phase executes
// ============================================================================

func TestBudgetCheck_BlocksWhenOverBudget(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	globalDB := storage.NewTestGlobalDB(t)

	// Set up a project budget: $2,000 limit, $2,143 spent (over budget)
	projectID := t.TempDir() // Use workingDir as projectID (matches cost tracking)
	err := globalDB.SetBudget(db.CostBudget{
		ProjectID:             projectID,
		MonthlyLimitUSD:       2000.00,
		AlertThresholdPercent: 80,
		CurrentMonth:          currentMonth(),
		CurrentMonthSpent:     2143.00,
	})
	if err != nil {
		t.Fatalf("set budget: %v", err)
	}

	// Create workflow with a single phase
	workflowID := "test-workflow"
	setupMinimalWorkflow(t, backend, workflowID)
	setupMinimalWorkflowGlobal(t, globalDB, workflowID)

	// Create task
	tk := task.NewProtoTask("TASK-001", "Test Task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	tk.WorkflowId = &workflowID
	tk.Execution = task.InitProtoExecutionState()
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Create executor with injected globalDB
	mockTurn := NewMockTurnExecutor(`{"status": "complete", "summary": "Done"}`)
	we := NewWorkflowExecutor(
		backend,
		backend.DB(),
		globalDB,
		&config.Config{Model: "sonnet"},
		projectID,
		WithWorkflowTurnExecutor(mockTurn),
	)

	// Run WITHOUT --ignore-budget
	_, err = we.Run(context.Background(), workflowID, WorkflowRunOptions{
		ContextType:  ContextTask,
		TaskID:       "TASK-001",
		IgnoreBudget: false,
	})

	// SC-1: Should return error
	if err == nil {
		t.Fatal("expected budget exceeded error, got nil")
	}

	// SC-1: Error should contain budget amounts and hint
	errMsg := err.Error()
	if !strings.Contains(errMsg, "2143") && !strings.Contains(errMsg, "2,143") {
		t.Errorf("error should contain spent amount ($2,143): %s", errMsg)
	}
	if !strings.Contains(errMsg, "2000") && !strings.Contains(errMsg, "2,000") {
		t.Errorf("error should contain limit amount ($2,000): %s", errMsg)
	}
	if !strings.Contains(errMsg, "--ignore-budget") {
		t.Errorf("error should contain '--ignore-budget' hint: %s", errMsg)
	}

	// SC-7: No phases should have executed
	if mockTurn.CallCount() > 0 {
		t.Errorf("no phases should execute when over budget, but %d turns were executed", mockTurn.CallCount())
	}
}

// ============================================================================
// SC-2: Executor proceeds when over budget with IgnoreBudget=true
// ============================================================================

func TestBudgetCheck_ProceedsWithIgnoreBudget(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	globalDB := storage.NewTestGlobalDB(t)

	// Same over-budget setup
	projectID := t.TempDir()
	err := globalDB.SetBudget(db.CostBudget{
		ProjectID:             projectID,
		MonthlyLimitUSD:       2000.00,
		AlertThresholdPercent: 80,
		CurrentMonth:          currentMonth(),
		CurrentMonthSpent:     2143.00,
	})
	if err != nil {
		t.Fatalf("set budget: %v", err)
	}

	workflowID := "test-workflow"
	setupMinimalWorkflow(t, backend, workflowID)
	setupMinimalWorkflowGlobal(t, globalDB, workflowID)

	tk := task.NewProtoTask("TASK-001", "Test Task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	tk.WorkflowId = &workflowID
	tk.Execution = task.InitProtoExecutionState()
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	mockTurn := NewMockTurnExecutor(`{"status": "complete", "summary": "Done"}`)
	we := NewWorkflowExecutor(
		backend,
		backend.DB(),
		globalDB,
		&config.Config{Model: "sonnet"},
		projectID,
		WithWorkflowTurnExecutor(mockTurn),
	)

	// Run WITH --ignore-budget
	_, err = we.Run(context.Background(), workflowID, WorkflowRunOptions{
		ContextType:  ContextTask,
		TaskID:       "TASK-001",
		IgnoreBudget: true,
	})

	// Should NOT get a budget error (may get other errors from minimal setup, that's OK)
	if err != nil && strings.Contains(err.Error(), "budget") {
		t.Errorf("should not get budget error with IgnoreBudget=true, got: %v", err)
	}

	// Phases should have been attempted
	if mockTurn.CallCount() == 0 {
		t.Error("phases should execute when IgnoreBudget=true, but no turns were executed")
	}
}

// ============================================================================
// SC-3: Warning logged at alert threshold, execution proceeds
// ============================================================================

func TestBudgetCheck_WarnsAtAlertThreshold(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	globalDB := storage.NewTestGlobalDB(t)

	// Budget at 87%: $1,740 of $2,000 (above 80% alert threshold, below limit)
	projectID := t.TempDir()
	err := globalDB.SetBudget(db.CostBudget{
		ProjectID:             projectID,
		MonthlyLimitUSD:       2000.00,
		AlertThresholdPercent: 80,
		CurrentMonth:          currentMonth(),
		CurrentMonthSpent:     1740.00,
	})
	if err != nil {
		t.Fatalf("set budget: %v", err)
	}

	workflowID := "test-workflow"
	setupMinimalWorkflow(t, backend, workflowID)
	setupMinimalWorkflowGlobal(t, globalDB, workflowID)

	tk := task.NewProtoTask("TASK-001", "Test Task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	tk.WorkflowId = &workflowID
	tk.Execution = task.InitProtoExecutionState()
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Capture log output to verify warning
	var logOutput budgetLogBuffer
	logger := slog.New(slog.NewTextHandler(&logOutput, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	}))

	mockTurn := NewMockTurnExecutor(`{"status": "complete", "summary": "Done"}`)
	we := NewWorkflowExecutor(
		backend,
		backend.DB(),
		globalDB,
		&config.Config{Model: "sonnet"},
		projectID,
		WithWorkflowTurnExecutor(mockTurn),
		WithWorkflowLogger(logger),
	)

	_, err = we.Run(context.Background(), workflowID, WorkflowRunOptions{
		ContextType: ContextTask,
		TaskID:      "TASK-001",
	})

	// Should NOT return budget error (under limit)
	if err != nil && strings.Contains(err.Error(), "budget") {
		t.Errorf("should not get budget error when under limit, got: %v", err)
	}

	// Should have logged a warning about approaching budget limit
	logs := logOutput.String()
	if !strings.Contains(strings.ToLower(logs), "budget") {
		t.Errorf("expected budget warning in logs, got: %s", logs)
	}

	// Execution should proceed
	if mockTurn.CallCount() == 0 {
		t.Error("phases should execute when under budget limit, but no turns were executed")
	}
}

// ============================================================================
// SC-4: No enforcement when no budget configured
// ============================================================================

func TestBudgetCheck_NoBudgetConfigured(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	globalDB := storage.NewTestGlobalDB(t)

	// No budget set for project
	projectID := t.TempDir()

	workflowID := "test-workflow"
	setupMinimalWorkflow(t, backend, workflowID)
	setupMinimalWorkflowGlobal(t, globalDB, workflowID)

	tk := task.NewProtoTask("TASK-001", "Test Task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	tk.WorkflowId = &workflowID
	tk.Execution = task.InitProtoExecutionState()
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Capture logs to verify NO budget messages
	var logOutput budgetLogBuffer
	logger := slog.New(slog.NewTextHandler(&logOutput, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	mockTurn := NewMockTurnExecutor(`{"status": "complete", "summary": "Done"}`)
	we := NewWorkflowExecutor(
		backend,
		backend.DB(),
		globalDB,
		&config.Config{Model: "sonnet"},
		projectID,
		WithWorkflowTurnExecutor(mockTurn),
		WithWorkflowLogger(logger),
	)

	_, err := we.Run(context.Background(), workflowID, WorkflowRunOptions{
		ContextType: ContextTask,
		TaskID:      "TASK-001",
	})

	// Should not return any budget-related error
	if err != nil && strings.Contains(err.Error(), "budget") {
		t.Errorf("should not get budget error when no budget configured, got: %v", err)
	}

	// No budget-related messages in logs
	logs := logOutput.String()
	if strings.Contains(strings.ToLower(logs), "budget") {
		t.Errorf("should not have budget-related log messages when no budget configured, got: %s", logs)
	}

	// Execution should proceed
	if mockTurn.CallCount() == 0 {
		t.Error("phases should execute when no budget configured")
	}
}

// ============================================================================
// SC-5: IgnoreBudget field exists on WorkflowRunOptions
// ============================================================================

func TestBudgetCheck_IgnoreBudgetFieldExists(t *testing.T) {
	t.Parallel()

	// This test verifies at compile time that IgnoreBudget exists on WorkflowRunOptions.
	// If the field doesn't exist, this test won't compile.
	opts := WorkflowRunOptions{
		IgnoreBudget: true,
	}
	if !opts.IgnoreBudget {
		t.Error("IgnoreBudget should be true when set")
	}

	opts2 := WorkflowRunOptions{}
	if opts2.IgnoreBudget {
		t.Error("IgnoreBudget should default to false")
	}
}

// ============================================================================
// Failure Mode: GlobalDB is nil → skip budget check entirely
// ============================================================================

func TestBudgetCheck_GlobalDBNil(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	workflowID := "test-workflow"
	setupMinimalWorkflow(t, backend, workflowID)

	tk := task.NewProtoTask("TASK-001", "Test Task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	tk.WorkflowId = &workflowID
	tk.Execution = task.InitProtoExecutionState()
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	mockTurn := NewMockTurnExecutor(`{"status": "complete", "summary": "Done"}`)
	we := NewWorkflowExecutor(
		backend,
		backend.DB(),
		testGlobalDBFrom(backend),
		&config.Config{Model: "sonnet"},
		t.TempDir(),
		WithWorkflowTurnExecutor(mockTurn),
	)
	we.globalDB = nil // Explicitly nil

	_, err := we.Run(context.Background(), workflowID, WorkflowRunOptions{
		ContextType: ContextTask,
		TaskID:      "TASK-001",
	})

	// Should not fail due to budget (nil globalDB = no budget check)
	if err != nil && strings.Contains(err.Error(), "budget") {
		t.Errorf("should not get budget error when globalDB is nil, got: %v", err)
	}
}

// ============================================================================
// Failure Mode: GetBudgetStatus returns DB error → log warning, proceed
// ============================================================================

func TestBudgetCheck_DBErrorProceeds(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	globalDB := storage.NewTestGlobalDB(t)

	// Use a project ID that will cause GetBudgetStatus to return nil, nil
	// (no budget row exists). For a real DB error we'd need to corrupt the DB,
	// but the important behavior is: errors during budget check don't block execution.
	// The implementation should handle GetBudgetStatus errors gracefully.
	projectID := t.TempDir()

	workflowID := "test-workflow"
	setupMinimalWorkflow(t, backend, workflowID)
	setupMinimalWorkflowGlobal(t, globalDB, workflowID)

	tk := task.NewProtoTask("TASK-001", "Test Task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	tk.WorkflowId = &workflowID
	tk.Execution = task.InitProtoExecutionState()
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Capture log output to verify warning on error
	var logOutput budgetLogBuffer
	logger := slog.New(slog.NewTextHandler(&logOutput, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	}))

	mockTurn := NewMockTurnExecutor(`{"status": "complete", "summary": "Done"}`)
	we := NewWorkflowExecutor(
		backend,
		backend.DB(),
		globalDB,
		&config.Config{Model: "sonnet"},
		projectID,
		WithWorkflowTurnExecutor(mockTurn),
		WithWorkflowLogger(logger),
	)

	// Close the globalDB to force a DB error on GetBudgetStatus
	_ = globalDB.Close()

	_, err := we.Run(context.Background(), workflowID, WorkflowRunOptions{
		ContextType: ContextTask,
		TaskID:      "TASK-001",
	})

	// Should NOT fail due to budget check error (best-effort)
	if err != nil && strings.Contains(err.Error(), "budget") {
		t.Errorf("budget DB error should not block execution, got: %v", err)
	}
}

// ============================================================================
// Failure Mode: Budget spent == limit → NOT over-budget (strict >)
// ============================================================================

func TestBudgetCheck_SpentEqualsLimit_NotOverBudget(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	globalDB := storage.NewTestGlobalDB(t)

	// Spent exactly equals limit: $2,000 / $2,000
	projectID := t.TempDir()
	err := globalDB.SetBudget(db.CostBudget{
		ProjectID:             projectID,
		MonthlyLimitUSD:       2000.00,
		AlertThresholdPercent: 80,
		CurrentMonth:          currentMonth(),
		CurrentMonthSpent:     2000.00, // Exactly at limit
	})
	if err != nil {
		t.Fatalf("set budget: %v", err)
	}

	workflowID := "test-workflow"
	setupMinimalWorkflow(t, backend, workflowID)
	setupMinimalWorkflowGlobal(t, globalDB, workflowID)

	tk := task.NewProtoTask("TASK-001", "Test Task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	tk.WorkflowId = &workflowID
	tk.Execution = task.InitProtoExecutionState()
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	mockTurn := NewMockTurnExecutor(`{"status": "complete", "summary": "Done"}`)
	we := NewWorkflowExecutor(
		backend,
		backend.DB(),
		globalDB,
		&config.Config{Model: "sonnet"},
		projectID,
		WithWorkflowTurnExecutor(mockTurn),
	)

	_, err = we.Run(context.Background(), workflowID, WorkflowRunOptions{
		ContextType: ContextTask,
		TaskID:      "TASK-001",
	})

	// spent == limit is NOT over budget (OverBudget uses strict >)
	// Should not get a budget exceeded error
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "budget exceeded") {
		t.Errorf("spent == limit should NOT be over-budget, got: %v", err)
	}
}

// ============================================================================
// Edge Case: Budget limit set to 0 → no enforcement
// ============================================================================

func TestBudgetCheck_LimitZero_NoEnforcement(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	globalDB := storage.NewTestGlobalDB(t)

	// Budget limit = 0 (disabled)
	projectID := t.TempDir()
	err := globalDB.SetBudget(db.CostBudget{
		ProjectID:             projectID,
		MonthlyLimitUSD:       0,
		AlertThresholdPercent: 80,
		CurrentMonth:          currentMonth(),
		CurrentMonthSpent:     5000.00, // High spend, but limit is 0
	})
	if err != nil {
		t.Fatalf("set budget: %v", err)
	}

	workflowID := "test-workflow"
	setupMinimalWorkflow(t, backend, workflowID)
	setupMinimalWorkflowGlobal(t, globalDB, workflowID)

	tk := task.NewProtoTask("TASK-001", "Test Task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	tk.WorkflowId = &workflowID
	tk.Execution = task.InitProtoExecutionState()
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	mockTurn := NewMockTurnExecutor(`{"status": "complete", "summary": "Done"}`)
	we := NewWorkflowExecutor(
		backend,
		backend.DB(),
		globalDB,
		&config.Config{Model: "sonnet"},
		projectID,
		WithWorkflowTurnExecutor(mockTurn),
	)

	_, err = we.Run(context.Background(), workflowID, WorkflowRunOptions{
		ContextType: ContextTask,
		TaskID:      "TASK-001",
	})

	// Limit=0 means no enforcement → should not get budget error
	if err != nil && strings.Contains(err.Error(), "budget") {
		t.Errorf("limit=0 should disable budget enforcement, got: %v", err)
	}
}

// ============================================================================
// Edge Case: Spend exactly at alert threshold → warning logged
// ============================================================================

func TestBudgetCheck_ExactlyAtAlertThreshold(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	globalDB := storage.NewTestGlobalDB(t)

	// Spend at exactly 80% of $2,000 = $1,600
	projectID := t.TempDir()
	err := globalDB.SetBudget(db.CostBudget{
		ProjectID:             projectID,
		MonthlyLimitUSD:       2000.00,
		AlertThresholdPercent: 80,
		CurrentMonth:          currentMonth(),
		CurrentMonthSpent:     1600.00, // Exactly 80%
	})
	if err != nil {
		t.Fatalf("set budget: %v", err)
	}

	workflowID := "test-workflow"
	setupMinimalWorkflow(t, backend, workflowID)
	setupMinimalWorkflowGlobal(t, globalDB, workflowID)

	tk := task.NewProtoTask("TASK-001", "Test Task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	tk.WorkflowId = &workflowID
	tk.Execution = task.InitProtoExecutionState()
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	var logOutput budgetLogBuffer
	logger := slog.New(slog.NewTextHandler(&logOutput, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	}))

	mockTurn := NewMockTurnExecutor(`{"status": "complete", "summary": "Done"}`)
	we := NewWorkflowExecutor(
		backend,
		backend.DB(),
		globalDB,
		&config.Config{Model: "sonnet"},
		projectID,
		WithWorkflowTurnExecutor(mockTurn),
		WithWorkflowLogger(logger),
	)

	_, err = we.Run(context.Background(), workflowID, WorkflowRunOptions{
		ContextType: ContextTask,
		TaskID:      "TASK-001",
	})

	// Should NOT return budget error (at threshold but not over limit)
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "budget exceeded") {
		t.Errorf("at-threshold should not block execution, got: %v", err)
	}

	// Should have logged a warning (AtAlertThreshold is >= threshold percentage)
	logs := logOutput.String()
	if !strings.Contains(strings.ToLower(logs), "budget") {
		t.Errorf("expected budget warning at exact threshold, got logs: %s", logs)
	}
}

// ============================================================================
// Edge Case: --ignore-budget with no budget configured → no effect
// ============================================================================

func TestBudgetCheck_IgnoreBudgetWithNoBudget(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	globalDB := storage.NewTestGlobalDB(t)

	// No budget configured
	projectID := t.TempDir()

	workflowID := "test-workflow"
	setupMinimalWorkflow(t, backend, workflowID)
	setupMinimalWorkflowGlobal(t, globalDB, workflowID)

	tk := task.NewProtoTask("TASK-001", "Test Task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	tk.WorkflowId = &workflowID
	tk.Execution = task.InitProtoExecutionState()
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	mockTurn := NewMockTurnExecutor(`{"status": "complete", "summary": "Done"}`)
	we := NewWorkflowExecutor(
		backend,
		backend.DB(),
		globalDB,
		&config.Config{Model: "sonnet"},
		projectID,
		WithWorkflowTurnExecutor(mockTurn),
	)

	_, err := we.Run(context.Background(), workflowID, WorkflowRunOptions{
		ContextType:  ContextTask,
		TaskID:       "TASK-001",
		IgnoreBudget: true, // Flag set but no budget exists
	})

	// Should not cause any budget-related errors or issues
	if err != nil && strings.Contains(err.Error(), "budget") {
		t.Errorf("--ignore-budget with no budget should have no effect, got: %v", err)
	}
}

// ============================================================================
// Test helpers
// ============================================================================

// budgetLogBuffer is a buffer for capturing slog output in budget tests.
type budgetLogBuffer struct {
	data []byte
}

func (b *budgetLogBuffer) Write(p []byte) (n int, err error) {
	b.data = append(b.data, p...)
	return len(p), nil
}

func (b *budgetLogBuffer) String() string {
	return string(b.data)
}

// currentMonth returns the current month in YYYY-MM format.
func currentMonth() string {
	return time.Now().UTC().Format("2006-01")
}

// setupMinimalWorkflow creates a workflow with a single implement phase.
func setupMinimalWorkflow(t *testing.T, backend storage.Backend, workflowID string) {
	t.Helper()

	wf := &db.Workflow{
		ID:          workflowID,
		Name:        "Test Workflow",
		Description: "Test",
	}
	if err := backend.SaveWorkflow(wf); err != nil {
		t.Fatalf("save workflow: %v", err)
	}

	// Phase template already seeded by NewTestBackend, just need workflow-phase link
	wfPhase := &db.WorkflowPhase{
		WorkflowID:      workflowID,
		PhaseTemplateID: "implement",
		Sequence:        1,
	}
	if err := backend.SaveWorkflowPhase(wfPhase); err != nil {
		t.Fatalf("save workflow phase: %v", err)
	}
}
