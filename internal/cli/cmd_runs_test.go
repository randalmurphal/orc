package cli

import (
	"bytes"
	"os"
	"os/exec"
	"testing"
	"time"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/internal/workflow"
)

// withRunsTestDir creates a temp directory with orc initialized and changes to it.
func withRunsTestDir(t *testing.T) string {
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

// createRunsTestBackend creates a backend in the given directory.
func createRunsTestBackend(t *testing.T, dir string) storage.Backend {
	t.Helper()
	backend, err := storage.NewDatabaseBackend(dir, nil)
	if err != nil {
		t.Fatalf("create backend: %v", err)
	}
	return backend
}

// createTestWorkflow creates a workflow for testing (needed due to foreign key constraints).
func createTestWorkflow(t *testing.T, pdb *db.ProjectDB) {
	t.Helper()
	now := time.Now()
	wf := &db.Workflow{
		ID:           "test-workflow",
		Name:         "Test Workflow",
		WorkflowType: "task",
		IsBuiltin:    false,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := pdb.SaveWorkflow(wf); err != nil {
		t.Fatalf("failed to create workflow: %v", err)
	}
}

// createTestWorkflowRun creates a workflow run for testing.
func createTestWorkflowRun(t *testing.T, pdb *db.ProjectDB, runID string, taskID *string, status string) *db.WorkflowRun {
	t.Helper()
	// Ensure workflow exists
	createTestWorkflow(t, pdb)

	now := time.Now()
	run := &db.WorkflowRun{
		ID:          runID,
		WorkflowID:  "test-workflow",
		ContextType: "task",
		Status:      status,
		TaskID:      taskID,
		Prompt:      "Test prompt",
		StartedAt:   &now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := pdb.SaveWorkflowRun(run); err != nil {
		t.Fatalf("failed to save workflow run: %v", err)
	}
	return run
}

// TestCancelRun_WithLinkedTask_SignalsProcess tests SC-1:
// When cancelling a run with a linked task that has a live executor PID, send SIGTERM to the process.
// We spawn a subprocess (sleep) that can safely receive SIGTERM.
func TestCancelRun_WithLinkedTask_SignalsProcess(t *testing.T) {
	tmpDir := withRunsTestDir(t)
	backend := createRunsTestBackend(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Spawn a subprocess that we can safely signal
	// Using 'sleep 60' gives us a process that will wait and can receive SIGTERM
	sleepCmd := exec.Command("sleep", "60")
	if err := sleepCmd.Start(); err != nil {
		t.Fatalf("failed to start sleep process: %v", err)
	}
	sleepPID := sleepCmd.Process.Pid
	t.Cleanup(func() {
		// Clean up the sleep process if it's still running
		_ = sleepCmd.Process.Kill()
		_ = sleepCmd.Wait()
	})

	// Verify the sleep process is running
	if !task.IsPIDAlive(sleepPID) {
		t.Fatalf("sleep process should be alive")
	}

	// Create a running task with the sleep process PID
	tk := task.NewProtoTask("TASK-001", "Test task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	tk.Weight = orcv1.TaskWeight_TASK_WEIGHT_SMALL
	tk.ExecutorPid = int32(sleepPID)
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Create a workflow run linked to the task
	taskID := "TASK-001"
	createTestWorkflowRun(t, backend.DB(), "RUN-001", &taskID, string(workflow.RunStatusRunning))

	// Execute cancel command
	cmd := newRunCancelCmd()
	cmd.SetArgs([]string{"RUN-001"})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	err := cmd.Execute()
	if err != nil {
		t.Errorf("Cancel should succeed: %v", err)
	}

	// Wait a short time for the signal to be processed
	time.Sleep(100 * time.Millisecond)

	// Verify output indicates process was signaled
	output := stdout.String()
	if !contains([]string{output}, "Cancelled workflow run 'RUN-001'") {
		t.Errorf("Expected cancel confirmation in output, got: %s", output)
	}
	if !contains([]string{output}, "signaled to terminate") {
		t.Errorf("Expected 'signaled to terminate' in output, got: %s", output)
	}

	// Verify run was cancelled
	run, err := backend.DB().GetWorkflowRun("RUN-001")
	if err != nil {
		t.Fatalf("failed to get workflow run: %v", err)
	}
	if run.Status != string(workflow.RunStatusCancelled) {
		t.Errorf("Expected run status 'cancelled', got: %s", run.Status)
	}

	// Verify the sleep process received the signal (should have exited)
	// Wait for the process to actually terminate using Wait() with timeout
	waitChan := make(chan error, 1)
	go func() {
		waitChan <- sleepCmd.Wait()
	}()

	select {
	case <-waitChan:
		// Process exited (SIGTERM received and processed)
	case <-time.After(2 * time.Second):
		t.Errorf("Sleep process should have been terminated by SIGTERM within 2 seconds")
	}
}

// TestCancelRun_WithLinkedTask_DeadPID tests SC-2:
// Cancel completes successfully when the task has a dead PID.
func TestCancelRun_WithLinkedTask_DeadPID(t *testing.T) {
	tmpDir := withRunsTestDir(t)
	backend := createRunsTestBackend(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Create a running task with a PID that doesn't exist
	tk := task.NewProtoTask("TASK-001", "Test task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	tk.Weight = orcv1.TaskWeight_TASK_WEIGHT_SMALL
	tk.ExecutorPid = 99999999 // Very high PID that shouldn't exist
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Create a workflow run linked to the task
	taskID := "TASK-001"
	createTestWorkflowRun(t, backend.DB(), "RUN-001", &taskID, string(workflow.RunStatusRunning))

	// Execute cancel command
	cmd := newRunCancelCmd()
	cmd.SetArgs([]string{"RUN-001"})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	err := cmd.Execute()
	if err != nil {
		t.Errorf("Cancel should succeed even when PID is dead: %v", err)
	}

	// Verify run was cancelled
	run, err := backend.DB().GetWorkflowRun("RUN-001")
	if err != nil {
		t.Fatalf("failed to get workflow run: %v", err)
	}
	if run.Status != string(workflow.RunStatusCancelled) {
		t.Errorf("Expected run status 'cancelled', got: %s", run.Status)
	}

	// Verify output says manual termination may be needed (since PID was dead)
	output := stdout.String()
	if !contains([]string{output}, "may still need to be terminated manually") {
		t.Errorf("Expected manual termination message for dead PID, got: %s", output)
	}
}

// TestCancelRun_NoLinkedTask tests SC-2:
// Cancel completes successfully when there's no linked task.
func TestCancelRun_NoLinkedTask(t *testing.T) {
	tmpDir := withRunsTestDir(t)
	backend := createRunsTestBackend(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Create a workflow run without a linked task (standalone run)
	createTestWorkflowRun(t, backend.DB(), "RUN-001", nil, string(workflow.RunStatusRunning))

	// Execute cancel command
	cmd := newRunCancelCmd()
	cmd.SetArgs([]string{"RUN-001"})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	err := cmd.Execute()
	if err != nil {
		t.Errorf("Cancel should succeed for standalone run: %v", err)
	}

	// Verify run was cancelled
	run, err := backend.DB().GetWorkflowRun("RUN-001")
	if err != nil {
		t.Fatalf("failed to get workflow run: %v", err)
	}
	if run.Status != string(workflow.RunStatusCancelled) {
		t.Errorf("Expected run status 'cancelled', got: %s", run.Status)
	}

	// Verify output says manual termination may be needed (since no task to check)
	output := stdout.String()
	if !contains([]string{output}, "may still need to be terminated manually") {
		t.Errorf("Expected manual termination message for no task, got: %s", output)
	}
}

// TestCancelRun_WithLinkedTask_NoPID tests SC-2:
// Cancel completes successfully when the task has ExecutorPid=0.
func TestCancelRun_WithLinkedTask_NoPID(t *testing.T) {
	tmpDir := withRunsTestDir(t)
	backend := createRunsTestBackend(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Create a running task with no executor PID (ExecutorPid=0)
	tk := task.NewProtoTask("TASK-001", "Test task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	tk.Weight = orcv1.TaskWeight_TASK_WEIGHT_SMALL
	tk.ExecutorPid = 0 // No executor PID
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Create a workflow run linked to the task
	taskID := "TASK-001"
	createTestWorkflowRun(t, backend.DB(), "RUN-001", &taskID, string(workflow.RunStatusRunning))

	// Execute cancel command
	cmd := newRunCancelCmd()
	cmd.SetArgs([]string{"RUN-001"})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	err := cmd.Execute()
	if err != nil {
		t.Errorf("Cancel should succeed when task has no executor PID: %v", err)
	}

	// Verify run was cancelled
	run, err := backend.DB().GetWorkflowRun("RUN-001")
	if err != nil {
		t.Fatalf("failed to get workflow run: %v", err)
	}
	if run.Status != string(workflow.RunStatusCancelled) {
		t.Errorf("Expected run status 'cancelled', got: %s", run.Status)
	}

	// Verify output says manual termination may be needed (since no PID to signal)
	output := stdout.String()
	if !contains([]string{output}, "may still need to be terminated manually") {
		t.Errorf("Expected manual termination message for no PID, got: %s", output)
	}
}

// TestCancelRun_NotFound tests that cancel fails when the run doesn't exist.
func TestCancelRun_NotFound(t *testing.T) {
	withRunsTestDir(t)

	cmd := newRunCancelCmd()
	cmd.SetArgs([]string{"RUN-999"})

	err := cmd.Execute()
	if err == nil {
		t.Error("Cancel should fail for non-existent run")
	}

	if err != nil && !contains([]string{err.Error()}, "not found") {
		t.Errorf("Expected 'not found' error, got: %v", err)
	}
}

// TestCancelRun_AlreadyCancelled tests that cancel fails when the run is already cancelled.
func TestCancelRun_AlreadyCancelled(t *testing.T) {
	tmpDir := withRunsTestDir(t)
	backend := createRunsTestBackend(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Create an already cancelled run
	createTestWorkflowRun(t, backend.DB(), "RUN-001", nil, string(workflow.RunStatusCancelled))

	cmd := newRunCancelCmd()
	cmd.SetArgs([]string{"RUN-001"})

	err := cmd.Execute()
	if err == nil {
		t.Error("Cancel should fail for already cancelled run")
	}

	if err != nil && !contains([]string{err.Error()}, "cannot cancel run with status") {
		t.Errorf("Expected 'cannot cancel run' error, got: %v", err)
	}
}
