// Package api provides HTTP API handlers for orc.
//
// TDD Tests for TASK-590: CreateTask workflow_id auto-assignment
//
// These tests verify the CreateTask API endpoint auto-assigns workflow_id
// based on task weight when workflow_id is not explicitly provided.
//
// Success Criteria Coverage:
// - SC-5: API CreateTask auto-assigns workflow based on weight when workflow_id is not provided
//
// Edge Cases:
// - Explicit workflow_id takes precedence over weight-derived
// - Unspecified weight does not auto-assign workflow
// - Medium weight (default) gets implement-medium
package api

import (
	"context"
	"testing"

	"connectrpc.com/connect"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/storage"
)

// ============================================================================
// SC-5: API CreateTask auto-assigns workflow based on weight
// ============================================================================

// TestCreateTask_AutoAssignsWorkflow_Small verifies SC-5 for small weight:
// When CreateTask is called with weight=small and no workflow_id,
// the task is created with workflow_id="implement-small"
func TestCreateTask_AutoAssignsWorkflow_Small(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	server := NewTaskServer(backend, nil, nil, nil, "", nil, nil)

	req := connect.NewRequest(&orcv1.CreateTaskRequest{
		Title:  "Task with small weight",
		Weight: orcv1.TaskWeight_TASK_WEIGHT_SMALL,
		// workflow_id is NOT provided - should be auto-assigned
	})

	resp, err := server.CreateTask(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	if resp.Msg.Task == nil {
		t.Fatal("response task is nil")
	}

	// Verify workflow_id was auto-assigned based on weight
	if resp.Msg.Task.WorkflowId == nil {
		t.Error("workflow_id should be auto-assigned for small weight, got nil")
	} else if *resp.Msg.Task.WorkflowId != "implement-small" {
		t.Errorf("workflow_id = %q, want %q", *resp.Msg.Task.WorkflowId, "implement-small")
	}

	// Verify persistence
	loaded, err := backend.LoadTask(resp.Msg.Task.Id)
	if err != nil {
		t.Fatalf("reload task: %v", err)
	}

	if loaded.WorkflowId == nil {
		t.Error("persisted workflow_id should not be nil")
	} else if *loaded.WorkflowId != "implement-small" {
		t.Errorf("persisted workflow_id = %q, want %q", *loaded.WorkflowId, "implement-small")
	}
}

// TestCreateTask_AutoAssignsWorkflow_Medium verifies SC-5 for medium weight:
// When CreateTask is called with weight=medium and no workflow_id,
// the task is created with workflow_id="implement-medium"
func TestCreateTask_AutoAssignsWorkflow_Medium(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	server := NewTaskServer(backend, nil, nil, nil, "", nil, nil)

	req := connect.NewRequest(&orcv1.CreateTaskRequest{
		Title:  "Task with medium weight",
		Weight: orcv1.TaskWeight_TASK_WEIGHT_MEDIUM,
		// workflow_id is NOT provided - should be auto-assigned
	})

	resp, err := server.CreateTask(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	if resp.Msg.Task == nil {
		t.Fatal("response task is nil")
	}

	// Verify workflow_id was auto-assigned based on weight
	if resp.Msg.Task.WorkflowId == nil {
		t.Error("workflow_id should be auto-assigned for medium weight, got nil")
	} else if *resp.Msg.Task.WorkflowId != "implement-medium" {
		t.Errorf("workflow_id = %q, want %q", *resp.Msg.Task.WorkflowId, "implement-medium")
	}
}

// TestCreateTask_AutoAssignsWorkflow_Trivial verifies trivial weight auto-assignment
func TestCreateTask_AutoAssignsWorkflow_Trivial(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	server := NewTaskServer(backend, nil, nil, nil, "", nil, nil)

	req := connect.NewRequest(&orcv1.CreateTaskRequest{
		Title:  "Trivial task",
		Weight: orcv1.TaskWeight_TASK_WEIGHT_TRIVIAL,
	})

	resp, err := server.CreateTask(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	if resp.Msg.Task.WorkflowId == nil {
		t.Error("workflow_id should be auto-assigned for trivial weight, got nil")
	} else if *resp.Msg.Task.WorkflowId != "implement-trivial" {
		t.Errorf("workflow_id = %q, want %q", *resp.Msg.Task.WorkflowId, "implement-trivial")
	}
}

// TestCreateTask_AutoAssignsWorkflow_Large verifies large weight auto-assignment
func TestCreateTask_AutoAssignsWorkflow_Large(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	server := NewTaskServer(backend, nil, nil, nil, "", nil, nil)

	req := connect.NewRequest(&orcv1.CreateTaskRequest{
		Title:  "Large task",
		Weight: orcv1.TaskWeight_TASK_WEIGHT_LARGE,
	})

	resp, err := server.CreateTask(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	if resp.Msg.Task.WorkflowId == nil {
		t.Error("workflow_id should be auto-assigned for large weight, got nil")
	} else if *resp.Msg.Task.WorkflowId != "implement-large" {
		t.Errorf("workflow_id = %q, want %q", *resp.Msg.Task.WorkflowId, "implement-large")
	}
}

// ============================================================================
// Edge Cases
// ============================================================================

// TestCreateTask_ExplicitWorkflow_TakesPrecedence verifies edge case:
// When both weight and explicit workflow_id are provided, explicit wins
func TestCreateTask_ExplicitWorkflow_TakesPrecedence(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	server := NewTaskServer(backend, nil, nil, nil, "", nil, nil)

	customWorkflow := "custom-workflow"
	req := connect.NewRequest(&orcv1.CreateTaskRequest{
		Title:      "Task with explicit workflow",
		Weight:     orcv1.TaskWeight_TASK_WEIGHT_SMALL, // Would normally get implement-small
		WorkflowId: &customWorkflow,                    // But explicit workflow should win
	})

	resp, err := server.CreateTask(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	// Explicit workflow should take precedence
	if resp.Msg.Task.WorkflowId == nil {
		t.Error("workflow_id should be set to explicit value, got nil")
	} else if *resp.Msg.Task.WorkflowId != "custom-workflow" {
		t.Errorf("workflow_id = %q, want %q (explicit should take precedence)", *resp.Msg.Task.WorkflowId, "custom-workflow")
	}
}

// TestCreateTask_UnspecifiedWeight_NoAutoAssign verifies edge case:
// When weight is unspecified, no workflow is auto-assigned
func TestCreateTask_UnspecifiedWeight_NoAutoAssign(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	server := NewTaskServer(backend, nil, nil, nil, "", nil, nil)

	req := connect.NewRequest(&orcv1.CreateTaskRequest{
		Title:  "Task with unspecified weight",
		Weight: orcv1.TaskWeight_TASK_WEIGHT_UNSPECIFIED,
		// No workflow_id provided
	})

	resp, err := server.CreateTask(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	// Unspecified weight should NOT auto-assign workflow
	if resp.Msg.Task.WorkflowId != nil {
		t.Errorf("workflow_id should be nil for unspecified weight, got %q", *resp.Msg.Task.WorkflowId)
	}
}

// TestCreateTask_EmptyExplicitWorkflow_NoAutoAssign verifies edge case:
// When explicit empty workflow_id is provided, it should be preserved (not overridden)
func TestCreateTask_EmptyExplicitWorkflow_PreservesEmpty(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	server := NewTaskServer(backend, nil, nil, nil, "", nil, nil)

	// Explicitly set empty workflow (user intent to clear)
	emptyWorkflow := ""
	req := connect.NewRequest(&orcv1.CreateTaskRequest{
		Title:      "Task with empty workflow",
		Weight:     orcv1.TaskWeight_TASK_WEIGHT_MEDIUM,
		WorkflowId: &emptyWorkflow,
	})

	resp, err := server.CreateTask(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	// Explicit empty workflow should be preserved, not auto-assigned
	// The user explicitly set it to empty, so we respect that
	if resp.Msg.Task.WorkflowId == nil {
		// This is acceptable - explicit empty could result in nil
		return
	}
	if *resp.Msg.Task.WorkflowId != "" {
		t.Errorf("workflow_id = %q, want empty (explicit empty should be preserved)", *resp.Msg.Task.WorkflowId)
	}
}

// TestCreateTask_AllWeights_AutoAssignment is a table-driven test for all weights
func TestCreateTask_AllWeights_AutoAssignment(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		weight     orcv1.TaskWeight
		wantWfID   string
		wantNilWfI bool // true if workflow_id should be nil
	}{
		{
			name:       "unspecified weight - no auto-assign",
			weight:     orcv1.TaskWeight_TASK_WEIGHT_UNSPECIFIED,
			wantNilWfI: true,
		},
		{
			name:     "trivial weight - implement-trivial",
			weight:   orcv1.TaskWeight_TASK_WEIGHT_TRIVIAL,
			wantWfID: "implement-trivial",
		},
		{
			name:     "small weight - implement-small",
			weight:   orcv1.TaskWeight_TASK_WEIGHT_SMALL,
			wantWfID: "implement-small",
		},
		{
			name:     "medium weight - implement-medium",
			weight:   orcv1.TaskWeight_TASK_WEIGHT_MEDIUM,
			wantWfID: "implement-medium",
		},
		{
			name:     "large weight - implement-large",
			weight:   orcv1.TaskWeight_TASK_WEIGHT_LARGE,
			wantWfID: "implement-large",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			backend := storage.NewTestBackend(t)
			server := NewTaskServer(backend, nil, nil, nil, "", nil, nil)

			req := connect.NewRequest(&orcv1.CreateTaskRequest{
				Title:  "Test task",
				Weight: tt.weight,
			})

			resp, err := server.CreateTask(context.Background(), req)
			if err != nil {
				t.Fatalf("CreateTask failed: %v", err)
			}

			if tt.wantNilWfI {
				if resp.Msg.Task.WorkflowId != nil {
					t.Errorf("workflow_id should be nil, got %q", *resp.Msg.Task.WorkflowId)
				}
				return
			}

			if resp.Msg.Task.WorkflowId == nil {
				t.Errorf("workflow_id should be %q, got nil", tt.wantWfID)
			} else if *resp.Msg.Task.WorkflowId != tt.wantWfID {
				t.Errorf("workflow_id = %q, want %q", *resp.Msg.Task.WorkflowId, tt.wantWfID)
			}
		})
	}
}
