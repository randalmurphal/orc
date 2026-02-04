// Package api provides HTTP API handlers for orc.
//
// TDD Tests for TASK-748: Kill weight from task model - API component.
//
// Success Criteria Coverage:
// - SC-7: Remove weight field from CreateTaskRequest proto message
// - SC-8: Remove weight field from UpdateTaskRequest proto message
// - SC-10: Remove weight from all API responses (task list, get task)
//
// These tests verify that weight-related fields are removed from API messages.
// Tests WILL FAIL until implementation removes weight from the proto.
package api

import (
	"context"
	"reflect"
	"testing"

	"connectrpc.com/connect"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/storage"
)

// ============================================================================
// SC-7: CreateTaskRequest should NOT have weight field
// ============================================================================

// TestCreateTaskRequest_WeightFieldRemoved verifies SC-7:
// The CreateTaskRequest proto message should not have a Weight field.
func TestCreateTaskRequest_WeightFieldRemoved(t *testing.T) {
	t.Parallel()

	reqType := reflect.TypeFor[orcv1.CreateTaskRequest]()

	// Look for Weight field - it should NOT exist after implementation
	_, hasWeight := reqType.FieldByName("Weight")
	if hasWeight {
		t.Error("SC-7 FAILED: CreateTaskRequest.Weight field still exists - should be removed from proto")
	}
}

// ============================================================================
// SC-8: UpdateTaskRequest should NOT have weight field
// ============================================================================

// TestUpdateTaskRequest_WeightFieldRemoved verifies SC-8:
// The UpdateTaskRequest proto message should not have a Weight field.
func TestUpdateTaskRequest_WeightFieldRemoved(t *testing.T) {
	t.Parallel()

	reqType := reflect.TypeFor[orcv1.UpdateTaskRequest]()

	// Look for Weight field - it should NOT exist after implementation
	_, hasWeight := reqType.FieldByName("Weight")
	if hasWeight {
		t.Error("SC-8 FAILED: UpdateTaskRequest.Weight field still exists - should be removed from proto")
	}
}

// ============================================================================
// SC-10: API responses should NOT contain weight
// ============================================================================

// TestCreateTask_ResponseNoWeight verifies SC-10 (partial):
// CreateTask response should not contain weight field.
func TestCreateTask_ResponseNoWeight(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	server := NewTaskServer(backend, nil, nil, nil, "", nil, nil)

	// Create task using workflow directly (no weight)
	workflowID := "implement-small"
	req := connect.NewRequest(&orcv1.CreateTaskRequest{
		Title:      "Task without weight",
		WorkflowId: &workflowID,
	})

	resp, err := server.CreateTask(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	if resp.Msg.Task == nil {
		t.Fatal("response task is nil")
	}

	// SC-10: Task in response should not have Weight field populated
	// After implementation, the Weight field won't exist at all
	taskType := reflect.TypeFor[orcv1.Task]()
	_, hasWeight := taskType.FieldByName("Weight")
	if hasWeight {
		t.Error("SC-10 FAILED: Task.Weight field still exists in response - should be removed")
	}
}

// TestGetTask_ResponseNoWeight verifies SC-10 (partial):
// GetTask response should not contain weight field.
func TestGetTask_ResponseNoWeight(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	server := NewTaskServer(backend, nil, nil, nil, "", nil, nil)

	// Create a task first
	workflowID := "implement-small"
	createReq := connect.NewRequest(&orcv1.CreateTaskRequest{
		Title:      "Test task for GetTask",
		WorkflowId: &workflowID,
	})
	createResp, err := server.CreateTask(context.Background(), createReq)
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	// Get the task
	getReq := connect.NewRequest(&orcv1.GetTaskRequest{
		TaskId: createResp.Msg.Task.Id,
	})
	getResp, err := server.GetTask(context.Background(), getReq)
	if err != nil {
		t.Fatalf("GetTask failed: %v", err)
	}

	if getResp.Msg.Task == nil {
		t.Fatal("response task is nil")
	}

	// SC-10: Verify Weight field doesn't exist
	taskType := reflect.TypeFor[orcv1.Task]()
	_, hasWeight := taskType.FieldByName("Weight")
	if hasWeight {
		t.Error("SC-10 FAILED: Task.Weight field still exists in GetTask response - should be removed")
	}
}

// TestListTasks_ResponseNoWeight verifies SC-10 (partial):
// ListTasks response should not contain weight field in any task.
func TestListTasks_ResponseNoWeight(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	server := NewTaskServer(backend, nil, nil, nil, "", nil, nil)

	// Create a couple of tasks
	for range 3 {
		workflowID := "implement-small"
		req := connect.NewRequest(&orcv1.CreateTaskRequest{
			Title:      "Test task",
			WorkflowId: &workflowID,
		})
		_, err := server.CreateTask(context.Background(), req)
		if err != nil {
			t.Fatalf("CreateTask failed: %v", err)
		}
	}

	// List all tasks
	listReq := connect.NewRequest(&orcv1.ListTasksRequest{})
	listResp, err := server.ListTasks(context.Background(), listReq)
	if err != nil {
		t.Fatalf("ListTasks failed: %v", err)
	}

	if len(listResp.Msg.Tasks) == 0 {
		t.Fatal("expected tasks in list response")
	}

	// SC-10: Verify Weight field doesn't exist on any task
	taskType := reflect.TypeFor[orcv1.Task]()
	_, hasWeight := taskType.FieldByName("Weight")
	if hasWeight {
		t.Error("SC-10 FAILED: Task.Weight field still exists in ListTasks response - should be removed")
	}
}

// ============================================================================
// Task creation without weight (workflow-first)
// ============================================================================

// TestCreateTask_WorkflowRequired verifies workflow-first model:
// After weight removal, workflow_id becomes the required field for execution.
// Creating a task without workflow should still succeed (can be set later).
func TestCreateTask_WithoutWeight_Succeeds(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	server := NewTaskServer(backend, nil, nil, nil, "", nil, nil)

	// Create task using workflow directly (the new way)
	workflowID := "implement-medium"
	req := connect.NewRequest(&orcv1.CreateTaskRequest{
		Title:      "Task with workflow only",
		WorkflowId: &workflowID,
		// No weight field - this is the new model
	})

	resp, err := server.CreateTask(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	if resp.Msg.Task == nil {
		t.Fatal("response task is nil")
	}

	// Verify workflow was set
	if resp.Msg.Task.WorkflowId == nil {
		t.Error("workflow_id should be set")
	} else if *resp.Msg.Task.WorkflowId != "implement-medium" {
		t.Errorf("workflow_id = %q, want %q", *resp.Msg.Task.WorkflowId, "implement-medium")
	}
}

// TestCreateTask_NoWorkflowNoWeight verifies edge case:
// Creating a task without workflow_id and without weight should still work
// (task can exist in backlog without execution plan).
func TestCreateTask_NoWorkflowNoWeight(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	server := NewTaskServer(backend, nil, nil, nil, "", nil, nil)

	req := connect.NewRequest(&orcv1.CreateTaskRequest{
		Title: "Task without workflow or weight",
		// No workflow_id, no weight - should still create task
	})

	resp, err := server.CreateTask(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	if resp.Msg.Task == nil {
		t.Fatal("response task is nil")
	}

	// Task should exist, just without workflow
	if resp.Msg.Task.Id == "" {
		t.Error("task should have an ID")
	}
}
