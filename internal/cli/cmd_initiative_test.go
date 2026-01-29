package cli

// NOTE: Tests in this file use os.Chdir() which is process-wide and not goroutine-safe.
// These tests MUST NOT use t.Parallel() and run sequentially within this package.

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
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
	defer func() { _ = backend.Close() }()

	// Create initiative
	init := initiative.New("INIT-001", "Test Initiative")
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create tasks
	task1 := task.NewProtoTask("TASK-001", "Task One")
	if err := backend.SaveTask(task1); err != nil {
		t.Fatalf("save task1: %v", err)
	}
	task2 := task.NewProtoTask("TASK-002", "Task Two")
	if err := backend.SaveTask(task2); err != nil {
		t.Fatalf("save task2: %v", err)
	}
	task3 := task.NewProtoTask("TASK-003", "Task Three")
	if err := backend.SaveTask(task3); err != nil {
		t.Fatalf("save task3: %v", err)
	}

	// Close backend before running command (command creates its own)
	_ = backend.Close()

	// Run link command
	cmd := newInitiativeLinkCmd()
	cmd.SetArgs([]string{"INIT-001", "TASK-001", "TASK-002", "TASK-003"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("link command failed: %v", err)
	}

	// Re-open backend to verify
	backend = createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Verify tasks are linked
	reloadedTask1, _ := backend.LoadTask("TASK-001")
	if reloadedTask1.GetInitiativeId() != "INIT-001" {
		t.Errorf("task1 InitiativeId = %q, want %q", reloadedTask1.GetInitiativeId(), "INIT-001")
	}

	reloadedTask2, _ := backend.LoadTask("TASK-002")
	if reloadedTask2.GetInitiativeId() != "INIT-001" {
		t.Errorf("task2 InitiativeId = %q, want %q", reloadedTask2.GetInitiativeId(), "INIT-001")
	}

	reloadedTask3, _ := backend.LoadTask("TASK-003")
	if reloadedTask3.GetInitiativeId() != "INIT-001" {
		t.Errorf("task3 InitiativeId = %q, want %q", reloadedTask3.GetInitiativeId(), "INIT-001")
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
	defer func() { _ = backend.Close() }()

	// Create initiative
	init := initiative.New("INIT-001", "Test Initiative")
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create tasks with different titles
	authTask1 := task.NewProtoTask("TASK-001", "User Authentication")
	if err := backend.SaveTask(authTask1); err != nil {
		t.Fatalf("save authTask1: %v", err)
	}
	authTask2 := task.NewProtoTask("TASK-002", "Auth Token Validation")
	if err := backend.SaveTask(authTask2); err != nil {
		t.Fatalf("save authTask2: %v", err)
	}
	otherTask := task.NewProtoTask("TASK-003", "Database Migration")
	if err := backend.SaveTask(otherTask); err != nil {
		t.Fatalf("save otherTask: %v", err)
	}

	// Close backend before running command
	_ = backend.Close()

	// Run link command with --all-matching
	cmd := newInitiativeLinkCmd()
	cmd.SetArgs([]string{"INIT-001", "--all-matching", "auth"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("link command failed: %v", err)
	}

	// Re-open backend to verify
	backend = createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Verify only auth tasks are linked
	reloadedTask1, _ := backend.LoadTask("TASK-001")
	if reloadedTask1.GetInitiativeId() != "INIT-001" {
		t.Errorf("auth task1 should be linked, got InitiativeID = %q", reloadedTask1.GetInitiativeId())
	}

	reloadedTask2, _ := backend.LoadTask("TASK-002")
	if reloadedTask2.GetInitiativeId() != "INIT-001" {
		t.Errorf("auth task2 should be linked, got InitiativeID = %q", reloadedTask2.GetInitiativeId())
	}

	reloadedTask3, _ := backend.LoadTask("TASK-003")
	if reloadedTask3.GetInitiativeId() != "" {
		t.Errorf("other task should not be linked, got InitiativeID = %q", reloadedTask3.GetInitiativeId())
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
	defer func() { _ = backend.Close() }()

	// Create initiative with tasks
	init := initiative.New("INIT-001", "Test Initiative")

	// Create and link tasks
	task1 := task.NewProtoTask("TASK-001", "Task One")
	task.SetInitiativeProto(task1,"INIT-001")
	if err := backend.SaveTask(task1); err != nil {
		t.Fatalf("save task1: %v", err)
	}
	init.AddTask("TASK-001", "Task One", nil)

	task2 := task.NewProtoTask("TASK-002", "Task Two")
	task.SetInitiativeProto(task2,"INIT-001")
	if err := backend.SaveTask(task2); err != nil {
		t.Fatalf("save task2: %v", err)
	}
	init.AddTask("TASK-002", "Task Two", nil)

	task3 := task.NewProtoTask("TASK-003", "Task Three")
	task.SetInitiativeProto(task3,"INIT-001")
	if err := backend.SaveTask(task3); err != nil {
		t.Fatalf("save task3: %v", err)
	}
	init.AddTask("TASK-003", "Task Three", nil)

	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Close backend before running command
	_ = backend.Close()

	// Run unlink command for 2 tasks
	cmd := newInitiativeUnlinkCmd()
	cmd.SetArgs([]string{"INIT-001", "TASK-001", "TASK-002"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unlink command failed: %v", err)
	}

	// Re-open backend to verify
	backend = createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Verify tasks 1 and 2 are unlinked
	reloadedTask1, _ := backend.LoadTask("TASK-001")
	if reloadedTask1.GetInitiativeId() != "" {
		t.Errorf("task1 should be unlinked, got InitiativeID = %q", reloadedTask1.GetInitiativeId())
	}

	reloadedTask2, _ := backend.LoadTask("TASK-002")
	if reloadedTask2.GetInitiativeId() != "" {
		t.Errorf("task2 should be unlinked, got InitiativeID = %q", reloadedTask2.GetInitiativeId())
	}

	// Task 3 should still be linked
	reloadedTask3, _ := backend.LoadTask("TASK-003")
	if reloadedTask3.GetInitiativeId() != "INIT-001" {
		t.Errorf("task3 should still be linked, got InitiativeID = %q", reloadedTask3.GetInitiativeId())
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
	defer func() { _ = backend.Close() }()

	// Create initiative with tasks
	init := initiative.New("INIT-001", "Test Initiative")

	// Create and link tasks
	task1 := task.NewProtoTask("TASK-001", "Task One")
	task.SetInitiativeProto(task1,"INIT-001")
	if err := backend.SaveTask(task1); err != nil {
		t.Fatalf("save task1: %v", err)
	}
	init.AddTask("TASK-001", "Task One", nil)

	task2 := task.NewProtoTask("TASK-002", "Task Two")
	task.SetInitiativeProto(task2,"INIT-001")
	if err := backend.SaveTask(task2); err != nil {
		t.Fatalf("save task2: %v", err)
	}
	init.AddTask("TASK-002", "Task Two", nil)

	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Close backend before running command
	_ = backend.Close()

	// Run unlink command with --all
	cmd := newInitiativeUnlinkCmd()
	cmd.SetArgs([]string{"INIT-001", "--all"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unlink command failed: %v", err)
	}

	// Re-open backend to verify
	backend = createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Verify all tasks are unlinked
	reloadedTask1, _ := backend.LoadTask("TASK-001")
	if reloadedTask1.GetInitiativeId() != "" {
		t.Errorf("task1 should be unlinked, got InitiativeID = %q", reloadedTask1.GetInitiativeId())
	}

	reloadedTask2, _ := backend.LoadTask("TASK-002")
	if reloadedTask2.GetInitiativeId() != "" {
		t.Errorf("task2 should be unlinked, got InitiativeID = %q", reloadedTask2.GetInitiativeId())
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
	defer func() { _ = backend.Close() }()

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
	task1 := task.NewProtoTask("TASK-001", "Task One")
	task.SetInitiativeProto(task1,"INIT-002")
	if err := backend.SaveTask(task1); err != nil {
		t.Fatalf("save task1: %v", err)
	}

	// Create unlinked task
	task2 := task.NewProtoTask("TASK-002", "Task Two")
	if err := backend.SaveTask(task2); err != nil {
		t.Fatalf("save task2: %v", err)
	}

	// Close backend before running command
	_ = backend.Close()

	// Run link command to link both tasks to INIT-001
	cmd := newInitiativeLinkCmd()
	cmd.SetArgs([]string{"INIT-001", "TASK-001", "TASK-002"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("link command failed: %v", err)
	}

	// Re-open backend to verify
	backend = createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// TASK-001 should still be linked to INIT-002 (skipped)
	reloadedTask1, _ := backend.LoadTask("TASK-001")
	if reloadedTask1.GetInitiativeId() != "INIT-002" {
		t.Errorf("task1 should still be linked to INIT-002, got %q", reloadedTask1.GetInitiativeId())
	}

	// TASK-002 should now be linked to INIT-001
	reloadedTask2, _ := backend.LoadTask("TASK-002")
	if reloadedTask2.GetInitiativeId() != "INIT-001" {
		t.Errorf("task2 should be linked to INIT-001, got %q", reloadedTask2.GetInitiativeId())
	}
}

func TestInitiativeLinkFixesPartialLink(t *testing.T) {
	// This tests the case where task has initiative_id set but is NOT in
	// the initiative's task list. The link command should add it to the list.
	tmpDir := withInitiativeTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Create initiative WITHOUT any tasks
	init := initiative.New("INIT-001", "Test Initiative")
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create task that claims to be linked to INIT-001 but isn't in the list
	task1 := task.NewProtoTask("TASK-001", "Task One")
	task.SetInitiativeProto(task1,"INIT-001") // Set initiative_id
	if err := backend.SaveTask(task1); err != nil {
		t.Fatalf("save task1: %v", err)
	}

	// Verify the initial broken state: task has initiative_id but init has no tasks
	if len(init.Tasks) != 0 {
		t.Fatalf("expected 0 tasks in initiative, got %d", len(init.Tasks))
	}
	if task1.GetInitiativeId() != "INIT-001" {
		t.Fatalf("task should have initiative_id INIT-001, got %q", task1.GetInitiativeId())
	}

	// Close backend before running command
	_ = backend.Close()

	// Run link command - should add task to initiative despite having initiative_id
	cmd := newInitiativeLinkCmd()
	cmd.SetArgs([]string{"INIT-001", "TASK-001"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("link command failed: %v", err)
	}

	// Re-open backend to verify
	backend = createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

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
	defer func() { _ = backend.Close() }()

	// Create initiative with TASK-001 already in the list
	init := initiative.New("INIT-001", "Test Initiative")
	init.AddTask("TASK-001", "Task One", nil)
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create task with initiative_id set
	task1 := task.NewProtoTask("TASK-001", "Task One")
	task.SetInitiativeProto(task1,"INIT-001")
	if err := backend.SaveTask(task1); err != nil {
		t.Fatalf("save task1: %v", err)
	}

	// Also create a second task that should be linked
	task2 := task.NewProtoTask("TASK-002", "Task Two")
	if err := backend.SaveTask(task2); err != nil {
		t.Fatalf("save task2: %v", err)
	}

	// Close backend before running command
	_ = backend.Close()

	// Run link command - should skip TASK-001 and link TASK-002
	cmd := newInitiativeLinkCmd()
	cmd.SetArgs([]string{"INIT-001", "TASK-001", "TASK-002"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("link command failed: %v", err)
	}

	// Re-open backend to verify
	backend = createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

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
	if reloadedTask2.GetInitiativeId() != "INIT-001" {
		t.Errorf("task2 should be linked to INIT-001, got %q", reloadedTask2.GetInitiativeId())
	}
}

func TestInitiativeLinkPatternFixesPartialLink(t *testing.T) {
	// Tests that --all-matching also fixes partial links
	tmpDir := withInitiativeTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Create initiative WITHOUT any tasks
	init := initiative.New("INIT-001", "Test Initiative")
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create task that claims to be linked to INIT-001 but isn't in the list
	task1 := task.NewProtoTask("TASK-001", "Auth Login")
	task.SetInitiativeProto(task1,"INIT-001") // Set initiative_id, but not in initiative's list
	if err := backend.SaveTask(task1); err != nil {
		t.Fatalf("save task1: %v", err)
	}

	// Close backend before running command
	_ = backend.Close()

	// Run link command with pattern matching
	cmd := newInitiativeLinkCmd()
	cmd.SetArgs([]string{"INIT-001", "--all-matching", "auth"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("link command failed: %v", err)
	}

	// Re-open backend to verify
	backend = createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

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

// Tests for initiative plan command

func TestInitiativePlanCommand_Structure(t *testing.T) {
	cmd := newInitiativePlanCmd()

	// Verify command structure
	if cmd.Use != "plan <manifest.yaml>" {
		t.Errorf("command Use = %q, want %q", cmd.Use, "plan <manifest.yaml>")
	}

	// Verify flags exist
	if cmd.Flag("dry-run") == nil {
		t.Error("missing --dry-run flag")
	}
	if cmd.Flag("yes") == nil {
		t.Error("missing --yes flag")
	}
	if cmd.Flag("create-initiative") == nil {
		t.Error("missing --create-initiative flag")
	}
}

func TestInitiativePlanCommand_RequiresArg(t *testing.T) {
	cmd := newInitiativePlanCmd()

	// Should require exactly one argument
	if err := cmd.Args(cmd, []string{}); err == nil {
		t.Error("expected error for zero args")
	}
	if err := cmd.Args(cmd, []string{"manifest.yaml"}); err != nil {
		t.Errorf("unexpected error for one arg: %v", err)
	}
	if err := cmd.Args(cmd, []string{"one.yaml", "two.yaml"}); err == nil {
		t.Error("expected error for two args")
	}
}

func TestInitiativePlanWithExistingInitiative(t *testing.T) {
	tmpDir := withInitiativeTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Create initiative
	init := initiative.New("INIT-001", "Test Initiative")
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create manifest file
	manifest := `version: 1
initiative: INIT-001
tasks:
  - id: 1
    title: "First task"
    weight: small
  - id: 2
    title: "Second task"
    weight: medium
    depends_on: [1]
`
	manifestPath := filepath.Join(tmpDir, "tasks.yaml")
	if err := os.WriteFile(manifestPath, []byte(manifest), 0644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	// Close backend before running command
	_ = backend.Close()

	// Run plan command with --yes to skip confirmation
	cmd := newInitiativePlanCmd()
	cmd.SetArgs([]string{manifestPath, "--yes"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("plan command failed: %v", err)
	}

	// Re-open backend to verify
	backend = createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Verify tasks were created
	allTasks, err := backend.LoadAllTasks()
	if err != nil {
		t.Fatalf("load tasks: %v", err)
	}
	if len(allTasks) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(allTasks))
	}

	// Verify initiative has tasks
	reloadedInit, err := backend.LoadInitiative("INIT-001")
	if err != nil {
		t.Fatalf("reload initiative: %v", err)
	}
	if len(reloadedInit.Tasks) != 2 {
		t.Errorf("initiative should have 2 tasks, got %d", len(reloadedInit.Tasks))
	}

	// Verify dependencies
	var secondTask *orcv1.Task
	for _, tk := range allTasks {
		if tk.Title == "Second task" {
			secondTask = tk
			break
		}
	}
	if secondTask == nil {
		t.Fatal("could not find second task")
	}
	if len(secondTask.BlockedBy) != 1 {
		t.Errorf("second task should have 1 blocker, got %d", len(secondTask.BlockedBy))
	}
}

func TestInitiativePlanWithCreateInitiative(t *testing.T) {
	tmpDir := withInitiativeTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Create manifest file with create_initiative
	manifest := `version: 1
create_initiative:
  title: "New Auth System"
  vision: "OAuth2 support for Google and GitHub"
tasks:
  - id: 1
    title: "Add OAuth config"
    weight: small
    spec: |
      # Specification
      Add OAuth configuration structure.
`
	manifestPath := filepath.Join(tmpDir, "tasks.yaml")
	if err := os.WriteFile(manifestPath, []byte(manifest), 0644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	// Close backend before running command
	_ = backend.Close()

	// Run plan command
	cmd := newInitiativePlanCmd()
	cmd.SetArgs([]string{manifestPath, "--yes"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("plan command failed: %v", err)
	}

	// Re-open backend to verify
	backend = createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Verify initiative was created
	initiatives, err := backend.LoadAllInitiatives()
	if err != nil {
		t.Fatalf("load initiatives: %v", err)
	}
	if len(initiatives) != 1 {
		t.Fatalf("expected 1 initiative, got %d", len(initiatives))
	}

	createdInit := initiatives[0]
	if createdInit.Title != "New Auth System" {
		t.Errorf("initiative title = %q, want %q", createdInit.Title, "New Auth System")
	}
	if createdInit.Vision != "OAuth2 support for Google and GitHub" {
		t.Errorf("initiative vision = %q, want %q", createdInit.Vision, "OAuth2 support for Google and GitHub")
	}

	// Verify task was created
	tasks, err := backend.LoadAllTasks()
	if err != nil {
		t.Fatalf("load tasks: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}

	// Verify spec was stored
	spec, err := backend.GetSpecForTask(tasks[0].Id)
	if err != nil {
		t.Fatalf("get spec: %v", err)
	}
	if spec == "" {
		t.Error("spec should not be empty")
	}
	if !strings.Contains(spec, "Add OAuth configuration") {
		t.Errorf("spec should contain 'Add OAuth configuration', got %q", spec)
	}
}

func TestInitiativePlanDryRun(t *testing.T) {
	tmpDir := withInitiativeTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Create initiative
	init := initiative.New("INIT-001", "Test Initiative")
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create manifest file
	manifest := `version: 1
initiative: INIT-001
tasks:
  - id: 1
    title: "Task that should not be created"
    weight: small
`
	manifestPath := filepath.Join(tmpDir, "tasks.yaml")
	if err := os.WriteFile(manifestPath, []byte(manifest), 0644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	// Close backend before running command
	_ = backend.Close()

	// Run plan command with --dry-run
	cmd := newInitiativePlanCmd()
	cmd.SetArgs([]string{manifestPath, "--dry-run"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("plan command failed: %v", err)
	}

	// Re-open backend to verify
	backend = createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Verify NO tasks were created
	tasks, err := backend.LoadAllTasks()
	if err != nil {
		t.Fatalf("load tasks: %v", err)
	}
	if len(tasks) != 0 {
		t.Errorf("dry run should not create tasks, got %d", len(tasks))
	}
}

func TestInitiativePlanMissingInitiative(t *testing.T) {
	tmpDir := withInitiativeTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Create manifest file referencing non-existent initiative
	manifest := `version: 1
initiative: INIT-999
tasks:
  - id: 1
    title: "Task"
    weight: small
`
	manifestPath := filepath.Join(tmpDir, "tasks.yaml")
	if err := os.WriteFile(manifestPath, []byte(manifest), 0644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	// Close backend before running command
	_ = backend.Close()

	// Run plan command - should fail
	cmd := newInitiativePlanCmd()
	cmd.SetArgs([]string{manifestPath, "--yes"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing initiative")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention 'not found', got: %v", err)
	}
}

func TestInitiativePlanCreateInitiativeFlag(t *testing.T) {
	tmpDir := withInitiativeTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Create manifest file referencing non-existent initiative
	manifest := `version: 1
initiative: INIT-001
tasks:
  - id: 1
    title: "Task"
    weight: small
`
	manifestPath := filepath.Join(tmpDir, "tasks.yaml")
	if err := os.WriteFile(manifestPath, []byte(manifest), 0644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	// Close backend before running command
	_ = backend.Close()

	// Run plan command with --create-initiative
	cmd := newInitiativePlanCmd()
	cmd.SetArgs([]string{manifestPath, "--yes", "--create-initiative"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("plan command failed: %v", err)
	}

	// Re-open backend to verify
	backend = createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Verify initiative was created
	exists, err := backend.InitiativeExists("INIT-001")
	if err != nil {
		t.Fatalf("check initiative: %v", err)
	}
	if !exists {
		t.Error("initiative should have been created")
	}

	// Verify task was created
	tasks, err := backend.LoadAllTasks()
	if err != nil {
		t.Fatalf("load tasks: %v", err)
	}
	if len(tasks) != 1 {
		t.Errorf("expected 1 task, got %d", len(tasks))
	}
}

func TestInitiativePlanDependencyOrder(t *testing.T) {
	tmpDir := withInitiativeTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Create initiative
	init := initiative.New("INIT-001", "Test Initiative")
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create manifest with dependencies that require proper ordering
	// Task 3 depends on 1 and 2, Task 2 depends on 1
	manifest := `version: 1
initiative: INIT-001
tasks:
  - id: 3
    title: "Third task"
    depends_on: [1, 2]
  - id: 1
    title: "First task"
  - id: 2
    title: "Second task"
    depends_on: [1]
`
	manifestPath := filepath.Join(tmpDir, "tasks.yaml")
	if err := os.WriteFile(manifestPath, []byte(manifest), 0644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	// Close backend before running command
	_ = backend.Close()

	// Run plan command
	cmd := newInitiativePlanCmd()
	cmd.SetArgs([]string{manifestPath, "--yes"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("plan command failed: %v", err)
	}

	// Re-open backend to verify
	backend = createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Verify all tasks were created
	tasks, err := backend.LoadAllTasks()
	if err != nil {
		t.Fatalf("load tasks: %v", err)
	}
	if len(tasks) != 3 {
		t.Errorf("expected 3 tasks, got %d", len(tasks))
	}

	// Verify dependencies reference correct task IDs
	taskByTitle := make(map[string]*orcv1.Task)
	for _, tk := range tasks {
		taskByTitle[tk.Title] = tk
	}

	// Third task should depend on First and Second
	thirdTask := taskByTitle["Third task"]
	if thirdTask == nil {
		t.Fatal("could not find third task")
	}
	if len(thirdTask.BlockedBy) != 2 {
		t.Errorf("third task should have 2 blockers, got %d", len(thirdTask.BlockedBy))
	}

	// Second task should depend on First
	secondTask := taskByTitle["Second task"]
	if secondTask == nil {
		t.Fatal("could not find second task")
	}
	if len(secondTask.BlockedBy) != 1 {
		t.Errorf("second task should have 1 blocker, got %d", len(secondTask.BlockedBy))
	}

	// The blocker IDs should be the real task IDs
	firstTask := taskByTitle["First task"]
	if firstTask == nil {
		t.Fatal("could not find first task")
	}
	if secondTask.BlockedBy[0] != firstTask.Id {
		t.Errorf("second task blocker = %q, want %q", secondTask.BlockedBy[0], firstTask.Id)
	}
}

func TestInitiativePlanTasksHaveCorrectInitiative(t *testing.T) {
	tmpDir := withInitiativeTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Create initiative
	init := initiative.New("INIT-001", "Test Initiative")
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create manifest file
	manifest := `version: 1
initiative: INIT-001
tasks:
  - id: 1
    title: "Task One"
  - id: 2
    title: "Task Two"
`
	manifestPath := filepath.Join(tmpDir, "tasks.yaml")
	if err := os.WriteFile(manifestPath, []byte(manifest), 0644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	// Close backend before running command
	_ = backend.Close()

	// Run plan command
	cmd := newInitiativePlanCmd()
	cmd.SetArgs([]string{manifestPath, "--yes"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("plan command failed: %v", err)
	}

	// Re-open backend to verify
	backend = createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Verify all tasks are linked to initiative
	tasks, err := backend.LoadAllTasks()
	if err != nil {
		t.Fatalf("load tasks: %v", err)
	}
	for _, tk := range tasks {
		if tk.GetInitiativeId() != "INIT-001" {
			t.Errorf("task %s InitiativeID = %q, want INIT-001", tk.Id, tk.GetInitiativeId())
		}
	}
}

// =============================================================================
// Tests for initiative auto-completion on list/show (TASK-525)
// =============================================================================

// TestInitiativeListAutoCompletes tests that the list command triggers
// auto-completion for eligible initiatives when all their tasks are done.
// Covers SC-2: CheckAndCompleteInitiativeNoBranch is called when listing.
func TestInitiativeListAutoCompletes(t *testing.T) {
	tmpDir := withInitiativeTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Create initiative WITHOUT BranchBase, in active status
	init := initiative.New("INIT-001", "Should Auto-Complete")
	init.Status = initiative.StatusActive
	init.AddTask("TASK-001", "Completed task", nil)
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create completed task
	tk := task.NewProtoTask("TASK-001", "Completed task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	task.SetInitiativeProto(tk,"INIT-001")
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Close backend before running command
	_ = backend.Close()

	// Run list command - this should trigger auto-completion
	cmd := newInitiativeListCmd()
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	// Re-open backend to verify status change
	backend = createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	reloaded, err := backend.LoadInitiative("INIT-001")
	if err != nil {
		t.Fatalf("reload initiative: %v", err)
	}

	// Initiative should now be completed (auto-completed by list)
	if reloaded.Status != initiative.StatusCompleted {
		t.Errorf("initiative Status = %q, want %q (should auto-complete on list)",
			reloaded.Status, initiative.StatusCompleted)
	}
}

// TestInitiativeListDoesNotAutoCompleteWithBranchBase tests that initiatives
// with BranchBase are skipped during auto-completion (they use merge flow).
func TestInitiativeListDoesNotAutoCompleteWithBranchBase(t *testing.T) {
	tmpDir := withInitiativeTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Create initiative WITH BranchBase
	init := initiative.New("INIT-001", "Feature Branch Initiative")
	init.Status = initiative.StatusActive
	init.BranchBase = "feature/auth" // Has branch base - should use merge flow
	init.AddTask("TASK-001", "Task", nil)
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create completed task
	tk := task.NewProtoTask("TASK-001", "Task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	task.SetInitiativeProto(tk,"INIT-001")
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Close backend before running command
	_ = backend.Close()

	// Run list command
	cmd := newInitiativeListCmd()
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	// Re-open backend to verify status is unchanged
	backend = createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	reloaded, _ := backend.LoadInitiative("INIT-001")
	if reloaded.Status != initiative.StatusActive {
		t.Errorf("initiative Status = %q, want %q (BranchBase initiatives should use merge flow)",
			reloaded.Status, initiative.StatusActive)
	}
}

// TestInitiativeListCompletedNotShowBlocked tests that completed initiatives
// don't show "[BLOCKED]" in the CLI output even if they have BlockedBy deps.
// Covers SC-3: Completed initiatives don't show "[BLOCKED]" in CLI output.
func TestInitiativeListCompletedNotShowBlocked(t *testing.T) {
	tmpDir := withInitiativeTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Create "blocker" initiative that is NOT completed
	blocker := initiative.New("INIT-001", "Blocker Initiative")
	blocker.Status = initiative.StatusActive // Not completed!
	if err := backend.SaveInitiative(blocker); err != nil {
		t.Fatalf("save blocker: %v", err)
	}

	// Create completed initiative that has BlockedBy dependency on the blocker
	completed := initiative.New("INIT-002", "Completed With Blocker")
	completed.Status = initiative.StatusCompleted // Already completed
	completed.BlockedBy = []string{"INIT-001"}    // Has unmet blocker!
	if err := backend.SaveInitiative(completed); err != nil {
		t.Fatalf("save completed: %v", err)
	}

	// Close backend before running command
	_ = backend.Close()

	// Capture stdout to verify output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := newInitiativeListCmd()
	if err := cmd.Execute(); err != nil {
		os.Stdout = oldStdout
		t.Fatalf("list command failed: %v", err)
	}

	// Restore stdout and read captured output
	_ = w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	// Re-open backend for verification
	backend = createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Verify that INIT-002 is still completed
	reloaded, _ := backend.LoadInitiative("INIT-002")
	if reloaded.Status != initiative.StatusCompleted {
		t.Errorf("completed initiative status changed unexpectedly to %q", reloaded.Status)
	}

	// The key test: completed initiatives should NOT show "[BLOCKED]"
	// Note: The current implementation appends "[BLOCKED]" even for completed.
	// After the fix, completed initiatives should NOT show "[BLOCKED]".
	if strings.Contains(output, "completed [BLOCKED]") {
		t.Errorf("completed initiative should not show [BLOCKED], got output:\n%s", output)
	}
}

// TestInitiativeShowAutoCompletes tests that the show command displays
// correct status after triggering auto-completion.
// Covers SC-5: orc initiative show shows correct status after auto-completion.
func TestInitiativeShowAutoCompletes(t *testing.T) {
	tmpDir := withInitiativeTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Create initiative WITHOUT BranchBase, in active status
	init := initiative.New("INIT-001", "Should Complete on Show")
	init.Status = initiative.StatusActive
	init.AddTask("TASK-001", "Done task", nil)
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create completed task
	tk := task.NewProtoTask("TASK-001", "Done task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	task.SetInitiativeProto(tk,"INIT-001")
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Close backend before running command
	_ = backend.Close()

	// Run show command - this should trigger auto-completion
	cmd := newInitiativeShowCmd()
	cmd.SetArgs([]string{"INIT-001"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("show command failed: %v", err)
	}

	// Re-open backend to verify status change
	backend = createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	reloaded, err := backend.LoadInitiative("INIT-001")
	if err != nil {
		t.Fatalf("reload initiative: %v", err)
	}

	// Initiative should now be completed (auto-completed by show)
	if reloaded.Status != initiative.StatusCompleted {
		t.Errorf("initiative Status = %q, want %q (should auto-complete on show)",
			reloaded.Status, initiative.StatusCompleted)
	}
}

// TestInitiativeListAutoCompleteDoesNotBreakOnError tests that auto-completion
// errors on one initiative don't prevent listing other initiatives.
// Failure mode: Task loader fails should skip auto-completion, log warning.
func TestInitiativeListAutoCompleteDoesNotBreakOnError(t *testing.T) {
	tmpDir := withInitiativeTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Create first initiative with missing task (will fail auto-completion check)
	init1 := initiative.New("INIT-001", "Has Missing Task")
	init1.Status = initiative.StatusActive
	init1.AddTask("TASK-MISSING", "Task doesn't exist", nil)
	if err := backend.SaveInitiative(init1); err != nil {
		t.Fatalf("save init1: %v", err)
	}

	// Create second initiative that should auto-complete successfully
	init2 := initiative.New("INIT-002", "Should Complete")
	init2.Status = initiative.StatusActive
	init2.AddTask("TASK-001", "Completed task", nil)
	if err := backend.SaveInitiative(init2); err != nil {
		t.Fatalf("save init2: %v", err)
	}

	// Create the task for init2
	tk := task.NewProtoTask("TASK-001", "Completed task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	task.SetInitiativeProto(tk,"INIT-002")
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Close backend before running command
	_ = backend.Close()

	// Run list command - should not fail even with one problematic initiative
	cmd := newInitiativeListCmd()
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list command should not fail due to one initiative error: %v", err)
	}

	// Re-open backend to verify
	backend = createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// INIT-001 should remain active (couldn't verify tasks)
	reloaded1, _ := backend.LoadInitiative("INIT-001")
	if reloaded1.Status != initiative.StatusActive {
		t.Errorf("INIT-001 Status = %q, want %q", reloaded1.Status, initiative.StatusActive)
	}

	// INIT-002 should be completed
	reloaded2, _ := backend.LoadInitiative("INIT-002")
	if reloaded2.Status != initiative.StatusCompleted {
		t.Errorf("INIT-002 Status = %q, want %q", reloaded2.Status, initiative.StatusCompleted)
	}
}

// =============================================================================
// Tests for initiative show dependency display (TASK-644)
// =============================================================================

// TestInitiativeShowDisplaysDepsForTaskWithBlockedBy verifies that when a task
// has blocked_by dependencies, `orc initiative show` displays them in the
// deps column instead of "-".
// Covers SC-1: Task with blocked_by shows deps: TASK-XXX
// Covers SC-2: Task with no dependencies shows deps: -
func TestInitiativeShowDisplaysDepsForTaskWithBlockedBy(t *testing.T) {
	tmpDir := withInitiativeTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Create initiative with two tasks
	init := initiative.New("INIT-001", "Deps Test Initiative")
	init.Status = initiative.StatusActive
	init.AddTask("TASK-001", "Independent Task", nil)
	init.AddTask("TASK-002", "Dependent Task", nil)
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create TASK-001 with no dependencies
	tk1 := task.NewProtoTask("TASK-001", "Independent Task")
	tk1.Status = orcv1.TaskStatus_TASK_STATUS_PLANNED
	task.SetInitiativeProto(tk1, "INIT-001")
	if err := backend.SaveTask(tk1); err != nil {
		t.Fatalf("save task1: %v", err)
	}

	// Create TASK-002 that depends on TASK-001
	tk2 := task.NewProtoTask("TASK-002", "Dependent Task")
	tk2.Status = orcv1.TaskStatus_TASK_STATUS_PLANNED
	tk2.BlockedBy = []string{"TASK-001"}
	task.SetInitiativeProto(tk2, "INIT-001")
	if err := backend.SaveTask(tk2); err != nil {
		t.Fatalf("save task2: %v", err)
	}

	// Close backend before running command (command creates its own)
	_ = backend.Close()

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := newInitiativeShowCmd()
	cmd.SetArgs([]string{"INIT-001"})
	if err := cmd.Execute(); err != nil {
		os.Stdout = oldStdout
		t.Fatalf("show command failed: %v", err)
	}

	_ = w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	// Find lines for each task (use HasPrefix to avoid matching deps column)
	lines := strings.Split(output, "\n")
	var task1Line, task2Line string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "TASK-001") {
			task1Line = line
		}
		if strings.HasPrefix(trimmed, "TASK-002") {
			task2Line = line
		}
	}

	// SC-2: Task with no deps should show "deps: -"
	if task1Line == "" {
		t.Fatal("TASK-001 not found in output")
	}
	if !strings.Contains(task1Line, "deps: -") {
		t.Errorf("TASK-001 should show 'deps: -', got line: %s", task1Line)
	}

	// SC-1: Task with blocked_by should show the dependency
	if task2Line == "" {
		t.Fatal("TASK-002 not found in output")
	}
	if !strings.Contains(task2Line, "deps: TASK-001") {
		t.Errorf("TASK-002 should show 'deps: TASK-001', got line: %s", task2Line)
	}
}

// TestInitiativeShowDisplaysMultipleDeps verifies that tasks with multiple
// blocked_by dependencies show all of them comma-separated.
// Covers SC-3: Multiple dependencies shown comma-separated
func TestInitiativeShowDisplaysMultipleDeps(t *testing.T) {
	tmpDir := withInitiativeTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Create initiative with three tasks
	init := initiative.New("INIT-001", "Multi Deps Initiative")
	init.Status = initiative.StatusActive
	init.AddTask("TASK-001", "First", nil)
	init.AddTask("TASK-002", "Second", nil)
	init.AddTask("TASK-003", "Third", nil)
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create TASK-001 and TASK-002 (no deps)
	tk1 := task.NewProtoTask("TASK-001", "First")
	tk1.Status = orcv1.TaskStatus_TASK_STATUS_PLANNED
	task.SetInitiativeProto(tk1, "INIT-001")
	if err := backend.SaveTask(tk1); err != nil {
		t.Fatalf("save task1: %v", err)
	}

	tk2 := task.NewProtoTask("TASK-002", "Second")
	tk2.Status = orcv1.TaskStatus_TASK_STATUS_PLANNED
	task.SetInitiativeProto(tk2, "INIT-001")
	if err := backend.SaveTask(tk2); err != nil {
		t.Fatalf("save task2: %v", err)
	}

	// Create TASK-003 that depends on both TASK-001 and TASK-002
	tk3 := task.NewProtoTask("TASK-003", "Third")
	tk3.Status = orcv1.TaskStatus_TASK_STATUS_PLANNED
	tk3.BlockedBy = []string{"TASK-001", "TASK-002"}
	task.SetInitiativeProto(tk3, "INIT-001")
	if err := backend.SaveTask(tk3); err != nil {
		t.Fatalf("save task3: %v", err)
	}

	// Close backend before running command
	_ = backend.Close()

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := newInitiativeShowCmd()
	cmd.SetArgs([]string{"INIT-001"})
	if err := cmd.Execute(); err != nil {
		os.Stdout = oldStdout
		t.Fatalf("show command failed: %v", err)
	}

	_ = w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	// Find TASK-003 line
	lines := strings.Split(output, "\n")
	var task3Line string
	for _, line := range lines {
		if strings.Contains(line, "TASK-003") {
			task3Line = line
			break
		}
	}

	if task3Line == "" {
		t.Fatal("TASK-003 not found in output")
	}

	// SC-3: Should show both deps comma-separated
	if !strings.Contains(task3Line, "TASK-001") || !strings.Contains(task3Line, "TASK-002") {
		t.Errorf("TASK-003 should show both deps, got line: %s", task3Line)
	}
	// Verify comma separation format
	if !strings.Contains(task3Line, "deps: TASK-001, TASK-002") {
		t.Errorf("TASK-003 deps should be comma-separated, got line: %s", task3Line)
	}
}

// TestInitiativeShowDepsWhenBlockerTaskDeletedFromDB verifies graceful handling
// when a task's blocker dependency no longer exists in the database.
// The task itself exists, but one of its blocked_by tasks was deleted.
// Edge case: deps still show the ID even if the blocker task is gone
func TestInitiativeShowDepsWhenBlockerTaskDeletedFromDB(t *testing.T) {
	tmpDir := withInitiativeTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Create initiative with TASK-001
	init := initiative.New("INIT-001", "Deleted Blocker Initiative")
	init.Status = initiative.StatusActive
	init.AddTask("TASK-001", "Task with deleted blocker", nil)
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create TASK-001 that depends on TASK-999 (which doesn't exist in DB)
	tk := task.NewProtoTask("TASK-001", "Task with deleted blocker")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_PLANNED
	tk.BlockedBy = []string{"TASK-999"}
	task.SetInitiativeProto(tk, "INIT-001")
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Close backend before running command
	_ = backend.Close()

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := newInitiativeShowCmd()
	cmd.SetArgs([]string{"INIT-001"})
	if err := cmd.Execute(); err != nil {
		os.Stdout = oldStdout
		t.Fatalf("show command failed: %v", err)
	}

	_ = w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	// TASK-001 should show deps: TASK-999 even though TASK-999 doesn't exist
	// (the dependency is stored on TASK-001, not on the blocker)
	lines := strings.Split(output, "\n")
	var taskLine string
	for _, line := range lines {
		if strings.Contains(line, "TASK-001") {
			taskLine = line
			break
		}
	}

	if taskLine == "" {
		t.Fatalf("TASK-001 not found in output. Full output:\n%s", output)
	}
	if !strings.Contains(taskLine, "deps: TASK-999") {
		t.Errorf("task should show 'deps: TASK-999' even if blocker is deleted, got line: %s", taskLine)
	}
}

// TestInitiativeShowDepsOnExternalTask verifies that when a task depends on
// a task NOT in the initiative, the dep is still shown.
// Edge case: dependency on task outside the initiative
func TestInitiativeShowDepsOnExternalTask(t *testing.T) {
	tmpDir := withInitiativeTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Create initiative with only TASK-002
	init := initiative.New("INIT-001", "External Dep Initiative")
	init.Status = initiative.StatusActive
	init.AddTask("TASK-002", "Dependent Task", nil)
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create TASK-099 (NOT in the initiative, but exists in DB)
	tkExternal := task.NewProtoTask("TASK-099", "External Task")
	tkExternal.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	if err := backend.SaveTask(tkExternal); err != nil {
		t.Fatalf("save external task: %v", err)
	}

	// Create TASK-002 that depends on TASK-099 (which is not in the initiative)
	tk2 := task.NewProtoTask("TASK-002", "Dependent Task")
	tk2.Status = orcv1.TaskStatus_TASK_STATUS_PLANNED
	tk2.BlockedBy = []string{"TASK-099"}
	task.SetInitiativeProto(tk2, "INIT-001")
	if err := backend.SaveTask(tk2); err != nil {
		t.Fatalf("save task2: %v", err)
	}

	// Close backend before running command
	_ = backend.Close()

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := newInitiativeShowCmd()
	cmd.SetArgs([]string{"INIT-001"})
	if err := cmd.Execute(); err != nil {
		os.Stdout = oldStdout
		t.Fatalf("show command failed: %v", err)
	}

	_ = w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	// TASK-002 should show deps: TASK-099 even though TASK-099 is not in initiative
	lines := strings.Split(output, "\n")
	var task2Line string
	for _, line := range lines {
		if strings.Contains(line, "TASK-002") {
			task2Line = line
			break
		}
	}

	if task2Line == "" {
		t.Fatal("TASK-002 not found in output")
	}
	if !strings.Contains(task2Line, "deps: TASK-099") {
		t.Errorf("TASK-002 should show 'deps: TASK-099', got line: %s", task2Line)
	}
}

// TestInitiativeRunDisplaysDeps verifies that the `orc initiative run` command
// also displays correct dependency information when no tasks are ready.
// Covers SC-4: initiative run output shows correct deps
func TestInitiativeRunDisplaysDeps(t *testing.T) {
	tmpDir := withInitiativeTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Create initiative with tasks that have deps (all blocked, none ready)
	init := initiative.New("INIT-001", "Run Deps Initiative")
	init.Status = initiative.StatusActive
	init.AddTask("TASK-001", "First Task", nil)
	init.AddTask("TASK-002", "Second Task", []string{"TASK-001"})
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create TASK-001 as running (not completed, so TASK-002 stays blocked)
	tk1 := task.NewProtoTask("TASK-001", "First Task")
	tk1.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	task.SetInitiativeProto(tk1, "INIT-001")
	if err := backend.SaveTask(tk1); err != nil {
		t.Fatalf("save task1: %v", err)
	}

	// Create TASK-002 that depends on TASK-001
	tk2 := task.NewProtoTask("TASK-002", "Second Task")
	tk2.Status = orcv1.TaskStatus_TASK_STATUS_PLANNED
	tk2.BlockedBy = []string{"TASK-001"}
	task.SetInitiativeProto(tk2, "INIT-001")
	if err := backend.SaveTask(tk2); err != nil {
		t.Fatalf("save task2: %v", err)
	}

	// Close backend before running command
	_ = backend.Close()

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := newInitiativeRunCmd()
	cmd.SetArgs([]string{"INIT-001"})
	// Don't use --execute; just preview mode (default)
	if err := cmd.Execute(); err != nil {
		os.Stdout = oldStdout
		t.Fatalf("run command failed: %v", err)
	}

	_ = w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	// The run command should show "No tasks ready to run" with task status
	// TASK-002 should show its dependency on TASK-001
	if !strings.Contains(output, "TASK-002") {
		t.Fatalf("output should contain TASK-002, got:\n%s", output)
	}

	// SC-4: TASK-002's line should show depends on TASK-001
	lines := strings.Split(output, "\n")
	var task2Line string
	for _, line := range lines {
		if strings.Contains(line, "TASK-002") {
			task2Line = line
			break
		}
	}

	if task2Line == "" {
		t.Fatal("TASK-002 not found in output")
	}
	if !strings.Contains(task2Line, "TASK-001") {
		t.Errorf("TASK-002 line should reference dependency TASK-001, got: %s", task2Line)
	}
}
