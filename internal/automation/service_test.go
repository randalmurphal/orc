package automation

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/randalmurphal/orc/internal/config"
)

// mockDB implements Database interface for testing.
type mockDB struct {
	execStats *ExecutionStats
	execErr   error
}

func (m *mockDB) SaveTrigger(ctx context.Context, trigger *Trigger) error { return nil }
func (m *mockDB) LoadTrigger(ctx context.Context, id string) (*Trigger, error) {
	return nil, nil
}
func (m *mockDB) LoadAllTriggers(ctx context.Context) ([]*Trigger, error) { return nil, nil }
func (m *mockDB) IncrementTriggerCount(ctx context.Context, id string, triggeredAt time.Time) (int, error) {
	return 0, nil
}
func (m *mockDB) SetTriggerEnabled(ctx context.Context, id string, enabled bool) error { return nil }
func (m *mockDB) GetCounter(ctx context.Context, triggerID, metric string) (int, error) {
	return 0, nil
}
func (m *mockDB) IncrementCounter(ctx context.Context, triggerID, metric string) error { return nil }
func (m *mockDB) IncrementAndGetCounter(ctx context.Context, triggerID, metric string) (int, error) {
	return 0, nil
}
func (m *mockDB) ResetCounter(ctx context.Context, triggerID, metric string) error { return nil }
func (m *mockDB) CreateExecution(ctx context.Context, exec *Execution) error      { return nil }
func (m *mockDB) UpdateExecutionStatus(ctx context.Context, id int64, status ExecutionStatus, errorMsg string) error {
	return nil
}
func (m *mockDB) GetRecentExecutions(ctx context.Context, triggerID string, limit int) ([]*Execution, error) {
	return nil, nil
}
func (m *mockDB) RecordMetric(ctx context.Context, metric *Metric) error { return nil }
func (m *mockDB) GetLatestMetric(ctx context.Context, name string) (*Metric, error) {
	return nil, nil
}
func (m *mockDB) CreateNotification(ctx context.Context, notif *Notification) error { return nil }
func (m *mockDB) GetActiveNotifications(ctx context.Context) ([]*Notification, error) {
	return nil, nil
}
func (m *mockDB) DismissNotification(ctx context.Context, id string) error    { return nil }
func (m *mockDB) DismissAllNotifications(ctx context.Context) error           { return nil }
func (m *mockDB) GetExecutionStats(ctx context.Context) (*ExecutionStats, error) {
	return m.execStats, m.execErr
}

func TestGetStats_ReturnsExecutionCounts(t *testing.T) {
	t.Parallel()

	db := &mockDB{
		execStats: &ExecutionStats{
			Pending:   2,
			Running:   1,
			Completed: 10,
			Failed:    3,
		},
	}

	cfg := &config.Config{
		Automation: config.AutomationConfig{
			Triggers: []config.TriggerConfig{
				{ID: "t1", Enabled: true},
				{ID: "t2", Enabled: false},
				{ID: "t3", Enabled: true},
			},
		},
	}

	svc := NewService(cfg, db, slog.Default())

	stats, err := svc.GetStats(context.Background())
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}

	// Verify trigger counts from config
	if stats.TotalTriggers != 3 {
		t.Errorf("TotalTriggers = %d, want 3", stats.TotalTriggers)
	}
	if stats.EnabledTriggers != 2 {
		t.Errorf("EnabledTriggers = %d, want 2", stats.EnabledTriggers)
	}

	// Verify execution stats from database
	if stats.PendingTasks != 2 {
		t.Errorf("PendingTasks = %d, want 2", stats.PendingTasks)
	}
	if stats.RunningTasks != 1 {
		t.Errorf("RunningTasks = %d, want 1", stats.RunningTasks)
	}
	if stats.CompletedTasks != 10 {
		t.Errorf("CompletedTasks = %d, want 10", stats.CompletedTasks)
	}
	if stats.FailedTasks != 3 {
		t.Errorf("FailedTasks = %d, want 3", stats.FailedTasks)
	}
}

func TestGetStats_NoExecutions(t *testing.T) {
	t.Parallel()

	db := &mockDB{
		execStats: &ExecutionStats{
			Pending:   0,
			Running:   0,
			Completed: 0,
			Failed:    0,
		},
	}

	cfg := &config.Config{}
	svc := NewService(cfg, db, slog.Default())

	stats, err := svc.GetStats(context.Background())
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}

	if stats.PendingTasks != 0 || stats.RunningTasks != 0 ||
		stats.CompletedTasks != 0 || stats.FailedTasks != 0 {
		t.Errorf("Expected all zeros, got pending=%d running=%d completed=%d failed=%d",
			stats.PendingTasks, stats.RunningTasks, stats.CompletedTasks, stats.FailedTasks)
	}
}
