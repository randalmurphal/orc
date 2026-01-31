// Package executor provides activity tracking for Claude API calls.
package executor

import (
	"context"
	"sync"
	"time"
)

// ActivityState represents what the executor is currently doing.
type ActivityState string

const (
	// ActivityIdle indicates no activity.
	ActivityIdle ActivityState = "idle"
	// ActivityWaitingAPI indicates waiting for Claude API response.
	ActivityWaitingAPI ActivityState = "waiting_api"
	// ActivityStreaming indicates actively receiving streaming response.
	ActivityStreaming ActivityState = "streaming"
	// ActivityRunningTool indicates Claude is running a tool.
	ActivityRunningTool ActivityState = "running_tool"
	// ActivityProcessing indicates processing response.
	ActivityProcessing ActivityState = "processing"
	// ActivitySpecAnalyzing indicates Claude is analyzing the codebase during spec phase.
	ActivitySpecAnalyzing ActivityState = "spec_analyzing"
	// ActivitySpecWriting indicates Claude is writing the specification document.
	ActivitySpecWriting ActivityState = "spec_writing"
)

// String returns a human-readable description of the activity state.
func (s ActivityState) String() string {
	switch s {
	case ActivityIdle:
		return "Idle"
	case ActivityWaitingAPI:
		return "Waiting for API"
	case ActivityStreaming:
		return "Receiving response"
	case ActivityRunningTool:
		return "Running tool"
	case ActivityProcessing:
		return "Processing"
	case ActivitySpecAnalyzing:
		return "Analyzing codebase"
	case ActivitySpecWriting:
		return "Writing specification"
	default:
		return string(s)
	}
}

// IsSpecPhaseActivity returns true if the activity state is specific to the spec phase.
func (s ActivityState) IsSpecPhaseActivity() bool {
	return s == ActivitySpecAnalyzing || s == ActivitySpecWriting
}

// ActivityTracker tracks the current activity state and provides
// progress information for long-running operations.
type ActivityTracker struct {
	mu               sync.RWMutex
	state            ActivityState
	lastActivity     time.Time
	turnStart        time.Time
	lastChunk        time.Time
	chunksReceived   int
	currentIteration int
	maxIterations    int

	// Callbacks for state changes
	onStateChange func(ActivityState)
	onIdleWarning func(time.Duration)
	onHeartbeat   func()
	onTurnTimeout func()

	// Configuration
	heartbeatInterval time.Duration
	idleTimeout       time.Duration
	turnTimeout       time.Duration

	// Control
	stopCh   chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
}

// ActivityTrackerOption configures an ActivityTracker.
type ActivityTrackerOption func(*ActivityTracker)

// WithHeartbeatInterval sets the interval for progress heartbeats.
func WithHeartbeatInterval(d time.Duration) ActivityTrackerOption {
	return func(t *ActivityTracker) { t.heartbeatInterval = d }
}

// WithIdleTimeout sets the timeout before warning about idle state.
func WithIdleTimeout(d time.Duration) ActivityTrackerOption {
	return func(t *ActivityTracker) { t.idleTimeout = d }
}

// WithTurnTimeout sets the maximum time for a single API turn.
func WithTurnTimeout(d time.Duration) ActivityTrackerOption {
	return func(t *ActivityTracker) { t.turnTimeout = d }
}

// WithStateChangeCallback sets a callback for state changes.
func WithStateChangeCallback(fn func(ActivityState)) ActivityTrackerOption {
	return func(t *ActivityTracker) { t.onStateChange = fn }
}

// WithIdleWarningCallback sets a callback for idle warnings.
func WithIdleWarningCallback(fn func(time.Duration)) ActivityTrackerOption {
	return func(t *ActivityTracker) { t.onIdleWarning = fn }
}

// WithHeartbeatCallback sets a callback for heartbeat events.
func WithHeartbeatCallback(fn func()) ActivityTrackerOption {
	return func(t *ActivityTracker) { t.onHeartbeat = fn }
}

// WithActivityMaxIterations sets the maximum iterations for progress display.
func WithActivityMaxIterations(max int) ActivityTrackerOption {
	return func(t *ActivityTracker) { t.maxIterations = max }
}

// NewActivityTracker creates a new activity tracker.
func NewActivityTracker(opts ...ActivityTrackerOption) *ActivityTracker {
	t := &ActivityTracker{
		state:             ActivityIdle,
		lastActivity:      time.Now(),
		heartbeatInterval: 30 * time.Second,
		idleTimeout:       2 * time.Minute,
		turnTimeout:       10 * time.Minute,
		stopCh:            make(chan struct{}),
	}

	for _, opt := range opts {
		opt(t)
	}

	return t
}

// Start begins the activity monitoring goroutine.
func (t *ActivityTracker) Start(ctx context.Context) {
	t.wg.Add(1)
	go t.monitor(ctx)
}

// Stop stops the activity monitoring.
func (t *ActivityTracker) Stop() {
	t.stopOnce.Do(func() {
		close(t.stopCh)
	})
	t.wg.Wait()
}

// State returns the current activity state.
func (t *ActivityTracker) State() ActivityState {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.state
}

// SetState updates the activity state.
func (t *ActivityTracker) SetState(state ActivityState) {
	t.mu.Lock()
	oldState := t.state
	t.state = state
	t.lastActivity = time.Now()
	if state == ActivityWaitingAPI {
		t.turnStart = time.Now()
		t.chunksReceived = 0
	}
	callback := t.onStateChange
	t.mu.Unlock()

	if callback != nil && oldState != state {
		callback(state)
	}
}

// RecordChunk records that a streaming chunk was received.
func (t *ActivityTracker) RecordChunk() {
	t.mu.Lock()
	t.lastActivity = time.Now()
	t.lastChunk = time.Now()
	t.chunksReceived++
	if t.state == ActivityWaitingAPI {
		t.state = ActivityStreaming
	}
	t.mu.Unlock()
}

// SetIteration updates the current iteration for progress display.
func (t *ActivityTracker) SetIteration(iteration int) {
	t.mu.Lock()
	t.currentIteration = iteration
	t.mu.Unlock()
}

// TurnDuration returns how long the current turn has been running.
func (t *ActivityTracker) TurnDuration() time.Duration {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.turnStart.IsZero() {
		return 0
	}
	return time.Since(t.turnStart)
}

// ChunksReceived returns the number of chunks received this turn.
func (t *ActivityTracker) ChunksReceived() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.chunksReceived
}

// Progress returns iteration progress info.
func (t *ActivityTracker) Progress() (current, max int) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.currentIteration, t.maxIterations
}

// monitor runs the background monitoring loop.
func (t *ActivityTracker) monitor(ctx context.Context) {
	defer t.wg.Done()

	// Use a ticker for heartbeat checks
	var heartbeatTicker *time.Ticker
	var heartbeatCh <-chan time.Time

	if t.heartbeatInterval > 0 {
		heartbeatTicker = time.NewTicker(t.heartbeatInterval)
		heartbeatCh = heartbeatTicker.C
		defer heartbeatTicker.Stop()
	}

	// Use a ticker for idle checks (check at 1/4 of idle timeout, min 1s, max 10s)
	idleCheckInterval := t.idleTimeout / 4
	if idleCheckInterval < time.Second {
		idleCheckInterval = t.idleTimeout / 2 // For very short timeouts
		if idleCheckInterval < 10*time.Millisecond {
			idleCheckInterval = 10 * time.Millisecond
		}
	}
	if idleCheckInterval > 10*time.Second {
		idleCheckInterval = 10 * time.Second
	}
	idleTicker := time.NewTicker(idleCheckInterval)
	defer idleTicker.Stop()

	var lastIdleWarning time.Time

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.stopCh:
			return

		case <-heartbeatCh:
			t.mu.RLock()
			state := t.state
			callback := t.onHeartbeat
			t.mu.RUnlock()

			// Only emit heartbeat when waiting for API or streaming
			if callback != nil && (state == ActivityWaitingAPI || state == ActivityStreaming) {
				callback()
			}

		case <-idleTicker.C:
			t.mu.RLock()
			state := t.state
			idleDuration := time.Since(t.lastActivity)
			turnDuration := time.Duration(0)
			if !t.turnStart.IsZero() {
				turnDuration = time.Since(t.turnStart)
			}
			idleCallback := t.onIdleWarning
			timeoutCallback := t.onTurnTimeout
			idleTimeout := t.idleTimeout
			turnTimeout := t.turnTimeout
			t.mu.RUnlock()

			// Check for idle warning (no activity for idleTimeout)
			if state == ActivityWaitingAPI || state == ActivityStreaming {
				if idleTimeout > 0 && idleDuration > idleTimeout {
					// Only warn once per idle period
					if time.Since(lastIdleWarning) > idleTimeout {
						if idleCallback != nil {
							idleCallback(idleDuration)
						}
						lastIdleWarning = time.Now()
					}
				}
			}

			// Check for turn timeout
			if state == ActivityWaitingAPI || state == ActivityStreaming {
				if turnTimeout > 0 && turnDuration > turnTimeout {
					if timeoutCallback != nil {
						timeoutCallback()
					}
				}
			}
		}
	}
}
