package executor

import (
	"context"
	"testing"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// ============================================================================
// SC-6: Cost entries include user_id from context
// ============================================================================

func TestCostEntry_IncludesUserID(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	globalDB := storage.NewTestGlobalDB(t)

	workflowID := "test-workflow"
	setupMinimalWorkflow(t, backend, workflowID)
	setupMinimalWorkflowGlobal(t, globalDB, workflowID)

	tk := task.NewProtoTask("TASK-001", "Test Cost UserID")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	tk.WorkflowId = &workflowID
	tk.Execution = task.InitProtoExecutionState()
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	mockTurn := NewMockTurnExecutor(`{"status": "complete", "summary": "Done"}`)
	we := NewWorkflowExecutor(
		backend,
		backend.DB(),
		globalDB,
		&config.Config{Model: "sonnet"},
		t.TempDir(),
		WithWorkflowTurnExecutor(mockTurn),
	)

	// Set user ID in context
	ctx := ContextWithUserID(context.Background(), "user-alice")

	// Run workflow — phases will execute and record costs
	_, _ = we.Run(ctx, workflowID, WorkflowRunOptions{
		ContextType: ContextTask,
		TaskID:      "TASK-001",
	})

	// Query cost_log to verify user_id was recorded
	entries := queryCostEntries(t, globalDB, "TASK-001")
	if len(entries) == 0 {
		t.Fatal("expected at least one cost entry after phase execution")
	}

	for _, entry := range entries {
		if entry.UserID != "user-alice" {
			t.Errorf("cost entry user_id = %q, want %q", entry.UserID, "user-alice")
		}
	}
}

// ============================================================================
// SC-6 error path: No user ID in context → empty UserID (no error)
// ============================================================================

func TestCostEntry_EmptyUserIDWhenNoContext(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	globalDB := storage.NewTestGlobalDB(t)

	workflowID := "test-workflow"
	setupMinimalWorkflow(t, backend, workflowID)
	setupMinimalWorkflowGlobal(t, globalDB, workflowID)

	tk := task.NewProtoTask("TASK-002", "Test Cost No UserID")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	tk.WorkflowId = &workflowID
	tk.Execution = task.InitProtoExecutionState()
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	mockTurn := NewMockTurnExecutor(`{"status": "complete", "summary": "Done"}`)
	we := NewWorkflowExecutor(
		backend,
		backend.DB(),
		globalDB,
		&config.Config{Model: "sonnet"},
		t.TempDir(),
		WithWorkflowTurnExecutor(mockTurn),
	)

	// Run WITHOUT user ID in context
	ctx := context.Background()

	_, _ = we.Run(ctx, workflowID, WorkflowRunOptions{
		ContextType: ContextTask,
		TaskID:      "TASK-002",
	})

	// Query cost_log — user_id should be empty, not an error
	entries := queryCostEntries(t, globalDB, "TASK-002")
	if len(entries) == 0 {
		t.Fatal("expected at least one cost entry after phase execution")
	}

	for _, entry := range entries {
		if entry.UserID != "" {
			t.Errorf("cost entry user_id = %q, want empty string when no user in context", entry.UserID)
		}
	}
}

// ============================================================================
// Test helpers
// ============================================================================

// queryCostEntries returns all cost entries for a given task ID from the global database.
func queryCostEntries(t *testing.T, globalDB *db.GlobalDB, taskID string) []db.CostEntry {
	t.Helper()

	rows, err := globalDB.Query(
		"SELECT task_id, phase, model, cost_usd, user_id FROM cost_log WHERE task_id = ?",
		taskID,
	)
	if err != nil {
		t.Fatalf("query cost entries: %v", err)
	}
	defer func() { _ = rows.Close() }()

	var entries []db.CostEntry
	for rows.Next() {
		var e db.CostEntry
		if err := rows.Scan(&e.TaskID, &e.Phase, &e.Model, &e.CostUSD, &e.UserID); err != nil {
			t.Fatalf("scan cost entry: %v", err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate cost entries: %v", err)
	}
	return entries
}
