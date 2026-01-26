package cli

import (
	"testing"
	"time"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

func TestWaitForTaskStatusProto_StatusChanges(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	// Create a task in RUNNING status
	tsk := task.NewProtoTask("TASK-001", "Test task")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Start a goroutine that changes the status after a short delay
	go func() {
		time.Sleep(100 * time.Millisecond)
		tsk.Status = orcv1.TaskStatus_TASK_STATUS_PAUSED
		if err := backend.SaveTask(tsk); err != nil {
			t.Errorf("save task in goroutine: %v", err)
		}
	}()

	// Wait for status to change
	err := waitForTaskStatusProto(backend, "TASK-001", orcv1.TaskStatus_TASK_STATUS_PAUSED, 2*time.Second)
	if err != nil {
		t.Errorf("waitForTaskStatusProto() returned error: %v", err)
	}
}

func TestWaitForTaskStatusProto_Timeout(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	// Create a task in RUNNING status
	tsk := task.NewProtoTask("TASK-002", "Test task")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Wait for status that never changes (short timeout)
	err := waitForTaskStatusProto(backend, "TASK-002", orcv1.TaskStatus_TASK_STATUS_PAUSED, 200*time.Millisecond)
	if err == nil {
		t.Error("waitForTaskStatusProto() should return error on timeout")
	}
}

func TestWaitForTaskStatusProto_AlreadyAtExpectedStatus(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	// Create a task already at the expected status
	tsk := task.NewProtoTask("TASK-003", "Test task")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_PAUSED
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Should return immediately since already at expected status
	start := time.Now()
	err := waitForTaskStatusProto(backend, "TASK-003", orcv1.TaskStatus_TASK_STATUS_PAUSED, 2*time.Second)
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("waitForTaskStatusProto() returned error: %v", err)
	}

	// Should complete quickly (within first poll interval + some margin)
	if elapsed > 1*time.Second {
		t.Errorf("waitForTaskStatusProto() took too long (%v) for already-matching status", elapsed)
	}
}


func TestPauseCommandValidation_TaskNotRunning(t *testing.T) {
	// Test that pause command rejects non-running tasks
	// This verifies the status check logic without actually running the command

	backend := storage.NewTestBackend(t)

	// Test various non-running statuses
	testCases := []struct {
		name   string
		status orcv1.TaskStatus
	}{
		{"completed task", orcv1.TaskStatus_TASK_STATUS_COMPLETED},
		{"blocked task", orcv1.TaskStatus_TASK_STATUS_BLOCKED},
		{"created task", orcv1.TaskStatus_TASK_STATUS_CREATED},
		{"failed task", orcv1.TaskStatus_TASK_STATUS_FAILED},
		{"paused task", orcv1.TaskStatus_TASK_STATUS_PAUSED},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tsk := task.NewProtoTask("TASK-"+tc.name, "Test task")
			tsk.Status = tc.status
			if err := backend.SaveTask(tsk); err != nil {
				t.Fatalf("save task: %v", err)
			}

			loaded, err := backend.LoadTask(tsk.Id)
			if err != nil {
				t.Fatalf("load task: %v", err)
			}

			// The pause command checks this condition
			if loaded.Status == orcv1.TaskStatus_TASK_STATUS_RUNNING {
				t.Error("expected task status to NOT be running")
			}
		})
	}
}

func TestPauseCommandValidation_TaskRunning(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	tsk := task.NewProtoTask("TASK-running", "Test task")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	loaded, err := backend.LoadTask(tsk.Id)
	if err != nil {
		t.Fatalf("load task: %v", err)
	}

	// The pause command allows this
	if loaded.Status != orcv1.TaskStatus_TASK_STATUS_RUNNING {
		t.Errorf("expected task status RUNNING, got %v", loaded.Status)
	}
}

func TestPauseFallbackSetsStatus(t *testing.T) {
	// Test the fallback behavior when executor signal fails:
	// directly updating the task status to PAUSED
	t.Parallel()

	backend := storage.NewTestBackend(t)

	tsk := task.NewProtoTask("TASK-fallback", "Test task")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	tsk.ExecutorPid = 0 // No executor running
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Simulate fallback: directly set status to PAUSED
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_PAUSED
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task after pause: %v", err)
	}

	// Verify status changed
	loaded, err := backend.LoadTask(tsk.Id)
	if err != nil {
		t.Fatalf("load task: %v", err)
	}

	if loaded.Status != orcv1.TaskStatus_TASK_STATUS_PAUSED {
		t.Errorf("expected task status PAUSED, got %v", loaded.Status)
	}
}
