package cli

// NOTE: Tests in this file use os.Chdir() which is process-wide and not goroutine-safe.
// These tests MUST NOT use t.Parallel() and run sequentially within this package.

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/randalmurphal/orc/internal/initiative"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// withListTestDir creates a temp directory with task structure, changes to it,
// and restores the original working directory when the test completes.
func withListTestDir(t *testing.T) string {
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

// createListTestBackend creates a backend in the given directory.
func createListTestBackend(t *testing.T, dir string) storage.Backend {
	t.Helper()
	backend, err := storage.NewDatabaseBackend(dir, nil)
	if err != nil {
		t.Fatalf("create backend: %v", err)
	}
	return backend
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
	if cmd.Flag("initiative") == nil {
		t.Error("missing --initiative flag")
	}
	if cmd.Flag("status") == nil {
		t.Error("missing --status flag")
	}
	if cmd.Flag("weight") == nil {
		t.Error("missing --weight flag")
	}

	// Verify shorthand flags
	if cmd.Flag("initiative").Shorthand != "i" {
		t.Errorf("initiative shorthand = %q, want 'i'", cmd.Flag("initiative").Shorthand)
	}
	if cmd.Flag("status").Shorthand != "s" {
		t.Errorf("status shorthand = %q, want 's'", cmd.Flag("status").Shorthand)
	}
	if cmd.Flag("weight").Shorthand != "w" {
		t.Errorf("weight shorthand = %q, want 'w'", cmd.Flag("weight").Shorthand)
	}
}

func TestListCommand_InitiativeFilter(t *testing.T) {
	tmpDir := withListTestDir(t)

	// Create backend and save test data
	backend := createListTestBackend(t, tmpDir)

	// Create an initiative
	init := initiative.New("INIT-001", "Test Initiative")
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create tasks with different initiative assignments
	t1 := task.New("TASK-001", "Task in initiative")
	t1.InitiativeID = "INIT-001"
	if err := backend.SaveTask(t1); err != nil {
		t.Fatalf("save task 1: %v", err)
	}

	t2 := task.New("TASK-002", "Task without initiative")
	if err := backend.SaveTask(t2); err != nil {
		t.Fatalf("save task 2: %v", err)
	}

	t3 := task.New("TASK-003", "Another task in initiative")
	t3.InitiativeID = "INIT-001"
	if err := backend.SaveTask(t3); err != nil {
		t.Fatalf("save task 3: %v", err)
	}

	// Close backend before running command
	_ = backend.Close()

	// Test: Filter by initiative ID
	cmd := newListCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--initiative", "INIT-001"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute command: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "TASK-001") {
		t.Error("output should contain TASK-001")
	}
	if strings.Contains(output, "TASK-002") {
		t.Error("output should NOT contain TASK-002 (not in initiative)")
	}
	if !strings.Contains(output, "TASK-003") {
		t.Error("output should contain TASK-003")
	}
}

func TestListCommand_UnassignedFilter(t *testing.T) {
	tmpDir := withListTestDir(t)

	// Create backend and save test data
	backend := createListTestBackend(t, tmpDir)

	// Create an initiative
	init := initiative.New("INIT-001", "Test Initiative")
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create tasks with different initiative assignments
	t1 := task.New("TASK-001", "Task in initiative")
	t1.InitiativeID = "INIT-001"
	if err := backend.SaveTask(t1); err != nil {
		t.Fatalf("save task 1: %v", err)
	}

	t2 := task.New("TASK-002", "Task without initiative")
	if err := backend.SaveTask(t2); err != nil {
		t.Fatalf("save task 2: %v", err)
	}

	// Close backend before running command
	_ = backend.Close()

	// Test: Filter by "unassigned"
	cmd := newListCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--initiative", "unassigned"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute command: %v", err)
	}

	output := out.String()
	if strings.Contains(output, "TASK-001") {
		t.Error("output should NOT contain TASK-001 (has initiative)")
	}
	if !strings.Contains(output, "TASK-002") {
		t.Error("output should contain TASK-002 (no initiative)")
	}
}

func TestListCommand_EmptyInitiativeFilter(t *testing.T) {
	tmpDir := withListTestDir(t)

	// Create backend and save test data
	backend := createListTestBackend(t, tmpDir)

	// Create tasks
	t1 := task.New("TASK-001", "Task with initiative")
	t1.InitiativeID = "INIT-001"
	if err := backend.SaveTask(t1); err != nil {
		t.Fatalf("save task 1: %v", err)
	}

	t2 := task.New("TASK-002", "Task without initiative")
	if err := backend.SaveTask(t2); err != nil {
		t.Fatalf("save task 2: %v", err)
	}

	// Close backend before running command
	_ = backend.Close()

	// Test: Filter by empty string (same as unassigned)
	cmd := newListCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--initiative", ""})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute command: %v", err)
	}

	output := out.String()
	if strings.Contains(output, "TASK-001") {
		t.Error("output should NOT contain TASK-001 (has initiative)")
	}
	if !strings.Contains(output, "TASK-002") {
		t.Error("output should contain TASK-002 (no initiative)")
	}
}

func TestListCommand_InvalidInitiative(t *testing.T) {
	tmpDir := withListTestDir(t)

	// Create backend and save test data
	backend := createListTestBackend(t, tmpDir)

	// Create a task
	t1 := task.New("TASK-001", "Test task")
	if err := backend.SaveTask(t1); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Close backend before running command
	_ = backend.Close()

	// Test: Filter by non-existent initiative
	cmd := newListCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--initiative", "INIT-NONEXISTENT"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for non-existent initiative")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention 'not found', got: %v", err)
	}
}

func TestListCommand_CombinedFilters(t *testing.T) {
	tmpDir := withListTestDir(t)

	// Create backend and save test data
	backend := createListTestBackend(t, tmpDir)

	// Create an initiative
	init := initiative.New("INIT-001", "Test Initiative")
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create tasks with different properties
	t1 := task.New("TASK-001", "Running task in initiative")
	t1.InitiativeID = "INIT-001"
	t1.Status = task.StatusRunning
	t1.Weight = task.WeightSmall
	if err := backend.SaveTask(t1); err != nil {
		t.Fatalf("save task 1: %v", err)
	}

	t2 := task.New("TASK-002", "Completed task in initiative")
	t2.InitiativeID = "INIT-001"
	t2.Status = task.StatusCompleted
	t2.Weight = task.WeightSmall
	if err := backend.SaveTask(t2); err != nil {
		t.Fatalf("save task 2: %v", err)
	}

	t3 := task.New("TASK-003", "Running task without initiative")
	t3.Status = task.StatusRunning
	t3.Weight = task.WeightSmall
	if err := backend.SaveTask(t3); err != nil {
		t.Fatalf("save task 3: %v", err)
	}

	// Close backend before running command
	_ = backend.Close()

	// Test: Filter by initiative AND status
	cmd := newListCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--initiative", "INIT-001", "--status", "running"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute command: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "TASK-001") {
		t.Error("output should contain TASK-001 (in initiative and running)")
	}
	if strings.Contains(output, "TASK-002") {
		t.Error("output should NOT contain TASK-002 (in initiative but completed)")
	}
	if strings.Contains(output, "TASK-003") {
		t.Error("output should NOT contain TASK-003 (running but not in initiative)")
	}
}

func TestListCommand_NoMatchingTasks(t *testing.T) {
	tmpDir := withListTestDir(t)

	// Create backend and save test data
	backend := createListTestBackend(t, tmpDir)

	// Create an initiative
	init := initiative.New("INIT-001", "Test Initiative")
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create a task NOT in the initiative
	t1 := task.New("TASK-001", "Task without initiative")
	if err := backend.SaveTask(t1); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Close backend before running command
	_ = backend.Close()

	// Test: Filter by initiative with no matching tasks
	cmd := newListCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--initiative", "INIT-001"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute command: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "No tasks found matching") {
		t.Error("output should mention 'No tasks found matching'")
	}
	if !strings.Contains(output, "initiative INIT-001") {
		t.Error("output should mention the initiative filter")
	}
}

func TestCompleteInitiativeIDs(t *testing.T) {
	tmpDir := withListTestDir(t)

	// Create backend and save test data
	backend := createListTestBackend(t, tmpDir)

	// Create some initiatives
	init1 := initiative.New("INIT-001", "First Initiative")
	if err := backend.SaveInitiative(init1); err != nil {
		t.Fatalf("save initiative 1: %v", err)
	}

	init2 := initiative.New("INIT-002", "Second Initiative")
	if err := backend.SaveInitiative(init2); err != nil {
		t.Fatalf("save initiative 2: %v", err)
	}

	// Close backend before running completion
	_ = backend.Close()

	// Test completion function
	cmd := newListCmd()
	completions, directive := completeInitiativeIDs(cmd, []string{}, "")

	// Should have at least "unassigned" and our two initiatives
	if len(completions) < 3 {
		t.Errorf("expected at least 3 completions, got %d", len(completions))
	}

	// Check directive
	if directive != 0x4 { // ShellCompDirectiveNoFileComp
		t.Errorf("directive = %v, want NoFileComp", directive)
	}

	// Should contain "unassigned"
	found := false
	for _, c := range completions {
		if strings.HasPrefix(c, "unassigned") {
			found = true
			break
		}
	}
	if !found {
		t.Error("completions should include 'unassigned'")
	}

	// Should contain INIT-001 and INIT-002
	foundInit1 := false
	foundInit2 := false
	for _, c := range completions {
		if strings.HasPrefix(c, "INIT-001") {
			foundInit1 = true
		}
		if strings.HasPrefix(c, "INIT-002") {
			foundInit2 = true
		}
	}
	if !foundInit1 {
		t.Error("completions should include INIT-001")
	}
	if !foundInit2 {
		t.Error("completions should include INIT-002")
	}
}

func TestCompleteInitiativeIDs_Filtering(t *testing.T) {
	tmpDir := withListTestDir(t)

	// Create backend and save test data
	backend := createListTestBackend(t, tmpDir)

	// Create initiatives with different prefixes
	init1 := initiative.New("INIT-001", "First Initiative")
	if err := backend.SaveInitiative(init1); err != nil {
		t.Fatalf("save initiative 1: %v", err)
	}

	init2 := initiative.New("INIT-002", "Second Initiative")
	if err := backend.SaveInitiative(init2); err != nil {
		t.Fatalf("save initiative 2: %v", err)
	}

	// Close backend before running completion
	_ = backend.Close()

	// Test completion with prefix filter
	cmd := newListCmd()
	completions, _ := completeInitiativeIDs(cmd, []string{}, "INIT-001")

	// Should contain "unassigned" and INIT-001, but not INIT-002
	foundInit1 := false
	foundInit2 := false
	for _, c := range completions {
		if strings.HasPrefix(c, "INIT-001") {
			foundInit1 = true
		}
		if strings.HasPrefix(c, "INIT-002") {
			foundInit2 = true
		}
	}
	if !foundInit1 {
		t.Error("completions should include INIT-001")
	}
	if foundInit2 {
		t.Error("completions should NOT include INIT-002 when filtering by INIT-001")
	}
}
