// Package api tests for AutofixComment endpoint.
package api

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/hosting"
	"github.com/randalmurphal/orc/internal/storage"
)

// mockGitHubProvider implements hosting.Provider for testing.
type mockGitHubProvider struct {
	GetPRCommentFunc   func(ctx context.Context, prNumber int, commentID int64) (*hosting.PRComment, error)
	FindPRByBranchFunc func(ctx context.Context, branch string) (*hosting.PR, error)
	// Other methods can be added as needed
}

func (m *mockGitHubProvider) CreatePR(ctx context.Context, opts hosting.PRCreateOptions) (*hosting.PR, error) {
	return nil, errors.New("not implemented")
}

func (m *mockGitHubProvider) GetPR(ctx context.Context, number int) (*hosting.PR, error) {
	return nil, errors.New("not implemented")
}

func (m *mockGitHubProvider) UpdatePR(ctx context.Context, number int, opts hosting.PRUpdateOptions) error {
	return errors.New("not implemented")
}

func (m *mockGitHubProvider) MergePR(ctx context.Context, number int, opts hosting.PRMergeOptions) error {
	return errors.New("not implemented")
}

func (m *mockGitHubProvider) ListPRComments(ctx context.Context, number int) ([]hosting.PRComment, error) {
	return nil, errors.New("not implemented")
}

func (m *mockGitHubProvider) CreatePRComment(ctx context.Context, number int, comment hosting.PRCommentCreate) (*hosting.PRComment, error) {
	return nil, errors.New("not implemented")
}

func (m *mockGitHubProvider) ReplyToComment(ctx context.Context, number int, threadID int64, body string) (*hosting.PRComment, error) {
	return nil, errors.New("not implemented")
}

func (m *mockGitHubProvider) GetPRComment(ctx context.Context, prNumber int, commentID int64) (*hosting.PRComment, error) {
	if m.GetPRCommentFunc != nil {
		return m.GetPRCommentFunc(ctx, prNumber, commentID)
	}
	return nil, errors.New("not implemented")
}

func (m *mockGitHubProvider) EnableAutoMerge(_ context.Context, _ int, _ string) error {
	return errors.New("not implemented")
}

func (m *mockGitHubProvider) UpdatePRBranch(_ context.Context, _ int) error {
	return errors.New("not implemented")
}

func (m *mockGitHubProvider) GetCheckRuns(ctx context.Context, ref string) ([]hosting.CheckRun, error) {
	return nil, errors.New("not implemented")
}

func (m *mockGitHubProvider) GetPRReviews(ctx context.Context, number int) ([]hosting.PRReview, error) {
	return nil, errors.New("not implemented")
}

func (m *mockGitHubProvider) FindPRByBranch(ctx context.Context, branch string) (*hosting.PR, error) {
	if m.FindPRByBranchFunc != nil {
		return m.FindPRByBranchFunc(ctx, branch)
	}
	return nil, errors.New("not implemented")
}

func (m *mockGitHubProvider) ApprovePR(ctx context.Context, number int, body string) error {
	return errors.New("not implemented")
}

func (m *mockGitHubProvider) GetPRStatusSummary(ctx context.Context, pr *hosting.PR) (*hosting.PRStatusSummary, error) {
	return nil, errors.New("not implemented")
}

func (m *mockGitHubProvider) DeleteBranch(ctx context.Context, branch string) error {
	return errors.New("not implemented")
}

func (m *mockGitHubProvider) CheckAuth(ctx context.Context) error {
	return nil
}

func (m *mockGitHubProvider) Name() hosting.ProviderType {
	return hosting.ProviderGitHub
}

func (m *mockGitHubProvider) OwnerRepo() (string, string) {
	return "owner", "repo"
}

// ============================================================================
// SC-1: AutofixComment starts execution
// ============================================================================

// TestAutofixComment_StartsExecution verifies SC-1:
// AutofixComment accepts task_id and comment_id and initiates a fix operation.
func TestAutofixComment_StartsExecution(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	publisher := events.NewMemoryPublisher()
	defer publisher.Close()

	workflowID := "medium"
	task := &orcv1.Task{
		Id:         "TASK-001",
		Title:      "Test task",
		Weight:     orcv1.TaskWeight_TASK_WEIGHT_MEDIUM,
		Status:     orcv1.TaskStatus_TASK_STATUS_PLANNED,
		WorkflowId: &workflowID,
		Branch:     "orc/TASK-001",
	}

	if err := backend.SaveTask(task); err != nil {
		t.Fatalf("save task: %v", err)
	}

	var executorCalled atomic.Bool
	var executorTaskID string
	mockExecutor := func(taskID string) error {
		executorCalled.Store(true)
		executorTaskID = taskID
		return nil
	}

	mockProvider := &mockGitHubProvider{
		GetPRCommentFunc: func(ctx context.Context, prNumber int, commentID int64) (*hosting.PRComment, error) {
			return &hosting.PRComment{
				ID:   commentID,
				Body: "Please fix this error handling",
				Path: "internal/api/handler.go",
				Line: 42,
			}, nil
		},
		FindPRByBranchFunc: func(ctx context.Context, branch string) (*hosting.PR, error) {
			return &hosting.PR{Number: 123}, nil
		},
	}

	server := NewHostingServerWithExecutor(
		backend,
		".",
		nil, // logger
		publisher,
		nil, // config
		mockExecutor,
		func(ctx context.Context) (hosting.Provider, error) {
			return mockProvider, nil
		},
	)

	req := connect.NewRequest(&orcv1.AutofixCommentRequest{
		TaskId:    "TASK-001",
		CommentId: 12345,
	})

	resp, err := server.AutofixComment(context.Background(), req)
	if err != nil {
		t.Fatalf("AutofixComment failed: %v", err)
	}

	if !executorCalled.Load() {
		t.Error("expected executor to be called")
	}

	if executorTaskID != "TASK-001" {
		t.Errorf("executor called with task ID %q, want %q", executorTaskID, "TASK-001")
	}

	if resp.Msg.Result == nil || !resp.Msg.Result.Success {
		t.Error("expected success=true in response")
	}
}

// ============================================================================
// SC-2: AutofixComment fetches comment
// ============================================================================

// TestAutofixComment_FetchesComment verifies SC-2:
// The comment content from GitHub is fetched and stored in Execution.RetryContext.
func TestAutofixComment_FetchesComment(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	publisher := events.NewMemoryPublisher()
	defer publisher.Close()

	workflowID := "medium"
	task := &orcv1.Task{
		Id:         "TASK-001",
		Title:      "Test task",
		Weight:     orcv1.TaskWeight_TASK_WEIGHT_MEDIUM,
		Status:     orcv1.TaskStatus_TASK_STATUS_PLANNED,
		WorkflowId: &workflowID,
		Branch:     "orc/TASK-001",
	}

	if err := backend.SaveTask(task); err != nil {
		t.Fatalf("save task: %v", err)
	}

	commentBody := "Error handling is missing for the nil case"
	mockExecutor := func(taskID string) error {
		return nil
	}

	mockProvider := &mockGitHubProvider{
		GetPRCommentFunc: func(ctx context.Context, prNumber int, commentID int64) (*hosting.PRComment, error) {
			return &hosting.PRComment{
				ID:     commentID,
				Body:   commentBody,
				Path:   "internal/api/handler.go",
				Line:   42,
				Author: "reviewer",
			}, nil
		},
		FindPRByBranchFunc: func(ctx context.Context, branch string) (*hosting.PR, error) {
			return &hosting.PR{Number: 123}, nil
		},
	}

	server := NewHostingServerWithExecutor(
		backend,
		".",
		nil,
		publisher,
		nil,
		mockExecutor,
		func(ctx context.Context) (hosting.Provider, error) {
			return mockProvider, nil
		},
	)

	req := connect.NewRequest(&orcv1.AutofixCommentRequest{
		TaskId:    "TASK-001",
		CommentId: 12345,
	})

	_, err := server.AutofixComment(context.Background(), req)
	if err != nil {
		t.Fatalf("AutofixComment failed: %v", err)
	}

	// Reload task to check retry context
	loaded, err := backend.LoadTask("TASK-001")
	if err != nil {
		t.Fatalf("reload task: %v", err)
	}

	if loaded.Execution == nil || loaded.Execution.RetryContext == nil {
		t.Fatal("expected RetryContext to be set")
	}

	rc := loaded.Execution.RetryContext
	if rc.FailureOutput == nil {
		t.Fatal("expected FailureOutput to be set")
	}

	if !strings.Contains(*rc.FailureOutput, commentBody) {
		t.Errorf("RetryContext.FailureOutput should contain comment body %q, got %q",
			commentBody, *rc.FailureOutput)
	}
}

// ============================================================================
// SC-4: AutofixComment returns immediately
// ============================================================================

// TestAutofixComment_ReturnsImmediately verifies SC-4:
// AutofixComment returns within 100ms with success=true.
func TestAutofixComment_ReturnsImmediately(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	publisher := events.NewMemoryPublisher()
	defer publisher.Close()

	workflowID := "medium"
	task := &orcv1.Task{
		Id:         "TASK-001",
		Title:      "Quick response test",
		Weight:     orcv1.TaskWeight_TASK_WEIGHT_MEDIUM,
		Status:     orcv1.TaskStatus_TASK_STATUS_PLANNED,
		WorkflowId: &workflowID,
		Branch:     "orc/TASK-001",
	}

	if err := backend.SaveTask(task); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Slow executor to simulate real execution
	executorStarted := make(chan struct{})
	executorDone := make(chan struct{})
	mockExecutor := func(taskID string) error {
		close(executorStarted)
		<-executorDone // Block until test says to finish
		return nil
	}
	defer close(executorDone)

	mockProvider := &mockGitHubProvider{
		GetPRCommentFunc: func(ctx context.Context, prNumber int, commentID int64) (*hosting.PRComment, error) {
			return &hosting.PRComment{ID: commentID, Body: "fix this"}, nil
		},
		FindPRByBranchFunc: func(ctx context.Context, branch string) (*hosting.PR, error) {
			return &hosting.PR{Number: 123}, nil
		},
	}

	server := NewHostingServerWithExecutor(
		backend,
		".",
		nil,
		publisher,
		nil,
		mockExecutor,
		func(ctx context.Context) (hosting.Provider, error) {
			return mockProvider, nil
		},
	)

	req := connect.NewRequest(&orcv1.AutofixCommentRequest{
		TaskId:    "TASK-001",
		CommentId: 12345,
	})

	// Time the call
	start := time.Now()
	resp, err := server.AutofixComment(context.Background(), req)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("AutofixComment failed: %v", err)
	}

	// Must return within 100ms (spec requirement)
	if elapsed > 100*time.Millisecond {
		t.Errorf("AutofixComment took %v, want < 100ms", elapsed)
	}

	// Must indicate success (autofix started)
	if resp.Msg.Result == nil || !resp.Msg.Result.Success {
		t.Error("expected success=true for immediate response")
	}

	// Wait for executor to start (verifies async execution)
	select {
	case <-executorStarted:
		// Good - executor was called
	case <-time.After(1 * time.Second):
		t.Error("executor was not started within 1 second")
	}
}

// ============================================================================
// SC-5: AutofixComment publishes completion event
// ============================================================================

// TestAutofixComment_PublishesCompletionEvent verifies SC-5:
// Autofix publishes events for the operation.
func TestAutofixComment_PublishesCompletionEvent(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	publisher := events.NewMemoryPublisher()
	defer publisher.Close()

	// Subscribe to events before making the call
	eventCh := publisher.Subscribe("TASK-001")
	defer publisher.Unsubscribe("TASK-001", eventCh)

	workflowID := "medium"
	task := &orcv1.Task{
		Id:         "TASK-001",
		Title:      "Event test",
		Weight:     orcv1.TaskWeight_TASK_WEIGHT_MEDIUM,
		Status:     orcv1.TaskStatus_TASK_STATUS_PLANNED,
		WorkflowId: &workflowID,
		Branch:     "orc/TASK-001",
	}

	if err := backend.SaveTask(task); err != nil {
		t.Fatalf("save task: %v", err)
	}

	mockExecutor := func(taskID string) error {
		return nil
	}

	mockProvider := &mockGitHubProvider{
		GetPRCommentFunc: func(ctx context.Context, prNumber int, commentID int64) (*hosting.PRComment, error) {
			return &hosting.PRComment{ID: commentID, Body: "fix"}, nil
		},
		FindPRByBranchFunc: func(ctx context.Context, branch string) (*hosting.PR, error) {
			return &hosting.PR{Number: 123}, nil
		},
	}

	server := NewHostingServerWithExecutor(
		backend,
		".",
		nil,
		publisher,
		nil,
		mockExecutor,
		func(ctx context.Context) (hosting.Provider, error) {
			return mockProvider, nil
		},
	)

	req := connect.NewRequest(&orcv1.AutofixCommentRequest{
		TaskId:    "TASK-001",
		CommentId: 12345,
	})

	_, err := server.AutofixComment(context.Background(), req)
	if err != nil {
		t.Fatalf("AutofixComment failed: %v", err)
	}

	// Check that events were published
	select {
	case evt := <-eventCh:
		if evt.Type != events.EventTaskUpdated {
			t.Errorf("expected task_updated event, got %s", evt.Type)
		}
	case <-time.After(1 * time.Second):
		t.Error("timeout waiting for task_updated event")
	}
}

// ============================================================================
// SC-7: Task already running
// ============================================================================

// TestAutofixComment_TaskAlreadyRunning verifies SC-7:
// If the task is already running, return CodeFailedPrecondition.
func TestAutofixComment_TaskAlreadyRunning(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	workflowID := "medium"
	task := &orcv1.Task{
		Id:         "TASK-001",
		Title:      "Running task",
		Weight:     orcv1.TaskWeight_TASK_WEIGHT_MEDIUM,
		Status:     orcv1.TaskStatus_TASK_STATUS_RUNNING, // Already running
		WorkflowId: &workflowID,
		Branch:     "orc/TASK-001",
	}

	if err := backend.SaveTask(task); err != nil {
		t.Fatalf("save task: %v", err)
	}

	mockProvider := &mockGitHubProvider{
		GetPRCommentFunc: func(ctx context.Context, prNumber int, commentID int64) (*hosting.PRComment, error) {
			return &hosting.PRComment{ID: commentID, Body: "fix"}, nil
		},
	}

	mockExecutor := func(taskID string) error {
		t.Error("executor should not be called for already-running task")
		return nil
	}

	server := NewHostingServerWithExecutor(
		backend,
		".",
		nil,
		nil,
		nil,
		mockExecutor,
		func(ctx context.Context) (hosting.Provider, error) {
			return mockProvider, nil
		},
	)

	req := connect.NewRequest(&orcv1.AutofixCommentRequest{
		TaskId:    "TASK-001",
		CommentId: 12345,
	})

	_, err := server.AutofixComment(context.Background(), req)

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

	if !containsIgnoreCase(connectErr.Message(), "running") {
		t.Errorf("error message should mention 'running', got: %s", connectErr.Message())
	}
}

// ============================================================================
// SC-8: GitHub authentication failure
// ============================================================================

// TestAutofixComment_NoGitHubAuth verifies SC-8:
// If GitHub authentication fails, return CodeUnauthenticated.
func TestAutofixComment_NoGitHubAuth(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	workflowID := "medium"
	task := &orcv1.Task{
		Id:         "TASK-001",
		Title:      "Auth test",
		Weight:     orcv1.TaskWeight_TASK_WEIGHT_MEDIUM,
		Status:     orcv1.TaskStatus_TASK_STATUS_PLANNED,
		WorkflowId: &workflowID,
		Branch:     "orc/TASK-001",
	}

	if err := backend.SaveTask(task); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Factory that returns auth error
	server := NewHostingServerWithExecutor(
		backend,
		".",
		nil,
		nil,
		nil,
		nil,
		func(ctx context.Context) (hosting.Provider, error) {
			return nil, errors.New("not logged in to GitHub")
		},
	)

	req := connect.NewRequest(&orcv1.AutofixCommentRequest{
		TaskId:    "TASK-001",
		CommentId: 12345,
	})

	_, err := server.AutofixComment(context.Background(), req)

	if err == nil {
		t.Fatal("expected error for auth failure")
	}

	connectErr, ok := err.(*connect.Error)
	if !ok {
		t.Fatalf("expected connect.Error, got %T", err)
	}

	if connectErr.Code() != connect.CodeUnauthenticated {
		t.Errorf("expected CodeUnauthenticated, got %v", connectErr.Code())
	}
}

// ============================================================================
// Edge Cases: Input Validation
// ============================================================================

func TestAutofixComment_EmptyTaskId(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	server := NewHostingServerWithExecutor(backend, ".", nil, nil, nil, nil, nil)

	req := connect.NewRequest(&orcv1.AutofixCommentRequest{
		TaskId:    "",
		CommentId: 12345,
	})

	_, err := server.AutofixComment(context.Background(), req)

	if err == nil {
		t.Fatal("expected error for empty task_id")
	}

	connectErr := err.(*connect.Error)
	if connectErr.Code() != connect.CodeInvalidArgument {
		t.Errorf("expected CodeInvalidArgument, got %v", connectErr.Code())
	}
}

func TestAutofixComment_ZeroCommentId(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	server := NewHostingServerWithExecutor(backend, ".", nil, nil, nil, nil, nil)

	req := connect.NewRequest(&orcv1.AutofixCommentRequest{
		TaskId:    "TASK-001",
		CommentId: 0,
	})

	_, err := server.AutofixComment(context.Background(), req)

	if err == nil {
		t.Fatal("expected error for zero comment_id")
	}

	connectErr := err.(*connect.Error)
	if connectErr.Code() != connect.CodeInvalidArgument {
		t.Errorf("expected CodeInvalidArgument, got %v", connectErr.Code())
	}
}

func TestAutofixComment_TaskNotFound(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	server := NewHostingServerWithExecutor(backend, ".", nil, nil, nil, nil, nil)

	req := connect.NewRequest(&orcv1.AutofixCommentRequest{
		TaskId:    "NONEXISTENT",
		CommentId: 12345,
	})

	_, err := server.AutofixComment(context.Background(), req)

	if err == nil {
		t.Fatal("expected error for nonexistent task")
	}

	connectErr := err.(*connect.Error)
	if connectErr.Code() != connect.CodeNotFound {
		t.Errorf("expected CodeNotFound, got %v", connectErr.Code())
	}
}

func TestAutofixComment_CommentNotFound(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	workflowID := "medium"
	task := &orcv1.Task{
		Id:         "TASK-001",
		Title:      "Test",
		Status:     orcv1.TaskStatus_TASK_STATUS_PLANNED,
		WorkflowId: &workflowID,
		Branch:     "orc/TASK-001",
	}

	if err := backend.SaveTask(task); err != nil {
		t.Fatalf("save task: %v", err)
	}

	mockProvider := &mockGitHubProvider{
		GetPRCommentFunc: func(ctx context.Context, prNumber int, commentID int64) (*hosting.PRComment, error) {
			return nil, errors.New("comment not found: 12345")
		},
		FindPRByBranchFunc: func(ctx context.Context, branch string) (*hosting.PR, error) {
			return &hosting.PR{Number: 123}, nil
		},
	}

	server := NewHostingServerWithExecutor(
		backend,
		".",
		nil,
		nil,
		nil,
		nil,
		func(ctx context.Context) (hosting.Provider, error) {
			return mockProvider, nil
		},
	)

	req := connect.NewRequest(&orcv1.AutofixCommentRequest{
		TaskId:    "TASK-001",
		CommentId: 12345,
	})

	_, err := server.AutofixComment(context.Background(), req)

	if err == nil {
		t.Fatal("expected error for nonexistent comment")
	}

	connectErr := err.(*connect.Error)
	if connectErr.Code() != connect.CodeNotFound {
		t.Errorf("expected CodeNotFound, got %v", connectErr.Code())
	}
}

func TestAutofixComment_NoBranch(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	workflowID := "medium"
	task := &orcv1.Task{
		Id:         "TASK-001",
		Title:      "No branch task",
		Status:     orcv1.TaskStatus_TASK_STATUS_PLANNED,
		WorkflowId: &workflowID,
		Branch:     "", // No branch
	}

	if err := backend.SaveTask(task); err != nil {
		t.Fatalf("save task: %v", err)
	}

	server := NewHostingServerWithExecutor(backend, ".", nil, nil, nil, nil, nil)

	req := connect.NewRequest(&orcv1.AutofixCommentRequest{
		TaskId:    "TASK-001",
		CommentId: 12345,
	})

	_, err := server.AutofixComment(context.Background(), req)

	if err == nil {
		t.Fatal("expected error for task without branch")
	}

	connectErr := err.(*connect.Error)
	if connectErr.Code() != connect.CodeFailedPrecondition {
		t.Errorf("expected CodeFailedPrecondition, got %v", connectErr.Code())
	}
}

func TestAutofixComment_TaskCompleted(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	workflowID := "medium"
	task := &orcv1.Task{
		Id:         "TASK-001",
		Title:      "Completed task",
		Status:     orcv1.TaskStatus_TASK_STATUS_COMPLETED,
		WorkflowId: &workflowID,
		Branch:     "orc/TASK-001",
	}

	if err := backend.SaveTask(task); err != nil {
		t.Fatalf("save task: %v", err)
	}

	server := NewHostingServerWithExecutor(backend, ".", nil, nil, nil, nil, nil)

	req := connect.NewRequest(&orcv1.AutofixCommentRequest{
		TaskId:    "TASK-001",
		CommentId: 12345,
	})

	_, err := server.AutofixComment(context.Background(), req)

	if err == nil {
		t.Fatal("expected error for completed task")
	}

	connectErr := err.(*connect.Error)
	if connectErr.Code() != connect.CodeFailedPrecondition {
		t.Errorf("expected CodeFailedPrecondition, got %v", connectErr.Code())
	}
}

func TestAutofixComment_TaskPaused_AllowsAutofix(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	publisher := events.NewMemoryPublisher()
	defer publisher.Close()

	workflowID := "medium"
	task := &orcv1.Task{
		Id:         "TASK-001",
		Title:      "Paused task",
		Status:     orcv1.TaskStatus_TASK_STATUS_PAUSED,
		WorkflowId: &workflowID,
		Branch:     "orc/TASK-001",
	}

	if err := backend.SaveTask(task); err != nil {
		t.Fatalf("save task: %v", err)
	}

	var executorCalled atomic.Bool
	mockExecutor := func(taskID string) error {
		executorCalled.Store(true)
		return nil
	}

	mockProvider := &mockGitHubProvider{
		GetPRCommentFunc: func(ctx context.Context, prNumber int, commentID int64) (*hosting.PRComment, error) {
			return &hosting.PRComment{ID: commentID, Body: "fix"}, nil
		},
		FindPRByBranchFunc: func(ctx context.Context, branch string) (*hosting.PR, error) {
			return &hosting.PR{Number: 123}, nil
		},
	}

	server := NewHostingServerWithExecutor(
		backend,
		".",
		nil,
		publisher,
		nil,
		mockExecutor,
		func(ctx context.Context) (hosting.Provider, error) {
			return mockProvider, nil
		},
	)

	req := connect.NewRequest(&orcv1.AutofixCommentRequest{
		TaskId:    "TASK-001",
		CommentId: 12345,
	})

	resp, err := server.AutofixComment(context.Background(), req)

	// Paused tasks should allow autofix (similar to resume behavior)
	if err != nil {
		t.Fatalf("AutofixComment should allow paused tasks: %v", err)
	}

	if !executorCalled.Load() {
		t.Error("executor should be called for paused task autofix")
	}

	if resp.Msg.Result == nil || !resp.Msg.Result.Success {
		t.Error("expected success=true for paused task autofix")
	}
}

// ============================================================================
// Edge Case: Executor Spawn Fails
// ============================================================================

// TestAutofixComment_ExecutorFails verifies executor failure returns CodeInternal.
func TestAutofixComment_ExecutorFails(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	publisher := events.NewMemoryPublisher()
	defer publisher.Close()

	workflowID := "medium"
	task := &orcv1.Task{
		Id:         "TASK-001",
		Title:      "Executor fail test",
		Weight:     orcv1.TaskWeight_TASK_WEIGHT_MEDIUM,
		Status:     orcv1.TaskStatus_TASK_STATUS_PLANNED,
		WorkflowId: &workflowID,
		Branch:     "orc/TASK-001",
	}

	if err := backend.SaveTask(task); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Executor that always fails
	mockExecutor := func(taskID string) error {
		return errors.New("no Claude process available")
	}

	mockProvider := &mockGitHubProvider{
		GetPRCommentFunc: func(ctx context.Context, prNumber int, commentID int64) (*hosting.PRComment, error) {
			return &hosting.PRComment{ID: commentID, Body: "fix"}, nil
		},
		FindPRByBranchFunc: func(ctx context.Context, branch string) (*hosting.PR, error) {
			return &hosting.PR{Number: 123}, nil
		},
	}

	server := NewHostingServerWithExecutor(
		backend,
		".",
		nil,
		publisher,
		nil,
		mockExecutor,
		func(ctx context.Context) (hosting.Provider, error) {
			return mockProvider, nil
		},
	)

	req := connect.NewRequest(&orcv1.AutofixCommentRequest{
		TaskId:    "TASK-001",
		CommentId: 12345,
	})

	_, err := server.AutofixComment(context.Background(), req)

	if err == nil {
		t.Fatal("expected error when executor fails")
	}

	connectErr, ok := err.(*connect.Error)
	if !ok {
		t.Fatalf("expected connect.Error, got %T", err)
	}

	if connectErr.Code() != connect.CodeInternal {
		t.Errorf("expected CodeInternal, got %v", connectErr.Code())
	}

	if !containsIgnoreCase(connectErr.Message(), "executor") && !containsIgnoreCase(connectErr.Message(), "spawn") {
		t.Errorf("error message should mention executor/spawn, got: %s", connectErr.Message())
	}

	// Verify task status is NOT running after failure
	loaded, err := backend.LoadTask("TASK-001")
	if err != nil {
		t.Fatalf("reload task: %v", err)
	}

	if loaded.Status == orcv1.TaskStatus_TASK_STATUS_RUNNING {
		t.Error("task status should not be RUNNING after executor failure")
	}
}

// ============================================================================
// Edge Case: GitHub Rate Limited
// ============================================================================

// TestAutofixComment_RateLimited verifies rate limit returns CodeResourceExhausted.
func TestAutofixComment_RateLimited(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	workflowID := "medium"
	task := &orcv1.Task{
		Id:         "TASK-001",
		Title:      "Rate limit test",
		Status:     orcv1.TaskStatus_TASK_STATUS_PLANNED,
		WorkflowId: &workflowID,
		Branch:     "orc/TASK-001",
	}

	if err := backend.SaveTask(task); err != nil {
		t.Fatalf("save task: %v", err)
	}

	mockProvider := &mockGitHubProvider{
		GetPRCommentFunc: func(ctx context.Context, prNumber int, commentID int64) (*hosting.PRComment, error) {
			return nil, errors.New("rate limit exceeded")
		},
		FindPRByBranchFunc: func(ctx context.Context, branch string) (*hosting.PR, error) {
			return &hosting.PR{Number: 123}, nil
		},
	}

	server := NewHostingServerWithExecutor(
		backend,
		".",
		nil,
		nil,
		nil,
		nil,
		func(ctx context.Context) (hosting.Provider, error) {
			return mockProvider, nil
		},
	)

	req := connect.NewRequest(&orcv1.AutofixCommentRequest{
		TaskId:    "TASK-001",
		CommentId: 12345,
	})

	_, err := server.AutofixComment(context.Background(), req)

	if err == nil {
		t.Fatal("expected error for rate limit")
	}

	connectErr := err.(*connect.Error)
	if connectErr.Code() != connect.CodeResourceExhausted {
		t.Errorf("expected CodeResourceExhausted, got %v", connectErr.Code())
	}
}

// ============================================================================
// Edge Case: Long Comment Truncated
// ============================================================================

// TestAutofixComment_LongCommentTruncated verifies comments >10KB are truncated.
func TestAutofixComment_LongCommentTruncated(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	publisher := events.NewMemoryPublisher()
	defer publisher.Close()

	workflowID := "medium"
	task := &orcv1.Task{
		Id:         "TASK-001",
		Title:      "Long comment test",
		Status:     orcv1.TaskStatus_TASK_STATUS_PLANNED,
		WorkflowId: &workflowID,
		Branch:     "orc/TASK-001",
	}

	if err := backend.SaveTask(task); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Create a comment body > 10KB
	longBody := strings.Repeat("This is a very long comment. ", 500) // ~15KB

	mockExecutor := func(taskID string) error {
		return nil
	}

	mockProvider := &mockGitHubProvider{
		GetPRCommentFunc: func(ctx context.Context, prNumber int, commentID int64) (*hosting.PRComment, error) {
			return &hosting.PRComment{
				ID:   commentID,
				Body: longBody,
				Path: "file.go",
				Line: 1,
			}, nil
		},
		FindPRByBranchFunc: func(ctx context.Context, branch string) (*hosting.PR, error) {
			return &hosting.PR{Number: 123}, nil
		},
	}

	server := NewHostingServerWithExecutor(
		backend,
		".",
		nil,
		publisher,
		nil,
		mockExecutor,
		func(ctx context.Context) (hosting.Provider, error) {
			return mockProvider, nil
		},
	)

	req := connect.NewRequest(&orcv1.AutofixCommentRequest{
		TaskId:    "TASK-001",
		CommentId: 12345,
	})

	_, err := server.AutofixComment(context.Background(), req)
	if err != nil {
		t.Fatalf("AutofixComment failed: %v", err)
	}

	// Reload task to check retry context
	loaded, err := backend.LoadTask("TASK-001")
	if err != nil {
		t.Fatalf("reload task: %v", err)
	}

	if loaded.Execution == nil || loaded.Execution.RetryContext == nil {
		t.Fatal("expected RetryContext to be set")
	}

	rc := loaded.Execution.RetryContext
	if rc.FailureOutput == nil {
		t.Fatal("expected FailureOutput to be set")
	}

	// Should contain truncation indicator
	if !strings.Contains(*rc.FailureOutput, "truncated") {
		t.Error("expected long comment to be truncated with indicator")
	}

	// Should be reasonable size (not full 15KB)
	if len(*rc.FailureOutput) > 12*1024 { // Allow some overhead for formatting
		t.Errorf("FailureOutput should be truncated, got %d bytes", len(*rc.FailureOutput))
	}
}
