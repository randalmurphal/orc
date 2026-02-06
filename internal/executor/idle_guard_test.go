package executor

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ============================================================================
// SC-1: IdleGuard heartbeat-based stale detection at executor level
// ============================================================================

// TestIdleGuard_DetectsStaleHeartbeat verifies that IdleGuard fires its callback
// when the heartbeat hasn't been updated within the timeout period.
// Covers: SC-1
func TestIdleGuard_DetectsStaleHeartbeat(t *testing.T) {
	t.Parallel()

	callbackFired := atomic.Bool{}
	checker := &mockIdleChecker{
		lastHeartbeat: time.Now().Add(-10 * time.Minute), // Very stale
	}

	guard := NewIdleGuard(IdleGuardConfig{
		CheckInterval: 50 * time.Millisecond,
		StaleTimeout:  100 * time.Millisecond,
		Checker:       checker,
		OnStale: func(age time.Duration) {
			callbackFired.Store(true)
		},
		Logger: slog.Default(),
	})

	ctx := context.Background()
	guard.Start(ctx)

	// Wait for at least one check cycle
	time.Sleep(200 * time.Millisecond)
	guard.Stop()

	if !callbackFired.Load() {
		t.Error("expected stale callback to fire when heartbeat is stale")
	}
}

// TestIdleGuard_NoCallbackWhenFresh verifies that IdleGuard does NOT fire its
// callback when heartbeats are fresh.
// Covers: SC-1
func TestIdleGuard_NoCallbackWhenFresh(t *testing.T) {
	t.Parallel()

	callbackFired := atomic.Bool{}
	checker := &mockIdleChecker{
		lastHeartbeat: time.Now(), // Fresh
	}

	guard := NewIdleGuard(IdleGuardConfig{
		CheckInterval: 50 * time.Millisecond,
		StaleTimeout:  5 * time.Second, // Much longer than test duration
		Checker:       checker,
		OnStale: func(age time.Duration) {
			callbackFired.Store(true)
		},
		Logger: slog.Default(),
	})

	ctx := context.Background()
	guard.Start(ctx)

	// Run for several check cycles
	time.Sleep(200 * time.Millisecond)
	guard.Stop()

	if callbackFired.Load() {
		t.Error("expected stale callback to NOT fire when heartbeat is fresh")
	}
}

// TestIdleGuard_StopsCleanly verifies that Stop() terminates the guard loop
// and no further callbacks fire after Stop().
// Covers: SC-1
func TestIdleGuard_StopsCleanly(t *testing.T) {
	t.Parallel()

	callbackCount := atomic.Int64{}
	checker := &mockIdleChecker{
		lastHeartbeat: time.Now().Add(-10 * time.Minute), // Stale
	}

	guard := NewIdleGuard(IdleGuardConfig{
		CheckInterval: 50 * time.Millisecond,
		StaleTimeout:  10 * time.Millisecond,
		Checker:       checker,
		OnStale: func(age time.Duration) {
			callbackCount.Add(1)
		},
		Logger: slog.Default(),
	})

	ctx := context.Background()
	guard.Start(ctx)

	// Let it fire at least once
	time.Sleep(150 * time.Millisecond)
	guard.Stop()

	countAfterStop := callbackCount.Load()

	// Wait and verify no more callbacks
	time.Sleep(200 * time.Millisecond)
	countLater := callbackCount.Load()

	if countLater > countAfterStop {
		t.Errorf("expected no callbacks after Stop(), got %d more", countLater-countAfterStop)
	}
}

// TestIdleGuard_StopsOnContextCancel verifies that context cancellation
// terminates the guard loop.
// Covers: SC-1
func TestIdleGuard_StopsOnContextCancel(t *testing.T) {
	t.Parallel()

	callbackCount := atomic.Int64{}
	checker := &mockIdleChecker{
		lastHeartbeat: time.Now().Add(-10 * time.Minute), // Stale
	}

	guard := NewIdleGuard(IdleGuardConfig{
		CheckInterval: 50 * time.Millisecond,
		StaleTimeout:  10 * time.Millisecond,
		Checker:       checker,
		OnStale: func(age time.Duration) {
			callbackCount.Add(1)
		},
		Logger: slog.Default(),
	})

	ctx, cancel := context.WithCancel(context.Background())
	guard.Start(ctx)

	// Let it fire at least once
	time.Sleep(150 * time.Millisecond)
	cancel()

	// Allow goroutine to exit
	time.Sleep(100 * time.Millisecond)
	countAfterCancel := callbackCount.Load()

	// Wait and verify no more callbacks
	time.Sleep(200 * time.Millisecond)
	countLater := callbackCount.Load()

	if countLater > countAfterCancel {
		t.Errorf("expected no callbacks after context cancel, got %d more", countLater-countAfterCancel)
	}
}

// TestIdleGuard_ContinuesOnCheckError verifies that errors from the checker
// don't crash the guard loop.
// Covers: FM-3 (heartbeat update failures)
func TestIdleGuard_ContinuesOnCheckError(t *testing.T) {
	t.Parallel()

	checkCount := atomic.Int64{}
	checker := &mockIdleChecker{
		shouldError: true,
	}

	guard := NewIdleGuard(IdleGuardConfig{
		CheckInterval: 50 * time.Millisecond,
		StaleTimeout:  100 * time.Millisecond,
		Checker: &countingIdleChecker{
			inner: func() (time.Time, error) {
				return checker.LastHeartbeatTime()
			},
			checkCount: &checkCount,
		},
		OnStale: func(age time.Duration) {},
		Logger:  slog.Default(),
	})

	ctx := context.Background()
	guard.Start(ctx)

	// Wait for multiple check cycles
	time.Sleep(250 * time.Millisecond)
	guard.Stop()

	// Should have attempted multiple checks despite errors
	count := checkCount.Load()
	if count < 2 {
		t.Errorf("expected at least 2 check attempts despite errors, got %d", count)
	}
}

// TestIdleGuard_TransitionFromFreshToStale verifies that the guard detects
// when a previously fresh heartbeat becomes stale.
// Covers: SC-1 (long-running task heartbeat stops)
func TestIdleGuard_TransitionFromFreshToStale(t *testing.T) {
	t.Parallel()

	callbackFired := atomic.Bool{}
	checker := &dynamicIdleChecker{
		mu:            sync.Mutex{},
		lastHeartbeat: time.Now(), // Start fresh
	}

	guard := NewIdleGuard(IdleGuardConfig{
		CheckInterval: 50 * time.Millisecond,
		StaleTimeout:  150 * time.Millisecond,
		Checker:       checker,
		OnStale: func(age time.Duration) {
			callbackFired.Store(true)
		},
		Logger: slog.Default(),
	})

	ctx := context.Background()
	guard.Start(ctx)

	// Initially fresh - callback should NOT fire
	time.Sleep(100 * time.Millisecond)
	if callbackFired.Load() {
		t.Fatal("expected callback NOT to fire while heartbeat is fresh")
	}

	// Simulate heartbeat stopping (don't update checker.lastHeartbeat)
	// Wait for stale timeout + check interval to pass
	time.Sleep(250 * time.Millisecond)
	guard.Stop()

	if !callbackFired.Load() {
		t.Error("expected stale callback to fire after heartbeat stopped updating")
	}
}

// ============================================================================
// Test Doubles
// ============================================================================

// mockIdleChecker returns a fixed heartbeat time.
// Implements the HeartbeatChecker interface expected by IdleGuardConfig.
type mockIdleChecker struct {
	lastHeartbeat time.Time
	shouldError   bool
}

func (m *mockIdleChecker) LastHeartbeatTime() (time.Time, error) {
	if m.shouldError {
		return time.Time{}, errors.New("heartbeat check failed")
	}
	return m.lastHeartbeat, nil
}

// dynamicIdleChecker allows changing the heartbeat time during tests.
type dynamicIdleChecker struct {
	mu            sync.Mutex
	lastHeartbeat time.Time
}

func (d *dynamicIdleChecker) LastHeartbeatTime() (time.Time, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.lastHeartbeat, nil
}

// countingIdleChecker wraps a function and counts calls.
type countingIdleChecker struct {
	inner      func() (time.Time, error)
	checkCount *atomic.Int64
}

func (c *countingIdleChecker) LastHeartbeatTime() (time.Time, error) {
	c.checkCount.Add(1)
	return c.inner()
}
