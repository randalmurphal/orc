package cli

// Tests for TASK-617: orc status --plain shows stale/incorrect phase labels

import (
	"bytes"
	"os"
	"strings"
	"testing"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/initiative"
	"github.com/randalmurphal/orc/internal/task"
)

// TestStatusCommand_RunningTaskShowsPhase verifies SC-2: `orc status` shows
// correct phase for running tasks directly from the task record. A running task
// with CurrentPhase="implement" should display [implement], not [starting].
func TestStatusCommand_RunningTaskShowsPhase(t *testing.T) {
	tmpDir := withStatusTestDir(t)

	backend := createStatusTestBackend(t, tmpDir)

	// Create a running task with CurrentPhase set directly on the task record
	t1 := task.NewProtoTask("TASK-001", "Running with implement phase")
	t1.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	t1.ExecutorPid = int32(os.Getpid()) // Prevent orphan detection
	task.SetCurrentPhaseProto(t1, "implement")
	if err := backend.SaveTask(t1); err != nil {
		t.Fatalf("save task: %v", err)
	}

	_ = backend.Close()

	cmd := newStatusCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute command: %v", err)
	}

	output := out.String()

	// The task should show [implement], NOT [starting]
	if !strings.Contains(output, "[implement]") {
		t.Errorf("expected output to contain [implement], got:\n%s", output)
	}
	if strings.Contains(output, "[starting]") {
		t.Errorf("output should NOT contain [starting] when CurrentPhase is set, got:\n%s", output)
	}
}

// TestStatusCommand_RunningTaskNoPhase verifies SC-2 edge case: a task that
// was just started and has no phase yet should legitimately show [starting].
func TestStatusCommand_RunningTaskNoPhase(t *testing.T) {
	tmpDir := withStatusTestDir(t)

	backend := createStatusTestBackend(t, tmpDir)

	// Create a running task with NO CurrentPhase (just started)
	t1 := task.NewProtoTask("TASK-001", "Just started task")
	t1.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	t1.ExecutorPid = int32(os.Getpid())
	// Deliberately NOT setting CurrentPhase — it should be nil
	if err := backend.SaveTask(t1); err != nil {
		t.Fatalf("save task: %v", err)
	}

	_ = backend.Close()

	cmd := newStatusCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute command: %v", err)
	}

	output := out.String()

	// "starting" is the correct default for genuinely empty CurrentPhase
	if !strings.Contains(output, "[starting]") {
		t.Errorf("expected output to contain [starting] for task with no phase, got:\n%s", output)
	}
}

// TestStatusCommand_MultipleRunningTasksShowCorrectPhases verifies BDD-3:
// given 3 tasks running in parallel with different phases, each task shows
// its own correct current phase independently.
func TestStatusCommand_MultipleRunningTasksShowCorrectPhases(t *testing.T) {
	tmpDir := withStatusTestDir(t)

	backend := createStatusTestBackend(t, tmpDir)

	// Create 3 running tasks each in a different phase
	phases := map[string]string{
		"TASK-001": "spec",
		"TASK-002": "implement",
		"TASK-003": "review",
	}

	for id, phase := range phases {
		tsk := task.NewProtoTask(id, "Task in "+phase+" phase")
		tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
		tsk.ExecutorPid = int32(os.Getpid())
		task.SetCurrentPhaseProto(tsk, phase)
		if err := backend.SaveTask(tsk); err != nil {
			t.Fatalf("save task %s: %v", id, err)
		}
	}

	_ = backend.Close()

	cmd := newStatusCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute command: %v", err)
	}

	output := out.String()

	// Each task should display its own correct phase
	for id, phase := range phases {
		if !strings.Contains(output, id) {
			t.Errorf("output should contain %s", id)
		}
		if !strings.Contains(output, "["+phase+"]") {
			t.Errorf("output should contain [%s] for %s, got:\n%s", phase, id, output)
		}
	}

	// None should show [starting]
	if strings.Contains(output, "[starting]") {
		t.Errorf("no task should show [starting] when all have CurrentPhase set, got:\n%s", output)
	}
}

// TestStatusCommand_PhaseFromTaskNotWorkflowRun verifies SC-5: the status command
// reads CurrentPhase directly from the task record, NOT from workflow_runs enrichment.
// This test creates a task with CurrentPhase set on the task record but WITHOUT
// any corresponding workflow_run entry. The phase should still display correctly.
func TestStatusCommand_PhaseFromTaskNotWorkflowRun(t *testing.T) {
	tmpDir := withStatusTestDir(t)

	backend := createStatusTestBackend(t, tmpDir)

	// Create a running task with CurrentPhase set on the task itself
	t1 := task.NewProtoTask("TASK-001", "Task with phase on record")
	t1.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	t1.ExecutorPid = int32(os.Getpid())
	task.SetCurrentPhaseProto(t1, "tdd_write")
	if err := backend.SaveTask(t1); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// IMPORTANT: We deliberately do NOT create any workflow_run entries.
	// If the status command relies on workflow_run enrichment, this test will
	// show [starting] instead of [tdd_write].

	_ = backend.Close()

	cmd := newStatusCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute command: %v", err)
	}

	output := out.String()

	// Must show [tdd_write] from the task record itself
	if !strings.Contains(output, "[tdd_write]") {
		t.Errorf("expected output to contain [tdd_write] from task record, got:\n%s", output)
	}
}

// TestStatusCommand_PhaseDisplayWithInitiativeFilter verifies that phase labels
// display correctly even when filtered by initiative.
func TestStatusCommand_PhaseDisplayWithInitiativeFilter(t *testing.T) {
	tmpDir := withStatusTestDir(t)

	backend := createStatusTestBackend(t, tmpDir)

	// Need initiative import
	init1 := initiative.New("INIT-001", "Test Initiative")
	if err := backend.SaveInitiative(init1); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	t1 := task.NewProtoTask("TASK-001", "Running in initiative")
	task.SetInitiativeProto(t1, "INIT-001")
	t1.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	t1.ExecutorPid = int32(os.Getpid())
	task.SetCurrentPhaseProto(t1, "review")
	if err := backend.SaveTask(t1); err != nil {
		t.Fatalf("save task: %v", err)
	}

	_ = backend.Close()

	cmd := newStatusCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--initiative", "INIT-001"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute command: %v", err)
	}

	output := out.String()

	if !strings.Contains(output, "[review]") {
		t.Errorf("expected output to contain [review] with initiative filter, got:\n%s", output)
	}
}

