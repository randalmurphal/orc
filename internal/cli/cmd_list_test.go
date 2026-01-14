package cli

// NOTE: Tests in this file use os.Chdir() which is process-wide and not goroutine-safe.
// These tests MUST NOT use t.Parallel() and run sequentially within this package.

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/randalmurphal/orc/internal/task"
)

// withListTestDir creates a temp directory with task structure, changes to it,
// and restores the original working directory when the test completes.
func withListTestDir(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, task.OrcDir, task.TasksDir)
	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		t.Fatalf("create tasks directory: %v", err)
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

func TestListCommand_Flags(t *testing.T) {
	cmd := newListCmd()

	// Verify command structure
	if cmd.Use != "list" {
		t.Errorf("command Use = %q, want %q", cmd.Use, "list")
	}

	// Verify aliases
	if len(cmd.Aliases) != 1 || cmd.Aliases[0] != "ls" {
		t.Errorf("command Aliases = %v, want [ls]", cmd.Aliases)
	}

	// Verify flags exist
	if cmd.Flag("status") == nil {
		t.Error("missing --status flag")
	}
	if cmd.Flag("weight") == nil {
		t.Error("missing --weight flag")
	}

	// Verify shorthand flags
	if cmd.Flag("status").Shorthand != "s" {
		t.Errorf("status shorthand = %q, want 's'", cmd.Flag("status").Shorthand)
	}
	if cmd.Flag("weight").Shorthand != "w" {
		t.Errorf("weight shorthand = %q, want 'w'", cmd.Flag("weight").Shorthand)
	}
}

func TestListCommand_StatusFilter(t *testing.T) {
	withListTestDir(t)

	// Create tasks with different statuses
	tk1 := task.New("TASK-001", "Running task")
	tk1.Status = task.StatusRunning
	tk1.Weight = task.WeightSmall
	if err := tk1.Save(); err != nil {
		t.Fatalf("failed to save task 1: %v", err)
	}

	tk2 := task.New("TASK-002", "Completed task")
	tk2.Status = task.StatusCompleted
	tk2.Weight = task.WeightMedium
	if err := tk2.Save(); err != nil {
		t.Fatalf("failed to save task 2: %v", err)
	}

	tk3 := task.New("TASK-003", "Another running task")
	tk3.Status = task.StatusRunning
	tk3.Weight = task.WeightLarge
	if err := tk3.Save(); err != nil {
		t.Fatalf("failed to save task 3: %v", err)
	}

	// Test filtering by running status
	cmd := newListCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--status", "running"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "TASK-001") {
		t.Error("expected TASK-001 in output")
	}
	if !strings.Contains(output, "TASK-003") {
		t.Error("expected TASK-003 in output")
	}
	if strings.Contains(output, "TASK-002") {
		t.Error("did not expect TASK-002 (completed) in output")
	}
}

func TestListCommand_WeightFilter(t *testing.T) {
	withListTestDir(t)

	// Create tasks with different weights
	tk1 := task.New("TASK-001", "Small task")
	tk1.Weight = task.WeightSmall
	tk1.Status = task.StatusPlanned
	if err := tk1.Save(); err != nil {
		t.Fatalf("failed to save task 1: %v", err)
	}

	tk2 := task.New("TASK-002", "Large task")
	tk2.Weight = task.WeightLarge
	tk2.Status = task.StatusPlanned
	if err := tk2.Save(); err != nil {
		t.Fatalf("failed to save task 2: %v", err)
	}

	tk3 := task.New("TASK-003", "Another small task")
	tk3.Weight = task.WeightSmall
	tk3.Status = task.StatusPlanned
	if err := tk3.Save(); err != nil {
		t.Fatalf("failed to save task 3: %v", err)
	}

	// Test filtering by small weight
	cmd := newListCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--weight", "small"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "TASK-001") {
		t.Error("expected TASK-001 in output")
	}
	if !strings.Contains(output, "TASK-003") {
		t.Error("expected TASK-003 in output")
	}
	if strings.Contains(output, "TASK-002") {
		t.Error("did not expect TASK-002 (large) in output")
	}
}

func TestListCommand_CombinedFilters(t *testing.T) {
	withListTestDir(t)

	// Create tasks with different statuses and weights
	tk1 := task.New("TASK-001", "Running small")
	tk1.Status = task.StatusRunning
	tk1.Weight = task.WeightSmall
	if err := tk1.Save(); err != nil {
		t.Fatalf("failed to save task 1: %v", err)
	}

	tk2 := task.New("TASK-002", "Running large")
	tk2.Status = task.StatusRunning
	tk2.Weight = task.WeightLarge
	if err := tk2.Save(); err != nil {
		t.Fatalf("failed to save task 2: %v", err)
	}

	tk3 := task.New("TASK-003", "Completed small")
	tk3.Status = task.StatusCompleted
	tk3.Weight = task.WeightSmall
	if err := tk3.Save(); err != nil {
		t.Fatalf("failed to save task 3: %v", err)
	}

	// Test filtering by running status AND small weight
	cmd := newListCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--status", "running", "--weight", "small"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "TASK-001") {
		t.Error("expected TASK-001 in output")
	}
	if strings.Contains(output, "TASK-002") {
		t.Error("did not expect TASK-002 (running large) in output")
	}
	if strings.Contains(output, "TASK-003") {
		t.Error("did not expect TASK-003 (completed small) in output")
	}
}

func TestListCommand_InvalidStatus(t *testing.T) {
	withListTestDir(t)

	cmd := newListCmd()
	var buf bytes.Buffer
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--status", "invalid"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid status")
	}
	if !strings.Contains(err.Error(), "invalid status") {
		t.Errorf("error message should mention invalid status, got: %v", err)
	}
	if !strings.Contains(err.Error(), "valid values") {
		t.Errorf("error message should list valid values, got: %v", err)
	}
}

func TestListCommand_InvalidWeight(t *testing.T) {
	withListTestDir(t)

	cmd := newListCmd()
	var buf bytes.Buffer
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--weight", "invalid"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid weight")
	}
	if !strings.Contains(err.Error(), "invalid weight") {
		t.Errorf("error message should mention invalid weight, got: %v", err)
	}
	if !strings.Contains(err.Error(), "valid values") {
		t.Errorf("error message should list valid values, got: %v", err)
	}
}

func TestListCommand_NoMatches(t *testing.T) {
	withListTestDir(t)

	// Create a completed task
	tk := task.New("TASK-001", "Completed task")
	tk.Status = task.StatusCompleted
	tk.Weight = task.WeightSmall
	if err := tk.Save(); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Filter for running tasks (should return empty)
	cmd := newListCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--status", "running"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "No tasks match the specified filters") {
		t.Errorf("expected 'No tasks match' message, got: %s", output)
	}
}

func TestListCommand_NoTasks(t *testing.T) {
	withListTestDir(t)

	// No tasks created
	cmd := newListCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "No tasks found") {
		t.Errorf("expected 'No tasks found' message, got: %s", output)
	}
}
