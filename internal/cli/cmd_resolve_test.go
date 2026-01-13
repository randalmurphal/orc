package cli

// NOTE: Tests in this file use os.Chdir() which is process-wide and not goroutine-safe.
// These tests MUST NOT use t.Parallel() and run sequentially within this package.

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/randalmurphal/orc/internal/task"
)

// withResolveTestDir creates a temp directory with task structure, changes to it,
// and restores the original working directory when the test completes.
func withResolveTestDir(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, task.OrcDir, task.TasksDir)
	taskDir := filepath.Join(tasksDir, "TASK-001")
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		t.Fatalf("create task directory: %v", err)
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

func TestResolveCommand_Structure(t *testing.T) {
	cmd := newResolveCmd()

	// Verify command structure
	if cmd.Use != "resolve <task-id>" {
		t.Errorf("command Use = %q, want %q", cmd.Use, "resolve <task-id>")
	}

	// Verify flags exist
	if cmd.Flag("force") == nil {
		t.Error("missing --force flag")
	}
	if cmd.Flag("message") == nil {
		t.Error("missing --message flag")
	}

	// Verify shorthand flags
	if cmd.Flag("force").Shorthand != "f" {
		t.Errorf("force shorthand = %q, want 'f'", cmd.Flag("force").Shorthand)
	}
	if cmd.Flag("message").Shorthand != "m" {
		t.Errorf("message shorthand = %q, want 'm'", cmd.Flag("message").Shorthand)
	}
}

func TestResolveCommand_RequiresArg(t *testing.T) {
	cmd := newResolveCmd()

	// Should require exactly one argument
	if err := cmd.Args(cmd, []string{}); err == nil {
		t.Error("expected error for zero args")
	}
	if err := cmd.Args(cmd, []string{"TASK-001"}); err != nil {
		t.Errorf("unexpected error for one arg: %v", err)
	}
	if err := cmd.Args(cmd, []string{"TASK-001", "TASK-002"}); err == nil {
		t.Error("expected error for two args")
	}
}

func TestResolveCommand_FailedTask(t *testing.T) {
	withResolveTestDir(t)

	// Create a failed task
	tk := task.New("TASK-001", "Test task")
	tk.Status = task.StatusFailed
	if err := tk.Save(); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Run resolve with --force to skip confirmation
	cmd := newResolveCmd()
	cmd.SetArgs([]string{"TASK-001", "--force"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("resolve command failed: %v", err)
	}

	// Reload task and verify status
	reloaded, err := task.Load("TASK-001")
	if err != nil {
		t.Fatalf("failed to reload task: %v", err)
	}

	if reloaded.Status != task.StatusCompleted {
		t.Errorf("task status = %s, want %s", reloaded.Status, task.StatusCompleted)
	}

	if reloaded.CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	}

	// Verify metadata
	if reloaded.Metadata["resolved"] != "true" {
		t.Errorf("metadata resolved = %q, want 'true'", reloaded.Metadata["resolved"])
	}

	if reloaded.Metadata["resolved_at"] == "" {
		t.Error("expected resolved_at metadata to be set")
	}

	// Verify resolved_at is a valid timestamp
	_, err = time.Parse(time.RFC3339, reloaded.Metadata["resolved_at"])
	if err != nil {
		t.Errorf("resolved_at is not valid RFC3339: %v", err)
	}
}

func TestResolveCommand_WithMessage(t *testing.T) {
	withResolveTestDir(t)

	// Create a failed task
	tk := task.New("TASK-001", "Test task")
	tk.Status = task.StatusFailed
	if err := tk.Save(); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Run resolve with message
	cmd := newResolveCmd()
	cmd.SetArgs([]string{"TASK-001", "--force", "-m", "Fixed manually by updating config"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("resolve command failed: %v", err)
	}

	// Reload task and verify message
	reloaded, err := task.Load("TASK-001")
	if err != nil {
		t.Fatalf("failed to reload task: %v", err)
	}

	expectedMsg := "Fixed manually by updating config"
	if reloaded.Metadata["resolution_message"] != expectedMsg {
		t.Errorf("metadata resolution_message = %q, want %q",
			reloaded.Metadata["resolution_message"], expectedMsg)
	}
}

func TestResolveCommand_OnlyFailedTasks(t *testing.T) {
	withResolveTestDir(t)

	// Test various non-failed statuses
	statuses := []task.Status{
		task.StatusCreated,
		task.StatusPlanned,
		task.StatusRunning,
		task.StatusPaused,
		task.StatusBlocked,
		task.StatusCompleted,
	}

	for _, status := range statuses {
		t.Run(string(status), func(t *testing.T) {
			// Create task with this status
			tk := task.New("TASK-001", "Test task")
			tk.Status = status
			if err := tk.Save(); err != nil {
				t.Fatalf("failed to save task: %v", err)
			}

			// Run resolve - should fail
			cmd := newResolveCmd()
			cmd.SetArgs([]string{"TASK-001", "--force"})
			err := cmd.Execute()
			if err == nil {
				t.Errorf("expected error for status %s, got nil", status)
			}
		})
	}
}

func TestResolveCommand_TaskNotFound(t *testing.T) {
	withResolveTestDir(t)

	cmd := newResolveCmd()
	cmd.SetArgs([]string{"TASK-999", "--force"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for non-existent task")
	}
}

func TestResolveCommand_PreservesExistingMetadata(t *testing.T) {
	withResolveTestDir(t)

	// Create a failed task with existing metadata
	tk := task.New("TASK-001", "Test task")
	tk.Status = task.StatusFailed
	tk.Metadata = map[string]string{
		"existing_key": "existing_value",
		"another_key":  "another_value",
	}
	if err := tk.Save(); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Run resolve
	cmd := newResolveCmd()
	cmd.SetArgs([]string{"TASK-001", "--force", "-m", "Test message"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("resolve command failed: %v", err)
	}

	// Reload task and verify all metadata
	reloaded, err := task.Load("TASK-001")
	if err != nil {
		t.Fatalf("failed to reload task: %v", err)
	}

	// Original metadata should be preserved
	if reloaded.Metadata["existing_key"] != "existing_value" {
		t.Errorf("existing_key = %q, want 'existing_value'", reloaded.Metadata["existing_key"])
	}
	if reloaded.Metadata["another_key"] != "another_value" {
		t.Errorf("another_key = %q, want 'another_value'", reloaded.Metadata["another_key"])
	}

	// New metadata should be added
	if reloaded.Metadata["resolved"] != "true" {
		t.Errorf("resolved = %q, want 'true'", reloaded.Metadata["resolved"])
	}
	if reloaded.Metadata["resolution_message"] != "Test message" {
		t.Errorf("resolution_message = %q, want 'Test message'", reloaded.Metadata["resolution_message"])
	}
}
