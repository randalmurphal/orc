package cli

// NOTE: Tests in this file use os.Chdir() which is process-wide and not goroutine-safe.
// These tests MUST NOT use t.Parallel() and run sequentially within this package.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/randalmurphal/orc/internal/initiative"
	"github.com/randalmurphal/orc/internal/task"
)

// withInitiativeTestDir creates a temp directory with initiative and task structure,
// changes to it, and restores the original working directory when the test completes.
func withInitiativeTestDir(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()

	// Create initiative directories
	initDir := filepath.Join(tmpDir, ".orc", "initiatives", "INIT-001")
	if err := os.MkdirAll(initDir, 0755); err != nil {
		t.Fatalf("create initiative directory: %v", err)
	}

	// Create tasks directory
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
	withInitiativeTestDir(t)

	// Create initiative
	init := initiative.New("INIT-001", "Test Initiative")
	if err := init.Save(); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create tasks
	task1 := task.New("TASK-001", "Task One")
	if err := task1.Save(); err != nil {
		t.Fatalf("save task1: %v", err)
	}
	task2 := task.New("TASK-002", "Task Two")
	if err := task2.Save(); err != nil {
		t.Fatalf("save task2: %v", err)
	}
	task3 := task.New("TASK-003", "Task Three")
	if err := task3.Save(); err != nil {
		t.Fatalf("save task3: %v", err)
	}

	// Run link command
	cmd := newInitiativeLinkCmd()
	cmd.SetArgs([]string{"INIT-001", "TASK-001", "TASK-002", "TASK-003"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("link command failed: %v", err)
	}

	// Verify tasks are linked
	reloadedTask1, _ := task.Load("TASK-001")
	if reloadedTask1.InitiativeID != "INIT-001" {
		t.Errorf("task1 InitiativeID = %q, want %q", reloadedTask1.InitiativeID, "INIT-001")
	}

	reloadedTask2, _ := task.Load("TASK-002")
	if reloadedTask2.InitiativeID != "INIT-001" {
		t.Errorf("task2 InitiativeID = %q, want %q", reloadedTask2.InitiativeID, "INIT-001")
	}

	reloadedTask3, _ := task.Load("TASK-003")
	if reloadedTask3.InitiativeID != "INIT-001" {
		t.Errorf("task3 InitiativeID = %q, want %q", reloadedTask3.InitiativeID, "INIT-001")
	}

	// Verify initiative has all tasks
	reloadedInit, _ := initiative.Load("INIT-001")
	if len(reloadedInit.Tasks) != 3 {
		t.Errorf("initiative tasks count = %d, want 3", len(reloadedInit.Tasks))
	}
}

func TestInitiativeLinkWithPattern(t *testing.T) {
	withInitiativeTestDir(t)

	// Create initiative
	init := initiative.New("INIT-001", "Test Initiative")
	if err := init.Save(); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create tasks with different titles
	authTask1 := task.New("TASK-001", "User Authentication")
	if err := authTask1.Save(); err != nil {
		t.Fatalf("save authTask1: %v", err)
	}
	authTask2 := task.New("TASK-002", "Auth Token Validation")
	if err := authTask2.Save(); err != nil {
		t.Fatalf("save authTask2: %v", err)
	}
	otherTask := task.New("TASK-003", "Database Migration")
	if err := otherTask.Save(); err != nil {
		t.Fatalf("save otherTask: %v", err)
	}

	// Run link command with --all-matching
	cmd := newInitiativeLinkCmd()
	cmd.SetArgs([]string{"INIT-001", "--all-matching", "auth"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("link command failed: %v", err)
	}

	// Verify only auth tasks are linked
	reloadedTask1, _ := task.Load("TASK-001")
	if reloadedTask1.InitiativeID != "INIT-001" {
		t.Errorf("auth task1 should be linked, got InitiativeID = %q", reloadedTask1.InitiativeID)
	}

	reloadedTask2, _ := task.Load("TASK-002")
	if reloadedTask2.InitiativeID != "INIT-001" {
		t.Errorf("auth task2 should be linked, got InitiativeID = %q", reloadedTask2.InitiativeID)
	}

	reloadedTask3, _ := task.Load("TASK-003")
	if reloadedTask3.InitiativeID != "" {
		t.Errorf("other task should not be linked, got InitiativeID = %q", reloadedTask3.InitiativeID)
	}

	// Verify initiative has only 2 tasks
	reloadedInit, _ := initiative.Load("INIT-001")
	if len(reloadedInit.Tasks) != 2 {
		t.Errorf("initiative tasks count = %d, want 2", len(reloadedInit.Tasks))
	}
}

func TestInitiativeUnlinkMultipleTasks(t *testing.T) {
	withInitiativeTestDir(t)

	// Create initiative with tasks
	init := initiative.New("INIT-001", "Test Initiative")

	// Create and link tasks
	task1 := task.New("TASK-001", "Task One")
	task1.SetInitiative("INIT-001")
	if err := task1.Save(); err != nil {
		t.Fatalf("save task1: %v", err)
	}
	init.AddTask("TASK-001", "Task One", nil)

	task2 := task.New("TASK-002", "Task Two")
	task2.SetInitiative("INIT-001")
	if err := task2.Save(); err != nil {
		t.Fatalf("save task2: %v", err)
	}
	init.AddTask("TASK-002", "Task Two", nil)

	task3 := task.New("TASK-003", "Task Three")
	task3.SetInitiative("INIT-001")
	if err := task3.Save(); err != nil {
		t.Fatalf("save task3: %v", err)
	}
	init.AddTask("TASK-003", "Task Three", nil)

	if err := init.Save(); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Run unlink command for 2 tasks
	cmd := newInitiativeUnlinkCmd()
	cmd.SetArgs([]string{"INIT-001", "TASK-001", "TASK-002"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unlink command failed: %v", err)
	}

	// Verify tasks 1 and 2 are unlinked
	reloadedTask1, _ := task.Load("TASK-001")
	if reloadedTask1.InitiativeID != "" {
		t.Errorf("task1 should be unlinked, got InitiativeID = %q", reloadedTask1.InitiativeID)
	}

	reloadedTask2, _ := task.Load("TASK-002")
	if reloadedTask2.InitiativeID != "" {
		t.Errorf("task2 should be unlinked, got InitiativeID = %q", reloadedTask2.InitiativeID)
	}

	// Task 3 should still be linked
	reloadedTask3, _ := task.Load("TASK-003")
	if reloadedTask3.InitiativeID != "INIT-001" {
		t.Errorf("task3 should still be linked, got InitiativeID = %q", reloadedTask3.InitiativeID)
	}

	// Verify initiative has only 1 task
	reloadedInit, _ := initiative.Load("INIT-001")
	if len(reloadedInit.Tasks) != 1 {
		t.Errorf("initiative tasks count = %d, want 1", len(reloadedInit.Tasks))
	}
}

func TestInitiativeUnlinkAll(t *testing.T) {
	withInitiativeTestDir(t)

	// Create initiative with tasks
	init := initiative.New("INIT-001", "Test Initiative")

	// Create and link tasks
	task1 := task.New("TASK-001", "Task One")
	task1.SetInitiative("INIT-001")
	if err := task1.Save(); err != nil {
		t.Fatalf("save task1: %v", err)
	}
	init.AddTask("TASK-001", "Task One", nil)

	task2 := task.New("TASK-002", "Task Two")
	task2.SetInitiative("INIT-001")
	if err := task2.Save(); err != nil {
		t.Fatalf("save task2: %v", err)
	}
	init.AddTask("TASK-002", "Task Two", nil)

	if err := init.Save(); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Run unlink command with --all
	cmd := newInitiativeUnlinkCmd()
	cmd.SetArgs([]string{"INIT-001", "--all"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unlink command failed: %v", err)
	}

	// Verify all tasks are unlinked
	reloadedTask1, _ := task.Load("TASK-001")
	if reloadedTask1.InitiativeID != "" {
		t.Errorf("task1 should be unlinked, got InitiativeID = %q", reloadedTask1.InitiativeID)
	}

	reloadedTask2, _ := task.Load("TASK-002")
	if reloadedTask2.InitiativeID != "" {
		t.Errorf("task2 should be unlinked, got InitiativeID = %q", reloadedTask2.InitiativeID)
	}

	// Verify initiative has no tasks
	reloadedInit, _ := initiative.Load("INIT-001")
	if len(reloadedInit.Tasks) != 0 {
		t.Errorf("initiative tasks count = %d, want 0", len(reloadedInit.Tasks))
	}
}

func TestInitiativeLinkSkipsAlreadyLinkedToOther(t *testing.T) {
	withInitiativeTestDir(t)

	// Create two initiatives
	init1 := initiative.New("INIT-001", "First Initiative")
	if err := init1.Save(); err != nil {
		t.Fatalf("save init1: %v", err)
	}

	initDir2 := filepath.Join(".orc", "initiatives", "INIT-002")
	if err := os.MkdirAll(initDir2, 0755); err != nil {
		t.Fatalf("create initiative2 directory: %v", err)
	}
	init2 := initiative.New("INIT-002", "Second Initiative")
	if err := init2.Save(); err != nil {
		t.Fatalf("save init2: %v", err)
	}

	// Create task linked to INIT-002
	task1 := task.New("TASK-001", "Task One")
	task1.SetInitiative("INIT-002")
	if err := task1.Save(); err != nil {
		t.Fatalf("save task1: %v", err)
	}

	// Create unlinked task
	task2 := task.New("TASK-002", "Task Two")
	if err := task2.Save(); err != nil {
		t.Fatalf("save task2: %v", err)
	}

	// Run link command to link both tasks to INIT-001
	cmd := newInitiativeLinkCmd()
	cmd.SetArgs([]string{"INIT-001", "TASK-001", "TASK-002"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("link command failed: %v", err)
	}

	// TASK-001 should still be linked to INIT-002 (skipped)
	reloadedTask1, _ := task.Load("TASK-001")
	if reloadedTask1.InitiativeID != "INIT-002" {
		t.Errorf("task1 should still be linked to INIT-002, got %q", reloadedTask1.InitiativeID)
	}

	// TASK-002 should now be linked to INIT-001
	reloadedTask2, _ := task.Load("TASK-002")
	if reloadedTask2.InitiativeID != "INIT-001" {
		t.Errorf("task2 should be linked to INIT-001, got %q", reloadedTask2.InitiativeID)
	}
}
