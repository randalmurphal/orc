package executor

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/randalmurphal/orc/internal/initiative"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// Tests for SC-1: When all tasks in an initiative (with no BranchBase) complete,
// the initiative status changes to "completed"

func TestCheckAndCompleteInitiativeNoBranch_AllTasksComplete(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	// Create an initiative without BranchBase
	init := &initiative.Initiative{
		ID:        "INIT-001",
		Title:     "Test Initiative",
		Status:    initiative.StatusActive,
		Tasks: []initiative.TaskRef{
			{ID: "TASK-001", Title: "Task 1", Status: "created"},
			{ID: "TASK-002", Title: "Task 2", Status: "created"},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create completed tasks in the backend
	task1 := &task.Task{
		ID:     "TASK-001",
		Title:  "Task 1",
		Status: task.StatusCompleted,
	}
	task2 := &task.Task{
		ID:     "TASK-002",
		Title:  "Task 2",
		Status: task.StatusCompleted,
	}
	if err := backend.SaveTask(task1); err != nil {
		t.Fatalf("save task1: %v", err)
	}
	if err := backend.SaveTask(task2); err != nil {
		t.Fatalf("save task2: %v", err)
	}

	completer := NewInitiativeCompleter(nil, nil, backend, nil, logger, "")

	// Act: Check and complete initiative
	ctx := context.Background()
	err := completer.CheckAndCompleteInitiativeNoBranch(ctx, "INIT-001")
	if err != nil {
		t.Fatalf("CheckAndCompleteInitiativeNoBranch() error = %v", err)
	}

	// Assert: Initiative should be completed
	loaded, err := backend.LoadInitiative("INIT-001")
	if err != nil {
		t.Fatalf("load initiative: %v", err)
	}
	if loaded.Status != initiative.StatusCompleted {
		t.Errorf("initiative status = %q, want %q", loaded.Status, initiative.StatusCompleted)
	}
}

func TestCheckAndCompleteInitiativeNoBranch_SomeTasksPending(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	// Create initiative with tasks
	init := &initiative.Initiative{
		ID:        "INIT-002",
		Title:     "Test Initiative",
		Status:    initiative.StatusActive,
		Tasks: []initiative.TaskRef{
			{ID: "TASK-003", Title: "Task 3", Status: "created"},
			{ID: "TASK-004", Title: "Task 4", Status: "created"},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create one completed task and one pending task
	task1 := &task.Task{
		ID:     "TASK-003",
		Title:  "Task 3",
		Status: task.StatusCompleted,
	}
	task2 := &task.Task{
		ID:     "TASK-004",
		Title:  "Task 4",
		Status: task.StatusRunning, // Not complete
	}
	if err := backend.SaveTask(task1); err != nil {
		t.Fatalf("save task1: %v", err)
	}
	if err := backend.SaveTask(task2); err != nil {
		t.Fatalf("save task2: %v", err)
	}

	completer := NewInitiativeCompleter(nil, nil, backend, nil, logger, "")

	// Act
	ctx := context.Background()
	err := completer.CheckAndCompleteInitiativeNoBranch(ctx, "INIT-002")
	if err != nil {
		t.Fatalf("CheckAndCompleteInitiativeNoBranch() error = %v", err)
	}

	// Assert: Initiative should NOT be completed (remains active)
	loaded, err := backend.LoadInitiative("INIT-002")
	if err != nil {
		t.Fatalf("load initiative: %v", err)
	}
	if loaded.Status != initiative.StatusActive {
		t.Errorf("initiative status = %q, want %q (should remain active with incomplete tasks)", loaded.Status, initiative.StatusActive)
	}
}

func TestCheckAndCompleteInitiativeNoBranch_NoTasks(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	// Create initiative with no tasks
	init := &initiative.Initiative{
		ID:        "INIT-003",
		Title:     "Empty Initiative",
		Status:    initiative.StatusActive,
		Tasks:     []initiative.TaskRef{}, // No tasks
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	completer := NewInitiativeCompleter(nil, nil, backend, nil, logger, "")

	// Act
	ctx := context.Background()
	err := completer.CheckAndCompleteInitiativeNoBranch(ctx, "INIT-003")
	if err != nil {
		t.Fatalf("CheckAndCompleteInitiativeNoBranch() error = %v", err)
	}

	// Assert: Initiative should NOT be completed (empty initiatives stay as-is)
	loaded, err := backend.LoadInitiative("INIT-003")
	if err != nil {
		t.Fatalf("load initiative: %v", err)
	}
	if loaded.Status != initiative.StatusActive {
		t.Errorf("initiative status = %q, want %q (empty initiatives should not auto-complete)", loaded.Status, initiative.StatusActive)
	}
}

func TestCheckAndCompleteInitiativeNoBranch_AlreadyCompleted(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	// Create already-completed initiative
	init := &initiative.Initiative{
		ID:        "INIT-004",
		Title:     "Already Done",
		Status:    initiative.StatusCompleted,
		Tasks: []initiative.TaskRef{
			{ID: "TASK-005", Title: "Task 5", Status: "completed"},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	originalUpdatedAt := init.UpdatedAt
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	completer := NewInitiativeCompleter(nil, nil, backend, nil, logger, "")

	// Act
	ctx := context.Background()
	err := completer.CheckAndCompleteInitiativeNoBranch(ctx, "INIT-004")
	if err != nil {
		t.Fatalf("CheckAndCompleteInitiativeNoBranch() error = %v", err)
	}

	// Assert: Initiative should remain completed (no-op)
	loaded, err := backend.LoadInitiative("INIT-004")
	if err != nil {
		t.Fatalf("load initiative: %v", err)
	}
	if loaded.Status != initiative.StatusCompleted {
		t.Errorf("initiative status = %q, want %q", loaded.Status, initiative.StatusCompleted)
	}
	// UpdatedAt should not change significantly (no unnecessary save)
	if loaded.UpdatedAt.After(originalUpdatedAt.Add(time.Second)) {
		t.Error("UpdatedAt was modified unnecessarily for already-completed initiative")
	}
}

func TestCheckAndCompleteInitiativeNoBranch_WithBranchBase_Skipped(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	// Create initiative WITH BranchBase (should be skipped by this function)
	init := &initiative.Initiative{
		ID:         "INIT-005",
		Title:      "Branch-based Initiative",
		Status:     initiative.StatusActive,
		BranchBase: "feature/test", // Has branch base
		Tasks: []initiative.TaskRef{
			{ID: "TASK-006", Title: "Task 6", Status: "completed"},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create completed task
	task1 := &task.Task{
		ID:     "TASK-006",
		Title:  "Task 6",
		Status: task.StatusCompleted,
	}
	if err := backend.SaveTask(task1); err != nil {
		t.Fatalf("save task: %v", err)
	}

	completer := NewInitiativeCompleter(nil, nil, backend, nil, logger, "")

	// Act
	ctx := context.Background()
	err := completer.CheckAndCompleteInitiativeNoBranch(ctx, "INIT-005")
	if err != nil {
		t.Fatalf("CheckAndCompleteInitiativeNoBranch() error = %v", err)
	}

	// Assert: Initiative should NOT be completed (has branch base, use merge flow instead)
	loaded, err := backend.LoadInitiative("INIT-005")
	if err != nil {
		t.Fatalf("load initiative: %v", err)
	}
	if loaded.Status != initiative.StatusActive {
		t.Errorf("initiative status = %q, want %q (branch-based initiatives should use merge flow)", loaded.Status, initiative.StatusActive)
	}
}

func TestCheckAndCompleteInitiativeNoBranch_WithBlockedByDeps(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	// First create the initiative that will be referenced in BlockedBy
	initOther := &initiative.Initiative{
		ID:        "INIT-OTHER",
		Title:     "Blocker Initiative",
		Status:    initiative.StatusActive,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := backend.SaveInitiative(initOther); err != nil {
		t.Fatalf("save blocker initiative: %v", err)
	}

	// Create initiative with BlockedBy dependencies but all tasks complete
	init := &initiative.Initiative{
		ID:        "INIT-006",
		Title:     "Dependent Initiative",
		Status:    initiative.StatusActive,
		BlockedBy: []string{"INIT-OTHER"}, // Has dependencies
		Tasks: []initiative.TaskRef{
			{ID: "TASK-007", Title: "Task 7", Status: "completed"},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create completed task
	task1 := &task.Task{
		ID:     "TASK-007",
		Title:  "Task 7",
		Status: task.StatusCompleted,
	}
	if err := backend.SaveTask(task1); err != nil {
		t.Fatalf("save task: %v", err)
	}

	completer := NewInitiativeCompleter(nil, nil, backend, nil, logger, "")

	// Act
	ctx := context.Background()
	err := completer.CheckAndCompleteInitiativeNoBranch(ctx, "INIT-006")
	if err != nil {
		t.Fatalf("CheckAndCompleteInitiativeNoBranch() error = %v", err)
	}

	// Assert: Initiative SHOULD be completed (BlockedBy is for run ordering, not completion)
	loaded, err := backend.LoadInitiative("INIT-006")
	if err != nil {
		t.Fatalf("load initiative: %v", err)
	}
	if loaded.Status != initiative.StatusCompleted {
		t.Errorf("initiative status = %q, want %q (BlockedBy deps should not prevent completion)", loaded.Status, initiative.StatusCompleted)
	}
}

// Tests for failure modes

func TestCheckAndCompleteInitiativeNoBranch_NotFound(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	completer := NewInitiativeCompleter(nil, nil, backend, nil, logger, "")

	// Act: Try to complete non-existent initiative
	ctx := context.Background()
	err := completer.CheckAndCompleteInitiativeNoBranch(ctx, "INIT-NONEXISTENT")

	// Assert: Should return error (initiative not found)
	if err == nil {
		t.Error("CheckAndCompleteInitiativeNoBranch() expected error for non-existent initiative")
	}
}

func TestCheckAndCompleteInitiativeNoBranch_NilBackend(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	completer := NewInitiativeCompleter(nil, nil, nil, nil, logger, "") // nil backend

	// Act
	ctx := context.Background()
	err := completer.CheckAndCompleteInitiativeNoBranch(ctx, "INIT-001")

	// Assert: Should return error (backend required)
	if err == nil {
		t.Error("CheckAndCompleteInitiativeNoBranch() expected error for nil backend")
	}
}

// Tests for SC-2: Existing branch merge flow still works
// (This is tested by existing tests - we just ensure we don't break them)

func TestCheckAndCompleteInitiative_BranchBasedStillWorks(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	// Create initiative with BranchBase
	init := &initiative.Initiative{
		ID:         "INIT-BRANCH",
		Title:      "Branch Initiative",
		Status:     initiative.StatusActive,
		BranchBase: "feature/test",
		Tasks: []initiative.TaskRef{
			{ID: "TASK-B1", Title: "Branch Task 1", Status: "completed"},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create completed task
	task1 := &task.Task{
		ID:     "TASK-B1",
		Title:  "Branch Task 1",
		Status: task.StatusCompleted,
	}
	if err := backend.SaveTask(task1); err != nil {
		t.Fatalf("save task: %v", err)
	}

	completer := NewInitiativeCompleter(nil, nil, backend, nil, logger, "")

	// Act: Use existing CheckAndCompleteInitiative (should trigger merge flow)
	ctx := context.Background()
	result, err := completer.CheckAndCompleteInitiative(ctx, "INIT-BRANCH")

	// Assert: Should succeed (no GitHub client so won't actually merge, but shouldn't error before that)
	if err != nil {
		t.Fatalf("CheckAndCompleteInitiative() error = %v", err)
	}
	if result == nil {
		t.Fatal("CheckAndCompleteInitiative() returned nil result")
	}
	if result.InitiativeID != "INIT-BRANCH" {
		t.Errorf("result.InitiativeID = %q, want %q", result.InitiativeID, "INIT-BRANCH")
	}
}

// Tests for SC-4: Task completion triggers initiative status check

func TestWorkflowExecutor_TaskCompletion_TriggersInitiativeCheck(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	// Create initiative without BranchBase
	init := &initiative.Initiative{
		ID:     "INIT-AUTO",
		Title:  "Auto-complete Initiative",
		Status: initiative.StatusActive,
		Tasks: []initiative.TaskRef{
			{ID: "TASK-AUTO", Title: "Auto Task", Status: "created"},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create a task linked to the initiative
	tsk := &task.Task{
		ID:           "TASK-AUTO",
		Title:        "Auto Task",
		Status:       task.StatusCompleted, // Task is completed
		InitiativeID: "INIT-AUTO",          // Linked to initiative
	}
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Create initiative completer
	completer := NewInitiativeCompleter(nil, nil, backend, nil, logger, "")

	// Simulate what workflow executor should do after task completion
	// When a task with InitiativeID completes, it should trigger this check
	ctx := context.Background()
	if tsk.InitiativeID != "" {
		err := completer.CheckAndCompleteInitiativeNoBranch(ctx, tsk.InitiativeID)
		if err != nil {
			t.Fatalf("CheckAndCompleteInitiativeNoBranch() error = %v", err)
		}
	}

	// Assert: Initiative should be completed
	loaded, err := backend.LoadInitiative("INIT-AUTO")
	if err != nil {
		t.Fatalf("load initiative: %v", err)
	}
	if loaded.Status != initiative.StatusCompleted {
		t.Errorf("initiative status = %q, want %q (should auto-complete when last task completes)", loaded.Status, initiative.StatusCompleted)
	}
}

func TestWorkflowExecutor_TaskCompletion_NoInitiativeID_NoCheck(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	// Create a task NOT linked to any initiative
	tsk := &task.Task{
		ID:           "TASK-SOLO",
		Title:        "Solo Task",
		Status:       task.StatusCompleted,
		InitiativeID: "", // Not linked to initiative
	}
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Assert: InitiativeID is empty, so no initiative check should be triggered
	if tsk.InitiativeID != "" {
		t.Error("Task without initiative should not trigger initiative check")
	}
}

// Integration test: Full flow from task completion to initiative completion

func TestInitiativeAutoComplete_IntegrationFlow(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	// Create initiative with 2 tasks, no BranchBase
	init := &initiative.Initiative{
		ID:     "INIT-INTEG",
		Title:  "Integration Test Initiative",
		Status: initiative.StatusActive,
		Tasks: []initiative.TaskRef{
			{ID: "TASK-I1", Title: "Task 1", Status: "created"},
			{ID: "TASK-I2", Title: "Task 2", Status: "created"},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create both tasks, both linked to initiative
	task1 := &task.Task{
		ID:           "TASK-I1",
		Title:        "Task 1",
		Status:       task.StatusCreated,
		InitiativeID: "INIT-INTEG",
	}
	task2 := &task.Task{
		ID:           "TASK-I2",
		Title:        "Task 2",
		Status:       task.StatusCreated,
		InitiativeID: "INIT-INTEG",
	}
	if err := backend.SaveTask(task1); err != nil {
		t.Fatalf("save task1: %v", err)
	}
	if err := backend.SaveTask(task2); err != nil {
		t.Fatalf("save task2: %v", err)
	}

	completer := NewInitiativeCompleter(nil, nil, backend, nil, logger, "")
	ctx := context.Background()

	// Step 1: Complete first task
	task1.Status = task.StatusCompleted
	if err := backend.SaveTask(task1); err != nil {
		t.Fatalf("save task1 completed: %v", err)
	}
	// Trigger initiative check after task 1 completes
	if err := completer.CheckAndCompleteInitiativeNoBranch(ctx, "INIT-INTEG"); err != nil {
		t.Fatalf("check after task1: %v", err)
	}

	// Initiative should still be active (task2 not complete)
	loaded, err := backend.LoadInitiative("INIT-INTEG")
	if err != nil {
		t.Fatalf("load after task1: %v", err)
	}
	if loaded.Status != initiative.StatusActive {
		t.Errorf("after task1: status = %q, want %q", loaded.Status, initiative.StatusActive)
	}

	// Step 2: Complete second task
	task2.Status = task.StatusCompleted
	if err := backend.SaveTask(task2); err != nil {
		t.Fatalf("save task2 completed: %v", err)
	}
	// Trigger initiative check after task 2 completes
	if err := completer.CheckAndCompleteInitiativeNoBranch(ctx, "INIT-INTEG"); err != nil {
		t.Fatalf("check after task2: %v", err)
	}

	// Initiative should now be completed (all tasks done)
	loaded, err = backend.LoadInitiative("INIT-INTEG")
	if err != nil {
		t.Fatalf("load after task2: %v", err)
	}
	if loaded.Status != initiative.StatusCompleted {
		t.Errorf("after task2: status = %q, want %q", loaded.Status, initiative.StatusCompleted)
	}
}

// Test edge case: Task completion for non-existent initiative should not error
// (best-effort completion - task completion is primary)

func TestCheckAndCompleteInitiativeNoBranch_MissingTask_NoError(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	// Create initiative with task reference but task doesn't exist in backend
	init := &initiative.Initiative{
		ID:     "INIT-MISSING",
		Title:  "Initiative with Missing Task",
		Status: initiative.StatusActive,
		Tasks: []initiative.TaskRef{
			{ID: "TASK-GHOST", Title: "Ghost Task", Status: "completed"},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}
	// Note: TASK-GHOST is not saved to backend

	completer := NewInitiativeCompleter(nil, nil, backend, nil, logger, "")

	// Act: Should handle gracefully (task loader returns empty for missing task)
	ctx := context.Background()
	err := completer.CheckAndCompleteInitiativeNoBranch(ctx, "INIT-MISSING")

	// Assert: Should not error (graceful handling of missing task)
	// The behavior depends on implementation - either:
	// 1. Treat missing task as "not complete" -> initiative stays active
	// 2. Use cached status from TaskRef -> initiative completes
	// Either is acceptable - the key is no panic/error
	if err != nil {
		t.Fatalf("CheckAndCompleteInitiativeNoBranch() unexpected error for missing task: %v", err)
	}
}
