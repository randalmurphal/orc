// Package cli implements the orc command-line interface.
//
// TDD Tests for TASK-652: Lifecycle triggers from CLI task creation.
//
// NOTE: Tests in this file use os.Chdir() which is process-wide and not goroutine-safe.
// These tests MUST NOT use t.Parallel() and run sequentially within this package.
//
// Success Criteria Coverage:
// - SC-7: on_task_created triggers fire after SaveTask() in cmd_new.go (CLI)
// - SC-9: Gate-mode on_task_created trigger rejection returns error
package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/trigger"
	"github.com/randalmurphal/orc/internal/workflow"
)

// --- SC-7: on_task_created triggers fire from CLI cmd_new ---

func TestNewCmd_LifecycleTrigger(t *testing.T) {
	tmpDir := withNewCmdTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Pre-populate a task to ensure task ID generation works
	// (backend needs at least one task for GetNextTaskID)

	mockRunner := &mockCLITriggerRunner{}

	// Close backend - command will reopen
	_ = backend.Close()

	// Run the new command with trigger runner injected
	cmd := newNewCmdWithTriggerRunner(mockRunner)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"Test lifecycle trigger task", "-w", "medium"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("new command failed: %v", err)
	}

	// Trigger runner should have been called
	if !mockRunner.lifecycleCalled {
		t.Error("lifecycle trigger should have been called after task creation in CLI")
	}
	if mockRunner.lastEvent != workflow.WorkflowTriggerEventOnTaskCreated {
		t.Errorf("event = %q, want %q", mockRunner.lastEvent, workflow.WorkflowTriggerEventOnTaskCreated)
	}
}

// --- SC-9: Gate-mode on_task_created trigger rejection in CLI ---

func TestNewCmd_LifecycleTrigger_GateRejects(t *testing.T) {
	tmpDir := withNewCmdTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	mockRunner := &mockCLITriggerRunner{
		lifecycleErr: &trigger.GateRejectionError{
			AgentID: "quality-gate",
			Reason:  "description too vague for medium weight task",
		},
	}

	_ = backend.Close()

	cmd := newNewCmdWithTriggerRunner(mockRunner)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"Fix stuff", "-w", "medium"})

	err := cmd.Execute()

	// The command may or may not return error - but the output should mention rejection
	output := buf.String()

	if err == nil {
		// If no error, verify the output mentions the rejection/blocked status
		if !strings.Contains(output, "blocked") && !strings.Contains(output, "rejected") &&
			!strings.Contains(output, "BLOCKED") {
			t.Error("CLI output should mention blocked/rejected status when gate rejects")
		}
	}

	// Task should be saved (per SC-9: "Task already saved; blocking sets status to BLOCKED, not deleted")
	reopened := createTestBackendInDir(t, tmpDir)
	defer func() { _ = reopened.Close() }()

	tasks, loadErr := reopened.LoadAllTasks()
	if loadErr != nil {
		t.Fatalf("load tasks: %v", loadErr)
	}

	if len(tasks) == 0 {
		t.Fatal("task should exist in DB even after gate rejection")
	}

	if tasks[0].Status != orcv1.TaskStatus_TASK_STATUS_BLOCKED {
		t.Errorf("task status = %v, want BLOCKED after gate rejection", tasks[0].Status)
	}
}

// --- Edge case: No workflow assigned, no triggers ---

func TestNewCmd_NoWorkflow_NoTriggers(t *testing.T) {
	tmpDir := withNewCmdTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	mockRunner := &mockCLITriggerRunner{}

	_ = backend.Close()

	// Create a trivial task (no workflow auto-assigned for unspecified weight)
	cmd := newNewCmdWithTriggerRunner(mockRunner)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"Simple fix"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("new command failed: %v", err)
	}

	// No triggers should fire when no workflow
	if mockRunner.lifecycleCalled {
		t.Error("trigger runner should not be called when no workflow assigned")
	}
}

// --- Helpers ---

// withNewCmdTestDir creates a temp directory with .orc structure for CLI testing.
func withNewCmdTestDir(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()

	orcDir := filepath.Join(tmpDir, ".orc")
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatalf("create .orc directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(orcDir, "config.yaml"), []byte("version: 1\n"), 0644); err != nil {
		t.Fatalf("create config.yaml: %v", err)
	}

	// Initialize git repo (required for branch validation in new command)
	gitDir := filepath.Join(tmpDir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatalf("create .git directory: %v", err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir to temp dir: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(origDir); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})
	return tmpDir
}

// --- Mock trigger runner for CLI tests ---

type mockCLITriggerRunner struct {
	lifecycleCalled bool
	lastEvent       workflow.WorkflowTriggerEvent
	lastTask        *orcv1.Task
	lifecycleErr    error
}

func (m *mockCLITriggerRunner) RunLifecycleTriggers(
	ctx context.Context,
	event workflow.WorkflowTriggerEvent,
	triggers []workflow.WorkflowTrigger,
	tsk *orcv1.Task,
) error {
	m.lifecycleCalled = true
	m.lastEvent = event
	m.lastTask = tsk
	return m.lifecycleErr
}
