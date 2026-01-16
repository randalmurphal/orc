package executor

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/randalmurphal/orc/internal/state"
)

// mockStateSaver implements the StateSaver interface for testing
type mockStateSaver struct {
	mu         sync.Mutex
	saveCount  int
	lastState  *state.State
	shouldFail bool
}

func (m *mockStateSaver) SaveState(s *state.State) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.saveCount++
	m.lastState = s
	if m.shouldFail {
		return os.ErrPermission
	}
	return nil
}

func (m *mockStateSaver) getSaveCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.saveCount
}

func TestHeartbeatRunner_UpdatesHeartbeat(t *testing.T) {
	// Create a state with execution info
	s := state.New("test-task")
	s.StartExecution(os.Getpid(), "test-host")
	initialHeartbeat := s.Execution.LastHeartbeat

	// Create mock saver
	saver := &mockStateSaver{}

	// Create heartbeat runner with short interval for testing
	runner := NewHeartbeatRunner(saver, s, slog.Default())
	runner.interval = 50 * time.Millisecond

	// Start the heartbeat
	ctx := context.Background()
	runner.Start(ctx)

	// Wait for a few heartbeats
	time.Sleep(150 * time.Millisecond)

	// Stop the heartbeat
	runner.Stop()

	// Verify heartbeats were saved
	saveCount := saver.getSaveCount()
	if saveCount < 2 {
		t.Errorf("expected at least 2 heartbeat saves, got %d", saveCount)
	}

	// Verify heartbeat timestamp was updated
	if !s.Execution.LastHeartbeat.After(initialHeartbeat) {
		t.Error("expected heartbeat timestamp to be updated")
	}
}

func TestHeartbeatRunner_StopsOnContextCancel(t *testing.T) {
	s := state.New("test-task")
	s.StartExecution(os.Getpid(), "test-host")

	saver := &mockStateSaver{}
	runner := NewHeartbeatRunner(saver, s, slog.Default())
	runner.interval = 50 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	runner.Start(ctx)

	// Cancel context
	cancel()

	// Give it time to stop
	time.Sleep(100 * time.Millisecond)

	// Save count before waiting
	countBefore := saver.getSaveCount()

	// Wait a bit more - no new saves should happen
	time.Sleep(100 * time.Millisecond)

	countAfter := saver.getSaveCount()
	if countAfter > countBefore {
		t.Error("heartbeat should have stopped on context cancel")
	}
}

func TestHeartbeatRunner_StopsOnStopSignal(t *testing.T) {
	s := state.New("test-task")
	s.StartExecution(os.Getpid(), "test-host")

	saver := &mockStateSaver{}
	runner := NewHeartbeatRunner(saver, s, slog.Default())
	runner.interval = 50 * time.Millisecond

	ctx := context.Background()
	runner.Start(ctx)

	// Stop via Stop() call
	runner.Stop()

	// Save count right after stop
	countBefore := saver.getSaveCount()

	// Wait a bit - no new saves should happen
	time.Sleep(150 * time.Millisecond)

	countAfter := saver.getSaveCount()
	if countAfter > countBefore {
		t.Error("heartbeat should have stopped on Stop() call")
	}
}

func TestHeartbeatRunner_ContinuesOnSaveFailure(t *testing.T) {
	s := state.New("test-task")
	s.StartExecution(os.Getpid(), "test-host")

	saver := &mockStateSaver{shouldFail: true}
	runner := NewHeartbeatRunner(saver, s, slog.Default())
	runner.interval = 50 * time.Millisecond

	ctx := context.Background()
	runner.Start(ctx)

	// Wait for multiple heartbeat attempts
	time.Sleep(200 * time.Millisecond)

	runner.Stop()

	// Even with failures, multiple save attempts should be made
	saveCount := saver.getSaveCount()
	if saveCount < 2 {
		t.Errorf("expected at least 2 heartbeat attempts even with failures, got %d", saveCount)
	}
}

func TestHeartbeatRunner_UpdateState(t *testing.T) {
	s1 := state.New("task-1")
	s1.StartExecution(os.Getpid(), "test-host")

	s2 := state.New("task-2")
	s2.StartExecution(os.Getpid(), "test-host")

	saver := &mockStateSaver{}
	runner := NewHeartbeatRunner(saver, s1, slog.Default())
	runner.interval = 50 * time.Millisecond

	ctx := context.Background()
	runner.Start(ctx)

	// Wait for first heartbeat
	time.Sleep(75 * time.Millisecond)

	// Update state reference
	runner.UpdateState(s2)

	// Wait for another heartbeat
	time.Sleep(75 * time.Millisecond)

	runner.Stop()

	// The last state saved should be s2
	saver.mu.Lock()
	lastTaskID := saver.lastState.TaskID
	saver.mu.Unlock()

	if lastTaskID != "task-2" {
		t.Errorf("expected last heartbeat to use task-2, got %s", lastTaskID)
	}
}
