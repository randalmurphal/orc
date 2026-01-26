package executor

import (
	"context"
	"log/slog"
	"os"
	"testing"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/initiative"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// TestAutoComplete_InitiativeNoBranchWithAllTasksComplete tests that an initiative
// without BranchBase is automatically completed when all its tasks are done.
// This covers SC-1: Initiatives without BranchBase auto-complete when all tasks completed.
func TestAutoComplete_InitiativeNoBranchWithAllTasksComplete(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	// Create initiative WITHOUT BranchBase (the key condition)
	init := initiative.New("INIT-001", "Test Initiative")
	init.Status = initiative.StatusActive
	init.AddTask("TASK-001", "First task", nil)
	init.AddTask("TASK-002", "Second task", nil)
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create completed tasks
	task1 := task.NewProtoTask("TASK-001", "First task")
	task1.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	task.SetInitiativeProto(task1, "INIT-001")
	if err := backend.SaveTask(task1); err != nil {
		t.Fatalf("save task1: %v", err)
	}

	task2 := task.NewProtoTask("TASK-002", "Second task")
	task2.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	task.SetInitiativeProto(task2, "INIT-001")
	if err := backend.SaveTask(task2); err != nil {
		t.Fatalf("save task2: %v", err)
	}

	// Create completer and run check
	completer := NewInitiativeCompleter(nil, nil, backend, nil, logger, "")
	err := completer.CheckAndCompleteInitiativeNoBranch(context.Background(), "INIT-001")
	if err != nil {
		t.Fatalf("CheckAndCompleteInitiativeNoBranch error: %v", err)
	}

	// Verify initiative status changed to completed
	reloaded, err := backend.LoadInitiative("INIT-001")
	if err != nil {
		t.Fatalf("reload initiative: %v", err)
	}
	if reloaded.Status != initiative.StatusCompleted {
		t.Errorf("initiative Status = %q, want %q", reloaded.Status, initiative.StatusCompleted)
	}
}

// TestAutoComplete_AlreadyCompleted tests that already-completed initiatives are skipped.
// Edge case from spec: Initiative already completed should not change.
func TestAutoComplete_AlreadyCompleted(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	// Create already-completed initiative
	init := initiative.New("INIT-001", "Already Done")
	init.Status = initiative.StatusCompleted // Already completed!
	init.AddTask("TASK-001", "Task", nil)
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create completed task
	task1 := task.NewProtoTask("TASK-001", "Task")
	task1.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	if err := backend.SaveTask(task1); err != nil {
		t.Fatalf("save task: %v", err)
	}

	completer := NewInitiativeCompleter(nil, nil, backend, nil, logger, "")
	err := completer.CheckAndCompleteInitiativeNoBranch(context.Background(), "INIT-001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Initiative should still be completed (no error, no change)
	reloaded, _ := backend.LoadInitiative("INIT-001")
	if reloaded.Status != initiative.StatusCompleted {
		t.Errorf("status changed unexpectedly to %q", reloaded.Status)
	}
}

// TestAutoComplete_WithBranchBase tests that initiatives with BranchBase are skipped.
// Edge case from spec: Initiative has BranchBase should skip (uses merge flow).
func TestAutoComplete_WithBranchBase(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	// Create initiative WITH BranchBase (should be skipped)
	init := initiative.New("INIT-001", "Feature Branch Initiative")
	init.Status = initiative.StatusActive
	init.BranchBase = "feature/auth" // Has branch base!
	init.AddTask("TASK-001", "Task", nil)
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create completed task
	task1 := task.NewProtoTask("TASK-001", "Task")
	task1.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	if err := backend.SaveTask(task1); err != nil {
		t.Fatalf("save task: %v", err)
	}

	completer := NewInitiativeCompleter(nil, nil, backend, nil, logger, "")
	err := completer.CheckAndCompleteInitiativeNoBranch(context.Background(), "INIT-001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Initiative should still be active (skipped due to BranchBase)
	reloaded, _ := backend.LoadInitiative("INIT-001")
	if reloaded.Status != initiative.StatusActive {
		t.Errorf("status changed unexpectedly to %q, want %q (should skip BranchBase initiatives)",
			reloaded.Status, initiative.StatusActive)
	}
}

// TestAutoComplete_NoTasks tests that initiatives with no tasks are not auto-completed.
// Edge case from spec: Initiative has no tasks should not auto-complete.
func TestAutoComplete_NoTasks(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	// Create initiative with NO tasks
	init := initiative.New("INIT-001", "Empty Initiative")
	init.Status = initiative.StatusActive
	// No tasks added!
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	completer := NewInitiativeCompleter(nil, nil, backend, nil, logger, "")
	err := completer.CheckAndCompleteInitiativeNoBranch(context.Background(), "INIT-001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Initiative should still be active (empty initiatives don't auto-complete)
	reloaded, _ := backend.LoadInitiative("INIT-001")
	if reloaded.Status != initiative.StatusActive {
		t.Errorf("status changed to %q, want %q (empty initiatives should not auto-complete)",
			reloaded.Status, initiative.StatusActive)
	}
}

// TestAutoComplete_SomePending tests that initiatives with some pending tasks
// are not auto-completed.
// Edge case from spec: Initiative with some tasks pending should not auto-complete.
func TestAutoComplete_SomePending(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	// Create initiative with mixed task statuses
	init := initiative.New("INIT-001", "Partial Initiative")
	init.Status = initiative.StatusActive
	init.AddTask("TASK-001", "Done task", nil)
	init.AddTask("TASK-002", "Pending task", nil)
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// One completed, one pending
	task1 := task.NewProtoTask("TASK-001", "Done task")
	task1.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	if err := backend.SaveTask(task1); err != nil {
		t.Fatalf("save task1: %v", err)
	}

	task2 := task.NewProtoTask("TASK-002", "Pending task")
	task2.Status = orcv1.TaskStatus_TASK_STATUS_CREATED // Not completed!
	if err := backend.SaveTask(task2); err != nil {
		t.Fatalf("save task2: %v", err)
	}

	completer := NewInitiativeCompleter(nil, nil, backend, nil, logger, "")
	err := completer.CheckAndCompleteInitiativeNoBranch(context.Background(), "INIT-001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Initiative should still be active (has pending tasks)
	reloaded, _ := backend.LoadInitiative("INIT-001")
	if reloaded.Status != initiative.StatusActive {
		t.Errorf("status changed to %q, want %q (should not complete with pending tasks)",
			reloaded.Status, initiative.StatusActive)
	}
}

// TestAutoComplete_TaskLoadError tests that task loading errors are handled gracefully.
// Edge case from spec: Initiative with task loading errors should skip that initiative.
// Note: The existing implementation fetches from backend, so we test with missing tasks.
func TestAutoComplete_TaskLoadError(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	// Create initiative referencing tasks that don't exist in backend
	init := initiative.New("INIT-001", "Orphaned Tasks Initiative")
	init.Status = initiative.StatusActive
	init.AddTask("TASK-MISSING-001", "Missing task", nil)
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Don't create the task - it's missing!

	completer := NewInitiativeCompleter(nil, nil, backend, nil, logger, "")
	err := completer.CheckAndCompleteInitiativeNoBranch(context.Background(), "INIT-001")
	// Should not error - graceful handling
	if err != nil {
		t.Fatalf("expected no error for missing tasks, got: %v", err)
	}

	// Initiative should remain active (can't verify task completion)
	reloaded, _ := backend.LoadInitiative("INIT-001")
	if reloaded.Status != initiative.StatusActive {
		t.Errorf("status changed to %q, want %q (missing tasks should not auto-complete)",
			reloaded.Status, initiative.StatusActive)
	}
}

// TestAutoComplete_NotFound tests error handling when initiative doesn't exist.
// Failure mode from spec: Error logged if initiative not found.
func TestAutoComplete_NotFound(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	completer := NewInitiativeCompleter(nil, nil, backend, nil, logger, "")
	err := completer.CheckAndCompleteInitiativeNoBranch(context.Background(), "INIT-NONEXISTENT")

	// Should return error for non-existent initiative
	if err == nil {
		t.Error("expected error for non-existent initiative")
	}
}

// TestAutoComplete_NilBackend tests error handling when backend is nil.
// Failure mode from spec: Backend unavailable should return clear error.
func TestAutoComplete_NilBackend(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	completer := NewInitiativeCompleter(nil, nil, nil, nil, logger, "")
	err := completer.CheckAndCompleteInitiativeNoBranch(context.Background(), "INIT-001")

	// Should return error for nil backend
	if err == nil {
		t.Error("expected error for nil backend")
	}
}
