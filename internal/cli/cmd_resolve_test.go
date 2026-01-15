package cli

// NOTE: Tests in this file use os.Chdir() which is process-wide and not goroutine-safe.
// These tests MUST NOT use t.Parallel() and run sequentially within this package.

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// withResolveTestDir creates a temp directory with task structure, changes to it,
// and restores the original working directory when the test completes.
func withResolveTestDir(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()

	// Create .orc directory for project detection
	orcDir := filepath.Join(tmpDir, ".orc")
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatalf("create .orc directory: %v", err)
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

// createResolveTestBackend creates a backend in the given directory.
func createResolveTestBackend(t *testing.T, dir string) storage.Backend {
	t.Helper()
	backend, err := storage.NewDatabaseBackend(dir, nil)
	if err != nil {
		t.Fatalf("create backend: %v", err)
	}
	return backend
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
	if cmd.Flag("cleanup") == nil {
		t.Error("missing --cleanup flag")
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
	tmpDir := withResolveTestDir(t)

	// Create backend and save test data
	backend := createResolveTestBackend(t, tmpDir)

	// Create a failed task
	tk := task.New("TASK-001", "Test task")
	tk.Status = task.StatusFailed
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Close backend before running command
	backend.Close()

	// Run resolve with --force to skip confirmation
	cmd := newResolveCmd()
	cmd.SetArgs([]string{"TASK-001", "--force"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("resolve command failed: %v", err)
	}

	// Reload task and verify status
	backend = createResolveTestBackend(t, tmpDir)
	defer backend.Close()

	reloaded, err := backend.LoadTask("TASK-001")
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
	tmpDir := withResolveTestDir(t)

	// Create backend and save test data
	backend := createResolveTestBackend(t, tmpDir)

	// Create a failed task
	tk := task.New("TASK-001", "Test task")
	tk.Status = task.StatusFailed
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Close backend before running command
	backend.Close()

	// Run resolve with message
	cmd := newResolveCmd()
	cmd.SetArgs([]string{"TASK-001", "--force", "-m", "Fixed manually by updating config"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("resolve command failed: %v", err)
	}

	// Reload task and verify message
	backend = createResolveTestBackend(t, tmpDir)
	defer backend.Close()

	reloaded, err := backend.LoadTask("TASK-001")
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
	tmpDir := withResolveTestDir(t)

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
			// Create backend and save task with this status
			backend := createResolveTestBackend(t, tmpDir)
			tk := task.New("TASK-001", "Test task")
			tk.Status = status
			if err := backend.SaveTask(tk); err != nil {
				t.Fatalf("failed to save task: %v", err)
			}
			backend.Close()

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
	tmpDir := withResolveTestDir(t)

	// Create backend and save test data
	backend := createResolveTestBackend(t, tmpDir)

	// Create a failed task with existing metadata
	tk := task.New("TASK-001", "Test task")
	tk.Status = task.StatusFailed
	tk.Metadata = map[string]string{
		"existing_key": "existing_value",
		"another_key":  "another_value",
	}
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Close backend before running command
	backend.Close()

	// Run resolve
	cmd := newResolveCmd()
	cmd.SetArgs([]string{"TASK-001", "--force", "-m", "Test message"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("resolve command failed: %v", err)
	}

	// Reload task and verify all metadata
	backend = createResolveTestBackend(t, tmpDir)
	defer backend.Close()

	reloaded, err := backend.LoadTask("TASK-001")
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

func TestCheckWorktreeStatus_NoGitOps(t *testing.T) {
	// When gitOps is nil, should return empty status without error
	status, err := checkWorktreeStatus("TASK-001", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.exists {
		t.Error("expected exists to be false with nil gitOps")
	}
}

func TestWorktreeStatus_Struct(t *testing.T) {
	// Test the struct can hold all expected values
	status := worktreeStatus{
		exists:         true,
		path:           "/tmp/worktree/orc-TASK-001",
		isDirty:        true,
		hasConflicts:   true,
		conflictFiles:  []string{"file1.go", "file2.go"},
		rebaseInProg:   false,
		mergeInProg:    true,
		uncommittedMsg: "3 uncommitted file(s)",
	}

	if !status.exists {
		t.Error("expected exists to be true")
	}
	if status.path != "/tmp/worktree/orc-TASK-001" {
		t.Errorf("path = %q, want '/tmp/worktree/orc-TASK-001'", status.path)
	}
	if !status.isDirty {
		t.Error("expected isDirty to be true")
	}
	if !status.hasConflicts {
		t.Error("expected hasConflicts to be true")
	}
	if len(status.conflictFiles) != 2 {
		t.Errorf("conflictFiles length = %d, want 2", len(status.conflictFiles))
	}
	if status.rebaseInProg {
		t.Error("expected rebaseInProg to be false")
	}
	if !status.mergeInProg {
		t.Error("expected mergeInProg to be true")
	}
	if status.uncommittedMsg != "3 uncommitted file(s)" {
		t.Errorf("uncommittedMsg = %q, want '3 uncommitted file(s)'", status.uncommittedMsg)
	}
}
