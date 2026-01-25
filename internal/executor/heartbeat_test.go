package executor

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"
)

// mockHeartbeatUpdater implements the HeartbeatUpdater interface for testing
type mockHeartbeatUpdater struct {
	mu          sync.Mutex
	updateCount int
	lastTaskID  string
	shouldFail  bool
}

func (m *mockHeartbeatUpdater) UpdateTaskHeartbeat(taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updateCount++
	m.lastTaskID = taskID
	if m.shouldFail {
		return os.ErrPermission
	}
	return nil
}

func (m *mockHeartbeatUpdater) getUpdateCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.updateCount
}

func (m *mockHeartbeatUpdater) getLastTaskID() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastTaskID
}

func TestHeartbeatRunner_UpdatesHeartbeat(t *testing.T) {
	t.Parallel()

	updater := &mockHeartbeatUpdater{}

	// Create heartbeat runner with short interval for testing
	runner := NewHeartbeatRunner(updater, "test-task", slog.Default())
	runner.interval = 50 * time.Millisecond

	// Start the heartbeat
	ctx := context.Background()
	runner.Start(ctx)

	// Wait for a few heartbeats
	time.Sleep(150 * time.Millisecond)

	// Stop the heartbeat
	runner.Stop()

	// Verify heartbeats were updated
	updateCount := updater.getUpdateCount()
	if updateCount < 2 {
		t.Errorf("expected at least 2 heartbeat updates, got %d", updateCount)
	}

	// Verify correct task ID was used
	if updater.getLastTaskID() != "test-task" {
		t.Errorf("expected task ID 'test-task', got %s", updater.getLastTaskID())
	}
}

func TestHeartbeatRunner_StopsOnContextCancel(t *testing.T) {
	t.Parallel()

	updater := &mockHeartbeatUpdater{}
	runner := NewHeartbeatRunner(updater, "test-task", slog.Default())
	runner.interval = 50 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	runner.Start(ctx)

	// Cancel context
	cancel()

	// Give it time to stop
	time.Sleep(100 * time.Millisecond)

	// Update count before waiting
	countBefore := updater.getUpdateCount()

	// Wait a bit more - no new updates should happen
	time.Sleep(100 * time.Millisecond)

	countAfter := updater.getUpdateCount()
	if countAfter > countBefore {
		t.Error("heartbeat should have stopped on context cancel")
	}
}

func TestHeartbeatRunner_StopsOnStopSignal(t *testing.T) {
	t.Parallel()

	updater := &mockHeartbeatUpdater{}
	runner := NewHeartbeatRunner(updater, "test-task", slog.Default())
	runner.interval = 50 * time.Millisecond

	ctx := context.Background()
	runner.Start(ctx)

	// Stop via Stop() call
	runner.Stop()

	// Update count right after stop
	countBefore := updater.getUpdateCount()

	// Wait a bit - no new updates should happen
	time.Sleep(150 * time.Millisecond)

	countAfter := updater.getUpdateCount()
	if countAfter > countBefore {
		t.Error("heartbeat should have stopped on Stop() call")
	}
}

func TestHeartbeatRunner_ContinuesOnUpdateFailure(t *testing.T) {
	t.Parallel()

	updater := &mockHeartbeatUpdater{shouldFail: true}
	runner := NewHeartbeatRunner(updater, "test-task", slog.Default())
	runner.interval = 50 * time.Millisecond

	ctx := context.Background()
	runner.Start(ctx)

	// Wait for multiple heartbeat attempts
	time.Sleep(200 * time.Millisecond)

	runner.Stop()

	// Even with failures, multiple update attempts should be made
	updateCount := updater.getUpdateCount()
	if updateCount < 2 {
		t.Errorf("expected at least 2 heartbeat attempts even with failures, got %d", updateCount)
	}
}
