// Package api provides HTTP API handlers for orc.
//
// TDD Tests for UpdateTask workflow_id handling
//
// Tests for TASK-536: Add workflow_id to UpdateTaskRequest
//
// Success Criteria Coverage:
// - SC-5: Proto schema includes workflow_id in UpdateTaskRequest (build check)
// - SC-6: Backend UpdateTask handler processes workflow_id changes
package api

import (
	"context"
	"testing"

	"connectrpc.com/connect"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/storage"
)

// TestUpdateTask_WorkflowId tests the UpdateTask handler's workflow_id processing
// SC-6: Backend UpdateTask handler processes workflow_id changes
func TestUpdateTask_WorkflowId(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		initialTask    *orcv1.Task
		request        *orcv1.UpdateTaskRequest
		wantWorkflowId string
		wantErr        bool
		wantErrCode    connect.Code
	}{
		{
			name: "update workflow_id to valid value",
			initialTask: &orcv1.Task{
				Id:         "TASK-001",
				Title:      "Test Task",
				Weight:     orcv1.TaskWeight_TASK_WEIGHT_MEDIUM,
				Status:     orcv1.TaskStatus_TASK_STATUS_CREATED,
				WorkflowId: strPtr("small"),
			},
			request: &orcv1.UpdateTaskRequest{
				TaskId:         "TASK-001",
				WorkflowId: strPtr("medium"),
			},
			wantWorkflowId: "medium",
			wantErr:        false,
		},
		{
			name: "set workflow_id from nil",
			initialTask: &orcv1.Task{
				Id:         "TASK-002",
				Title:      "Task Without Workflow",
				Weight:     orcv1.TaskWeight_TASK_WEIGHT_SMALL,
				Status:     orcv1.TaskStatus_TASK_STATUS_CREATED,
				WorkflowId: nil,
			},
			request: &orcv1.UpdateTaskRequest{
				TaskId:         "TASK-002",
				WorkflowId: strPtr("small"),
			},
			wantWorkflowId: "small",
			wantErr:        false,
		},
		{
			name: "clear workflow_id with empty string",
			initialTask: &orcv1.Task{
				Id:         "TASK-003",
				Title:      "Task With Workflow",
				Weight:     orcv1.TaskWeight_TASK_WEIGHT_MEDIUM,
				Status:     orcv1.TaskStatus_TASK_STATUS_CREATED,
				WorkflowId: strPtr("medium"),
			},
			request: &orcv1.UpdateTaskRequest{
				TaskId:         "TASK-003",
				WorkflowId: strPtr(""),
			},
			wantWorkflowId: "",
			wantErr:        false,
		},
		{
			name: "preserve workflow when field not in request",
			initialTask: &orcv1.Task{
				Id:         "TASK-004",
				Title:      "Test Task",
				Weight:     orcv1.TaskWeight_TASK_WEIGHT_MEDIUM,
				Status:     orcv1.TaskStatus_TASK_STATUS_CREATED,
				WorkflowId: strPtr("medium"),
			},
			request: &orcv1.UpdateTaskRequest{
				TaskId:    "TASK-004",
				Title: strPtr("Updated Title"),
				// WorkflowId is nil - should preserve existing
			},
			wantWorkflowId: "medium",
			wantErr:        false,
		},
		{
			name: "update to custom workflow id",
			initialTask: &orcv1.Task{
				Id:         "TASK-005",
				Title:      "Custom Workflow Test",
				Weight:     orcv1.TaskWeight_TASK_WEIGHT_MEDIUM,
				Status:     orcv1.TaskStatus_TASK_STATUS_CREATED,
				WorkflowId: strPtr("small"),
			},
			request: &orcv1.UpdateTaskRequest{
				TaskId:         "TASK-005",
				WorkflowId: strPtr("custom-workflow"),
			},
			wantWorkflowId: "custom-workflow",
			wantErr:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create test backend
			backend := storage.NewTestBackend(t)

			// Save initial task
			if err := backend.SaveTask(tt.initialTask); err != nil {
				t.Fatalf("failed to save initial task: %v", err)
			}

			// Create server
			server := NewTaskServer(backend, nil, nil, nil, "", nil, nil)

			// Execute request
			req := connect.NewRequest(tt.request)
			resp, err := server.UpdateTask(context.Background(), req)

			// Check error expectations
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error but got none")
				}
				if connectErr, ok := err.(*connect.Error); ok {
					if connectErr.Code() != tt.wantErrCode {
						t.Errorf("expected error code %v, got %v", tt.wantErrCode, connectErr.Code())
					}
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Verify workflow_id in response
			if resp.Msg.Task == nil {
				t.Fatal("response task is nil")
			}

			gotWorkflowId := ""
			if resp.Msg.Task.WorkflowId != nil {
				gotWorkflowId = *resp.Msg.Task.WorkflowId
			}

			if gotWorkflowId != tt.wantWorkflowId {
				t.Errorf("workflow_id = %q, want %q", gotWorkflowId, tt.wantWorkflowId)
			}

			// Verify persistence
			loaded, err := backend.LoadTask(tt.request.TaskId)
			if err != nil {
				t.Fatalf("failed to reload task: %v", err)
			}

			loadedWorkflowId := ""
			if loaded.WorkflowId != nil {
				loadedWorkflowId = *loaded.WorkflowId
			}

			if loadedWorkflowId != tt.wantWorkflowId {
				t.Errorf("persisted workflow_id = %q, want %q", loadedWorkflowId, tt.wantWorkflowId)
			}
		})
	}
}

// TestUpdateTask_WorkflowId_TaskNotFound tests error when task doesn't exist
func TestUpdateTask_WorkflowId_TaskNotFound(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	server := NewTaskServer(backend, nil, nil, nil, "", nil, nil)

	req := connect.NewRequest(&orcv1.UpdateTaskRequest{
		TaskId:         "TASK-NONEXISTENT",
		WorkflowId: strPtr("medium"),
	})

	_, err := server.UpdateTask(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for non-existent task")
	}

	if connectErr, ok := err.(*connect.Error); ok {
		if connectErr.Code() != connect.CodeNotFound {
			t.Errorf("expected CodeNotFound, got %v", connectErr.Code())
		}
	}
}

// TestUpdateTask_WorkflowId_MultipleTasks verifies workflow updates don't affect other tasks
func TestUpdateTask_WorkflowId_MultipleTasks(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	// Create two tasks with different workflows
	task1 := &orcv1.Task{
		Id:         "TASK-001",
		Title:      "Task 1",
		Weight:     orcv1.TaskWeight_TASK_WEIGHT_MEDIUM,
		Status:     orcv1.TaskStatus_TASK_STATUS_CREATED,
		WorkflowId: strPtr("small"),
	}
	task2 := &orcv1.Task{
		Id:         "TASK-002",
		Title:      "Task 2",
		Weight:     orcv1.TaskWeight_TASK_WEIGHT_MEDIUM,
		Status:     orcv1.TaskStatus_TASK_STATUS_CREATED,
		WorkflowId: strPtr("large"),
	}

	if err := backend.SaveTask(task1); err != nil {
		t.Fatalf("failed to save task 1: %v", err)
	}
	if err := backend.SaveTask(task2); err != nil {
		t.Fatalf("failed to save task 2: %v", err)
	}

	server := NewTaskServer(backend, nil, nil, nil, "", nil, nil)

	// Update task1's workflow
	req := connect.NewRequest(&orcv1.UpdateTaskRequest{
		TaskId:         "TASK-001",
		WorkflowId: strPtr("medium"),
	})
	_, err := server.UpdateTask(context.Background(), req)
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}

	// Verify task1 was updated
	loaded1, err := backend.LoadTask("TASK-001")
	if err != nil {
		t.Fatalf("failed to reload task 1: %v", err)
	}
	if loaded1.WorkflowId == nil || *loaded1.WorkflowId != "medium" {
		t.Errorf("task1 workflow_id = %v, want %q", loaded1.WorkflowId, "medium")
	}

	// Verify task2 was NOT affected
	loaded2, err := backend.LoadTask("TASK-002")
	if err != nil {
		t.Fatalf("failed to reload task 2: %v", err)
	}
	if loaded2.WorkflowId == nil || *loaded2.WorkflowId != "large" {
		t.Errorf("task2 workflow_id = %v, want %q (should be unchanged)", loaded2.WorkflowId, "large")
	}
}

// TestUpdateTask_WorkflowId_WithOtherFieldUpdates tests workflow update combined with other fields
func TestUpdateTask_WorkflowId_WithOtherFieldUpdates(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	task := &orcv1.Task{
		Id:         "TASK-001",
		Title:      "Original Title",
		Weight:     orcv1.TaskWeight_TASK_WEIGHT_MEDIUM,
		Status:     orcv1.TaskStatus_TASK_STATUS_CREATED,
		WorkflowId: strPtr("small"),
	}

	if err := backend.SaveTask(task); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	server := NewTaskServer(backend, nil, nil, nil, "", nil, nil)

	// Update multiple fields including workflow
	req := connect.NewRequest(&orcv1.UpdateTaskRequest{
		TaskId:         "TASK-001",
		Title:      strPtr("Updated Title"),
		WorkflowId: strPtr("large"),
	})
	resp, err := server.UpdateTask(context.Background(), req)
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}

	// Verify both fields updated
	if resp.Msg.Task.Title != "Updated Title" {
		t.Errorf("title = %q, want %q", resp.Msg.Task.Title, "Updated Title")
	}
	if resp.Msg.Task.WorkflowId == nil || *resp.Msg.Task.WorkflowId != "large" {
		t.Errorf("workflow_id = %v, want %q", resp.Msg.Task.WorkflowId, "large")
	}
}

// TestProtoSchema_UpdateTaskRequest_HasWorkflowId is a compile-time check
// that the proto schema includes workflow_id field (SC-5)
func TestProtoSchema_UpdateTaskRequest_HasWorkflowId(t *testing.T) {
	t.Parallel()

	// This test verifies the proto schema has workflow_id field.
	// If this compiles, the field exists. This will fail to compile
	// until the proto is regenerated with workflow_id field.
	req := &orcv1.UpdateTaskRequest{
		TaskId:         "TASK-001",
		WorkflowId: strPtr("medium"),
	}

	if req.WorkflowId == nil {
		t.Error("WorkflowId should not be nil when set")
	}

	if *req.WorkflowId != "medium" {
		t.Errorf("WorkflowId = %q, want %q", *req.WorkflowId, "medium")
	}
}

// strPtr returns a pointer to the given string (helper for optional proto fields)
func strPtr(s string) *string {
	return &s
}
