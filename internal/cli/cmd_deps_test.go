package cli

// NOTE: Tests in this file use os.Chdir() which is process-wide and not goroutine-safe.
// These tests MUST NOT use t.Parallel() and run sequentially within this package.

import (
	"os"
	"path/filepath"
	"testing"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// withDepsTestDir creates a temp directory with task structure, changes to it,
// and restores the original working directory when the test completes.
// Returns the temp dir and a backend for database operations.
func withDepsTestDir(t *testing.T) (string, storage.Backend) {
	t.Helper()
	tmpDir := t.TempDir()
	orcDir := filepath.Join(tmpDir, task.OrcDir)
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatalf("create orc directory: %v", err)
	}
	// Create config.yaml to mark as orc project
	if err := os.WriteFile(filepath.Join(orcDir, "config.yaml"), []byte(""), 0644); err != nil {
		t.Fatalf("create config.yaml: %v", err)
	}

	// Create backend for database operations
	backend, err := storage.NewDatabaseBackend(tmpDir, nil)
	if err != nil {
		t.Fatalf("create backend: %v", err)
	}
	t.Cleanup(func() {
		_ = backend.Close()
	})

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
	return tmpDir, backend
}

func TestDepsCommand_Structure(t *testing.T) {
	cmd := newDepsCmd()

	// Verify command structure
	if cmd.Use != "deps [task-id]" {
		t.Errorf("command Use = %q, want %q", cmd.Use, "deps [task-id]")
	}

	// Verify flags exist
	if cmd.Flag("tree") == nil {
		t.Error("missing --tree flag")
	}
	if cmd.Flag("graph") == nil {
		t.Error("missing --graph flag")
	}
	if cmd.Flag("initiative") == nil {
		t.Error("missing --initiative flag")
	}

	// Verify shorthand for initiative
	if cmd.Flag("initiative").Shorthand != "i" {
		t.Errorf("initiative shorthand = %q, want 'i'", cmd.Flag("initiative").Shorthand)
	}
}

func TestDepsCommand_MaxArgs(t *testing.T) {
	cmd := newDepsCmd()

	// Should accept 0 or 1 argument
	if err := cmd.Args(cmd, []string{}); err != nil {
		t.Errorf("unexpected error for zero args: %v", err)
	}
	if err := cmd.Args(cmd, []string{"TASK-001"}); err != nil {
		t.Errorf("unexpected error for one arg: %v", err)
	}
	if err := cmd.Args(cmd, []string{"TASK-001", "TASK-002"}); err == nil {
		t.Error("expected error for two args")
	}
}

func TestShowDependencyTree_SingleTask(t *testing.T) {
	_, backend := withDepsTestDir(t)

	// Create a task with no dependencies
	tk := task.NewProtoTask("TASK-001", "Root task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_PLANNED
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Load all tasks and populate computed fields
	allTasks, err := backend.LoadAllTasks()
	if err != nil {
		t.Fatalf("failed to load tasks: %v", err)
	}
	task.PopulateComputedFieldsProto(allTasks)

	// Build task map
	taskMap := make(map[string]*orcv1.Task)
	for _, tk := range allTasks {
		taskMap[tk.Id] = tk
	}

	// Should not error for a task with no dependencies
	if err := showDependencyTree(taskMap["TASK-001"], taskMap); err != nil {
		t.Errorf("showDependencyTree() error = %v", err)
	}
}

func TestShowDependencyTree_WithDependencies(t *testing.T) {
	_, backend := withDepsTestDir(t)

	// Create tasks with dependencies: TASK-003 -> TASK-002 -> TASK-001
	tk1 := task.NewProtoTask("TASK-001", "Root task")
	tk1.Status = orcv1.TaskStatus_TASK_STATUS_PLANNED
	if err := backend.SaveTask(tk1); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	tk2 := task.NewProtoTask("TASK-002", "Middle task")
	tk2.Status = orcv1.TaskStatus_TASK_STATUS_PLANNED
	tk2.BlockedBy = []string{"TASK-001"}
	if err := backend.SaveTask(tk2); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	tk3 := task.NewProtoTask("TASK-003", "Leaf task")
	tk3.Status = orcv1.TaskStatus_TASK_STATUS_PLANNED
	tk3.BlockedBy = []string{"TASK-002"}
	if err := backend.SaveTask(tk3); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Load all tasks and populate computed fields
	allTasks, err := backend.LoadAllTasks()
	if err != nil {
		t.Fatalf("failed to load tasks: %v", err)
	}
	task.PopulateComputedFieldsProto(allTasks)

	// Build task map
	taskMap := make(map[string]*orcv1.Task)
	for _, tk := range allTasks {
		taskMap[tk.Id] = tk
	}

	// Verify blocks are computed correctly
	if len(taskMap["TASK-001"].Blocks) != 1 || taskMap["TASK-001"].Blocks[0] != "TASK-002" {
		t.Errorf("TASK-001 Blocks = %v, want [TASK-002]", taskMap["TASK-001"].Blocks)
	}
	if len(taskMap["TASK-002"].Blocks) != 1 || taskMap["TASK-002"].Blocks[0] != "TASK-003" {
		t.Errorf("TASK-002 Blocks = %v, want [TASK-003]", taskMap["TASK-002"].Blocks)
	}

	// Should not error
	if err := showDependencyTree(taskMap["TASK-003"], taskMap); err != nil {
		t.Errorf("showDependencyTree() error = %v", err)
	}
}

func TestShowDependencyOverview(t *testing.T) {
	_, backend := withDepsTestDir(t)

	// Create tasks: one blocking, one blocked, one independent
	tk1 := task.NewProtoTask("TASK-001", "Root task")
	tk1.Status = orcv1.TaskStatus_TASK_STATUS_PLANNED
	if err := backend.SaveTask(tk1); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	tk2 := task.NewProtoTask("TASK-002", "Blocked task")
	tk2.Status = orcv1.TaskStatus_TASK_STATUS_PLANNED
	tk2.BlockedBy = []string{"TASK-001"}
	if err := backend.SaveTask(tk2); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	tk3 := task.NewProtoTask("TASK-003", "Independent task")
	tk3.Status = orcv1.TaskStatus_TASK_STATUS_PLANNED
	if err := backend.SaveTask(tk3); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Load all tasks and populate computed fields
	allTasks, err := backend.LoadAllTasks()
	if err != nil {
		t.Fatalf("failed to load tasks: %v", err)
	}
	task.PopulateComputedFieldsProto(allTasks)

	// Build task map
	taskMap := make(map[string]*orcv1.Task)
	for _, tk := range allTasks {
		taskMap[tk.Id] = tk
	}

	// Should not error
	if err := showDependencyOverview(allTasks, taskMap); err != nil {
		t.Errorf("showDependencyOverview() error = %v", err)
	}
}

func TestShowDependencyGraph(t *testing.T) {
	_, backend := withDepsTestDir(t)

	// Create a simple dependency chain
	tk1 := task.NewProtoTask("TASK-001", "Root task")
	tk1.Status = orcv1.TaskStatus_TASK_STATUS_PLANNED
	if err := backend.SaveTask(tk1); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	tk2 := task.NewProtoTask("TASK-002", "Child task")
	tk2.Status = orcv1.TaskStatus_TASK_STATUS_PLANNED
	tk2.BlockedBy = []string{"TASK-001"}
	if err := backend.SaveTask(tk2); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Load all tasks and populate computed fields
	allTasks, err := backend.LoadAllTasks()
	if err != nil {
		t.Fatalf("failed to load tasks: %v", err)
	}
	task.PopulateComputedFieldsProto(allTasks)

	// Build task map
	taskMap := make(map[string]*orcv1.Task)
	for _, tk := range allTasks {
		taskMap[tk.Id] = tk
	}

	// Should not error with empty initiative filter (all tasks)
	if err := showDependencyGraph(allTasks, taskMap, ""); err != nil {
		t.Errorf("showDependencyGraph() error = %v", err)
	}
}

func TestFormatBlockerList(t *testing.T) {
	tests := []struct {
		name     string
		blockers []string
		want     string
	}{
		{
			name:     "empty",
			blockers: nil,
			want:     "",
		},
		{
			name:     "single",
			blockers: []string{"TASK-001"},
			want:     "TASK-001",
		},
		{
			name:     "two",
			blockers: []string{"TASK-001", "TASK-002"},
			want:     "TASK-001, TASK-002",
		},
		{
			name:     "three",
			blockers: []string{"TASK-001", "TASK-002", "TASK-003"},
			want:     "TASK-001, TASK-002, TASK-003",
		},
		{
			name:     "more than three",
			blockers: []string{"TASK-001", "TASK-002", "TASK-003", "TASK-004", "TASK-005"},
			want:     "TASK-001, TASK-002, TASK-003 +2 more",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatBlockerList(tt.blockers)
			if got != tt.want {
				t.Errorf("formatBlockerList(%v) = %q, want %q", tt.blockers, got, tt.want)
			}
		})
	}
}

func TestDepsOutput_JSON(t *testing.T) {
	_, backend := withDepsTestDir(t)

	// Create tasks with dependencies
	tk1 := task.NewProtoTask("TASK-001", "Root task")
	tk1.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	if err := backend.SaveTask(tk1); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	tk2 := task.NewProtoTask("TASK-002", "Child task")
	tk2.Status = orcv1.TaskStatus_TASK_STATUS_PLANNED
	tk2.BlockedBy = []string{"TASK-001"}
	if err := backend.SaveTask(tk2); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Load all tasks and populate computed fields
	allTasks, err := backend.LoadAllTasks()
	if err != nil {
		t.Fatalf("failed to load tasks: %v", err)
	}
	task.PopulateComputedFieldsProto(allTasks)

	// Build task map
	taskMap := make(map[string]*orcv1.Task)
	for _, tk := range allTasks {
		taskMap[tk.Id] = tk
	}

	// Set jsonOut to true for this test
	oldJsonOut := jsonOut
	jsonOut = true
	defer func() { jsonOut = oldJsonOut }()

	// Should not error when producing JSON output
	// Note: This writes to stdout which we can't easily capture in tests,
	// but we verify it doesn't crash
	if err := showDepsJSON(taskMap["TASK-002"], taskMap); err != nil {
		t.Errorf("showDepsJSON() error = %v", err)
	}
}

func TestGetChain(t *testing.T) {
	_, backend := withDepsTestDir(t)

	// Create a linear chain: TASK-003 -> TASK-002 -> TASK-001
	tk1 := task.NewProtoTask("TASK-001", "Root task")
	tk1.Status = orcv1.TaskStatus_TASK_STATUS_PLANNED
	if err := backend.SaveTask(tk1); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	tk2 := task.NewProtoTask("TASK-002", "Middle task")
	tk2.Status = orcv1.TaskStatus_TASK_STATUS_PLANNED
	tk2.BlockedBy = []string{"TASK-001"}
	if err := backend.SaveTask(tk2); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	tk3 := task.NewProtoTask("TASK-003", "Leaf task")
	tk3.Status = orcv1.TaskStatus_TASK_STATUS_PLANNED
	tk3.BlockedBy = []string{"TASK-002"}
	if err := backend.SaveTask(tk3); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Load all tasks and populate computed fields
	allTasks, err := backend.LoadAllTasks()
	if err != nil {
		t.Fatalf("failed to load tasks: %v", err)
	}
	task.PopulateComputedFieldsProto(allTasks)

	// Build task map
	taskMap := make(map[string]*orcv1.Task)
	for _, tk := range allTasks {
		taskMap[tk.Id] = tk
	}

	// Build filtered IDs (all tasks)
	filteredIDs := make(map[string]bool)
	for _, tk := range allTasks {
		filteredIDs[tk.Id] = true
	}

	// Test chain starting from root (TASK-001)
	printed := make(map[string]bool)
	chain := getChain(taskMap["TASK-001"], taskMap, filteredIDs, printed)

	// Should find a chain of TASK-001 -> TASK-002 -> TASK-003
	if chain == nil {
		t.Error("expected chain, got nil")
	} else if len(chain) != 3 {
		t.Errorf("chain length = %d, want 3", len(chain))
	}
}
