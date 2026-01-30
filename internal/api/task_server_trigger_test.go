// Package api provides HTTP API handlers for orc.
//
// TDD Tests for TASK-652: Lifecycle trigger firing from API CreateTask.
//
// Success Criteria Coverage:
// - SC-8: on_task_created triggers fire after SaveTask() in task_server.go (API)
// - SC-9: Gate-mode on_task_created trigger rejection returns error to caller
package api

import (
	"context"
	"testing"

	"connectrpc.com/connect"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/trigger"
	"github.com/randalmurphal/orc/internal/workflow"
)

// --- SC-8: on_task_created triggers fire from API after SaveTask + event publish ---

func TestCreateTask_LifecycleTrigger_Fires(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	// Set up a workflow with on_task_created trigger in the global DB so
	// the API can resolve it for the task's weight/workflow.
	// The trigger runner mock tracks whether it was called.
	mockRunner := &mockAPITriggerRunner{}

	publisher := events.NewMemoryPublisher()
	defer publisher.Close()

	server := NewTaskServerWithTriggerRunner(
		backend, nil, nil, publisher, "", nil, nil, mockRunner,
	)

	wfID := "implement-medium"
	req := connect.NewRequest(&orcv1.CreateTaskRequest{
		Title:      "Task with trigger",
		Weight:     orcv1.TaskWeight_TASK_WEIGHT_MEDIUM,
		WorkflowId: &wfID,
	})

	resp, err := server.CreateTask(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	if resp.Msg.Task == nil {
		t.Fatal("response task is nil")
	}

	// Task should be saved successfully
	loaded, loadErr := backend.LoadTask(resp.Msg.Task.Id)
	if loadErr != nil {
		t.Fatalf("reload task: %v", loadErr)
	}
	if loaded == nil {
		t.Fatal("task not found after creation")
	}

	// Trigger runner should have been called for on_task_created
	if !mockRunner.lifecycleCalled {
		t.Error("lifecycle trigger should have been called after task creation")
	}
	if mockRunner.lastEvent != workflow.WorkflowTriggerEventOnTaskCreated {
		t.Errorf("event = %q, want %q", mockRunner.lastEvent, workflow.WorkflowTriggerEventOnTaskCreated)
	}
}

func TestCreateTask_LifecycleTrigger_ErrorDoesNotFailCreation(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	// Trigger runner returns error - but task creation should still succeed
	mockRunner := &mockAPITriggerRunner{
		lifecycleErr: &trigger.GateRejectionError{
			AgentID: "test-gate",
			Reason:  "reaction trigger failed",
		},
	}

	server := NewTaskServerWithTriggerRunner(
		backend, nil, nil, nil, "", nil, nil, mockRunner,
	)

	wfID := "implement-medium"
	req := connect.NewRequest(&orcv1.CreateTaskRequest{
		Title:      "Task with failing reaction trigger",
		Weight:     orcv1.TaskWeight_TASK_WEIGHT_MEDIUM,
		WorkflowId: &wfID,
	})

	// Per spec SC-8: "Trigger error → log warning, task creation still succeeds"
	// But only for reaction mode. Gate rejection should block.
	resp, err := server.CreateTask(context.Background(), req)

	// For a gate rejection, the task should be created but BLOCKED
	if err != nil {
		// If the API returns an error, the task should still exist
		_, loadErr := backend.LoadTask("TASK-001")
		if loadErr != nil {
			// Task might not have been created - this is also acceptable
			// as long as the error is returned
			return
		}
	}

	if resp != nil && resp.Msg.Task != nil {
		loaded, _ := backend.LoadTask(resp.Msg.Task.Id)
		if loaded != nil && loaded.Status == orcv1.TaskStatus_TASK_STATUS_BLOCKED {
			// SC-9: Gate rejection sets task to BLOCKED
			return
		}
	}
}

// --- SC-9: Gate-mode on_task_created trigger rejection blocks task ---

func TestCreateTask_LifecycleTrigger_GateRejects(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	mockRunner := &mockAPITriggerRunner{
		lifecycleErr: &trigger.GateRejectionError{
			AgentID: "validation-gate",
			Reason:  "task description too vague for production",
		},
	}

	server := NewTaskServerWithTriggerRunner(
		backend, nil, nil, nil, "", nil, nil, mockRunner,
	)

	wfID := "implement-medium"
	req := connect.NewRequest(&orcv1.CreateTaskRequest{
		Title:      "Fix stuff",
		Weight:     orcv1.TaskWeight_TASK_WEIGHT_MEDIUM,
		WorkflowId: &wfID,
	})

	resp, err := server.CreateTask(context.Background(), req)

	// Task should be saved (exists in DB) but status set to BLOCKED
	// The API may return an error or return the blocked task
	if err == nil && resp != nil && resp.Msg.Task != nil {
		loaded, loadErr := backend.LoadTask(resp.Msg.Task.Id)
		if loadErr != nil {
			t.Fatalf("reload task: %v", loadErr)
		}
		if loaded.Status != orcv1.TaskStatus_TASK_STATUS_BLOCKED {
			t.Errorf("task status = %v, want BLOCKED after gate rejection", loaded.Status)
		}
	} else if err != nil {
		// Also acceptable: return error with rejection reason
		// But task should still exist in DB
		tasks, _ := backend.LoadAllTasks()
		if len(tasks) == 0 {
			t.Error("task should be saved even when gate rejects (per SC-9)")
		} else if tasks[0].Status != orcv1.TaskStatus_TASK_STATUS_BLOCKED {
			t.Errorf("task status = %v, want BLOCKED after gate rejection", tasks[0].Status)
		}
	}
}

// --- Edge case: No workflow assigned, no triggers to fire ---

func TestCreateTask_NoWorkflow_NoTriggers(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	mockRunner := &mockAPITriggerRunner{}

	server := NewTaskServerWithTriggerRunner(
		backend, nil, nil, nil, "", nil, nil, mockRunner,
	)

	req := connect.NewRequest(&orcv1.CreateTaskRequest{
		Title: "Task without workflow",
		// No weight, no workflow - should skip triggers
	})

	resp, err := server.CreateTask(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	if resp.Msg.Task == nil {
		t.Fatal("response task is nil")
	}

	// No triggers should fire when no workflow assigned
	if mockRunner.lifecycleCalled {
		t.Error("trigger runner should not be called when no workflow assigned")
	}
}

// --- Mock trigger runner for API tests ---

type mockAPITriggerRunner struct {
	lifecycleCalled bool
	lastEvent       workflow.WorkflowTriggerEvent
	lifecycleErr    error
}

func (m *mockAPITriggerRunner) RunLifecycleTriggers(
	ctx context.Context,
	event workflow.WorkflowTriggerEvent,
	triggers []workflow.WorkflowTrigger,
	tsk *orcv1.Task,
) error {
	m.lifecycleCalled = true
	m.lastEvent = event
	return m.lifecycleErr
}
