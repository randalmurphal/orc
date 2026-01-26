package cli

import (
	"bytes"
	"os"
	"testing"
	"time"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// withPauseTestDir creates a temp directory with orc initialized and changes to it.
func withPauseTestDir(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()
	if err := config.InitAt(tmpDir, false); err != nil {
		t.Fatalf("failed to init orc: %v", err)
	}

	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working dir: %v", err)
	}

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	t.Cleanup(func() {
		_ = os.Chdir(origWd)
	})

	return tmpDir
}

// createPauseTestBackend creates a backend in the given directory.
func createPauseTestBackend(t *testing.T, dir string) storage.Backend {
	t.Helper()
	backend, err := storage.NewDatabaseBackend(dir, nil)
	if err != nil {
		t.Fatalf("create backend: %v", err)
	}
	return backend
}

func TestPauseCommand_TaskNotFound(t *testing.T) {
	withPauseTestDir(t)

	cmd := newPauseCmd()
	cmd.SetArgs([]string{"TASK-999"})

	var stderr bytes.Buffer
	cmd.SetErr(&stderr)

	err := cmd.Execute()
	if err == nil {
		t.Error("Pause should fail for non-existent task")
	}

	if err != nil && !contains([]string{err.Error()}, "task TASK-999 not found") {
		t.Errorf("Expected 'task not found' error, got: %v", err)
	}
}

func TestPauseCommand_NotRunning(t *testing.T) {
	tests := []struct {
		name        string
		status      orcv1.TaskStatus
		errContains string
	}{
		{
			name:        "paused task",
			status:      orcv1.TaskStatus_TASK_STATUS_PAUSED,
			errContains: "not running",
		},
		{
			name:        "completed task",
			status:      orcv1.TaskStatus_TASK_STATUS_COMPLETED,
			errContains: "not running",
		},
		{
			name:        "created task",
			status:      orcv1.TaskStatus_TASK_STATUS_CREATED,
			errContains: "not running",
		},
		{
			name:        "blocked task",
			status:      orcv1.TaskStatus_TASK_STATUS_BLOCKED,
			errContains: "not running",
		},
		{
			name:        "failed task",
			status:      orcv1.TaskStatus_TASK_STATUS_FAILED,
			errContains: "not running",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := withPauseTestDir(t)
			backend := createPauseTestBackend(t, tmpDir)
			defer func() { _ = backend.Close() }()

			// Create task with specific status
			tk := task.NewProtoTask("TASK-001", "Test task")
			tk.Status = tt.status
			tk.Weight = orcv1.TaskWeight_TASK_WEIGHT_SMALL
			if err := backend.SaveTask(tk); err != nil {
				t.Fatalf("failed to save task: %v", err)
			}

			cmd := newPauseCmd()
			cmd.SetArgs([]string{"TASK-001"})

			var stderr bytes.Buffer
			cmd.SetErr(&stderr)

			err := cmd.Execute()
			if err == nil {
				t.Errorf("Pause should fail for %s task", tt.name)
			}

			if err != nil && !contains([]string{err.Error()}, tt.errContains) {
				t.Errorf("Expected error containing %q, got: %v", tt.errContains, err)
			}
		})
	}
}

func TestPauseCommand_DirectUpdate(t *testing.T) {
	// Tests the fallback path where executor is not alive
	tmpDir := withPauseTestDir(t)
	backend := createPauseTestBackend(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Create running task with no executor info (simulates dead executor)
	tk := task.NewProtoTask("TASK-001", "Test task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	tk.Weight = orcv1.TaskWeight_TASK_WEIGHT_SMALL
	// ExecutorPid defaults to 0, which means dead/orphaned
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	cmd := newPauseCmd()
	cmd.SetArgs([]string{"TASK-001"})

	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	err := cmd.Execute()
	if err != nil {
		t.Errorf("Pause should succeed for running task with dead executor: %v", err)
	}

	// Verify task was paused
	pausedTask, err := backend.LoadTask("TASK-001")
	if err != nil {
		t.Fatalf("failed to load task: %v", err)
	}

	if pausedTask.Status != orcv1.TaskStatus_TASK_STATUS_PAUSED {
		t.Errorf("Expected task status PAUSED, got %v", pausedTask.Status)
	}
}

func TestPauseCommand_RunningWithDeadPID(t *testing.T) {
	// Tests pause when executor PID is set but process is dead
	tmpDir := withPauseTestDir(t)
	backend := createPauseTestBackend(t, tmpDir)
	defer func() { _ = backend.Close() }()

	tk := task.NewProtoTask("TASK-001", "Test task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	tk.Weight = orcv1.TaskWeight_TASK_WEIGHT_SMALL
	// Use a very high PID that shouldn't exist
	tk.ExecutorPid = 99999999
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	cmd := newPauseCmd()
	cmd.SetArgs([]string{"TASK-001"})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	err := cmd.Execute()
	if err != nil {
		t.Errorf("Pause should succeed when executor PID is dead: %v", err)
	}

	// Verify task was paused
	pausedTask, err := backend.LoadTask("TASK-001")
	if err != nil {
		t.Fatalf("failed to load task: %v", err)
	}

	if pausedTask.Status != orcv1.TaskStatus_TASK_STATUS_PAUSED {
		t.Errorf("Expected task status PAUSED, got %v", pausedTask.Status)
	}
}

func TestWaitForTaskStatusProto(t *testing.T) {
	tmpDir := withPauseTestDir(t)
	backend := createPauseTestBackend(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Create a task that's already paused
	tk := task.NewProtoTask("TASK-001", "Test task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_PAUSED
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Should return immediately since status already matches
	err := waitForTaskStatusProto(backend, "TASK-001", orcv1.TaskStatus_TASK_STATUS_PAUSED, 1*time.Second)
	if err != nil {
		t.Errorf("Expected success when status already matches, got: %v", err)
	}
}

func TestWaitForTaskStatusProto_Timeout(t *testing.T) {
	tmpDir := withPauseTestDir(t)
	backend := createPauseTestBackend(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Create a task that's running (won't become paused)
	tk := task.NewProtoTask("TASK-001", "Test task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Should timeout waiting for paused status
	start := time.Now()
	err := waitForTaskStatusProto(backend, "TASK-001", orcv1.TaskStatus_TASK_STATUS_PAUSED, 1*time.Second)
	elapsed := time.Since(start)

	if err == nil {
		t.Error("Expected timeout error")
	}

	if !contains([]string{err.Error()}, "timeout") {
		t.Errorf("Expected timeout error, got: %v", err)
	}

	// Verify it waited approximately the timeout duration (with some tolerance)
	if elapsed < 800*time.Millisecond {
		t.Errorf("Should have waited at least 800ms, only waited %v", elapsed)
	}

	if elapsed > 3*time.Second {
		t.Errorf("Should have timed out within 3s, waited %v", elapsed)
	}
}

func TestWaitForTaskStatusProto_StatusChanges(t *testing.T) {
	tmpDir := withPauseTestDir(t)
	backend := createPauseTestBackend(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Create a running task
	tk := task.NewProtoTask("TASK-001", "Test task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Simulate status change in background
	go func() {
		time.Sleep(300 * time.Millisecond)
		tk.Status = orcv1.TaskStatus_TASK_STATUS_PAUSED
		_ = backend.SaveTask(tk)
	}()

	// Should succeed when status changes
	start := time.Now()
	err := waitForTaskStatusProto(backend, "TASK-001", orcv1.TaskStatus_TASK_STATUS_PAUSED, 5*time.Second)
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("Expected success after status change, got: %v", err)
	}

	// Should complete reasonably quickly after the status change
	if elapsed > 2*time.Second {
		t.Errorf("Should have completed faster, took %v", elapsed)
	}
}

// TestStopCommand_NotRunning tests that stop fails for non-running tasks appropriately.
func TestStopCommand_NotRunning(t *testing.T) {
	tmpDir := withPauseTestDir(t)
	backend := createPauseTestBackend(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Create completed task
	tk := task.NewProtoTask("TASK-001", "Test task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	cmd := newStopCmd()
	cmd.SetArgs([]string{"TASK-001", "--force"})

	err := cmd.Execute()
	if err == nil {
		t.Error("Stop should fail for completed task")
	}

	if err != nil && !contains([]string{err.Error()}, "already completed") {
		t.Errorf("Expected 'already completed' error, got: %v", err)
	}
}

// TestStopCommand_AlreadyFailed tests idempotent behavior for already failed tasks.
func TestStopCommand_AlreadyFailed(t *testing.T) {
	tmpDir := withPauseTestDir(t)
	backend := createPauseTestBackend(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Create failed task
	tk := task.NewProtoTask("TASK-001", "Test task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_FAILED
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	cmd := newStopCmd()
	cmd.SetArgs([]string{"TASK-001", "--force"})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	err := cmd.Execute()
	if err != nil {
		t.Errorf("Stop should succeed (idempotent) for already failed task: %v", err)
	}
}

// TestStopCommand_ForceStopsRunningTask tests force stopping a running task.
func TestStopCommand_ForceStopsRunningTask(t *testing.T) {
	tmpDir := withPauseTestDir(t)
	backend := createPauseTestBackend(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Create running task
	tk := task.NewProtoTask("TASK-001", "Test task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	cmd := newStopCmd()
	cmd.SetArgs([]string{"TASK-001", "--force"})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	err := cmd.Execute()
	if err != nil {
		t.Errorf("Stop should succeed with --force: %v", err)
	}

	// Verify task was marked failed
	stoppedTask, err := backend.LoadTask("TASK-001")
	if err != nil {
		t.Fatalf("failed to load task: %v", err)
	}

	if stoppedTask.Status != orcv1.TaskStatus_TASK_STATUS_FAILED {
		t.Errorf("Expected task status FAILED, got %v", stoppedTask.Status)
	}
}
