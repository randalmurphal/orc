package cli

// NOTE: Tests in this file use ORC_PROJECT_ROOT environment variable for test isolation.
// This avoids os.Chdir() which is process-wide and not goroutine-safe.

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/initiative"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// withListTestDir creates a temp directory with task structure and sets
// ORC_PROJECT_ROOT to point to it. This avoids os.Chdir() which causes
// race conditions when tests run in parallel across packages.
func withListTestDir(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()

	// Create .orc directory with config.yaml for project detection
	orcDir := filepath.Join(tmpDir, ".orc")
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatalf("create .orc directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(orcDir, "config.yaml"), []byte("version: 1\n"), 0644); err != nil {
		t.Fatalf("create config.yaml: %v", err)
	}

	// Set ORC_PROJECT_ROOT instead of using os.Chdir()
	// This is respected by config.RequireInit() and config.FindProjectRoot()
	origRoot := os.Getenv("ORC_PROJECT_ROOT")
	if err := os.Setenv("ORC_PROJECT_ROOT", tmpDir); err != nil {
		t.Fatalf("set ORC_PROJECT_ROOT: %v", err)
	}
	t.Cleanup(func() {
		if origRoot == "" {
			_ = os.Unsetenv("ORC_PROJECT_ROOT")
		} else {
			_ = os.Setenv("ORC_PROJECT_ROOT", origRoot)
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
	if cmd.Flag("limit") == nil {
		t.Error("missing --limit flag")
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
	if cmd.Flag("limit").Shorthand != "n" {
		t.Errorf("limit shorthand = %q, want 'n'", cmd.Flag("limit").Shorthand)
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
	t1 := task.NewProtoTask("TASK-001", "Task in initiative")
	task.SetInitiativeProto(t1, "INIT-001")
	if err := backend.SaveTask(t1); err != nil {
		t.Fatalf("save task 1: %v", err)
	}

	t2 := task.NewProtoTask("TASK-002", "Task without initiative")
	if err := backend.SaveTask(t2); err != nil {
		t.Fatalf("save task 2: %v", err)
	}

	t3 := task.NewProtoTask("TASK-003", "Another task in initiative")
	task.SetInitiativeProto(t3, "INIT-001")
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
	t1 := task.NewProtoTask("TASK-001", "Task in initiative")
	task.SetInitiativeProto(t1, "INIT-001")
	if err := backend.SaveTask(t1); err != nil {
		t.Fatalf("save task 1: %v", err)
	}

	t2 := task.NewProtoTask("TASK-002", "Task without initiative")
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
	t1 := task.NewProtoTask("TASK-001", "Task with initiative")
	task.SetInitiativeProto(t1, "INIT-001")
	if err := backend.SaveTask(t1); err != nil {
		t.Fatalf("save task 1: %v", err)
	}

	t2 := task.NewProtoTask("TASK-002", "Task without initiative")
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
	t1 := task.NewProtoTask("TASK-001", "Test task")
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
	t1 := task.NewProtoTask("TASK-001", "Running task in initiative")
	task.SetInitiativeProto(t1, "INIT-001")
	t1.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	t1.Weight = orcv1.TaskWeight_TASK_WEIGHT_SMALL
	if err := backend.SaveTask(t1); err != nil {
		t.Fatalf("save task 1: %v", err)
	}

	t2 := task.NewProtoTask("TASK-002", "Completed task in initiative")
	task.SetInitiativeProto(t2, "INIT-001")
	t2.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	t2.Weight = orcv1.TaskWeight_TASK_WEIGHT_SMALL
	if err := backend.SaveTask(t2); err != nil {
		t.Fatalf("save task 2: %v", err)
	}

	t3 := task.NewProtoTask("TASK-003", "Running task without initiative")
	t3.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	t3.Weight = orcv1.TaskWeight_TASK_WEIGHT_SMALL
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
	t1 := task.NewProtoTask("TASK-001", "Task without initiative")
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

func TestListCommand_LimitFlag(t *testing.T) {
	tmpDir := withListTestDir(t)

	// Create backend and save test data
	backend := createListTestBackend(t, tmpDir)

	// Create 5 tasks
	for i := 1; i <= 5; i++ {
		tk := task.NewProtoTask(fmt.Sprintf("TASK-%03d", i), fmt.Sprintf("Task %d", i))
		if err := backend.SaveTask(tk); err != nil {
			t.Fatalf("save task %d: %v", i, err)
		}
	}

	// Close backend before running command
	_ = backend.Close()

	// Test: Limit to 3 tasks
	cmd := newListCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--limit", "3"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute command: %v", err)
	}

	output := out.String()

	// Should show the last 3 tasks (most recent: TASK-003, TASK-004, TASK-005)
	if strings.Contains(output, "TASK-001") {
		t.Error("output should NOT contain TASK-001 (excluded by limit)")
	}
	if strings.Contains(output, "TASK-002") {
		t.Error("output should NOT contain TASK-002 (excluded by limit)")
	}
	if !strings.Contains(output, "TASK-003") {
		t.Error("output should contain TASK-003")
	}
	if !strings.Contains(output, "TASK-004") {
		t.Error("output should contain TASK-004")
	}
	if !strings.Contains(output, "TASK-005") {
		t.Error("output should contain TASK-005")
	}
}

func TestListCommand_LimitZero(t *testing.T) {
	tmpDir := withListTestDir(t)

	// Create backend and save test data
	backend := createListTestBackend(t, tmpDir)

	// Create 3 tasks
	for i := 1; i <= 3; i++ {
		tk := task.NewProtoTask(fmt.Sprintf("TASK-%03d", i), fmt.Sprintf("Task %d", i))
		if err := backend.SaveTask(tk); err != nil {
			t.Fatalf("save task %d: %v", i, err)
		}
	}

	// Close backend before running command
	_ = backend.Close()

	// Test: Limit 0 should show all tasks
	cmd := newListCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--limit", "0"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute command: %v", err)
	}

	output := out.String()

	// Should show all 3 tasks
	if !strings.Contains(output, "TASK-001") {
		t.Error("output should contain TASK-001")
	}
	if !strings.Contains(output, "TASK-002") {
		t.Error("output should contain TASK-002")
	}
	if !strings.Contains(output, "TASK-003") {
		t.Error("output should contain TASK-003")
	}
}

func TestListCommand_LimitWithFilters(t *testing.T) {
	tmpDir := withListTestDir(t)

	// Create backend and save test data
	backend := createListTestBackend(t, tmpDir)

	// Create tasks with different statuses
	for i := 1; i <= 5; i++ {
		tk := task.NewProtoTask(fmt.Sprintf("TASK-%03d", i), fmt.Sprintf("Task %d", i))
		if i <= 3 {
			tk.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
		} else {
			tk.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
		}
		if err := backend.SaveTask(tk); err != nil {
			t.Fatalf("save task %d: %v", i, err)
		}
	}

	// Close backend before running command
	_ = backend.Close()

	// Test: Filter by created status AND limit to 2
	cmd := newListCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--status", "created", "--limit", "2"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute command: %v", err)
	}

	output := out.String()

	// Should show the last 2 created tasks (TASK-002, TASK-003)
	if strings.Contains(output, "TASK-001") {
		t.Error("output should NOT contain TASK-001 (excluded by limit)")
	}
	if !strings.Contains(output, "TASK-002") {
		t.Error("output should contain TASK-002")
	}
	if !strings.Contains(output, "TASK-003") {
		t.Error("output should contain TASK-003")
	}
	if strings.Contains(output, "TASK-004") {
		t.Error("output should NOT contain TASK-004 (completed)")
	}
	if strings.Contains(output, "TASK-005") {
		t.Error("output should NOT contain TASK-005 (completed)")
	}
}

func TestListCommand_LimitExceedsTotal(t *testing.T) {
	tmpDir := withListTestDir(t)

	// Create backend and save test data
	backend := createListTestBackend(t, tmpDir)

	// Create only 3 tasks
	for i := 1; i <= 3; i++ {
		tk := task.NewProtoTask(fmt.Sprintf("TASK-%03d", i), fmt.Sprintf("Task %d", i))
		if err := backend.SaveTask(tk); err != nil {
			t.Fatalf("save task %d: %v", i, err)
		}
	}

	// Close backend before running command
	_ = backend.Close()

	// Test: Limit 100 (exceeds total of 3)
	cmd := newListCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--limit", "100"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute command: %v", err)
	}

	output := out.String()

	// Should show all 3 tasks
	if !strings.Contains(output, "TASK-001") {
		t.Error("output should contain TASK-001")
	}
	if !strings.Contains(output, "TASK-002") {
		t.Error("output should contain TASK-002")
	}
	if !strings.Contains(output, "TASK-003") {
		t.Error("output should contain TASK-003")
	}
}

func TestListCommand_LimitShorthand(t *testing.T) {
	tmpDir := withListTestDir(t)

	// Create backend and save test data
	backend := createListTestBackend(t, tmpDir)

	// Create 5 tasks
	for i := 1; i <= 5; i++ {
		tk := task.NewProtoTask(fmt.Sprintf("TASK-%03d", i), fmt.Sprintf("Task %d", i))
		if err := backend.SaveTask(tk); err != nil {
			t.Fatalf("save task %d: %v", i, err)
		}
	}

	// Close backend before running command
	_ = backend.Close()

	// Test: Use shorthand -n
	cmd := newListCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"-n", "2"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute command: %v", err)
	}

	output := out.String()

	// Should show the last 2 tasks (TASK-004, TASK-005)
	if strings.Contains(output, "TASK-001") {
		t.Error("output should NOT contain TASK-001")
	}
	if strings.Contains(output, "TASK-002") {
		t.Error("output should NOT contain TASK-002")
	}
	if strings.Contains(output, "TASK-003") {
		t.Error("output should NOT contain TASK-003")
	}
	if !strings.Contains(output, "TASK-004") {
		t.Error("output should contain TASK-004")
	}
	if !strings.Contains(output, "TASK-005") {
		t.Error("output should contain TASK-005")
	}
}
