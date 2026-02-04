// Package api provides HTTP API handlers for orc.
//
// TDD Tests for TASK-741: Backend API for real-time feedback to agents
//
// These tests verify the feedback API endpoints that allow users to send
// real-time feedback to agents during task execution.
//
// Success Criteria Coverage:
// - SC-1: AddFeedback creates feedback with required fields (ID, Type, Text, Timing)
// - SC-2: Feedback types are validated ("inline", "general", "approval", "direction")
// - SC-3: Inline feedback requires File and Line fields
// - SC-4: Timing options are validated ("now", "when_done", "manual")
// - SC-5: ListFeedback returns pending feedback for a task
// - SC-6: SendFeedback sends all queued feedback
// - SC-7: Feedback is persisted to database
// - SC-8: "Send Now" timing triggers task pause (integration test)
//
// Edge Cases:
// - Empty text content is rejected
// - Invalid task ID returns NotFound
// - Inline comment missing file/line returns InvalidArgument
// - Sending feedback when no pending feedback exists is allowed (no-op)
//
// Failure Modes:
// - Task not found: NotFound error
// - Invalid feedback type: InvalidArgument error
// - Missing required fields: InvalidArgument error
package api

import (
	"context"
	"testing"

	"connectrpc.com/connect"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/storage"
)

// ============================================================================
// SC-1: AddFeedback creates feedback with required fields
// ============================================================================

// TestAddFeedback_CreatesWithRequiredFields verifies SC-1:
// AddFeedback creates feedback with all required fields populated.
func TestAddFeedback_CreatesWithRequiredFields(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	server := NewFeedbackServer(backend, nil, nil)

	// Create a task first
	taskServer := NewTaskServer(backend, nil, nil, nil, "", nil, nil)
	taskReq := connect.NewRequest(&orcv1.CreateTaskRequest{
		Title:  "Test task for feedback",
	})
	taskResp, err := taskServer.CreateTask(context.Background(), taskReq)
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}
	taskID := taskResp.Msg.Task.Id

	// Add feedback
	req := connect.NewRequest(&orcv1.AddFeedbackRequest{
		TaskId: taskID,
		Type:   orcv1.FeedbackType_FEEDBACK_TYPE_GENERAL,
		Text:   "Please also add a test for edge cases",
		Timing: orcv1.FeedbackTiming_FEEDBACK_TIMING_WHEN_DONE,
	})

	resp, err := server.AddFeedback(context.Background(), req)
	if err != nil {
		t.Fatalf("AddFeedback failed: %v", err)
	}

	// Verify feedback was created with all fields
	feedback := resp.Msg.Feedback
	if feedback == nil {
		t.Fatal("response feedback is nil")
	}
	if feedback.Id == "" {
		t.Error("feedback.Id should be generated, got empty string")
	}
	if feedback.TaskId != taskID {
		t.Errorf("feedback.TaskId = %q, want %q", feedback.TaskId, taskID)
	}
	if feedback.Type != orcv1.FeedbackType_FEEDBACK_TYPE_GENERAL {
		t.Errorf("feedback.Type = %v, want GENERAL", feedback.Type)
	}
	if feedback.Text != "Please also add a test for edge cases" {
		t.Errorf("feedback.Text = %q, want %q", feedback.Text, "Please also add a test for edge cases")
	}
	if feedback.Timing != orcv1.FeedbackTiming_FEEDBACK_TIMING_WHEN_DONE {
		t.Errorf("feedback.Timing = %v, want WHEN_DONE", feedback.Timing)
	}
	if feedback.Received {
		t.Error("feedback.Received should be false for new feedback")
	}
}

// ============================================================================
// SC-2: Feedback type validation
// ============================================================================

// TestAddFeedback_ValidatesType_Inline verifies SC-2:
// AddFeedback accepts "inline" feedback type.
func TestAddFeedback_ValidatesType_Inline(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	server := NewFeedbackServer(backend, nil, nil)

	// Create task
	taskServer := NewTaskServer(backend, nil, nil, nil, "", nil, nil)
	taskResp, _ := taskServer.CreateTask(context.Background(), connect.NewRequest(&orcv1.CreateTaskRequest{
		Title:  "Test task",
	}))
	taskID := taskResp.Msg.Task.Id

	req := connect.NewRequest(&orcv1.AddFeedbackRequest{
		TaskId: taskID,
		Type:   orcv1.FeedbackType_FEEDBACK_TYPE_INLINE,
		Text:   "Use validateSession() instead",
		File:   "auth/login.go",
		Line:   47,
		Timing: orcv1.FeedbackTiming_FEEDBACK_TIMING_WHEN_DONE,
	})

	resp, err := server.AddFeedback(context.Background(), req)
	if err != nil {
		t.Fatalf("AddFeedback with INLINE type failed: %v", err)
	}

	if resp.Msg.Feedback.Type != orcv1.FeedbackType_FEEDBACK_TYPE_INLINE {
		t.Errorf("feedback.Type = %v, want INLINE", resp.Msg.Feedback.Type)
	}
}

// TestAddFeedback_ValidatesType_Approval verifies SC-2:
// AddFeedback accepts "approval" feedback type.
func TestAddFeedback_ValidatesType_Approval(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	server := NewFeedbackServer(backend, nil, nil)

	// Create task
	taskServer := NewTaskServer(backend, nil, nil, nil, "", nil, nil)
	taskResp, _ := taskServer.CreateTask(context.Background(), connect.NewRequest(&orcv1.CreateTaskRequest{
		Title:  "Test task",
	}))
	taskID := taskResp.Msg.Task.Id

	req := connect.NewRequest(&orcv1.AddFeedbackRequest{
		TaskId: taskID,
		Type:   orcv1.FeedbackType_FEEDBACK_TYPE_APPROVAL,
		Text:   "Looks good so far",
		Timing: orcv1.FeedbackTiming_FEEDBACK_TIMING_WHEN_DONE,
	})

	resp, err := server.AddFeedback(context.Background(), req)
	if err != nil {
		t.Fatalf("AddFeedback with APPROVAL type failed: %v", err)
	}

	if resp.Msg.Feedback.Type != orcv1.FeedbackType_FEEDBACK_TYPE_APPROVAL {
		t.Errorf("feedback.Type = %v, want APPROVAL", resp.Msg.Feedback.Type)
	}
}

// TestAddFeedback_ValidatesType_Direction verifies SC-2:
// AddFeedback accepts "direction" feedback type.
func TestAddFeedback_ValidatesType_Direction(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	server := NewFeedbackServer(backend, nil, nil)

	// Create task
	taskServer := NewTaskServer(backend, nil, nil, nil, "", nil, nil)
	taskResp, _ := taskServer.CreateTask(context.Background(), connect.NewRequest(&orcv1.CreateTaskRequest{
		Title:  "Test task",
	}))
	taskID := taskResp.Msg.Task.Id

	req := connect.NewRequest(&orcv1.AddFeedbackRequest{
		TaskId: taskID,
		Type:   orcv1.FeedbackType_FEEDBACK_TYPE_DIRECTION,
		Text:   "Try a different approach using Redis instead",
		Timing: orcv1.FeedbackTiming_FEEDBACK_TIMING_NOW,
	})

	resp, err := server.AddFeedback(context.Background(), req)
	if err != nil {
		t.Fatalf("AddFeedback with DIRECTION type failed: %v", err)
	}

	if resp.Msg.Feedback.Type != orcv1.FeedbackType_FEEDBACK_TYPE_DIRECTION {
		t.Errorf("feedback.Type = %v, want DIRECTION", resp.Msg.Feedback.Type)
	}
}

// TestAddFeedback_RejectsInvalidType verifies SC-2 error path:
// AddFeedback rejects unspecified feedback type.
func TestAddFeedback_RejectsInvalidType(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	server := NewFeedbackServer(backend, nil, nil)

	// Create task
	taskServer := NewTaskServer(backend, nil, nil, nil, "", nil, nil)
	taskResp, _ := taskServer.CreateTask(context.Background(), connect.NewRequest(&orcv1.CreateTaskRequest{
		Title:  "Test task",
	}))
	taskID := taskResp.Msg.Task.Id

	req := connect.NewRequest(&orcv1.AddFeedbackRequest{
		TaskId: taskID,
		Type:   orcv1.FeedbackType_FEEDBACK_TYPE_UNSPECIFIED,
		Text:   "Some feedback",
		Timing: orcv1.FeedbackTiming_FEEDBACK_TIMING_WHEN_DONE,
	})

	_, err := server.AddFeedback(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for UNSPECIFIED feedback type, got nil")
	}

	connectErr, ok := err.(*connect.Error)
	if !ok {
		t.Fatalf("expected connect.Error, got %T", err)
	}
	if connectErr.Code() != connect.CodeInvalidArgument {
		t.Errorf("error code = %v, want InvalidArgument", connectErr.Code())
	}
}

// ============================================================================
// SC-3: Inline feedback requires File and Line fields
// ============================================================================

// TestAddFeedback_InlineRequiresFile verifies SC-3:
// Inline feedback must have File field populated.
func TestAddFeedback_InlineRequiresFile(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	server := NewFeedbackServer(backend, nil, nil)

	// Create task
	taskServer := NewTaskServer(backend, nil, nil, nil, "", nil, nil)
	taskResp, _ := taskServer.CreateTask(context.Background(), connect.NewRequest(&orcv1.CreateTaskRequest{
		Title:  "Test task",
	}))
	taskID := taskResp.Msg.Task.Id

	req := connect.NewRequest(&orcv1.AddFeedbackRequest{
		TaskId: taskID,
		Type:   orcv1.FeedbackType_FEEDBACK_TYPE_INLINE,
		Text:   "Use validateSession() instead",
		// File is missing
		Line:   47,
		Timing: orcv1.FeedbackTiming_FEEDBACK_TIMING_WHEN_DONE,
	})

	_, err := server.AddFeedback(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for inline feedback without File, got nil")
	}

	connectErr, ok := err.(*connect.Error)
	if !ok {
		t.Fatalf("expected connect.Error, got %T", err)
	}
	if connectErr.Code() != connect.CodeInvalidArgument {
		t.Errorf("error code = %v, want InvalidArgument", connectErr.Code())
	}
}

// TestAddFeedback_InlineRequiresLine verifies SC-3:
// Inline feedback must have Line field populated (>0).
func TestAddFeedback_InlineRequiresLine(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	server := NewFeedbackServer(backend, nil, nil)

	// Create task
	taskServer := NewTaskServer(backend, nil, nil, nil, "", nil, nil)
	taskResp, _ := taskServer.CreateTask(context.Background(), connect.NewRequest(&orcv1.CreateTaskRequest{
		Title:  "Test task",
	}))
	taskID := taskResp.Msg.Task.Id

	req := connect.NewRequest(&orcv1.AddFeedbackRequest{
		TaskId: taskID,
		Type:   orcv1.FeedbackType_FEEDBACK_TYPE_INLINE,
		Text:   "Use validateSession() instead",
		File:   "auth/login.go",
		// Line is 0 (missing)
		Timing: orcv1.FeedbackTiming_FEEDBACK_TIMING_WHEN_DONE,
	})

	_, err := server.AddFeedback(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for inline feedback without Line, got nil")
	}

	connectErr, ok := err.(*connect.Error)
	if !ok {
		t.Fatalf("expected connect.Error, got %T", err)
	}
	if connectErr.Code() != connect.CodeInvalidArgument {
		t.Errorf("error code = %v, want InvalidArgument", connectErr.Code())
	}
}

// TestAddFeedback_InlineWithFileAndLine_Succeeds verifies SC-3 success path:
// Inline feedback with both File and Line succeeds.
func TestAddFeedback_InlineWithFileAndLine_Succeeds(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	server := NewFeedbackServer(backend, nil, nil)

	// Create task
	taskServer := NewTaskServer(backend, nil, nil, nil, "", nil, nil)
	taskResp, _ := taskServer.CreateTask(context.Background(), connect.NewRequest(&orcv1.CreateTaskRequest{
		Title:  "Test task",
	}))
	taskID := taskResp.Msg.Task.Id

	req := connect.NewRequest(&orcv1.AddFeedbackRequest{
		TaskId: taskID,
		Type:   orcv1.FeedbackType_FEEDBACK_TYPE_INLINE,
		Text:   "Use validateSession() instead",
		File:   "auth/login.go",
		Line:   47,
		Timing: orcv1.FeedbackTiming_FEEDBACK_TIMING_WHEN_DONE,
	})

	resp, err := server.AddFeedback(context.Background(), req)
	if err != nil {
		t.Fatalf("AddFeedback with valid inline feedback failed: %v", err)
	}

	feedback := resp.Msg.Feedback
	if feedback.File != "auth/login.go" {
		t.Errorf("feedback.File = %q, want %q", feedback.File, "auth/login.go")
	}
	if feedback.Line != 47 {
		t.Errorf("feedback.Line = %d, want %d", feedback.Line, 47)
	}
}

// ============================================================================
// SC-4: Timing options validation
// ============================================================================

// TestAddFeedback_ValidatesTiming_Now verifies SC-4:
// AddFeedback accepts "now" timing.
func TestAddFeedback_ValidatesTiming_Now(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	server := NewFeedbackServer(backend, nil, nil)

	// Create task
	taskServer := NewTaskServer(backend, nil, nil, nil, "", nil, nil)
	taskResp, _ := taskServer.CreateTask(context.Background(), connect.NewRequest(&orcv1.CreateTaskRequest{
		Title:  "Test task",
	}))
	taskID := taskResp.Msg.Task.Id

	req := connect.NewRequest(&orcv1.AddFeedbackRequest{
		TaskId: taskID,
		Type:   orcv1.FeedbackType_FEEDBACK_TYPE_DIRECTION,
		Text:   "Stop! Wrong approach",
		Timing: orcv1.FeedbackTiming_FEEDBACK_TIMING_NOW,
	})

	resp, err := server.AddFeedback(context.Background(), req)
	if err != nil {
		t.Fatalf("AddFeedback with NOW timing failed: %v", err)
	}

	if resp.Msg.Feedback.Timing != orcv1.FeedbackTiming_FEEDBACK_TIMING_NOW {
		t.Errorf("feedback.Timing = %v, want NOW", resp.Msg.Feedback.Timing)
	}
}

// TestAddFeedback_ValidatesTiming_WhenDone verifies SC-4:
// AddFeedback accepts "when_done" timing.
func TestAddFeedback_ValidatesTiming_WhenDone(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	server := NewFeedbackServer(backend, nil, nil)

	// Create task
	taskServer := NewTaskServer(backend, nil, nil, nil, "", nil, nil)
	taskResp, _ := taskServer.CreateTask(context.Background(), connect.NewRequest(&orcv1.CreateTaskRequest{
		Title:  "Test task",
	}))
	taskID := taskResp.Msg.Task.Id

	req := connect.NewRequest(&orcv1.AddFeedbackRequest{
		TaskId: taskID,
		Type:   orcv1.FeedbackType_FEEDBACK_TYPE_GENERAL,
		Text:   "Also consider edge cases",
		Timing: orcv1.FeedbackTiming_FEEDBACK_TIMING_WHEN_DONE,
	})

	resp, err := server.AddFeedback(context.Background(), req)
	if err != nil {
		t.Fatalf("AddFeedback with WHEN_DONE timing failed: %v", err)
	}

	if resp.Msg.Feedback.Timing != orcv1.FeedbackTiming_FEEDBACK_TIMING_WHEN_DONE {
		t.Errorf("feedback.Timing = %v, want WHEN_DONE", resp.Msg.Feedback.Timing)
	}
}

// TestAddFeedback_ValidatesTiming_Manual verifies SC-4:
// AddFeedback accepts "manual" timing.
func TestAddFeedback_ValidatesTiming_Manual(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	server := NewFeedbackServer(backend, nil, nil)

	// Create task
	taskServer := NewTaskServer(backend, nil, nil, nil, "", nil, nil)
	taskResp, _ := taskServer.CreateTask(context.Background(), connect.NewRequest(&orcv1.CreateTaskRequest{
		Title:  "Test task",
	}))
	taskID := taskResp.Msg.Task.Id

	req := connect.NewRequest(&orcv1.AddFeedbackRequest{
		TaskId: taskID,
		Type:   orcv1.FeedbackType_FEEDBACK_TYPE_GENERAL,
		Text:   "Just a note for later",
		Timing: orcv1.FeedbackTiming_FEEDBACK_TIMING_MANUAL,
	})

	resp, err := server.AddFeedback(context.Background(), req)
	if err != nil {
		t.Fatalf("AddFeedback with MANUAL timing failed: %v", err)
	}

	if resp.Msg.Feedback.Timing != orcv1.FeedbackTiming_FEEDBACK_TIMING_MANUAL {
		t.Errorf("feedback.Timing = %v, want MANUAL", resp.Msg.Feedback.Timing)
	}
}

// TestAddFeedback_RejectsUnspecifiedTiming verifies SC-4 error path:
// AddFeedback rejects unspecified timing.
func TestAddFeedback_RejectsUnspecifiedTiming(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	server := NewFeedbackServer(backend, nil, nil)

	// Create task
	taskServer := NewTaskServer(backend, nil, nil, nil, "", nil, nil)
	taskResp, _ := taskServer.CreateTask(context.Background(), connect.NewRequest(&orcv1.CreateTaskRequest{
		Title:  "Test task",
	}))
	taskID := taskResp.Msg.Task.Id

	req := connect.NewRequest(&orcv1.AddFeedbackRequest{
		TaskId: taskID,
		Type:   orcv1.FeedbackType_FEEDBACK_TYPE_GENERAL,
		Text:   "Some feedback",
		Timing: orcv1.FeedbackTiming_FEEDBACK_TIMING_UNSPECIFIED,
	})

	_, err := server.AddFeedback(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for UNSPECIFIED timing, got nil")
	}

	connectErr, ok := err.(*connect.Error)
	if !ok {
		t.Fatalf("expected connect.Error, got %T", err)
	}
	if connectErr.Code() != connect.CodeInvalidArgument {
		t.Errorf("error code = %v, want InvalidArgument", connectErr.Code())
	}
}

// ============================================================================
// SC-5: ListFeedback returns pending feedback for a task
// ============================================================================

// TestListFeedback_ReturnsPendingFeedback verifies SC-5:
// ListFeedback returns all pending (unsent) feedback for a task.
func TestListFeedback_ReturnsPendingFeedback(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	server := NewFeedbackServer(backend, nil, nil)

	// Create task
	taskServer := NewTaskServer(backend, nil, nil, nil, "", nil, nil)
	taskResp, _ := taskServer.CreateTask(context.Background(), connect.NewRequest(&orcv1.CreateTaskRequest{
		Title:  "Test task",
	}))
	taskID := taskResp.Msg.Task.Id

	// Add multiple feedback items
	_, _ = server.AddFeedback(context.Background(), connect.NewRequest(&orcv1.AddFeedbackRequest{
		TaskId: taskID,
		Type:   orcv1.FeedbackType_FEEDBACK_TYPE_GENERAL,
		Text:   "First feedback",
		Timing: orcv1.FeedbackTiming_FEEDBACK_TIMING_WHEN_DONE,
	}))

	_, _ = server.AddFeedback(context.Background(), connect.NewRequest(&orcv1.AddFeedbackRequest{
		TaskId: taskID,
		Type:   orcv1.FeedbackType_FEEDBACK_TYPE_INLINE,
		Text:   "Second feedback",
		File:   "test.go",
		Line:   10,
		Timing: orcv1.FeedbackTiming_FEEDBACK_TIMING_MANUAL,
	}))

	// List feedback
	listReq := connect.NewRequest(&orcv1.ListFeedbackRequest{
		TaskId: taskID,
	})

	resp, err := server.ListFeedback(context.Background(), listReq)
	if err != nil {
		t.Fatalf("ListFeedback failed: %v", err)
	}

	if len(resp.Msg.Feedback) != 2 {
		t.Errorf("expected 2 feedback items, got %d", len(resp.Msg.Feedback))
	}
}

// TestListFeedback_ReturnsEmptyForNoFeedback verifies SC-5 edge case:
// ListFeedback returns empty list when no feedback exists.
func TestListFeedback_ReturnsEmptyForNoFeedback(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	server := NewFeedbackServer(backend, nil, nil)

	// Create task
	taskServer := NewTaskServer(backend, nil, nil, nil, "", nil, nil)
	taskResp, _ := taskServer.CreateTask(context.Background(), connect.NewRequest(&orcv1.CreateTaskRequest{
		Title:  "Test task",
	}))
	taskID := taskResp.Msg.Task.Id

	// List feedback without adding any
	listReq := connect.NewRequest(&orcv1.ListFeedbackRequest{
		TaskId: taskID,
	})

	resp, err := server.ListFeedback(context.Background(), listReq)
	if err != nil {
		t.Fatalf("ListFeedback failed: %v", err)
	}

	if len(resp.Msg.Feedback) != 0 {
		t.Errorf("expected 0 feedback items for empty list, got %d", len(resp.Msg.Feedback))
	}
}

// TestListFeedback_ExcludesReceivedFeedback verifies SC-5:
// ListFeedback excludes feedback that has already been received by the agent.
func TestListFeedback_ExcludesReceivedFeedback(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	server := NewFeedbackServer(backend, nil, nil)

	// Create task
	taskServer := NewTaskServer(backend, nil, nil, nil, "", nil, nil)
	taskResp, _ := taskServer.CreateTask(context.Background(), connect.NewRequest(&orcv1.CreateTaskRequest{
		Title:  "Test task",
	}))
	taskID := taskResp.Msg.Task.Id

	// Add feedback
	_, _ = server.AddFeedback(context.Background(), connect.NewRequest(&orcv1.AddFeedbackRequest{
		TaskId: taskID,
		Type:   orcv1.FeedbackType_FEEDBACK_TYPE_GENERAL,
		Text:   "First feedback",
		Timing: orcv1.FeedbackTiming_FEEDBACK_TIMING_WHEN_DONE,
	}))

	_, _ = server.AddFeedback(context.Background(), connect.NewRequest(&orcv1.AddFeedbackRequest{
		TaskId: taskID,
		Type:   orcv1.FeedbackType_FEEDBACK_TYPE_GENERAL,
		Text:   "Second feedback",
		Timing: orcv1.FeedbackTiming_FEEDBACK_TIMING_WHEN_DONE,
	}))

	// Send feedback (marks as received)
	_, _ = server.SendFeedback(context.Background(), connect.NewRequest(&orcv1.SendFeedbackRequest{
		TaskId: taskID,
	}))

	// List feedback - should exclude sent ones
	listReq := connect.NewRequest(&orcv1.ListFeedbackRequest{
		TaskId:         taskID,
		ExcludeReceived: true,
	})

	resp, err := server.ListFeedback(context.Background(), listReq)
	if err != nil {
		t.Fatalf("ListFeedback failed: %v", err)
	}

	if len(resp.Msg.Feedback) != 0 {
		t.Errorf("expected 0 pending feedback items after send, got %d", len(resp.Msg.Feedback))
	}
}

// ============================================================================
// SC-6: SendFeedback sends all queued feedback
// ============================================================================

// TestSendFeedback_MarksAsReceived verifies SC-6:
// SendFeedback marks all pending feedback as received.
func TestSendFeedback_MarksAsReceived(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	server := NewFeedbackServer(backend, nil, nil)

	// Create task
	taskServer := NewTaskServer(backend, nil, nil, nil, "", nil, nil)
	taskResp, _ := taskServer.CreateTask(context.Background(), connect.NewRequest(&orcv1.CreateTaskRequest{
		Title:  "Test task",
	}))
	taskID := taskResp.Msg.Task.Id

	// Add feedback
	_, _ = server.AddFeedback(context.Background(), connect.NewRequest(&orcv1.AddFeedbackRequest{
		TaskId: taskID,
		Type:   orcv1.FeedbackType_FEEDBACK_TYPE_GENERAL,
		Text:   "Feedback to send",
		Timing: orcv1.FeedbackTiming_FEEDBACK_TIMING_WHEN_DONE,
	}))

	// Send feedback
	sendReq := connect.NewRequest(&orcv1.SendFeedbackRequest{
		TaskId: taskID,
	})

	resp, err := server.SendFeedback(context.Background(), sendReq)
	if err != nil {
		t.Fatalf("SendFeedback failed: %v", err)
	}

	if resp.Msg.SentCount != 1 {
		t.Errorf("SentCount = %d, want 1", resp.Msg.SentCount)
	}

	// List all feedback (including received)
	listResp, _ := server.ListFeedback(context.Background(), connect.NewRequest(&orcv1.ListFeedbackRequest{
		TaskId:         taskID,
		ExcludeReceived: false,
	}))

	if len(listResp.Msg.Feedback) != 1 {
		t.Errorf("expected 1 total feedback item, got %d", len(listResp.Msg.Feedback))
	}

	if !listResp.Msg.Feedback[0].Received {
		t.Error("feedback should be marked as received after SendFeedback")
	}
}

// TestSendFeedback_SetsSentAt verifies SC-6:
// SendFeedback sets the SentAt timestamp.
func TestSendFeedback_SetsSentAt(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	server := NewFeedbackServer(backend, nil, nil)

	// Create task
	taskServer := NewTaskServer(backend, nil, nil, nil, "", nil, nil)
	taskResp, _ := taskServer.CreateTask(context.Background(), connect.NewRequest(&orcv1.CreateTaskRequest{
		Title:  "Test task",
	}))
	taskID := taskResp.Msg.Task.Id

	// Add feedback
	_, _ = server.AddFeedback(context.Background(), connect.NewRequest(&orcv1.AddFeedbackRequest{
		TaskId: taskID,
		Type:   orcv1.FeedbackType_FEEDBACK_TYPE_GENERAL,
		Text:   "Feedback to send",
		Timing: orcv1.FeedbackTiming_FEEDBACK_TIMING_WHEN_DONE,
	}))

	// Send feedback
	_, _ = server.SendFeedback(context.Background(), connect.NewRequest(&orcv1.SendFeedbackRequest{
		TaskId: taskID,
	}))

	// List feedback
	listResp, _ := server.ListFeedback(context.Background(), connect.NewRequest(&orcv1.ListFeedbackRequest{
		TaskId: taskID,
	}))

	if listResp.Msg.Feedback[0].SentAt == nil {
		t.Error("feedback.SentAt should be set after SendFeedback")
	}
}

// TestSendFeedback_NoOpWhenEmpty verifies SC-6 edge case:
// SendFeedback is a no-op when no pending feedback exists.
func TestSendFeedback_NoOpWhenEmpty(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	server := NewFeedbackServer(backend, nil, nil)

	// Create task
	taskServer := NewTaskServer(backend, nil, nil, nil, "", nil, nil)
	taskResp, _ := taskServer.CreateTask(context.Background(), connect.NewRequest(&orcv1.CreateTaskRequest{
		Title:  "Test task",
	}))
	taskID := taskResp.Msg.Task.Id

	// Send feedback without adding any
	sendReq := connect.NewRequest(&orcv1.SendFeedbackRequest{
		TaskId: taskID,
	})

	resp, err := server.SendFeedback(context.Background(), sendReq)
	if err != nil {
		t.Fatalf("SendFeedback should not error on empty queue: %v", err)
	}

	if resp.Msg.SentCount != 0 {
		t.Errorf("SentCount = %d, want 0 for empty queue", resp.Msg.SentCount)
	}
}

// ============================================================================
// SC-7: Feedback is persisted to database
// ============================================================================

// TestAddFeedback_PersistsToDatabase verifies SC-7:
// Feedback survives server restart (persists to database).
func TestAddFeedback_PersistsToDatabase(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	server := NewFeedbackServer(backend, nil, nil)

	// Create task
	taskServer := NewTaskServer(backend, nil, nil, nil, "", nil, nil)
	taskResp, _ := taskServer.CreateTask(context.Background(), connect.NewRequest(&orcv1.CreateTaskRequest{
		Title:  "Test task",
	}))
	taskID := taskResp.Msg.Task.Id

	// Add feedback
	_, err := server.AddFeedback(context.Background(), connect.NewRequest(&orcv1.AddFeedbackRequest{
		TaskId: taskID,
		Type:   orcv1.FeedbackType_FEEDBACK_TYPE_GENERAL,
		Text:   "Persisted feedback",
		Timing: orcv1.FeedbackTiming_FEEDBACK_TIMING_WHEN_DONE,
	}))
	if err != nil {
		t.Fatalf("AddFeedback failed: %v", err)
	}

	// Create new server instance (simulates restart)
	server2 := NewFeedbackServer(backend, nil, nil)

	// List feedback from new server
	listResp, err := server2.ListFeedback(context.Background(), connect.NewRequest(&orcv1.ListFeedbackRequest{
		TaskId: taskID,
	}))
	if err != nil {
		t.Fatalf("ListFeedback on new server failed: %v", err)
	}

	if len(listResp.Msg.Feedback) != 1 {
		t.Errorf("expected 1 feedback item after restart, got %d", len(listResp.Msg.Feedback))
	}

	if listResp.Msg.Feedback[0].Text != "Persisted feedback" {
		t.Errorf("feedback.Text = %q, want %q", listResp.Msg.Feedback[0].Text, "Persisted feedback")
	}
}

// ============================================================================
// SC-8: "Send Now" timing triggers task pause (integration test)
// ============================================================================

// TestAddFeedback_NowTiming_PausesTask verifies SC-8:
// When feedback with "now" timing is added and the task is running,
// the task is paused so the feedback can be delivered immediately.
func TestAddFeedback_NowTiming_PausesTask(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	// Track if pause was called
	pauseCalled := false
	mockPauser := &mockTaskPauser{
		pauseFunc: func(taskID, projectID string) error {
			pauseCalled = true
			return nil
		},
	}

	server := NewFeedbackServerWithPauser(backend, nil, nil, mockPauser)

	// Create task and set it to running
	taskServer := NewTaskServer(backend, nil, nil, nil, "", nil, nil)
	taskResp, _ := taskServer.CreateTask(context.Background(), connect.NewRequest(&orcv1.CreateTaskRequest{
		Title:  "Test task",
	}))
	taskID := taskResp.Msg.Task.Id

	// Set task to running status
	task, _ := backend.LoadTask(taskID)
	task.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	_ = backend.SaveTask(task)

	// Add feedback with NOW timing
	req := connect.NewRequest(&orcv1.AddFeedbackRequest{
		TaskId: taskID,
		Type:   orcv1.FeedbackType_FEEDBACK_TYPE_DIRECTION,
		Text:   "Stop! Wrong approach",
		Timing: orcv1.FeedbackTiming_FEEDBACK_TIMING_NOW,
	})

	_, err := server.AddFeedback(context.Background(), req)
	if err != nil {
		t.Fatalf("AddFeedback with NOW timing failed: %v", err)
	}

	if !pauseCalled {
		t.Error("task should be paused when NOW timing feedback is added to running task")
	}
}

// TestAddFeedback_NowTiming_DoesNotPauseNonRunningTask verifies SC-8 edge case:
// NOW timing feedback does not pause a task that is not running.
func TestAddFeedback_NowTiming_DoesNotPauseNonRunningTask(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	pauseCalled := false
	mockPauser := &mockTaskPauser{
		pauseFunc: func(taskID, projectID string) error {
			pauseCalled = true
			return nil
		},
	}

	server := NewFeedbackServerWithPauser(backend, nil, nil, mockPauser)

	// Create task (default status is CREATED, not RUNNING)
	taskServer := NewTaskServer(backend, nil, nil, nil, "", nil, nil)
	taskResp, _ := taskServer.CreateTask(context.Background(), connect.NewRequest(&orcv1.CreateTaskRequest{
		Title:  "Test task",
	}))
	taskID := taskResp.Msg.Task.Id

	// Add feedback with NOW timing (task is not running)
	req := connect.NewRequest(&orcv1.AddFeedbackRequest{
		TaskId: taskID,
		Type:   orcv1.FeedbackType_FEEDBACK_TYPE_DIRECTION,
		Text:   "Stop! Wrong approach",
		Timing: orcv1.FeedbackTiming_FEEDBACK_TIMING_NOW,
	})

	_, err := server.AddFeedback(context.Background(), req)
	if err != nil {
		t.Fatalf("AddFeedback with NOW timing failed: %v", err)
	}

	if pauseCalled {
		t.Error("task should not be paused when NOW timing feedback is added to non-running task")
	}
}

// ============================================================================
// Edge Cases and Error Paths
// ============================================================================

// TestAddFeedback_RejectsEmptyText verifies error path:
// AddFeedback rejects feedback with empty text.
func TestAddFeedback_RejectsEmptyText(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	server := NewFeedbackServer(backend, nil, nil)

	// Create task
	taskServer := NewTaskServer(backend, nil, nil, nil, "", nil, nil)
	taskResp, _ := taskServer.CreateTask(context.Background(), connect.NewRequest(&orcv1.CreateTaskRequest{
		Title:  "Test task",
	}))
	taskID := taskResp.Msg.Task.Id

	req := connect.NewRequest(&orcv1.AddFeedbackRequest{
		TaskId: taskID,
		Type:   orcv1.FeedbackType_FEEDBACK_TYPE_GENERAL,
		Text:   "", // Empty text
		Timing: orcv1.FeedbackTiming_FEEDBACK_TIMING_WHEN_DONE,
	})

	_, err := server.AddFeedback(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for empty text, got nil")
	}

	connectErr, ok := err.(*connect.Error)
	if !ok {
		t.Fatalf("expected connect.Error, got %T", err)
	}
	if connectErr.Code() != connect.CodeInvalidArgument {
		t.Errorf("error code = %v, want InvalidArgument", connectErr.Code())
	}
}

// TestAddFeedback_RejectsInvalidTaskID verifies error path:
// AddFeedback returns NotFound for non-existent task.
func TestAddFeedback_RejectsInvalidTaskID(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	server := NewFeedbackServer(backend, nil, nil)

	req := connect.NewRequest(&orcv1.AddFeedbackRequest{
		TaskId: "TASK-999999", // Non-existent task
		Type:   orcv1.FeedbackType_FEEDBACK_TYPE_GENERAL,
		Text:   "Some feedback",
		Timing: orcv1.FeedbackTiming_FEEDBACK_TIMING_WHEN_DONE,
	})

	_, err := server.AddFeedback(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for invalid task ID, got nil")
	}

	connectErr, ok := err.(*connect.Error)
	if !ok {
		t.Fatalf("expected connect.Error, got %T", err)
	}
	if connectErr.Code() != connect.CodeNotFound {
		t.Errorf("error code = %v, want NotFound", connectErr.Code())
	}
}

// TestListFeedback_RejectsInvalidTaskID verifies error path:
// ListFeedback returns NotFound for non-existent task.
func TestListFeedback_RejectsInvalidTaskID(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	server := NewFeedbackServer(backend, nil, nil)

	req := connect.NewRequest(&orcv1.ListFeedbackRequest{
		TaskId: "TASK-999999", // Non-existent task
	})

	_, err := server.ListFeedback(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for invalid task ID, got nil")
	}

	connectErr, ok := err.(*connect.Error)
	if !ok {
		t.Fatalf("expected connect.Error, got %T", err)
	}
	if connectErr.Code() != connect.CodeNotFound {
		t.Errorf("error code = %v, want NotFound", connectErr.Code())
	}
}

// TestSendFeedback_RejectsInvalidTaskID verifies error path:
// SendFeedback returns NotFound for non-existent task.
func TestSendFeedback_RejectsInvalidTaskID(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	server := NewFeedbackServer(backend, nil, nil)

	req := connect.NewRequest(&orcv1.SendFeedbackRequest{
		TaskId: "TASK-999999", // Non-existent task
	})

	_, err := server.SendFeedback(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for invalid task ID, got nil")
	}

	connectErr, ok := err.(*connect.Error)
	if !ok {
		t.Fatalf("expected connect.Error, got %T", err)
	}
	if connectErr.Code() != connect.CodeNotFound {
		t.Errorf("error code = %v, want NotFound", connectErr.Code())
	}
}

// TestDeleteFeedback_RemovesFeedback verifies feedback can be deleted:
// DeleteFeedback removes specific feedback item.
func TestDeleteFeedback_RemovesFeedback(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	server := NewFeedbackServer(backend, nil, nil)

	// Create task
	taskServer := NewTaskServer(backend, nil, nil, nil, "", nil, nil)
	taskResp, _ := taskServer.CreateTask(context.Background(), connect.NewRequest(&orcv1.CreateTaskRequest{
		Title:  "Test task",
	}))
	taskID := taskResp.Msg.Task.Id

	// Add feedback
	addResp, _ := server.AddFeedback(context.Background(), connect.NewRequest(&orcv1.AddFeedbackRequest{
		TaskId: taskID,
		Type:   orcv1.FeedbackType_FEEDBACK_TYPE_GENERAL,
		Text:   "Feedback to delete",
		Timing: orcv1.FeedbackTiming_FEEDBACK_TIMING_MANUAL,
	}))
	feedbackID := addResp.Msg.Feedback.Id

	// Delete feedback
	_, err := server.DeleteFeedback(context.Background(), connect.NewRequest(&orcv1.DeleteFeedbackRequest{
		TaskId:     taskID,
		FeedbackId: feedbackID,
	}))
	if err != nil {
		t.Fatalf("DeleteFeedback failed: %v", err)
	}

	// Verify deleted
	listResp, _ := server.ListFeedback(context.Background(), connect.NewRequest(&orcv1.ListFeedbackRequest{
		TaskId: taskID,
	}))

	if len(listResp.Msg.Feedback) != 0 {
		t.Errorf("expected 0 feedback items after delete, got %d", len(listResp.Msg.Feedback))
	}
}

// ============================================================================
// Table-driven test for all feedback type and timing combinations
// ============================================================================

// TestAddFeedback_AllTypesAndTimings is a table-driven test for all valid combinations.
func TestAddFeedback_AllTypesAndTimings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		typ     orcv1.FeedbackType
		timing  orcv1.FeedbackTiming
		file    string
		line    int32
		wantErr bool
	}{
		{
			name:   "general/now",
			typ:    orcv1.FeedbackType_FEEDBACK_TYPE_GENERAL,
			timing: orcv1.FeedbackTiming_FEEDBACK_TIMING_NOW,
		},
		{
			name:   "general/when_done",
			typ:    orcv1.FeedbackType_FEEDBACK_TYPE_GENERAL,
			timing: orcv1.FeedbackTiming_FEEDBACK_TIMING_WHEN_DONE,
		},
		{
			name:   "general/manual",
			typ:    orcv1.FeedbackType_FEEDBACK_TYPE_GENERAL,
			timing: orcv1.FeedbackTiming_FEEDBACK_TIMING_MANUAL,
		},
		{
			name:   "approval/now",
			typ:    orcv1.FeedbackType_FEEDBACK_TYPE_APPROVAL,
			timing: orcv1.FeedbackTiming_FEEDBACK_TIMING_NOW,
		},
		{
			name:   "approval/when_done",
			typ:    orcv1.FeedbackType_FEEDBACK_TYPE_APPROVAL,
			timing: orcv1.FeedbackTiming_FEEDBACK_TIMING_WHEN_DONE,
		},
		{
			name:   "direction/now",
			typ:    orcv1.FeedbackType_FEEDBACK_TYPE_DIRECTION,
			timing: orcv1.FeedbackTiming_FEEDBACK_TIMING_NOW,
		},
		{
			name:   "direction/when_done",
			typ:    orcv1.FeedbackType_FEEDBACK_TYPE_DIRECTION,
			timing: orcv1.FeedbackTiming_FEEDBACK_TIMING_WHEN_DONE,
		},
		{
			name:   "inline/when_done with file and line",
			typ:    orcv1.FeedbackType_FEEDBACK_TYPE_INLINE,
			timing: orcv1.FeedbackTiming_FEEDBACK_TIMING_WHEN_DONE,
			file:   "test.go",
			line:   10,
		},
		{
			name:    "inline/when_done missing file",
			typ:     orcv1.FeedbackType_FEEDBACK_TYPE_INLINE,
			timing:  orcv1.FeedbackTiming_FEEDBACK_TIMING_WHEN_DONE,
			line:    10,
			wantErr: true,
		},
		{
			name:    "inline/when_done missing line",
			typ:     orcv1.FeedbackType_FEEDBACK_TYPE_INLINE,
			timing:  orcv1.FeedbackTiming_FEEDBACK_TIMING_WHEN_DONE,
			file:    "test.go",
			wantErr: true,
		},
		{
			name:    "unspecified type",
			typ:     orcv1.FeedbackType_FEEDBACK_TYPE_UNSPECIFIED,
			timing:  orcv1.FeedbackTiming_FEEDBACK_TIMING_WHEN_DONE,
			wantErr: true,
		},
		{
			name:    "unspecified timing",
			typ:     orcv1.FeedbackType_FEEDBACK_TYPE_GENERAL,
			timing:  orcv1.FeedbackTiming_FEEDBACK_TIMING_UNSPECIFIED,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			backend := storage.NewTestBackend(t)
			server := NewFeedbackServer(backend, nil, nil)

			// Create task
			taskServer := NewTaskServer(backend, nil, nil, nil, "", nil, nil)
			taskResp, _ := taskServer.CreateTask(context.Background(), connect.NewRequest(&orcv1.CreateTaskRequest{
				Title:  "Test task",
					}))
			taskID := taskResp.Msg.Task.Id

			req := connect.NewRequest(&orcv1.AddFeedbackRequest{
				TaskId: taskID,
				Type:   tt.typ,
				Text:   "Test feedback text",
				File:   tt.file,
				Line:   tt.line,
				Timing: tt.timing,
			})

			_, err := server.AddFeedback(context.Background(), req)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for %s, got nil", tt.name)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error for %s: %v", tt.name, err)
				}
			}
		})
	}
}

// ============================================================================
// Mock types for testing
// ============================================================================

// mockTaskPauser implements TaskPauser interface for testing.
type mockTaskPauser struct {
	pauseFunc func(taskID, projectID string) error
}

func (m *mockTaskPauser) PauseTask(taskID, projectID string) error {
	if m.pauseFunc != nil {
		return m.pauseFunc(taskID, projectID)
	}
	return nil
}
