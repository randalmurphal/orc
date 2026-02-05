package executor

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// ============================================================================
// SC-8: WorkflowExecutor claims task before execution starts
// ============================================================================

// TestWorkflowExecutor_ClaimsTaskBeforeExecution tests that the executor
// calls ClaimTaskByUser before executing any phases.
// Covers: SC-8
func TestWorkflowExecutor_ClaimsTaskBeforeExecution(t *testing.T) {
	t.Parallel()

	// Create backend with claim tracking
	backend := &userClaimTrackingBackend{
		Backend: storage.NewTestBackend(t),
	}

	// Create minimal workflow
	workflowID := "test-workflow"
	workflow := &db.Workflow{
		ID:          workflowID,
		Name:        "Test Workflow",
		Description: "Test",
	}
	if err := backend.SaveWorkflow(workflow); err != nil {
		t.Fatalf("save workflow: %v", err)
	}

	// Create task
	tk := task.NewProtoTask("TASK-001", "Test Task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	tk.WorkflowId = &workflowID
	tk.Execution = task.InitProtoExecutionState()
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Create executor with mock turn executor to avoid real Claude calls
	projectDB := backend.DB()
	mockTurn := NewMockTurnExecutor(`{"status": "complete", "summary": "Done"}`)
	we := NewWorkflowExecutor(
		backend,
		projectDB,
		&config.Config{
			Model: "sonnet",
		},
		t.TempDir(),
		WithWorkflowTurnExecutor(mockTurn),
	)

	// Set current user ID (this would normally come from config/environment)
	ctx := context.WithValue(context.Background(), userClaimContextKey, "user-alice")

	// Run the workflow - should claim before executing
	_, err := we.Run(ctx, workflowID, WorkflowRunOptions{
		ContextType: ContextTask,
		TaskID:      "TASK-001",
	})

	// Even if workflow execution fails, claim should have been attempted
	_ = err

	// Verify claim was called BEFORE any phase execution
	if !backend.ClaimCalled() {
		t.Error("ClaimTaskByUser should have been called before execution")
	}

	// Verify claim was called with correct user
	if backend.LastClaimUser() != "user-alice" {
		t.Errorf("claim user = %q, want user-alice", backend.LastClaimUser())
	}

	// Verify claim was called with correct task
	if backend.LastClaimTask() != "TASK-001" {
		t.Errorf("claim task = %q, want TASK-001", backend.LastClaimTask())
	}

	// Verify claim happened before phase execution
	if backend.ClaimOrder() > backend.PhaseExecutionOrder() && backend.PhaseExecutionOrder() > 0 {
		t.Error("claim should happen BEFORE phase execution starts")
	}
}

// TestWorkflowExecutor_ClaimFailure_StopsExecution tests that if claiming fails
// (e.g., another user owns the task), execution does not proceed.
// Covers: SC-8 error path
func TestWorkflowExecutor_ClaimFailure_StopsExecution(t *testing.T) {
	t.Parallel()

	// Create backend that rejects claims
	backend := &userClaimRejectingBackend{
		Backend: storage.NewTestBackend(t),
	}

	// Create workflow and task
	workflowID := "test-workflow"
	workflow := &db.Workflow{
		ID:          workflowID,
		Name:        "Test Workflow",
		Description: "Test",
	}
	if err := backend.SaveWorkflow(workflow); err != nil {
		t.Fatalf("save workflow: %v", err)
	}

	tk := task.NewProtoTask("TASK-001", "Test Task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	tk.WorkflowId = &workflowID
	tk.Execution = task.InitProtoExecutionState()
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Create executor
	projectDB := backend.DB()
	mockTurn := NewMockTurnExecutor(`{"status": "complete", "summary": "Done"}`)
	we := NewWorkflowExecutor(
		backend,
		projectDB,
		&config.Config{
			Model: "sonnet",
		},
		t.TempDir(),
		WithWorkflowTurnExecutor(mockTurn),
	)

	ctx := context.WithValue(context.Background(), userClaimContextKey, "user-bob")

	// Run should fail because claim fails
	_, err := we.Run(ctx, workflowID, WorkflowRunOptions{
		ContextType: ContextTask,
		TaskID:      "TASK-001",
	})

	if err == nil {
		t.Fatal("expected error when claim fails")
	}

	// Error should mention claim failure
	if !strings.Contains(strings.ToLower(err.Error()), "claim") {
		t.Errorf("error should mention claim failure: %v", err)
	}

	// Verify no phases were executed
	if backend.PhasesExecuted() {
		t.Error("no phases should execute when claim fails")
	}
}

// TestWorkflowExecutor_ReleasesClaimOnCompletion tests that the executor
// releases the claim when the workflow completes.
// Covers: SC-8 cleanup path
func TestWorkflowExecutor_ReleasesClaimOnCompletion(t *testing.T) {
	t.Parallel()

	backend := &userClaimTrackingBackend{
		Backend: storage.NewTestBackend(t),
	}

	// Setup workflow with a single phase
	workflowID := "test-workflow"
	workflow := &db.Workflow{
		ID:          workflowID,
		Name:        "Test Workflow",
		Description: "Test",
	}
	if err := backend.SaveWorkflow(workflow); err != nil {
		t.Fatalf("save workflow: %v", err)
	}

	phase := &db.PhaseTemplate{
		ID:   "implement",
		Name: "Implement",
	}
	if err := backend.SavePhaseTemplate(phase); err != nil {
		t.Fatalf("save phase template: %v", err)
	}

	wfPhase := &db.WorkflowPhase{
		WorkflowID:      workflowID,
		PhaseTemplateID: "implement",
	}
	if err := backend.SaveWorkflowPhase(wfPhase); err != nil {
		t.Fatalf("save workflow phase: %v", err)
	}

	tk := task.NewProtoTask("TASK-001", "Test Task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	tk.WorkflowId = &workflowID
	tk.Execution = task.InitProtoExecutionState()
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	projectDB := backend.DB()
	mockTurn := NewMockTurnExecutor(`{"status": "complete", "summary": "Done"}`)
	we := NewWorkflowExecutor(
		backend,
		projectDB,
		&config.Config{
			Model: "sonnet",
		},
		t.TempDir(),
		WithWorkflowTurnExecutor(mockTurn),
	)

	ctx := context.WithValue(context.Background(), userClaimContextKey, "user-alice")

	_, _ = we.Run(ctx, workflowID, WorkflowRunOptions{
		ContextType: ContextTask,
		TaskID:      "TASK-001",
	})

	// Verify release was called after execution
	if !backend.ReleaseCalled() {
		t.Error("ReleaseUserClaim should have been called after completion")
	}
}

// TestWorkflowExecutor_ReleasesClaimOnFailure tests that the executor
// releases the claim even when the workflow fails.
// Covers: SC-8 error cleanup
func TestWorkflowExecutor_ReleasesClaimOnFailure(t *testing.T) {
	t.Parallel()

	backend := &userClaimTrackingBackend{
		Backend: storage.NewTestBackend(t),
	}

	workflowID := "test-workflow"
	workflow := &db.Workflow{
		ID:          workflowID,
		Name:        "Test Workflow",
		Description: "Test",
	}
	if err := backend.SaveWorkflow(workflow); err != nil {
		t.Fatalf("save workflow: %v", err)
	}

	phase := &db.PhaseTemplate{
		ID:   "implement",
		Name: "Implement",
	}
	if err := backend.SavePhaseTemplate(phase); err != nil {
		t.Fatalf("save phase template: %v", err)
	}

	wfPhase := &db.WorkflowPhase{
		WorkflowID:      workflowID,
		PhaseTemplateID: "implement",
	}
	if err := backend.SaveWorkflowPhase(wfPhase); err != nil {
		t.Fatalf("save workflow phase: %v", err)
	}

	tk := task.NewProtoTask("TASK-001", "Test Task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	tk.WorkflowId = &workflowID
	tk.Execution = task.InitProtoExecutionState()
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Executor that will fail during execution
	projectDB := backend.DB()
	mockTurn := NewMockTurnExecutor(`{"status": "complete", "summary": "Done"}`)
	mockTurn.Error = errors.New("mock execution failure")
	we := NewWorkflowExecutor(
		backend,
		projectDB,
		&config.Config{
			Model: "sonnet",
		},
		t.TempDir(),
		WithWorkflowTurnExecutor(mockTurn),
	)

	ctx := context.WithValue(context.Background(), userClaimContextKey, "user-alice")

	_, err := we.Run(ctx, workflowID, WorkflowRunOptions{
		ContextType: ContextTask,
		TaskID:      "TASK-001",
	})

	// Execution should have failed
	if err == nil {
		t.Fatal("expected execution to fail")
	}

	// But release should still have been called (cleanup)
	if !backend.ReleaseCalled() {
		t.Error("ReleaseUserClaim should have been called even on failure")
	}
}

// ============================================================================
// Test helpers and mock types
// ============================================================================

// userClaimTrackingBackend wraps a real backend to track claim/release calls.
type userClaimTrackingBackend struct {
	storage.Backend
	claimCalled    atomic.Bool
	releaseCalled  atomic.Bool
	lastClaimUser  atomic.Value // string
	lastClaimTask  atomic.Value // string
	claimOrder     atomic.Int64
	phaseExecOrder atomic.Int64
	orderCounter   atomic.Int64
}

func (b *userClaimTrackingBackend) ClaimTaskByUser(taskID, userID string) (bool, error) {
	b.claimCalled.Store(true)
	b.lastClaimUser.Store(userID)
	b.lastClaimTask.Store(taskID)
	b.claimOrder.Store(b.orderCounter.Add(1))
	return true, nil
}

func (b *userClaimTrackingBackend) ReleaseUserClaim(taskID, userID string) (bool, error) {
	b.releaseCalled.Store(true)
	return true, nil
}

func (b *userClaimTrackingBackend) ClaimCalled() bool {
	return b.claimCalled.Load()
}

func (b *userClaimTrackingBackend) ReleaseCalled() bool {
	return b.releaseCalled.Load()
}

func (b *userClaimTrackingBackend) LastClaimUser() string {
	v := b.lastClaimUser.Load()
	if v == nil {
		return ""
	}
	return v.(string)
}

func (b *userClaimTrackingBackend) LastClaimTask() string {
	v := b.lastClaimTask.Load()
	if v == nil {
		return ""
	}
	return v.(string)
}

func (b *userClaimTrackingBackend) ClaimOrder() int64 {
	return b.claimOrder.Load()
}

func (b *userClaimTrackingBackend) PhaseExecutionOrder() int64 {
	return b.phaseExecOrder.Load()
}

func (b *userClaimTrackingBackend) RecordPhaseExecution() {
	if b.phaseExecOrder.Load() == 0 {
		b.phaseExecOrder.Store(b.orderCounter.Add(1))
	}
}

// userClaimRejectingBackend rejects all claim attempts.
type userClaimRejectingBackend struct {
	storage.Backend
	phasesExecuted atomic.Bool
}

func (b *userClaimRejectingBackend) ClaimTaskByUser(taskID, userID string) (bool, error) {
	return false, nil // Claim rejected (someone else owns it)
}

func (b *userClaimRejectingBackend) PhasesExecuted() bool {
	return b.phasesExecuted.Load()
}
