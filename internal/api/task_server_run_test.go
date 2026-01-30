// Package api provides HTTP API handlers for orc.
//
// TDD Tests for TASK-538: RunTask executor spawning
//
// These tests verify the RunTask RPC properly spawns an executor process,
// rather than just updating the task status.
//
// Success Criteria Coverage:
// - SC-1: Clicking Run button spawns executor process (TestRunTask_SpawnsExecutor)
// - SC-3: Task can be paused via Pause button while running (TestPauseTask_RunningTask)
// - SC-4: Task workflow_id validation before execution (TestRunTask_NoWorkflowId_ReturnsError)
// - SC-5: Existing resumeTask continues to work (covered by existing resumeTask tests)
//
// Edge Cases:
// - Task already running returns error
// - Task in COMPLETED status returns error
// - Task in FAILED status allows retry
// - Task in PAUSED status returns error (use resume instead)
// - Task blocked by dependencies returns error
// - Concurrent RunTask calls: second call fails
package api

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/storage"
)

// ============================================================================
// SC-4: Task workflow_id validation before execution
// ============================================================================

// TestRunTask_NoWorkflowId_ReturnsError verifies SC-4:
// RunTask returns error when task has no workflow_id set.
// Task status should NOT change - no partial state.
func TestRunTask_NoWorkflowId_ReturnsError(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	// Create a task without workflow_id
	task := &orcv1.Task{
		Id:         "TASK-001",
		Title:      "Task without workflow",
		Weight:     orcv1.TaskWeight_TASK_WEIGHT_MEDIUM,
		Status:     orcv1.TaskStatus_TASK_STATUS_PLANNED,
		WorkflowId: nil, // No workflow_id
	}

	if err := backend.SaveTask(task); err != nil {
		t.Fatalf("save task: %v", err)
	}

	server := NewTaskServer(backend, nil, nil, nil, "", nil, nil)

	req := connect.NewRequest(&orcv1.RunTaskRequest{TaskId: "TASK-001"})
	_, err := server.RunTask(context.Background(), req)

	// Must return error for missing workflow_id
	if err == nil {
		t.Fatal("expected error for task without workflow_id, got none")
	}

	connectErr, ok := err.(*connect.Error)
	if !ok {
		t.Fatalf("expected connect.Error, got %T", err)
	}

	if connectErr.Code() != connect.CodeFailedPrecondition {
		t.Errorf("expected CodeFailedPrecondition, got %v", connectErr.Code())
	}

	// Error message should mention workflow_id
	if !containsIgnoreCase(connectErr.Message(), "workflow") {
		t.Errorf("error message should mention workflow, got: %s", connectErr.Message())
	}

	// Verify task status unchanged (no partial state)
	loaded, err := backend.LoadTask("TASK-001")
	if err != nil {
		t.Fatalf("reload task: %v", err)
	}

	if loaded.Status != orcv1.TaskStatus_TASK_STATUS_PLANNED {
		t.Errorf("task status should be unchanged (PLANNED), got %v", loaded.Status)
	}
}

// TestRunTask_EmptyWorkflowId_ReturnsError verifies empty string workflow_id also fails.
func TestRunTask_EmptyWorkflowId_ReturnsError(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	emptyWorkflow := ""
	task := &orcv1.Task{
		Id:         "TASK-001",
		Title:      "Task with empty workflow",
		Weight:     orcv1.TaskWeight_TASK_WEIGHT_MEDIUM,
		Status:     orcv1.TaskStatus_TASK_STATUS_PLANNED,
		WorkflowId: &emptyWorkflow, // Empty string
	}

	if err := backend.SaveTask(task); err != nil {
		t.Fatalf("save task: %v", err)
	}

	server := NewTaskServer(backend, nil, nil, nil, "", nil, nil)

	req := connect.NewRequest(&orcv1.RunTaskRequest{TaskId: "TASK-001"})
	_, err := server.RunTask(context.Background(), req)

	if err == nil {
		t.Fatal("expected error for task with empty workflow_id, got none")
	}

	// Verify task status unchanged
	loaded, err := backend.LoadTask("TASK-001")
	if err != nil {
		t.Fatalf("reload task: %v", err)
	}

	if loaded.Status != orcv1.TaskStatus_TASK_STATUS_PLANNED {
		t.Errorf("task status should be unchanged (PLANNED), got %v", loaded.Status)
	}
}

// ============================================================================
// SC-1: Clicking Run button spawns executor process
// ============================================================================

// TestRunTask_SpawnsExecutor verifies SC-1:
// RunTask calls the executor callback to spawn an actual executor.
// This test verifies the callback mechanism exists and is invoked.
func TestRunTask_SpawnsExecutor(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	workflowID := "medium"
	task := &orcv1.Task{
		Id:         "TASK-001",
		Title:      "Ready to run",
		Weight:     orcv1.TaskWeight_TASK_WEIGHT_MEDIUM,
		Status:     orcv1.TaskStatus_TASK_STATUS_PLANNED,
		WorkflowId: &workflowID,
	}

	if err := backend.SaveTask(task); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Track if executor callback was invoked
	var executorCalled atomic.Bool
	var calledTaskID string
	var mu sync.Mutex

	// Create mock executor callback
	mockExecutor := func(taskID, projectID string) error {
		mu.Lock()
		calledTaskID = taskID
		mu.Unlock()
		executorCalled.Store(true)
		return nil
	}

	// NOTE: This test will fail to compile/run until taskServer is updated
	// to accept an executor callback. This is intentional TDD.
	server := NewTaskServerWithExecutor(backend, nil, nil, nil, "", nil, nil, mockExecutor)

	req := connect.NewRequest(&orcv1.RunTaskRequest{TaskId: "TASK-001"})
	resp, err := server.RunTask(context.Background(), req)

	if err != nil {
		t.Fatalf("RunTask failed: %v", err)
	}

	// Executor callback must have been called
	if !executorCalled.Load() {
		t.Fatal("executor callback was not invoked - task would appear running but not execute")
	}

	// Verify correct task ID passed
	mu.Lock()
	if calledTaskID != "TASK-001" {
		t.Errorf("executor called with wrong task ID: %q, want %q", calledTaskID, "TASK-001")
	}
	mu.Unlock()

	// Verify response contains updated task
	if resp.Msg.Task == nil {
		t.Fatal("response task is nil")
	}

	if resp.Msg.Task.Status != orcv1.TaskStatus_TASK_STATUS_RUNNING {
		t.Errorf("task status = %v, want RUNNING", resp.Msg.Task.Status)
	}
}

// TestRunTask_ExecutorFailure_RevertsStatus verifies executor failure handling:
// If executor callback returns error, task status should revert to previous state.
func TestRunTask_ExecutorFailure_RevertsStatus(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	workflowID := "medium"
	task := &orcv1.Task{
		Id:         "TASK-001",
		Title:      "Executor will fail",
		Weight:     orcv1.TaskWeight_TASK_WEIGHT_MEDIUM,
		Status:     orcv1.TaskStatus_TASK_STATUS_PLANNED,
		WorkflowId: &workflowID,
	}

	if err := backend.SaveTask(task); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Mock executor that always fails
	mockExecutor := func(taskID, projectID string) error {
		return errors.New("executor spawn failed: no Claude process available")
	}

	server := NewTaskServerWithExecutor(backend, nil, nil, nil, "", nil, nil, mockExecutor)

	req := connect.NewRequest(&orcv1.RunTaskRequest{TaskId: "TASK-001"})
	_, err := server.RunTask(context.Background(), req)

	// Should return error when executor fails
	if err == nil {
		t.Fatal("expected error when executor fails, got none")
	}

	// Task status should NOT be RUNNING (reverted or unchanged)
	loaded, err := backend.LoadTask("TASK-001")
	if err != nil {
		t.Fatalf("reload task: %v", err)
	}

	if loaded.Status == orcv1.TaskStatus_TASK_STATUS_RUNNING {
		t.Error("task status should not be RUNNING after executor failure")
	}
}

// ============================================================================
// Edge Case: Already Running
// ============================================================================

// TestRunTask_AlreadyRunning_ReturnsError verifies already-running check.
func TestRunTask_AlreadyRunning_ReturnsError(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	workflowID := "medium"
	task := &orcv1.Task{
		Id:         "TASK-001",
		Title:      "Already running",
		Weight:     orcv1.TaskWeight_TASK_WEIGHT_MEDIUM,
		Status:     orcv1.TaskStatus_TASK_STATUS_RUNNING, // Already running
		WorkflowId: &workflowID,
	}

	if err := backend.SaveTask(task); err != nil {
		t.Fatalf("save task: %v", err)
	}

	server := NewTaskServer(backend, nil, nil, nil, "", nil, nil)

	req := connect.NewRequest(&orcv1.RunTaskRequest{TaskId: "TASK-001"})
	_, err := server.RunTask(context.Background(), req)

	if err == nil {
		t.Fatal("expected error for already running task")
	}

	connectErr, ok := err.(*connect.Error)
	if !ok {
		t.Fatalf("expected connect.Error, got %T", err)
	}

	if connectErr.Code() != connect.CodeFailedPrecondition {
		t.Errorf("expected CodeFailedPrecondition, got %v", connectErr.Code())
	}
}

// ============================================================================
// Edge Case: Completed Task
// ============================================================================

// TestRunTask_CompletedTask_ReturnsError verifies completed tasks can't be run again.
func TestRunTask_CompletedTask_ReturnsError(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	workflowID := "medium"
	task := &orcv1.Task{
		Id:         "TASK-001",
		Title:      "Already completed",
		Weight:     orcv1.TaskWeight_TASK_WEIGHT_MEDIUM,
		Status:     orcv1.TaskStatus_TASK_STATUS_COMPLETED,
		WorkflowId: &workflowID,
	}

	if err := backend.SaveTask(task); err != nil {
		t.Fatalf("save task: %v", err)
	}

	server := NewTaskServer(backend, nil, nil, nil, "", nil, nil)

	req := connect.NewRequest(&orcv1.RunTaskRequest{TaskId: "TASK-001"})
	_, err := server.RunTask(context.Background(), req)

	if err == nil {
		t.Fatal("expected error for completed task")
	}

	connectErr, ok := err.(*connect.Error)
	if !ok {
		t.Fatalf("expected connect.Error, got %T", err)
	}

	if connectErr.Code() != connect.CodeFailedPrecondition {
		t.Errorf("expected CodeFailedPrecondition, got %v", connectErr.Code())
	}

	// Error message should indicate task is completed
	if !containsIgnoreCase(connectErr.Message(), "completed") && !containsIgnoreCase(connectErr.Message(), "already") {
		t.Errorf("error message should mention task is completed, got: %s", connectErr.Message())
	}
}

// ============================================================================
// Edge Case: Failed Task (Retry)
// ============================================================================

// TestRunTask_FailedTask_AllowsRetry verifies failed tasks can be re-run.
func TestRunTask_FailedTask_AllowsRetry(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	workflowID := "medium"
	task := &orcv1.Task{
		Id:         "TASK-001",
		Title:      "Previously failed",
		Weight:     orcv1.TaskWeight_TASK_WEIGHT_MEDIUM,
		Status:     orcv1.TaskStatus_TASK_STATUS_FAILED, // Failed status
		WorkflowId: &workflowID,
	}

	if err := backend.SaveTask(task); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Track executor invocation
	var executorCalled atomic.Bool
	mockExecutor := func(taskID, projectID string) error {
		executorCalled.Store(true)
		return nil
	}

	server := NewTaskServerWithExecutor(backend, nil, nil, nil, "", nil, nil, mockExecutor)

	req := connect.NewRequest(&orcv1.RunTaskRequest{TaskId: "TASK-001"})
	_, err := server.RunTask(context.Background(), req)

	// Failed tasks should be allowed to retry
	if err != nil {
		t.Fatalf("RunTask should allow retrying failed task: %v", err)
	}

	if !executorCalled.Load() {
		t.Error("executor should be called for failed task retry")
	}
}

// ============================================================================
// Edge Case: Paused Task
// ============================================================================

// TestRunTask_PausedTask_ReturnsError verifies paused tasks should use resume instead.
func TestRunTask_PausedTask_ReturnsError(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	workflowID := "medium"
	task := &orcv1.Task{
		Id:         "TASK-001",
		Title:      "Paused task",
		Weight:     orcv1.TaskWeight_TASK_WEIGHT_MEDIUM,
		Status:     orcv1.TaskStatus_TASK_STATUS_PAUSED,
		WorkflowId: &workflowID,
	}

	if err := backend.SaveTask(task); err != nil {
		t.Fatalf("save task: %v", err)
	}

	server := NewTaskServer(backend, nil, nil, nil, "", nil, nil)

	req := connect.NewRequest(&orcv1.RunTaskRequest{TaskId: "TASK-001"})
	_, err := server.RunTask(context.Background(), req)

	// Paused tasks should not be "run" - they should be "resumed"
	if err == nil {
		t.Fatal("expected error for paused task (should use resume instead)")
	}

	connectErr, ok := err.(*connect.Error)
	if !ok {
		t.Fatalf("expected connect.Error, got %T", err)
	}

	if connectErr.Code() != connect.CodeFailedPrecondition {
		t.Errorf("expected CodeFailedPrecondition, got %v", connectErr.Code())
	}

	// Error message should suggest using resume
	if !containsIgnoreCase(connectErr.Message(), "resume") && !containsIgnoreCase(connectErr.Message(), "paused") {
		t.Errorf("error message should mention resume or paused, got: %s", connectErr.Message())
	}
}

// ============================================================================
// Edge Case: Blocked Task
// ============================================================================

// TestRunTask_BlockedTask_ReturnsError verifies blocked tasks can't be run.
func TestRunTask_BlockedTask_ReturnsError(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	workflowID := "medium"
	task := &orcv1.Task{
		Id:         "TASK-001",
		Title:      "Blocked task",
		Weight:     orcv1.TaskWeight_TASK_WEIGHT_MEDIUM,
		Status:     orcv1.TaskStatus_TASK_STATUS_BLOCKED,
		WorkflowId: &workflowID,
	}

	if err := backend.SaveTask(task); err != nil {
		t.Fatalf("save task: %v", err)
	}

	server := NewTaskServer(backend, nil, nil, nil, "", nil, nil)

	req := connect.NewRequest(&orcv1.RunTaskRequest{TaskId: "TASK-001"})
	_, err := server.RunTask(context.Background(), req)

	if err == nil {
		t.Fatal("expected error for blocked task")
	}

	connectErr, ok := err.(*connect.Error)
	if !ok {
		t.Fatalf("expected connect.Error, got %T", err)
	}

	if connectErr.Code() != connect.CodeFailedPrecondition {
		t.Errorf("expected CodeFailedPrecondition, got %v", connectErr.Code())
	}
}

// ============================================================================
// Edge Case: Blocked by Dependency
// ============================================================================

// TestRunTask_BlockedByDependency_ReturnsError verifies dependency blocking.
// This tests a PLANNED task that has unmet blockers.
func TestRunTask_BlockedByDependency_ReturnsError(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	workflowID := "medium"

	// Create blocking task (not completed)
	blocker := &orcv1.Task{
		Id:         "TASK-001",
		Title:      "Blocker task",
		Weight:     orcv1.TaskWeight_TASK_WEIGHT_SMALL,
		Status:     orcv1.TaskStatus_TASK_STATUS_PLANNED, // Not completed
		WorkflowId: &workflowID,
	}

	// Create blocked task
	blockedTask := &orcv1.Task{
		Id:         "TASK-002",
		Title:      "Blocked task",
		Weight:     orcv1.TaskWeight_TASK_WEIGHT_MEDIUM,
		Status:     orcv1.TaskStatus_TASK_STATUS_PLANNED,
		WorkflowId: &workflowID,
		BlockedBy:  []string{"TASK-001"}, // Blocked by TASK-001
	}

	if err := backend.SaveTask(blocker); err != nil {
		t.Fatalf("save blocker: %v", err)
	}
	if err := backend.SaveTask(blockedTask); err != nil {
		t.Fatalf("save blocked task: %v", err)
	}

	server := NewTaskServer(backend, nil, nil, nil, "", nil, nil)

	// Try to run the blocked task
	req := connect.NewRequest(&orcv1.RunTaskRequest{TaskId: "TASK-002"})
	_, err := server.RunTask(context.Background(), req)

	if err == nil {
		t.Fatal("expected error for task blocked by dependency")
	}

	connectErr, ok := err.(*connect.Error)
	if !ok {
		t.Fatalf("expected connect.Error, got %T", err)
	}

	if connectErr.Code() != connect.CodeFailedPrecondition {
		t.Errorf("expected CodeFailedPrecondition, got %v", connectErr.Code())
	}

	// Error should mention what's blocking
	if !containsIgnoreCase(connectErr.Message(), "blocked") && !containsIgnoreCase(connectErr.Message(), "TASK-001") {
		t.Errorf("error message should mention blocker, got: %s", connectErr.Message())
	}
}

// TestRunTask_DependencyCompleted_AllowsRun verifies task can run when blockers are complete.
func TestRunTask_DependencyCompleted_AllowsRun(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	workflowID := "medium"

	// Create blocking task (COMPLETED)
	blocker := &orcv1.Task{
		Id:         "TASK-001",
		Title:      "Blocker task",
		Weight:     orcv1.TaskWeight_TASK_WEIGHT_SMALL,
		Status:     orcv1.TaskStatus_TASK_STATUS_COMPLETED, // Completed!
		WorkflowId: &workflowID,
	}

	// Create blocked task
	blockedTask := &orcv1.Task{
		Id:         "TASK-002",
		Title:      "Was blocked, now ready",
		Weight:     orcv1.TaskWeight_TASK_WEIGHT_MEDIUM,
		Status:     orcv1.TaskStatus_TASK_STATUS_PLANNED,
		WorkflowId: &workflowID,
		BlockedBy:  []string{"TASK-001"},
	}

	if err := backend.SaveTask(blocker); err != nil {
		t.Fatalf("save blocker: %v", err)
	}
	if err := backend.SaveTask(blockedTask); err != nil {
		t.Fatalf("save blocked task: %v", err)
	}

	var executorCalled atomic.Bool
	mockExecutor := func(taskID, projectID string) error {
		executorCalled.Store(true)
		return nil
	}

	server := NewTaskServerWithExecutor(backend, nil, nil, nil, "", nil, nil, mockExecutor)

	// Should be able to run now
	req := connect.NewRequest(&orcv1.RunTaskRequest{TaskId: "TASK-002"})
	_, err := server.RunTask(context.Background(), req)

	if err != nil {
		t.Fatalf("RunTask should succeed when blocker is completed: %v", err)
	}

	if !executorCalled.Load() {
		t.Error("executor should be called when dependencies are met")
	}
}

// ============================================================================
// Edge Case: Concurrent Calls
// ============================================================================

// TestRunTask_ConcurrentCalls_SecondFails verifies concurrent run protection.
func TestRunTask_ConcurrentCalls_SecondFails(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	workflowID := "medium"
	task := &orcv1.Task{
		Id:         "TASK-001",
		Title:      "Concurrent test",
		Weight:     orcv1.TaskWeight_TASK_WEIGHT_MEDIUM,
		Status:     orcv1.TaskStatus_TASK_STATUS_PLANNED,
		WorkflowId: &workflowID,
	}

	if err := backend.SaveTask(task); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Create a slow executor to simulate concurrent calls
	executorStarted := make(chan struct{})
	executorDone := make(chan struct{})

	mockExecutor := func(taskID, projectID string) error {
		close(executorStarted) // Signal we started
		<-executorDone        // Wait until test says to finish
		return nil
	}

	server := NewTaskServerWithExecutor(backend, nil, nil, nil, "", nil, nil, mockExecutor)

	// First call - should succeed and start executor
	var firstErr error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		req := connect.NewRequest(&orcv1.RunTaskRequest{TaskId: "TASK-001"})
		_, firstErr = server.RunTask(context.Background(), req)
	}()

	// Wait for first executor to start
	select {
	case <-executorStarted:
		// Good, first call is in progress
	case <-time.After(2 * time.Second):
		t.Fatal("executor didn't start within timeout")
	}

	// Second call while first is running - should fail
	req := connect.NewRequest(&orcv1.RunTaskRequest{TaskId: "TASK-001"})
	_, secondErr := server.RunTask(context.Background(), req)

	// Let first call finish
	close(executorDone)
	wg.Wait()

	// First call should succeed
	if firstErr != nil {
		t.Errorf("first RunTask call should succeed: %v", firstErr)
	}

	// Second call should fail with "already running"
	if secondErr == nil {
		t.Fatal("second concurrent RunTask call should fail")
	}

	connectErr, ok := secondErr.(*connect.Error)
	if !ok {
		t.Fatalf("expected connect.Error, got %T", secondErr)
	}

	if connectErr.Code() != connect.CodeFailedPrecondition {
		t.Errorf("expected CodeFailedPrecondition, got %v", connectErr.Code())
	}
}

// ============================================================================
// SC-3: Pause running task
// ============================================================================

// TestPauseTask_RunningTask verifies SC-3: pause cancels the executor context.
// Note: The actual cancellation is handled by the Server's runningTasks map.
// This test verifies the status transition.
func TestPauseTask_RunningTask(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	workflowID := "medium"
	task := &orcv1.Task{
		Id:         "TASK-001",
		Title:      "Running task",
		Weight:     orcv1.TaskWeight_TASK_WEIGHT_MEDIUM,
		Status:     orcv1.TaskStatus_TASK_STATUS_RUNNING,
		WorkflowId: &workflowID,
	}

	if err := backend.SaveTask(task); err != nil {
		t.Fatalf("save task: %v", err)
	}

	server := NewTaskServer(backend, nil, nil, nil, "", nil, nil)

	req := connect.NewRequest(&orcv1.PauseTaskRequest{TaskId: "TASK-001"})
	resp, err := server.PauseTask(context.Background(), req)

	if err != nil {
		t.Fatalf("PauseTask failed: %v", err)
	}

	if resp.Msg.Task == nil {
		t.Fatal("response task is nil")
	}

	if resp.Msg.Task.Status != orcv1.TaskStatus_TASK_STATUS_PAUSED {
		t.Errorf("task status = %v, want PAUSED", resp.Msg.Task.Status)
	}
}

// ============================================================================
// Task Not Found
// ============================================================================

// TestRunTask_NotFound verifies error for non-existent task.
func TestRunTask_NotFound(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	server := NewTaskServer(backend, nil, nil, nil, "", nil, nil)

	req := connect.NewRequest(&orcv1.RunTaskRequest{TaskId: "TASK-NONEXISTENT"})
	_, err := server.RunTask(context.Background(), req)

	if err == nil {
		t.Fatal("expected error for non-existent task")
	}

	connectErr, ok := err.(*connect.Error)
	if !ok {
		t.Fatalf("expected connect.Error, got %T", err)
	}

	if connectErr.Code() != connect.CodeNotFound {
		t.Errorf("expected CodeNotFound, got %v", connectErr.Code())
	}
}

// TestRunTask_EmptyId verifies error for empty task ID.
func TestRunTask_EmptyId(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	server := NewTaskServer(backend, nil, nil, nil, "", nil, nil)

	req := connect.NewRequest(&orcv1.RunTaskRequest{TaskId: ""})
	_, err := server.RunTask(context.Background(), req)

	if err == nil {
		t.Fatal("expected error for empty task ID")
	}

	connectErr, ok := err.(*connect.Error)
	if !ok {
		t.Fatalf("expected connect.Error, got %T", err)
	}

	if connectErr.Code() != connect.CodeInvalidArgument {
		t.Errorf("expected CodeInvalidArgument, got %v", connectErr.Code())
	}
}

// ============================================================================
// Helper Functions
// ============================================================================

// containsIgnoreCase checks if s contains substr (case-insensitive).
func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			len(substr) == 0 ||
			containsIgnoreCaseSlow(s, substr))
}

func containsIgnoreCaseSlow(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if equalFold(s[i:i+len(substr)], substr) {
			return true
		}
	}
	return false
}

func equalFold(s, t string) bool {
	if len(s) != len(t) {
		return false
	}
	for i := 0; i < len(s); i++ {
		sr := s[i]
		tr := t[i]
		// ASCII lowercase
		if 'A' <= sr && sr <= 'Z' {
			sr = sr + 'a' - 'A'
		}
		if 'A' <= tr && tr <= 'Z' {
			tr = tr + 'a' - 'A'
		}
		if sr != tr {
			return false
		}
	}
	return true
}

// Note: NewTaskServerWithExecutor is now implemented in task_server.go
