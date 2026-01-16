package executor

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestActivityState_String(t *testing.T) {
	tests := []struct {
		state    ActivityState
		expected string
	}{
		{ActivityIdle, "Idle"},
		{ActivityWaitingAPI, "Waiting for API"},
		{ActivityStreaming, "Receiving response"},
		{ActivityRunningTool, "Running tool"},
		{ActivityProcessing, "Processing"},
		{ActivitySpecAnalyzing, "Analyzing codebase"},
		{ActivitySpecWriting, "Writing specification"},
		{ActivityState("unknown"), "unknown"},
	}

	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			if got := tt.state.String(); got != tt.expected {
				t.Errorf("ActivityState.String() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestActivityState_IsSpecPhaseActivity(t *testing.T) {
	tests := []struct {
		state    ActivityState
		expected bool
	}{
		{ActivityIdle, false},
		{ActivityWaitingAPI, false},
		{ActivityStreaming, false},
		{ActivityRunningTool, false},
		{ActivityProcessing, false},
		{ActivitySpecAnalyzing, true},
		{ActivitySpecWriting, true},
		{ActivityState("unknown"), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			if got := tt.state.IsSpecPhaseActivity(); got != tt.expected {
				t.Errorf("ActivityState.IsSpecPhaseActivity() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestNewActivityTracker(t *testing.T) {
	tracker := NewActivityTracker()
	if tracker == nil {
		t.Fatal("NewActivityTracker() returned nil")
	}

	if tracker.state != ActivityIdle {
		t.Errorf("initial state = %v, want %v", tracker.state, ActivityIdle)
	}
	if tracker.heartbeatInterval != 30*time.Second {
		t.Errorf("heartbeatInterval = %v, want 30s", tracker.heartbeatInterval)
	}
	if tracker.idleTimeout != 2*time.Minute {
		t.Errorf("idleTimeout = %v, want 2m", tracker.idleTimeout)
	}
	if tracker.turnTimeout != 10*time.Minute {
		t.Errorf("turnTimeout = %v, want 10m", tracker.turnTimeout)
	}
}

func TestActivityTracker_Options(t *testing.T) {
	heartbeat := 1 * time.Second
	idle := 5 * time.Second
	turn := 30 * time.Second

	tracker := NewActivityTracker(
		WithHeartbeatInterval(heartbeat),
		WithIdleTimeout(idle),
		WithTurnTimeout(turn),
		WithActivityMaxIterations(10),
	)

	if tracker.heartbeatInterval != heartbeat {
		t.Errorf("heartbeatInterval = %v, want %v", tracker.heartbeatInterval, heartbeat)
	}
	if tracker.idleTimeout != idle {
		t.Errorf("idleTimeout = %v, want %v", tracker.idleTimeout, idle)
	}
	if tracker.turnTimeout != turn {
		t.Errorf("turnTimeout = %v, want %v", tracker.turnTimeout, turn)
	}
	if tracker.maxIterations != 10 {
		t.Errorf("maxIterations = %v, want 10", tracker.maxIterations)
	}
}

func TestActivityTracker_SetState(t *testing.T) {
	var receivedState ActivityState
	var mu sync.Mutex

	tracker := NewActivityTracker(
		WithStateChangeCallback(func(state ActivityState) {
			mu.Lock()
			receivedState = state
			mu.Unlock()
		}),
	)

	tracker.SetState(ActivityWaitingAPI)

	if got := tracker.State(); got != ActivityWaitingAPI {
		t.Errorf("State() = %v, want %v", got, ActivityWaitingAPI)
	}

	mu.Lock()
	if receivedState != ActivityWaitingAPI {
		t.Errorf("callback received state = %v, want %v", receivedState, ActivityWaitingAPI)
	}
	mu.Unlock()
}

func TestActivityTracker_RecordChunk(t *testing.T) {
	tracker := NewActivityTracker()
	tracker.SetState(ActivityWaitingAPI)

	before := tracker.ChunksReceived()
	tracker.RecordChunk()
	after := tracker.ChunksReceived()

	if after != before+1 {
		t.Errorf("ChunksReceived() = %d, want %d", after, before+1)
	}

	// State should change to streaming after first chunk
	if tracker.State() != ActivityStreaming {
		t.Errorf("State() = %v, want %v", tracker.State(), ActivityStreaming)
	}
}

func TestActivityTracker_TurnDuration(t *testing.T) {
	tracker := NewActivityTracker()

	// Before starting a turn, duration should be 0
	if d := tracker.TurnDuration(); d != 0 {
		t.Errorf("TurnDuration() before turn = %v, want 0", d)
	}

	// Start a turn
	tracker.SetState(ActivityWaitingAPI)
	time.Sleep(10 * time.Millisecond)

	d := tracker.TurnDuration()
	if d < 10*time.Millisecond {
		t.Errorf("TurnDuration() = %v, want >= 10ms", d)
	}
}

func TestActivityTracker_Progress(t *testing.T) {
	tracker := NewActivityTracker(WithActivityMaxIterations(20))
	tracker.SetIteration(5)

	current, max := tracker.Progress()
	if current != 5 {
		t.Errorf("Progress() current = %d, want 5", current)
	}
	if max != 20 {
		t.Errorf("Progress() max = %d, want 20", max)
	}
}

func TestActivityTracker_HeartbeatCallback(t *testing.T) {
	heartbeatCount := 0
	var mu sync.Mutex

	tracker := NewActivityTracker(
		WithHeartbeatInterval(50*time.Millisecond),
		WithHeartbeatCallback(func() {
			mu.Lock()
			heartbeatCount++
			mu.Unlock()
		}),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	tracker.Start(ctx)
	tracker.SetState(ActivityWaitingAPI)

	// Wait for heartbeats
	time.Sleep(180 * time.Millisecond)
	tracker.Stop()

	mu.Lock()
	// We should have received at least 2 heartbeats (at 50ms and 100ms)
	if heartbeatCount < 2 {
		t.Errorf("heartbeat callback count = %d, want >= 2", heartbeatCount)
	}
	mu.Unlock()
}

func TestActivityTracker_IdleWarningCallback(t *testing.T) {
	idleWarningCalled := false
	var mu sync.Mutex

	// Use a very short idle timeout and longer wait to ensure callback fires
	tracker := NewActivityTracker(
		WithIdleTimeout(20*time.Millisecond), // Very short idle timeout
		WithIdleWarningCallback(func(d time.Duration) {
			mu.Lock()
			idleWarningCalled = true
			mu.Unlock()
		}),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	tracker.Start(ctx)
	tracker.SetState(ActivityWaitingAPI)

	// Wait longer than idle timeout (checking happens every idleTimeout/4)
	// With 20ms idle timeout, checks happen every 5ms, so 150ms should be plenty
	time.Sleep(150 * time.Millisecond)
	tracker.Stop()

	mu.Lock()
	if !idleWarningCalled {
		t.Error("idle warning callback was not called")
	}
	mu.Unlock()
}

func TestActivityTracker_NoHeartbeatWhenIdle(t *testing.T) {
	heartbeatCount := 0
	var mu sync.Mutex

	tracker := NewActivityTracker(
		WithHeartbeatInterval(20*time.Millisecond),
		WithHeartbeatCallback(func() {
			mu.Lock()
			heartbeatCount++
			mu.Unlock()
		}),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	tracker.Start(ctx)
	// Don't set state - stays at ActivityIdle

	time.Sleep(80 * time.Millisecond)
	tracker.Stop()

	mu.Lock()
	// No heartbeats should be emitted when idle
	if heartbeatCount != 0 {
		t.Errorf("heartbeat count when idle = %d, want 0", heartbeatCount)
	}
	mu.Unlock()
}

func TestActivityTracker_DoubleStopSafety(t *testing.T) {
	tracker := NewActivityTracker()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	tracker.Start(ctx)
	tracker.SetState(ActivityWaitingAPI)

	// Stop twice - should not panic
	tracker.Stop()
	tracker.Stop()
}
