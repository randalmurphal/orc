package cli

// NOTE: Tests in this file use os.Chdir() which is process-wide and not goroutine-safe.
// These tests MUST NOT use t.Parallel() and run sequentially within this package.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/randalmurphal/orc/internal/initiative"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// withInitiativeTestDir creates a temp directory with initiative and task structure,
// changes to it, and restores the original working directory when the test completes.
// Returns the tmpDir path.
func withInitiativeTestDir(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()

	// Create .orc directory
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

// createTestBackendInDir creates a backend for the given directory.
func createTestBackendInDir(t *testing.T, dir string) storage.Backend {
	t.Helper()
	backend, err := storage.NewDatabaseBackend(dir, nil)
	if err != nil {
		t.Fatalf("create backend: %v", err)
	}
	return backend
}

func TestInitiativeLinkCommand_Structure(t *testing.T) {
	cmd := newInitiativeLinkCmd()

	// Verify command structure
	if cmd.Use != "link <initiative-id> <task-id>..." {
		t.Errorf("command Use = %q, want %q", cmd.Use, "link <initiative-id> <task-id>...")
	}

	// Verify flags exist
	if cmd.Flag("all-matching") == nil {
		t.Error("missing --all-matching flag")
	}
	if cmd.Flag("shared") == nil {
		t.Error("missing --shared flag")
	}
}

func TestInitiativeUnlinkCommand_Structure(t *testing.T) {
	cmd := newInitiativeUnlinkCmd()

	// Verify command structure
	if cmd.Use != "unlink <initiative-id> <task-id>..." {
		t.Errorf("command Use = %q, want %q", cmd.Use, "unlink <initiative-id> <task-id>...")
	}

	// Verify flags exist
	if cmd.Flag("all") == nil {
		t.Error("missing --all flag")
	}
	if cmd.Flag("shared") == nil {
		t.Error("missing --shared flag")
	}
}

func TestInitiativeLinkCommand_RequiresArg(t *testing.T) {
	cmd := newInitiativeLinkCmd()

	// Should require at least one argument (the initiative ID)
	if err := cmd.Args(cmd, []string{}); err == nil {
		t.Error("expected error for zero args")
	}
	if err := cmd.Args(cmd, []string{"INIT-001"}); err != nil {
		t.Errorf("unexpected error for one arg: %v", err)
	}
	if err := cmd.Args(cmd, []string{"INIT-001", "TASK-001"}); err != nil {
		t.Errorf("unexpected error for two args: %v", err)
	}
	if err := cmd.Args(cmd, []string{"INIT-001", "TASK-001", "TASK-002"}); err != nil {
		t.Errorf("unexpected error for three args: %v", err)
	}
}

func TestInitiativeUnlinkCommand_RequiresArg(t *testing.T) {
	cmd := newInitiativeUnlinkCmd()

	// Should require at least one argument (the initiative ID)
	if err := cmd.Args(cmd, []string{}); err == nil {
		t.Error("expected error for zero args")
	}
	if err := cmd.Args(cmd, []string{"INIT-001"}); err != nil {
		t.Errorf("unexpected error for one arg: %v", err)
	}
	if err := cmd.Args(cmd, []string{"INIT-001", "TASK-001"}); err != nil {
		t.Errorf("unexpected error for two args: %v", err)
	}
}

func TestInitiativeLinkMultipleTasks(t *testing.T) {
	tmpDir := withInitiativeTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	defer backend.Close()

	// Create initiative
	init := initiative.New("INIT-001", "Test Initiative")
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create tasks
	task1 := task.New("TASK-001", "Task One")
	if err := backend.SaveTask(task1); err != nil {
		t.Fatalf("save task1: %v", err)
	}
	task2 := task.New("TASK-002", "Task Two")
	if err := backend.SaveTask(task2); err != nil {
		t.Fatalf("save task2: %v", err)
	}
	task3 := task.New("TASK-003", "Task Three")
	if err := backend.SaveTask(task3); err != nil {
		t.Fatalf("save task3: %v", err)
	}

	// Close backend before running command (command creates its own)
	backend.Close()

	// Run link command
	cmd := newInitiativeLinkCmd()
	cmd.SetArgs([]string{"INIT-001", "TASK-001", "TASK-002", "TASK-003"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("link command failed: %v", err)
	}

	// Re-open backend to verify
	backend = createTestBackendInDir(t, tmpDir)
	defer backend.Close()

	// Verify tasks are linked
	reloadedTask1, _ := backend.LoadTask("TASK-001")
	if reloadedTask1.InitiativeID != "INIT-001" {
		t.Errorf("task1 InitiativeID = %q, want %q", reloadedTask1.InitiativeID, "INIT-001")
	}

	reloadedTask2, _ := backend.LoadTask("TASK-002")
	if reloadedTask2.InitiativeID != "INIT-001" {
		t.Errorf("task2 InitiativeID = %q, want %q", reloadedTask2.InitiativeID, "INIT-001")
	}

	reloadedTask3, _ := backend.LoadTask("TASK-003")
	if reloadedTask3.InitiativeID != "INIT-001" {
		t.Errorf("task3 InitiativeID = %q, want %q", reloadedTask3.InitiativeID, "INIT-001")
	}

	// Verify initiative has all tasks
	reloadedInit, _ := backend.LoadInitiative("INIT-001")
	if len(reloadedInit.Tasks) != 3 {
		t.Errorf("initiative tasks count = %d, want 3", len(reloadedInit.Tasks))
	}
}

func TestInitiativeLinkWithPattern(t *testing.T) {
	tmpDir := withInitiativeTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	defer backend.Close()

	// Create initiative
	init := initiative.New("INIT-001", "Test Initiative")
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create tasks with different titles
	authTask1 := task.New("TASK-001", "User Authentication")
	if err := backend.SaveTask(authTask1); err != nil {
		t.Fatalf("save authTask1: %v", err)
	}
	authTask2 := task.New("TASK-002", "Auth Token Validation")
	if err := backend.SaveTask(authTask2); err != nil {
		t.Fatalf("save authTask2: %v", err)
	}
	otherTask := task.New("TASK-003", "Database Migration")
	if err := backend.SaveTask(otherTask); err != nil {
		t.Fatalf("save otherTask: %v", err)
	}

	// Close backend before running command
	backend.Close()

	// Run link command with --all-matching
	cmd := newInitiativeLinkCmd()
	cmd.SetArgs([]string{"INIT-001", "--all-matching", "auth"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("link command failed: %v", err)
	}

	// Re-open backend to verify
	backend = createTestBackendInDir(t, tmpDir)
	defer backend.Close()

	// Verify only auth tasks are linked
	reloadedTask1, _ := backend.LoadTask("TASK-001")
	if reloadedTask1.InitiativeID != "INIT-001" {
		t.Errorf("auth task1 should be linked, got InitiativeID = %q", reloadedTask1.InitiativeID)
	}

	reloadedTask2, _ := backend.LoadTask("TASK-002")
	if reloadedTask2.InitiativeID != "INIT-001" {
		t.Errorf("auth task2 should be linked, got InitiativeID = %q", reloadedTask2.InitiativeID)
	}

	reloadedTask3, _ := backend.LoadTask("TASK-003")
	if reloadedTask3.InitiativeID != "" {
		t.Errorf("other task should not be linked, got InitiativeID = %q", reloadedTask3.InitiativeID)
	}

	// Verify initiative has only 2 tasks
	reloadedInit, _ := backend.LoadInitiative("INIT-001")
	if len(reloadedInit.Tasks) != 2 {
		t.Errorf("initiative tasks count = %d, want 2", len(reloadedInit.Tasks))
	}
}

func TestInitiativeUnlinkMultipleTasks(t *testing.T) {
	tmpDir := withInitiativeTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	defer backend.Close()

	// Create initiative with tasks
	init := initiative.New("INIT-001", "Test Initiative")

	// Create and link tasks
	task1 := task.New("TASK-001", "Task One")
	task1.SetInitiative("INIT-001")
	if err := backend.SaveTask(task1); err != nil {
		t.Fatalf("save task1: %v", err)
	}
	init.AddTask("TASK-001", "Task One", nil)

	task2 := task.New("TASK-002", "Task Two")
	task2.SetInitiative("INIT-001")
	if err := backend.SaveTask(task2); err != nil {
		t.Fatalf("save task2: %v", err)
	}
	init.AddTask("TASK-002", "Task Two", nil)

	task3 := task.New("TASK-003", "Task Three")
	task3.SetInitiative("INIT-001")
	if err := backend.SaveTask(task3); err != nil {
		t.Fatalf("save task3: %v", err)
	}
	init.AddTask("TASK-003", "Task Three", nil)

	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Close backend before running command
	backend.Close()

	// Run unlink command for 2 tasks
	cmd := newInitiativeUnlinkCmd()
	cmd.SetArgs([]string{"INIT-001", "TASK-001", "TASK-002"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unlink command failed: %v", err)
	}

	// Re-open backend to verify
	backend = createTestBackendInDir(t, tmpDir)
	defer backend.Close()

	// Verify tasks 1 and 2 are unlinked
	reloadedTask1, _ := backend.LoadTask("TASK-001")
	if reloadedTask1.InitiativeID != "" {
		t.Errorf("task1 should be unlinked, got InitiativeID = %q", reloadedTask1.InitiativeID)
	}

	reloadedTask2, _ := backend.LoadTask("TASK-002")
	if reloadedTask2.InitiativeID != "" {
		t.Errorf("task2 should be unlinked, got InitiativeID = %q", reloadedTask2.InitiativeID)
	}

	// Task 3 should still be linked
	reloadedTask3, _ := backend.LoadTask("TASK-003")
	if reloadedTask3.InitiativeID != "INIT-001" {
		t.Errorf("task3 should still be linked, got InitiativeID = %q", reloadedTask3.InitiativeID)
	}

	// Verify initiative has only 1 task
	reloadedInit, _ := backend.LoadInitiative("INIT-001")
	if len(reloadedInit.Tasks) != 1 {
		t.Errorf("initiative tasks count = %d, want 1", len(reloadedInit.Tasks))
	}
}

func TestInitiativeUnlinkAll(t *testing.T) {
	tmpDir := withInitiativeTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	defer backend.Close()

	// Create initiative with tasks
	init := initiative.New("INIT-001", "Test Initiative")

	// Create and link tasks
	task1 := task.New("TASK-001", "Task One")
	task1.SetInitiative("INIT-001")
	if err := backend.SaveTask(task1); err != nil {
		t.Fatalf("save task1: %v", err)
	}
	init.AddTask("TASK-001", "Task One", nil)

	task2 := task.New("TASK-002", "Task Two")
	task2.SetInitiative("INIT-001")
	if err := backend.SaveTask(task2); err != nil {
		t.Fatalf("save task2: %v", err)
	}
	init.AddTask("TASK-002", "Task Two", nil)

	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Close backend before running command
	backend.Close()

	// Run unlink command with --all
	cmd := newInitiativeUnlinkCmd()
	cmd.SetArgs([]string{"INIT-001", "--all"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unlink command failed: %v", err)
	}

	// Re-open backend to verify
	backend = createTestBackendInDir(t, tmpDir)
	defer backend.Close()

	// Verify all tasks are unlinked
	reloadedTask1, _ := backend.LoadTask("TASK-001")
	if reloadedTask1.InitiativeID != "" {
		t.Errorf("task1 should be unlinked, got InitiativeID = %q", reloadedTask1.InitiativeID)
	}

	reloadedTask2, _ := backend.LoadTask("TASK-002")
	if reloadedTask2.InitiativeID != "" {
		t.Errorf("task2 should be unlinked, got InitiativeID = %q", reloadedTask2.InitiativeID)
	}

	// Verify initiative has no tasks
	reloadedInit, _ := backend.LoadInitiative("INIT-001")
	if len(reloadedInit.Tasks) != 0 {
		t.Errorf("initiative tasks count = %d, want 0", len(reloadedInit.Tasks))
	}
}

func TestInitiativeLinkSkipsAlreadyLinkedToOther(t *testing.T) {
	tmpDir := withInitiativeTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	defer backend.Close()

	// Create two initiatives
	init1 := initiative.New("INIT-001", "First Initiative")
	if err := backend.SaveInitiative(init1); err != nil {
		t.Fatalf("save init1: %v", err)
	}

	init2 := initiative.New("INIT-002", "Second Initiative")
	if err := backend.SaveInitiative(init2); err != nil {
		t.Fatalf("save init2: %v", err)
	}

	// Create task linked to INIT-002
	task1 := task.New("TASK-001", "Task One")
	task1.SetInitiative("INIT-002")
	if err := backend.SaveTask(task1); err != nil {
		t.Fatalf("save task1: %v", err)
	}

	// Create unlinked task
	task2 := task.New("TASK-002", "Task Two")
	if err := backend.SaveTask(task2); err != nil {
		t.Fatalf("save task2: %v", err)
	}

	// Close backend before running command
	backend.Close()

	// Run link command to link both tasks to INIT-001
	cmd := newInitiativeLinkCmd()
	cmd.SetArgs([]string{"INIT-001", "TASK-001", "TASK-002"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("link command failed: %v", err)
	}

	// Re-open backend to verify
	backend = createTestBackendInDir(t, tmpDir)
	defer backend.Close()

	// TASK-001 should still be linked to INIT-002 (skipped)
	reloadedTask1, _ := backend.LoadTask("TASK-001")
	if reloadedTask1.InitiativeID != "INIT-002" {
		t.Errorf("task1 should still be linked to INIT-002, got %q", reloadedTask1.InitiativeID)
	}

	// TASK-002 should now be linked to INIT-001
	reloadedTask2, _ := backend.LoadTask("TASK-002")
	if reloadedTask2.InitiativeID != "INIT-001" {
		t.Errorf("task2 should be linked to INIT-001, got %q", reloadedTask2.InitiativeID)
	}
}

func TestInitiativeLinkFixesPartialLink(t *testing.T) {
	// This tests the case where task has initiative_id set but is NOT in
	// the initiative's task list. The link command should add it to the list.
	tmpDir := withInitiativeTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	defer backend.Close()

	// Create initiative WITHOUT any tasks
	init := initiative.New("INIT-001", "Test Initiative")
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create task that claims to be linked to INIT-001 but isn't in the list
	task1 := task.New("TASK-001", "Task One")
	task1.SetInitiative("INIT-001") // Set initiative_id
	if err := backend.SaveTask(task1); err != nil {
		t.Fatalf("save task1: %v", err)
	}

	// Verify the initial broken state: task has initiative_id but init has no tasks
	if len(init.Tasks) != 0 {
		t.Fatalf("expected 0 tasks in initiative, got %d", len(init.Tasks))
	}
	if task1.InitiativeID != "INIT-001" {
		t.Fatalf("task should have initiative_id INIT-001, got %q", task1.InitiativeID)
	}

	// Close backend before running command
	backend.Close()

	// Run link command - should add task to initiative despite having initiative_id
	cmd := newInitiativeLinkCmd()
	cmd.SetArgs([]string{"INIT-001", "TASK-001"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("link command failed: %v", err)
	}

	// Re-open backend to verify
	backend = createTestBackendInDir(t, tmpDir)
	defer backend.Close()

	// Verify task is now in the initiative's task list
	reloadedInit, err := backend.LoadInitiative("INIT-001")
	if err != nil {
		t.Fatalf("reload initiative: %v", err)
	}
	if len(reloadedInit.Tasks) != 1 {
		t.Errorf("initiative should have 1 task, got %d", len(reloadedInit.Tasks))
	}
	if len(reloadedInit.Tasks) > 0 && reloadedInit.Tasks[0].ID != "TASK-001" {
		t.Errorf("initiative task ID = %q, want TASK-001", reloadedInit.Tasks[0].ID)
	}
}

func TestInitiativeLinkSkipsFullyLinkedTask(t *testing.T) {
	// This tests that a task that is FULLY linked (both initiative_id set
	// AND in the initiative's task list) is correctly skipped.
	tmpDir := withInitiativeTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	defer backend.Close()

	// Create initiative with TASK-001 already in the list
	init := initiative.New("INIT-001", "Test Initiative")
	init.AddTask("TASK-001", "Task One", nil)
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create task with initiative_id set
	task1 := task.New("TASK-001", "Task One")
	task1.SetInitiative("INIT-001")
	if err := backend.SaveTask(task1); err != nil {
		t.Fatalf("save task1: %v", err)
	}

	// Also create a second task that should be linked
	task2 := task.New("TASK-002", "Task Two")
	if err := backend.SaveTask(task2); err != nil {
		t.Fatalf("save task2: %v", err)
	}

	// Close backend before running command
	backend.Close()

	// Run link command - should skip TASK-001 and link TASK-002
	cmd := newInitiativeLinkCmd()
	cmd.SetArgs([]string{"INIT-001", "TASK-001", "TASK-002"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("link command failed: %v", err)
	}

	// Re-open backend to verify
	backend = createTestBackendInDir(t, tmpDir)
	defer backend.Close()

	// Verify initiative has 2 tasks now
	reloadedInit, err := backend.LoadInitiative("INIT-001")
	if err != nil {
		t.Fatalf("reload initiative: %v", err)
	}
	if len(reloadedInit.Tasks) != 2 {
		t.Errorf("initiative should have 2 tasks, got %d", len(reloadedInit.Tasks))
	}

	// Verify TASK-002 is now linked
	reloadedTask2, _ := backend.LoadTask("TASK-002")
	if reloadedTask2.InitiativeID != "INIT-001" {
		t.Errorf("task2 should be linked to INIT-001, got %q", reloadedTask2.InitiativeID)
	}
}

func TestInitiativeLinkPatternFixesPartialLink(t *testing.T) {
	// Tests that --all-matching also fixes partial links
	tmpDir := withInitiativeTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	defer backend.Close()

	// Create initiative WITHOUT any tasks
	init := initiative.New("INIT-001", "Test Initiative")
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create task that claims to be linked to INIT-001 but isn't in the list
	task1 := task.New("TASK-001", "Auth Login")
	task1.SetInitiative("INIT-001") // Set initiative_id, but not in initiative's list
	if err := backend.SaveTask(task1); err != nil {
		t.Fatalf("save task1: %v", err)
	}

	// Close backend before running command
	backend.Close()

	// Run link command with pattern matching
	cmd := newInitiativeLinkCmd()
	cmd.SetArgs([]string{"INIT-001", "--all-matching", "auth"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("link command failed: %v", err)
	}

	// Re-open backend to verify
	backend = createTestBackendInDir(t, tmpDir)
	defer backend.Close()

	// Verify task is now in the initiative's task list
	reloadedInit, err := backend.LoadInitiative("INIT-001")
	if err != nil {
		t.Fatalf("reload initiative: %v", err)
	}
	if len(reloadedInit.Tasks) != 1 {
		t.Errorf("initiative should have 1 task, got %d", len(reloadedInit.Tasks))
	}
	if !reloadedInit.HasTask("TASK-001") {
		t.Error("initiative should have TASK-001 in task list")
	}
}
