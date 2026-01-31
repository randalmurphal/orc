package cli

// NOTE: Tests in this file use os.Chdir() which is process-wide and not goroutine-safe.
// These tests MUST NOT use t.Parallel() and run sequentially within this package.

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/initiative"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// withStatusTestDir creates a temp directory with task structure, changes to it,
// and restores the original working directory when the test completes.
func withStatusTestDir(t *testing.T) string {
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

// createStatusTestBackend creates a backend in the given directory.
func createStatusTestBackend(t *testing.T, dir string) storage.Backend {
	t.Helper()
	backend, err := storage.NewDatabaseBackend(dir, nil)
	if err != nil {
		t.Fatalf("create backend: %v", err)
	}
	return backend
}

// TestStatusCommand_Flags verifies the initiative flag exists with correct properties
func TestStatusCommand_Flags(t *testing.T) {
	cmd := newStatusCmd()

	// Verify command structure
	if cmd.Use != "status" {
		t.Errorf("command Use = %q, want %q", cmd.Use, "status")
	}

	// Verify initiative flag exists
	if cmd.Flag("initiative") == nil {
		t.Error("missing --initiative flag")
	}

	// Verify shorthand flag
	if cmd.Flag("initiative").Shorthand != "i" {
		t.Errorf("initiative shorthand = %q, want 'i'", cmd.Flag("initiative").Shorthand)
	}
}

// TestStatusCommand_InitiativeFilter tests SC-1: Filter tasks by initiative ID,
// preserving priority categories
func TestStatusCommand_InitiativeFilter(t *testing.T) {
	tmpDir := withStatusTestDir(t)

	// Create backend and save test data
	backend := createStatusTestBackend(t, tmpDir)

	// Create initiatives
	init1 := initiative.New("INIT-001", "First Initiative")
	if err := backend.SaveInitiative(init1); err != nil {
		t.Fatalf("save initiative 1: %v", err)
	}

	init2 := initiative.New("INIT-002", "Second Initiative")
	if err := backend.SaveInitiative(init2); err != nil {
		t.Fatalf("save initiative 2: %v", err)
	}

	// Create tasks with different statuses and initiatives
	// INIT-001 tasks
	t1 := task.NewProtoTask("TASK-001", "Running in INIT-001")
	task.SetInitiativeProto(t1, "INIT-001")
	t1.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	t1.Priority = orcv1.TaskPriority_TASK_PRIORITY_HIGH
	t1.ExecutorPid = int32(os.Getpid()) // Set PID so task appears running, not orphaned
	if err := backend.SaveTask(t1); err != nil {
		t.Fatalf("save task 1: %v", err)
	}

	t2 := task.NewProtoTask("TASK-002", "Ready in INIT-001")
	task.SetInitiativeProto(t2, "INIT-001")
	t2.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	t2.Priority = orcv1.TaskPriority_TASK_PRIORITY_NORMAL
	if err := backend.SaveTask(t2); err != nil {
		t.Fatalf("save task 2: %v", err)
	}

	// INIT-002 task (should not appear)
	t3 := task.NewProtoTask("TASK-003", "Ready in INIT-002")
	task.SetInitiativeProto(t3, "INIT-002")
	t3.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	t3.Priority = orcv1.TaskPriority_TASK_PRIORITY_HIGH
	if err := backend.SaveTask(t3); err != nil {
		t.Fatalf("save task 3: %v", err)
	}

	// Close backend before running command
	_ = backend.Close()

	// Test: Filter by INIT-001
	cmd := newStatusCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--initiative", "INIT-001"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute command: %v", err)
	}

	output := out.String()

	// Should contain INIT-001 tasks
	if !strings.Contains(output, "TASK-001") {
		t.Error("output should contain TASK-001 (INIT-001, running)")
	}
	if !strings.Contains(output, "TASK-002") {
		t.Error("output should contain TASK-002 (INIT-001, ready)")
	}

	// Should NOT contain INIT-002 tasks
	if strings.Contains(output, "TASK-003") {
		t.Error("output should NOT contain TASK-003 (INIT-002)")
	}

	// Verify priority categories are preserved
	if !strings.Contains(output, "RUNNING") {
		t.Error("output should contain RUNNING category")
	}
	if !strings.Contains(output, "READY") {
		t.Error("output should contain READY category")
	}
}

// TestStatusCommand_InitiativeShorthand tests SC-2: Shorthand -i flag works
func TestStatusCommand_InitiativeShorthand(t *testing.T) {
	tmpDir := withStatusTestDir(t)

	// Create backend and save test data
	backend := createStatusTestBackend(t, tmpDir)

	// Create initiative
	init := initiative.New("INIT-001", "Test Initiative")
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create task in initiative
	t1 := task.NewProtoTask("TASK-001", "Task in initiative")
	task.SetInitiativeProto(t1, "INIT-001")
	if err := backend.SaveTask(t1); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Create task outside initiative
	t2 := task.NewProtoTask("TASK-002", "Task without initiative")
	if err := backend.SaveTask(t2); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Close backend before running command
	_ = backend.Close()

	// Test: Use shorthand -i
	cmd := newStatusCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"-i", "INIT-001"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute command: %v", err)
	}

	output := out.String()

	// Should work identically to --initiative
	if !strings.Contains(output, "TASK-001") {
		t.Error("output should contain TASK-001")
	}
	if strings.Contains(output, "TASK-002") {
		t.Error("output should NOT contain TASK-002")
	}
}

// TestStatusCommand_EmptyInitiativeFilter tests SC-3: Helpful message when
// initiative exists but has no tasks
func TestStatusCommand_EmptyInitiativeFilter(t *testing.T) {
	tmpDir := withStatusTestDir(t)

	// Create backend and save test data
	backend := createStatusTestBackend(t, tmpDir)

	// Create initiative with no tasks
	init := initiative.New("INIT-001", "Empty Initiative")
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create task NOT in the initiative
	t1 := task.NewProtoTask("TASK-001", "Task without initiative")
	if err := backend.SaveTask(t1); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Close backend before running command
	_ = backend.Close()

	// Test: Filter by initiative with no tasks
	cmd := newStatusCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--initiative", "INIT-001"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute command: %v", err)
	}

	output := out.String()

	// Should show helpful message mentioning the filter
	if !strings.Contains(output, "No tasks found") {
		t.Error("output should mention 'No tasks found'")
	}
	if !strings.Contains(output, "INIT-001") {
		t.Error("output should mention initiative ID in empty message")
	}
}

// TestStatusCommand_UnassignedFilter tests SC-4: Filter tasks without initiative
func TestStatusCommand_UnassignedFilter(t *testing.T) {
	tmpDir := withStatusTestDir(t)

	// Create backend and save test data
	backend := createStatusTestBackend(t, tmpDir)

	// Create initiative
	init := initiative.New("INIT-001", "Test Initiative")
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create task with initiative
	t1 := task.NewProtoTask("TASK-001", "Task in initiative")
	task.SetInitiativeProto(t1, "INIT-001")
	t1.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	if err := backend.SaveTask(t1); err != nil {
		t.Fatalf("save task 1: %v", err)
	}

	// Create task without initiative
	t2 := task.NewProtoTask("TASK-002", "Task without initiative")
	t2.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := backend.SaveTask(t2); err != nil {
		t.Fatalf("save task 2: %v", err)
	}

	// Close backend before running command
	_ = backend.Close()

	// Test: Filter by "unassigned"
	cmd := newStatusCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--initiative", "unassigned"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute command: %v", err)
	}

	output := out.String()

	// Should NOT contain task with initiative
	if strings.Contains(output, "TASK-001") {
		t.Error("output should NOT contain TASK-001 (has initiative)")
	}

	// Should contain task without initiative
	if !strings.Contains(output, "TASK-002") {
		t.Error("output should contain TASK-002 (no initiative)")
	}
}

// TestStatusCommand_EmptyStringFilter tests SC-4: Empty string acts like "unassigned"
func TestStatusCommand_EmptyStringFilter(t *testing.T) {
	tmpDir := withStatusTestDir(t)

	// Create backend and save test data
	backend := createStatusTestBackend(t, tmpDir)

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

	// Test: Filter by empty string
	cmd := newStatusCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--initiative", ""})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute command: %v", err)
	}

	output := out.String()

	// Should work identically to "unassigned"
	if strings.Contains(output, "TASK-001") {
		t.Error("output should NOT contain TASK-001 (has initiative)")
	}
	if !strings.Contains(output, "TASK-002") {
		t.Error("output should contain TASK-002 (no initiative)")
	}
}

// TestStatusCommand_InitiativeWithAllFlag tests SC-6: Initiative filter works with --all
func TestStatusCommand_InitiativeWithAllFlag(t *testing.T) {
	tmpDir := withStatusTestDir(t)

	// Create backend and save test data
	backend := createStatusTestBackend(t, tmpDir)

	// Create initiative
	init := initiative.New("INIT-001", "Test Initiative")
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create tasks in INIT-001
	t1 := task.NewProtoTask("TASK-001", "Running in INIT-001")
	task.SetInitiativeProto(t1, "INIT-001")
	t1.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := backend.SaveTask(t1); err != nil {
		t.Fatalf("save task 1: %v", err)
	}

	t2 := task.NewProtoTask("TASK-002", "Completed in INIT-001")
	task.SetInitiativeProto(t2, "INIT-001")
	t2.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	t2.UpdatedAt = timestamppb.New(time.Now().Add(-48 * time.Hour)) // More than 24h ago
	if err := backend.SaveTask(t2); err != nil {
		t.Fatalf("save task 2: %v", err)
	}

	// Create task in INIT-002
	t3 := task.NewProtoTask("TASK-003", "Completed in INIT-002")
	task.SetInitiativeProto(t3, "INIT-002")
	t3.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	t3.UpdatedAt = timestamppb.New(time.Now().Add(-48 * time.Hour))
	if err := backend.SaveTask(t3); err != nil {
		t.Fatalf("save task 3: %v", err)
	}

	// Close backend before running command
	_ = backend.Close()

	// Test: Filter by initiative AND show all tasks
	cmd := newStatusCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--initiative", "INIT-001", "--all"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute command: %v", err)
	}

	output := out.String()

	// Should contain running task from INIT-001
	if !strings.Contains(output, "TASK-001") {
		t.Error("output should contain TASK-001 (running)")
	}

	// Should contain completed task from INIT-001 (--all)
	if !strings.Contains(output, "TASK-002") {
		t.Error("output should contain TASK-002 (completed in INIT-001)")
	}

	// Should NOT contain completed task from other initiative
	if strings.Contains(output, "TASK-003") {
		t.Error("output should NOT contain TASK-003 (different initiative)")
	}
}

// TestStatusCommand_NonexistentInitiative tests SC-7: Error for non-existent initiative
func TestStatusCommand_NonexistentInitiative(t *testing.T) {
	tmpDir := withStatusTestDir(t)

	// Create backend and save test data
	backend := createStatusTestBackend(t, tmpDir)

	// Create a task
	t1 := task.NewProtoTask("TASK-001", "Test task")
	if err := backend.SaveTask(t1); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Close backend before running command
	_ = backend.Close()

	// Test: Filter by non-existent initiative
	cmd := newStatusCmd()
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
	if !strings.Contains(err.Error(), "INIT-NONEXISTENT") {
		t.Errorf("error should mention initiative ID, got: %v", err)
	}
}

// TestStatusCommand_InitiativeSummaryLine tests SC-8: Summary reflects filtered counts
func TestStatusCommand_InitiativeSummaryLine(t *testing.T) {
	tmpDir := withStatusTestDir(t)

	// Create backend and save test data
	backend := createStatusTestBackend(t, tmpDir)

	// Create initiative
	init := initiative.New("INIT-001", "Test Initiative")
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create tasks in INIT-001
	t1 := task.NewProtoTask("TASK-001", "Running in INIT-001")
	task.SetInitiativeProto(t1, "INIT-001")
	t1.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	t1.ExecutorPid = int32(os.Getpid()) // Set PID so task appears running, not orphaned
	if err := backend.SaveTask(t1); err != nil {
		t.Fatalf("save task 1: %v", err)
	}

	t2 := task.NewProtoTask("TASK-002", "Ready in INIT-001")
	task.SetInitiativeProto(t2, "INIT-001")
	t2.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	if err := backend.SaveTask(t2); err != nil {
		t.Fatalf("save task 2: %v", err)
	}

	// Create tasks NOT in initiative (should not affect counts)
	t3 := task.NewProtoTask("TASK-003", "Running without initiative")
	t3.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	t3.ExecutorPid = int32(os.Getpid()) // Set PID so task appears running, not orphaned
	if err := backend.SaveTask(t3); err != nil {
		t.Fatalf("save task 3: %v", err)
	}

	t4 := task.NewProtoTask("TASK-004", "Completed without initiative")
	t4.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	if err := backend.SaveTask(t4); err != nil {
		t.Fatalf("save task 4: %v", err)
	}

	// Close backend before running command
	_ = backend.Close()

	// Test: Filter by initiative
	cmd := newStatusCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--initiative", "INIT-001"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute command: %v", err)
	}

	output := out.String()

	// Summary should show 2 tasks (filtered count)
	if !strings.Contains(output, "2 tasks") {
		t.Error("summary should show '2 tasks' (filtered count)")
	}

	// Summary should show 1 running
	if !strings.Contains(output, "1 running") {
		t.Error("summary should show '1 running'")
	}

	// Summary should show 1 ready
	if !strings.Contains(output, "1 ready") {
		t.Error("summary should show '1 ready'")
	}

	// Summary should NOT show totals from outside initiative
	if strings.Contains(output, "4 tasks") {
		t.Error("summary should NOT show total of all tasks")
	}
}

// TestStatusCommand_DependencyBlockedWithInitiative tests that dependency-blocked
// tasks are correctly categorized when filtered by initiative
func TestStatusCommand_DependencyBlockedWithInitiative(t *testing.T) {
	tmpDir := withStatusTestDir(t)

	// Create backend and save test data
	backend := createStatusTestBackend(t, tmpDir)

	// Create initiative
	init := initiative.New("INIT-001", "Test Initiative")
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create blocker task (not in initiative)
	t1 := task.NewProtoTask("TASK-001", "Blocker task")
	t1.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := backend.SaveTask(t1); err != nil {
		t.Fatalf("save task 1: %v", err)
	}

	// Create blocked task in initiative
	t2 := task.NewProtoTask("TASK-002", "Blocked task in INIT-001")
	task.SetInitiativeProto(t2, "INIT-001")
	t2.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	t2.BlockedBy = []string{"TASK-001"}
	if err := backend.SaveTask(t2); err != nil {
		t.Fatalf("save task 2: %v", err)
	}

	// Close backend before running command
	_ = backend.Close()

	// Test: Filter by initiative
	cmd := newStatusCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--initiative", "INIT-001"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute command: %v", err)
	}

	output := out.String()

	// Should show TASK-002 in BLOCKED category
	if !strings.Contains(output, "TASK-002") {
		t.Error("output should contain TASK-002")
	}
	if !strings.Contains(output, "BLOCKED") {
		t.Error("output should contain BLOCKED category")
	}

	// Should NOT show blocker task (not in initiative)
	if strings.Contains(output, "TASK-001") {
		t.Error("output should NOT contain TASK-001 (not in initiative)")
	}
}

// TestStatusCommand_MultipleCategories tests that tasks in different categories
// are all shown when filtering by initiative
func TestStatusCommand_MultipleCategories(t *testing.T) {
	tmpDir := withStatusTestDir(t)

	// Create backend and save test data
	backend := createStatusTestBackend(t, tmpDir)

	// Create initiative
	init := initiative.New("INIT-001", "Test Initiative")
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create tasks in various states, all in INIT-001
	t1 := task.NewProtoTask("TASK-001", "Running")
	task.SetInitiativeProto(t1, "INIT-001")
	t1.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	t1.ExecutorPid = int32(os.Getpid()) // Set PID so task appears running, not orphaned
	if err := backend.SaveTask(t1); err != nil {
		t.Fatalf("save task 1: %v", err)
	}

	t2 := task.NewProtoTask("TASK-002", "Ready")
	task.SetInitiativeProto(t2, "INIT-001")
	t2.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	if err := backend.SaveTask(t2); err != nil {
		t.Fatalf("save task 2: %v", err)
	}

	t3 := task.NewProtoTask("TASK-003", "Paused")
	task.SetInitiativeProto(t3, "INIT-001")
	t3.Status = orcv1.TaskStatus_TASK_STATUS_PAUSED
	if err := backend.SaveTask(t3); err != nil {
		t.Fatalf("save task 3: %v", err)
	}

	t4 := task.NewProtoTask("TASK-004", "Recent")
	task.SetInitiativeProto(t4, "INIT-001")
	t4.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	completedTime := time.Now().Add(-1 * time.Hour)
	t4.CompletedAt = timestamppb.New(completedTime)
	t4.UpdatedAt = timestamppb.New(time.Now().Add(-1 * time.Hour)) // Within 24h
	if err := backend.SaveTask(t4); err != nil {
		t.Fatalf("save task 4: %v", err)
	}

	// Close backend before running command
	_ = backend.Close()

	// Test: All categories should appear
	cmd := newStatusCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--initiative", "INIT-001"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute command: %v", err)
	}

	output := out.String()

	// DEBUG
	t.Logf("TASK-004 UpdatedAt: %v", t4.UpdatedAt)
	t.Logf("24h ago would be: %v", time.Now().Add(-24*time.Hour))
	t.Logf("Output:\n%s", output)

	// Verify all category headers appear
	if !strings.Contains(output, "RUNNING") {
		t.Error("output should contain RUNNING category")
	}
	if !strings.Contains(output, "READY") {
		t.Error("output should contain READY category")
	}
	if !strings.Contains(output, "PAUSED") {
		t.Error("output should contain PAUSED category")
	}
	if !strings.Contains(output, "RECENT") {
		t.Error("output should contain RECENT category")
	}

	// Verify all tasks appear
	if !strings.Contains(output, "TASK-001") {
		t.Error("output should contain TASK-001 (running)")
	}
	if !strings.Contains(output, "TASK-002") {
		t.Error("output should contain TASK-002 (ready)")
	}
	if !strings.Contains(output, "TASK-003") {
		t.Error("output should contain TASK-003 (paused)")
	}
	if !strings.Contains(output, "TASK-004") {
		t.Error("output should contain TASK-004 (recent)")
	}
}

// TestStatusCommand_CaseSensitiveInitiative tests that initiative IDs are
// case-sensitive
func TestStatusCommand_CaseSensitiveInitiative(t *testing.T) {
	tmpDir := withStatusTestDir(t)

	// Create backend and save test data
	backend := createStatusTestBackend(t, tmpDir)

	// Create initiative with uppercase ID
	init := initiative.New("INIT-001", "Test Initiative")
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create task in initiative
	t1 := task.NewProtoTask("TASK-001", "Task in initiative")
	task.SetInitiativeProto(t1, "INIT-001")
	if err := backend.SaveTask(t1); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Close backend before running command
	_ = backend.Close()

	// Test: Try with lowercase ID (should fail)
	cmd := newStatusCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--initiative", "init-001"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for case-mismatched initiative ID")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention 'not found', got: %v", err)
	}
}

// TestStatusCommand_NoInitiativeFilterShowsAll tests that without the filter,
// all tasks are shown (baseline behavior)
func TestStatusCommand_NoInitiativeFilterShowsAll(t *testing.T) {
	tmpDir := withStatusTestDir(t)

	// Create backend and save test data
	backend := createStatusTestBackend(t, tmpDir)

	// Create initiative
	init := initiative.New("INIT-001", "Test Initiative")
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create tasks with and without initiative
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

	// Test: No filter should show all tasks
	cmd := newStatusCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute command: %v", err)
	}

	output := out.String()

	// Should show both tasks
	if !strings.Contains(output, "TASK-001") {
		t.Error("output should contain TASK-001")
	}
	if !strings.Contains(output, "TASK-002") {
		t.Error("output should contain TASK-002")
	}

	// Summary should show 2 tasks
	if !strings.Contains(output, "2 tasks") {
		t.Error("summary should show '2 tasks'")
	}
}

// TestStatusCommand_SystemBlockedWithInitiative tests that system-blocked tasks
// (requiring human input) are shown when filtered by initiative
func TestStatusCommand_SystemBlockedWithInitiative(t *testing.T) {
	tmpDir := withStatusTestDir(t)

	// Create backend and save test data
	backend := createStatusTestBackend(t, tmpDir)

	// Create initiative
	init := initiative.New("INIT-001", "Test Initiative")
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create system-blocked task in initiative
	t1 := task.NewProtoTask("TASK-001", "Blocked task in INIT-001")
	task.SetInitiativeProto(t1, "INIT-001")
	t1.Status = orcv1.TaskStatus_TASK_STATUS_BLOCKED
	if err := backend.SaveTask(t1); err != nil {
		t.Fatalf("save task 1: %v", err)
	}

	// Create system-blocked task NOT in initiative
	t2 := task.NewProtoTask("TASK-002", "Blocked task without initiative")
	t2.Status = orcv1.TaskStatus_TASK_STATUS_BLOCKED
	if err := backend.SaveTask(t2); err != nil {
		t.Fatalf("save task 2: %v", err)
	}

	// Close backend before running command
	_ = backend.Close()

	// Test: Filter by initiative
	cmd := newStatusCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--initiative", "INIT-001"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute command: %v", err)
	}

	output := out.String()

	// Should show TASK-001 in ATTENTION NEEDED category
	if !strings.Contains(output, "TASK-001") {
		t.Error("output should contain TASK-001")
	}
	if !strings.Contains(output, "ATTENTION") {
		t.Error("output should contain ATTENTION NEEDED category")
	}

	// Should NOT show TASK-002 (not in initiative)
	if strings.Contains(output, "TASK-002") {
		t.Error("output should NOT contain TASK-002 (not in initiative)")
	}
}

// TestStatusCommand_UnassignedWithNoUnassignedTasks tests the message when
// filtering for unassigned tasks but none exist
func TestStatusCommand_UnassignedWithNoUnassignedTasks(t *testing.T) {
	tmpDir := withStatusTestDir(t)

	// Create backend and save test data
	backend := createStatusTestBackend(t, tmpDir)

	// Create initiative
	init := initiative.New("INIT-001", "Test Initiative")
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create task with initiative (no unassigned tasks)
	t1 := task.NewProtoTask("TASK-001", "Task in initiative")
	task.SetInitiativeProto(t1, "INIT-001")
	if err := backend.SaveTask(t1); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Close backend before running command
	_ = backend.Close()

	// Test: Filter by unassigned
	cmd := newStatusCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--initiative", "unassigned"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute command: %v", err)
	}

	output := out.String()

	// Should show helpful message
	if !strings.Contains(output, "No tasks found") {
		t.Error("output should mention 'No tasks found'")
	}
	// Should mention it's filtering for unassigned
	if !strings.Contains(output, "unassigned") || !strings.Contains(output, "initiative") {
		t.Error("output should mention unassigned filter context")
	}
}

// TestStatusCommand_PriorityOrderingPreserved tests that priority ordering
// within categories is preserved when filtering by initiative
func TestStatusCommand_PriorityOrderingPreserved(t *testing.T) {
	tmpDir := withStatusTestDir(t)

	// Create backend and save test data
	backend := createStatusTestBackend(t, tmpDir)

	// Create initiative
	init := initiative.New("INIT-001", "Test Initiative")
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create tasks with different priorities (all in READY state)
	t1 := task.NewProtoTask("TASK-001", "Critical priority")
	task.SetInitiativeProto(t1, "INIT-001")
	t1.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	t1.Priority = orcv1.TaskPriority_TASK_PRIORITY_CRITICAL
	if err := backend.SaveTask(t1); err != nil {
		t.Fatalf("save task 1: %v", err)
	}

	t2 := task.NewProtoTask("TASK-002", "Low priority")
	task.SetInitiativeProto(t2, "INIT-001")
	t2.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	t2.Priority = orcv1.TaskPriority_TASK_PRIORITY_LOW
	if err := backend.SaveTask(t2); err != nil {
		t.Fatalf("save task 2: %v", err)
	}

	t3 := task.NewProtoTask("TASK-003", "High priority")
	task.SetInitiativeProto(t3, "INIT-001")
	t3.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	t3.Priority = orcv1.TaskPriority_TASK_PRIORITY_HIGH
	if err := backend.SaveTask(t3); err != nil {
		t.Fatalf("save task 3: %v", err)
	}

	// Close backend before running command
	_ = backend.Close()

	// Test: Filter by initiative
	cmd := newStatusCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--initiative", "INIT-001"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute command: %v", err)
	}

	output := out.String()

	// All tasks should appear in READY section
	if !strings.Contains(output, "TASK-001") {
		t.Error("output should contain TASK-001 (critical)")
	}
	if !strings.Contains(output, "TASK-002") {
		t.Error("output should contain TASK-002 (low)")
	}
	if !strings.Contains(output, "TASK-003") {
		t.Error("output should contain TASK-003 (high)")
	}

	// Verify ordering: critical should appear before low
	idx001 := strings.Index(output, "TASK-001")
	idx002 := strings.Index(output, "TASK-002")
	if idx001 == -1 || idx002 == -1 || idx001 > idx002 {
		t.Error("TASK-001 (critical) should appear before TASK-002 (low)")
	}
}
