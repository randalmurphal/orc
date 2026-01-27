package automation

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/randalmurphal/orc/internal/db"
)

func TestProjectDBAdapter_GetExecutionStats(t *testing.T) {
	t.Parallel()

	// Create a temporary database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	pdb, err := db.OpenProject(dbPath)
	if err != nil {
		t.Fatalf("OpenProject failed: %v", err)
	}
	defer func() { _ = pdb.Close() }()

	adapter := NewProjectDBAdapter(pdb)
	ctx := context.Background()

	// Create triggers first (FK constraint requires them)
	for _, id := range []string{"t1", "t2"} {
		trigger := &Trigger{
			ID:          id,
			Type:        TriggerTypeCount,
			Description: "test trigger",
			Enabled:     true,
		}
		if err := adapter.SaveTrigger(ctx, trigger); err != nil {
			t.Fatalf("SaveTrigger(%s) failed: %v", id, err)
		}
	}

	// Insert test executions with various statuses
	executions := []struct {
		triggerID string
		status    ExecutionStatus
	}{
		{"t1", StatusPending},
		{"t1", StatusPending},
		{"t2", StatusRunning},
		{"t1", StatusCompleted},
		{"t1", StatusCompleted},
		{"t1", StatusCompleted},
		{"t2", StatusFailed},
		{"t2", StatusSkipped}, // Skipped should not count as failed
	}

	for i, exec := range executions {
		e := &Execution{
			TriggerID:     exec.triggerID,
			TriggeredAt:   time.Now(),
			TriggerReason: "test",
			Status:        exec.status,
		}
		if err := adapter.CreateExecution(ctx, e); err != nil {
			t.Fatalf("CreateExecution[%d] failed: %v", i, err)
		}
		// Update status after creation (CreateExecution sets StatusPending initially)
		if exec.status != StatusPending {
			if err := adapter.UpdateExecutionStatus(ctx, e.ID, exec.status, ""); err != nil {
				t.Fatalf("UpdateExecutionStatus[%d] failed: %v", i, err)
			}
		}
	}

	// Get stats
	stats, err := adapter.GetExecutionStats(ctx)
	if err != nil {
		t.Fatalf("GetExecutionStats failed: %v", err)
	}

	// Verify counts
	if stats.Pending != 2 {
		t.Errorf("Pending = %d, want 2", stats.Pending)
	}
	if stats.Running != 1 {
		t.Errorf("Running = %d, want 1", stats.Running)
	}
	if stats.Completed != 3 {
		t.Errorf("Completed = %d, want 3", stats.Completed)
	}
	if stats.Failed != 1 {
		t.Errorf("Failed = %d, want 1", stats.Failed)
	}
}

func TestProjectDBAdapter_GetExecutionStats_EmptyTable(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	pdb, err := db.OpenProject(dbPath)
	if err != nil {
		t.Fatalf("OpenProject failed: %v", err)
	}
	defer func() { _ = pdb.Close() }()

	adapter := NewProjectDBAdapter(pdb)

	stats, err := adapter.GetExecutionStats(context.Background())
	if err != nil {
		t.Fatalf("GetExecutionStats failed: %v", err)
	}

	if stats.Pending != 0 || stats.Running != 0 ||
		stats.Completed != 0 || stats.Failed != 0 {
		t.Errorf("Expected all zeros for empty table, got pending=%d running=%d completed=%d failed=%d",
			stats.Pending, stats.Running, stats.Completed, stats.Failed)
	}
}
