// Package api provides HTTP API handlers for orc.
//
// TDD Tests for UpdateTask status and manual_fix handling
//
// Tests for TASK-776: Add UpdateTask handler support for status and manual_fix fields
//
// Success Criteria Coverage:
// - SC-1: UpdateTask processes status field to change task status (FAILED → PAUSED)
// - SC-2: UpdateTask processes status field to change task status (FAILED → CLOSED)
// - SC-3: UpdateTask processes manual_fix flag and sets quality.manual_intervention
// - SC-4: UpdateTask rejects status changes for RUNNING tasks
package api

import (
	"context"
	"testing"

	"connectrpc.com/connect"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/storage"
)

// TestUpdateTask_Status_FailedToPaused tests changing status from FAILED to PAUSED
// SC-1: UpdateTask processes status field to change task status (FAILED → PAUSED)
func TestUpdateTask_Status_FailedToPaused(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	// Create a failed task
	initialTask := &orcv1.Task{
		Id:     "TASK-001",
		Title:  "Failed Task",
		Status: orcv1.TaskStatus_TASK_STATUS_FAILED,
	}
	if err := backend.SaveTask(initialTask); err != nil {
		t.Fatalf("failed to save initial task: %v", err)
	}

	server := NewTaskServer(backend, nil, nil, nil, "", nil, nil)

	// Update status to PAUSED
	statusPaused := orcv1.TaskStatus_TASK_STATUS_PAUSED
	req := connect.NewRequest(&orcv1.UpdateTaskRequest{
		TaskId: "TASK-001",
		Status: &statusPaused,
	})

	resp, err := server.UpdateTask(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify response
	if resp.Msg.Task == nil {
		t.Fatal("response task is nil")
	}
	if resp.Msg.Task.Status != orcv1.TaskStatus_TASK_STATUS_PAUSED {
		t.Errorf("status = %v, want %v", resp.Msg.Task.Status, orcv1.TaskStatus_TASK_STATUS_PAUSED)
	}

	// Verify persistence
	loaded, err := backend.LoadTask("TASK-001")
	if err != nil {
		t.Fatalf("failed to reload task: %v", err)
	}
	if loaded.Status != orcv1.TaskStatus_TASK_STATUS_PAUSED {
		t.Errorf("persisted status = %v, want %v", loaded.Status, orcv1.TaskStatus_TASK_STATUS_PAUSED)
	}
}

// TestUpdateTask_Status_FailedToClosed tests changing status from FAILED to CLOSED
// SC-2: UpdateTask processes status field to change task status (FAILED → CLOSED)
func TestUpdateTask_Status_FailedToClosed(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	// Create a failed task
	initialTask := &orcv1.Task{
		Id:     "TASK-002",
		Title:  "Failed Task to Close",
		Status: orcv1.TaskStatus_TASK_STATUS_FAILED,
	}
	if err := backend.SaveTask(initialTask); err != nil {
		t.Fatalf("failed to save initial task: %v", err)
	}

	server := NewTaskServer(backend, nil, nil, nil, "", nil, nil)

	// Update status to CLOSED
	statusClosed := orcv1.TaskStatus_TASK_STATUS_CLOSED
	req := connect.NewRequest(&orcv1.UpdateTaskRequest{
		TaskId: "TASK-002",
		Status: &statusClosed,
	})

	resp, err := server.UpdateTask(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify response
	if resp.Msg.Task == nil {
		t.Fatal("response task is nil")
	}
	if resp.Msg.Task.Status != orcv1.TaskStatus_TASK_STATUS_CLOSED {
		t.Errorf("status = %v, want %v", resp.Msg.Task.Status, orcv1.TaskStatus_TASK_STATUS_CLOSED)
	}

	// Verify persistence
	loaded, err := backend.LoadTask("TASK-002")
	if err != nil {
		t.Fatalf("failed to reload task: %v", err)
	}
	if loaded.Status != orcv1.TaskStatus_TASK_STATUS_CLOSED {
		t.Errorf("persisted status = %v, want %v", loaded.Status, orcv1.TaskStatus_TASK_STATUS_CLOSED)
	}
}

// TestUpdateTask_ManualFix tests setting manual_fix flag
// SC-3: UpdateTask processes manual_fix flag and sets quality.manual_intervention
func TestUpdateTask_ManualFix(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	// Create a failed task without quality metrics
	initialTask := &orcv1.Task{
		Id:     "TASK-003",
		Title:  "Failed Task for Manual Fix",
		Status: orcv1.TaskStatus_TASK_STATUS_FAILED,
	}
	if err := backend.SaveTask(initialTask); err != nil {
		t.Fatalf("failed to save initial task: %v", err)
	}

	server := NewTaskServer(backend, nil, nil, nil, "", nil, nil)

	// Set manual_fix=true and status=PAUSED
	statusPaused := orcv1.TaskStatus_TASK_STATUS_PAUSED
	manualFix := true
	req := connect.NewRequest(&orcv1.UpdateTaskRequest{
		TaskId:    "TASK-003",
		Status:    &statusPaused,
		ManualFix: &manualFix,
	})

	resp, err := server.UpdateTask(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify response
	if resp.Msg.Task == nil {
		t.Fatal("response task is nil")
	}
	if resp.Msg.Task.Status != orcv1.TaskStatus_TASK_STATUS_PAUSED {
		t.Errorf("status = %v, want %v", resp.Msg.Task.Status, orcv1.TaskStatus_TASK_STATUS_PAUSED)
	}
	if resp.Msg.Task.Quality == nil {
		t.Fatal("quality is nil after setting manual_fix")
	}
	if !resp.Msg.Task.Quality.ManualIntervention {
		t.Error("quality.manual_intervention = false, want true")
	}

	// Verify persistence
	loaded, err := backend.LoadTask("TASK-003")
	if err != nil {
		t.Fatalf("failed to reload task: %v", err)
	}
	if loaded.Quality == nil {
		t.Fatal("persisted quality is nil")
	}
	if !loaded.Quality.ManualIntervention {
		t.Error("persisted quality.manual_intervention = false, want true")
	}
}

// TestUpdateTask_ManualFix_ExistingQuality tests manual_fix with existing quality metrics
func TestUpdateTask_ManualFix_ExistingQuality(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	// Create a task with existing quality metrics
	initialTask := &orcv1.Task{
		Id:     "TASK-004",
		Title:  "Task with Existing Quality",
		Status: orcv1.TaskStatus_TASK_STATUS_FAILED,
		Quality: &orcv1.QualityMetrics{
			ReviewRejections: 2,
			TotalRetries:     3,
		},
	}
	if err := backend.SaveTask(initialTask); err != nil {
		t.Fatalf("failed to save initial task: %v", err)
	}

	server := NewTaskServer(backend, nil, nil, nil, "", nil, nil)

	// Set manual_fix=true (should preserve existing quality fields)
	statusPaused := orcv1.TaskStatus_TASK_STATUS_PAUSED
	manualFix := true
	req := connect.NewRequest(&orcv1.UpdateTaskRequest{
		TaskId:    "TASK-004",
		Status:    &statusPaused,
		ManualFix: &manualFix,
	})

	resp, err := server.UpdateTask(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify response preserves existing quality fields
	if resp.Msg.Task.Quality == nil {
		t.Fatal("quality is nil")
	}
	if resp.Msg.Task.Quality.ReviewRejections != 2 {
		t.Errorf("review_rejections = %d, want 2", resp.Msg.Task.Quality.ReviewRejections)
	}
	if resp.Msg.Task.Quality.TotalRetries != 3 {
		t.Errorf("total_retries = %d, want 3", resp.Msg.Task.Quality.TotalRetries)
	}
	if !resp.Msg.Task.Quality.ManualIntervention {
		t.Error("manual_intervention = false, want true")
	}
}

// TestUpdateTask_Status_RejectsRunningTask tests that status cannot be changed for running tasks
// SC-4: UpdateTask rejects status changes for RUNNING tasks
func TestUpdateTask_Status_RejectsRunningTask(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	// Create a running task
	initialTask := &orcv1.Task{
		Id:     "TASK-005",
		Title:  "Running Task",
		Status: orcv1.TaskStatus_TASK_STATUS_RUNNING,
	}
	if err := backend.SaveTask(initialTask); err != nil {
		t.Fatalf("failed to save initial task: %v", err)
	}

	server := NewTaskServer(backend, nil, nil, nil, "", nil, nil)

	// Try to change status to PAUSED
	statusPaused := orcv1.TaskStatus_TASK_STATUS_PAUSED
	req := connect.NewRequest(&orcv1.UpdateTaskRequest{
		TaskId: "TASK-005",
		Status: &statusPaused,
	})

	_, err := server.UpdateTask(context.Background(), req)
	if err == nil {
		t.Fatal("expected error when changing status of running task")
	}

	connectErr, ok := err.(*connect.Error)
	if !ok {
		t.Fatalf("expected connect.Error, got %T", err)
	}
	if connectErr.Code() != connect.CodeFailedPrecondition {
		t.Errorf("expected CodeFailedPrecondition, got %v", connectErr.Code())
	}
}

// TestUpdateTask_Status_NoChangeWhenNotProvided tests that status is preserved when not in request
func TestUpdateTask_Status_NoChangeWhenNotProvided(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	// Create a task with specific status
	initialTask := &orcv1.Task{
		Id:     "TASK-006",
		Title:  "Task with Status",
		Status: orcv1.TaskStatus_TASK_STATUS_BLOCKED,
	}
	if err := backend.SaveTask(initialTask); err != nil {
		t.Fatalf("failed to save initial task: %v", err)
	}

	server := NewTaskServer(backend, nil, nil, nil, "", nil, nil)

	// Update only title, no status in request
	req := connect.NewRequest(&orcv1.UpdateTaskRequest{
		TaskId: "TASK-006",
		Title:  strPtr("Updated Title"),
		// Status is nil - should preserve existing
	})

	resp, err := server.UpdateTask(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify status is preserved
	if resp.Msg.Task.Status != orcv1.TaskStatus_TASK_STATUS_BLOCKED {
		t.Errorf("status = %v, want %v (preserved)", resp.Msg.Task.Status, orcv1.TaskStatus_TASK_STATUS_BLOCKED)
	}
}

// TestProtoSchema_UpdateTaskRequest_HasStatusField tests that proto schema includes status field
func TestProtoSchema_UpdateTaskRequest_HasStatusField(t *testing.T) {
	t.Parallel()

	// This test verifies the proto schema has the status field
	// If the field doesn't exist, this test won't compile
	status := orcv1.TaskStatus_TASK_STATUS_PAUSED
	req := &orcv1.UpdateTaskRequest{
		TaskId: "TASK-TEST",
		Status: &status,
	}

	if req.Status == nil {
		t.Error("Status field should be settable")
	}
	if *req.Status != orcv1.TaskStatus_TASK_STATUS_PAUSED {
		t.Errorf("Status = %v, want %v", *req.Status, orcv1.TaskStatus_TASK_STATUS_PAUSED)
	}
}

// TestProtoSchema_UpdateTaskRequest_HasManualFixField tests that proto schema includes manual_fix field
func TestProtoSchema_UpdateTaskRequest_HasManualFixField(t *testing.T) {
	t.Parallel()

	// This test verifies the proto schema has the manual_fix field
	// If the field doesn't exist, this test won't compile
	manualFix := true
	req := &orcv1.UpdateTaskRequest{
		TaskId:    "TASK-TEST",
		ManualFix: &manualFix,
	}

	if req.ManualFix == nil {
		t.Error("ManualFix field should be settable")
	}
	if !*req.ManualFix {
		t.Error("ManualFix = false, want true")
	}
}
