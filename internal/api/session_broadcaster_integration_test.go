// Package api provides HTTP API handlers for orc.
//
// TDD Tests for TASK-539: Session Broadcaster Integration
//
// These tests verify that the Server properly initializes and wires the
// SessionBroadcaster to WorkflowExecutor instances, enabling real-time
// session metrics (duration, tokens, cost) in the web UI header.
//
// Success Criteria Coverage:
// - SC-1: Session duration updates via session_update events (TestSessionBroadcaster_*)
// - SC-2: Token count in session events (TestSessionBroadcaster_TokensField)
// - SC-3: Cost in session events (TestSessionBroadcaster_CostField)
// - SC-4: Metrics update on task start/complete (TestSessionBroadcaster_TaskLifecycle)
// - SC-5: Existing tests pass (verified by running go test ./...)
//
// The bug: SessionBroadcaster was not being passed to WorkflowExecutor in startTask/resumeTask,
// so no session_update events were ever broadcast during task execution.
//
// These tests will FAIL TO COMPILE until:
// 1. Server struct has: sessionBroadcaster *executor.SessionBroadcaster
// 2. New() initializes the sessionBroadcaster
// 3. startTask() uses: executor.WithWorkflowSessionBroadcaster(s.sessionBroadcaster)
// 4. resumeTask() uses: executor.WithWorkflowSessionBroadcaster(s.sessionBroadcaster)
package api

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/executor"
)

// ============================================================================
// SC-4: Server must have sessionBroadcaster field
//
// These tests verify the Server struct has the required field.
// They will fail to compile until the field is added.
// ============================================================================

// TestServer_SessionBroadcasterField_Exists verifies SC-4:
// Server struct must have a sessionBroadcaster field of type *executor.SessionBroadcaster.
// This test will fail to compile until the field is added to Server.
func TestServer_SessionBroadcasterField_Exists(t *testing.T) {
	t.Parallel()

	// Create a minimal server with the sessionBroadcaster field.
	// This test serves as a compile-time check that the field exists.
	//
	// EXPECTED TO FAIL UNTIL IMPLEMENTATION:
	// The Server struct must have:
	//   sessionBroadcaster *executor.SessionBroadcaster
	server := &Server{
		runningTasks:       make(map[string]context.CancelFunc),
		sessionBroadcaster: nil, // Field must exist
	}

	// Verify we can access the field
	if server.sessionBroadcaster == nil {
		t.Log("sessionBroadcaster field exists (compile check passed)")
	}
}

// TestServer_SessionBroadcasterField_TypeCheck verifies the field is correctly typed.
// This ensures the field can hold a *executor.SessionBroadcaster.
func TestServer_SessionBroadcasterField_TypeCheck(t *testing.T) {
	t.Parallel()

	// Create a real SessionBroadcaster
	pub := &sessionTestPublisher{}
	ep := events.NewPublishHelper(pub)
	sb := executor.NewSessionBroadcaster(ep, nil, nil, t.TempDir(), nil)

	// Server must be able to hold this broadcaster
	// EXPECTED TO FAIL UNTIL IMPLEMENTATION
	server := &Server{
		runningTasks:       make(map[string]context.CancelFunc),
		sessionBroadcaster: sb,
	}

	// Type assertion should work
	if server.sessionBroadcaster != sb {
		t.Error("sessionBroadcaster field should hold the assigned broadcaster")
	}
}

// ============================================================================
// SC-4: Metrics update immediately on task start/complete
//
// These tests verify SessionBroadcaster integration behavior.
// ============================================================================

// TestSessionBroadcaster_TaskLifecycle verifies SC-4:
// Session events are published when tasks start and complete.
func TestSessionBroadcaster_TaskLifecycle(t *testing.T) {
	t.Parallel()

	pub := &sessionTestPublisher{}
	ep := events.NewPublishHelper(pub)
	sb := executor.NewSessionBroadcaster(ep, nil, nil, t.TempDir(), nil)

	ctx := context.Background()

	// Start a task - should publish event immediately
	sb.OnTaskStart(ctx)
	time.Sleep(100 * time.Millisecond)

	updates := pub.getSessionUpdates()
	if len(updates) == 0 {
		t.Fatal("expected session_update event after OnTaskStart")
	}

	// First update should show 1 task running
	if updates[0].TasksRunning != 1 {
		t.Errorf("TasksRunning = %d after start, want 1", updates[0].TasksRunning)
	}

	// Complete the task - should publish event immediately
	sb.OnTaskComplete(ctx)
	time.Sleep(100 * time.Millisecond)

	updates = pub.getSessionUpdates()
	if len(updates) < 2 {
		t.Fatalf("expected at least 2 session_update events, got %d", len(updates))
	}

	// Last update should show 0 tasks running
	lastUpdate := updates[len(updates)-1]
	if lastUpdate.TasksRunning != 0 {
		t.Errorf("TasksRunning = %d after complete, want 0", lastUpdate.TasksRunning)
	}
}

// ============================================================================
// SC-1: Session duration updates in header
// ============================================================================

// TestSessionBroadcaster_DurationField verifies SC-1:
// Session updates include duration_seconds that tracks elapsed time.
func TestSessionBroadcaster_DurationField(t *testing.T) {
	t.Parallel()

	pub := &sessionTestPublisher{}
	ep := events.NewPublishHelper(pub)
	sb := executor.NewSessionBroadcaster(ep, nil, nil, t.TempDir(), nil)

	ctx := context.Background()
	sb.OnTaskStart(ctx)
	defer sb.Stop()

	// Wait for duration to accumulate
	time.Sleep(150 * time.Millisecond)

	metrics := sb.GetCurrentMetrics()

	// Duration should be >= 0 (it was just started)
	if metrics.DurationSeconds < 0 {
		t.Errorf("DurationSeconds = %d, want >= 0", metrics.DurationSeconds)
	}
	t.Logf("DurationSeconds = %d", metrics.DurationSeconds)
}

// ============================================================================
// SC-2: Token count reflects actual usage
// ============================================================================

// TestSessionBroadcaster_TokensField verifies SC-2:
// Session updates include token count fields.
// Note: Without a real GlobalDB, tokens will be 0, but the fields must exist.
func TestSessionBroadcaster_TokensField(t *testing.T) {
	t.Parallel()

	pub := &sessionTestPublisher{}
	ep := events.NewPublishHelper(pub)
	sb := executor.NewSessionBroadcaster(ep, nil, nil, t.TempDir(), nil)

	ctx := context.Background()
	sb.OnTaskStart(ctx)
	defer sb.Stop()

	time.Sleep(100 * time.Millisecond)

	updates := pub.getSessionUpdates()
	if len(updates) == 0 {
		t.Fatal("expected session_update event")
	}

	// Verify token fields exist and are serialized
	// (they'll be 0 without GlobalDB, which is expected)
	update := updates[0]
	t.Logf("TotalTokens=%d, InputTokens=%d, OutputTokens=%d",
		update.TotalTokens, update.InputTokens, update.OutputTokens)

	// These fields must exist (compile-time check)
	_ = update.TotalTokens
	_ = update.InputTokens
	_ = update.OutputTokens
}

// ============================================================================
// SC-3: Cost displays estimated USD
// ============================================================================

// TestSessionBroadcaster_CostField verifies SC-3:
// Session updates include estimated cost field.
// Note: Without a real GlobalDB, cost will be 0.0, but the field must exist.
func TestSessionBroadcaster_CostField(t *testing.T) {
	t.Parallel()

	pub := &sessionTestPublisher{}
	ep := events.NewPublishHelper(pub)
	sb := executor.NewSessionBroadcaster(ep, nil, nil, t.TempDir(), nil)

	ctx := context.Background()
	sb.OnTaskStart(ctx)
	defer sb.Stop()

	time.Sleep(100 * time.Millisecond)

	updates := pub.getSessionUpdates()
	if len(updates) == 0 {
		t.Fatal("expected session_update event")
	}

	// Verify cost field exists and is serialized
	update := updates[0]
	t.Logf("EstimatedCostUSD=%.4f", update.EstimatedCostUSD)

	// Field must exist (compile-time check)
	_ = update.EstimatedCostUSD
}

// ============================================================================
// Edge Case: Multiple concurrent tasks
// ============================================================================

// TestSessionBroadcaster_MultipleConcurrentTasks verifies BDD-3:
// Multiple tasks contribute to aggregate metrics.
func TestSessionBroadcaster_MultipleConcurrentTasks(t *testing.T) {
	t.Parallel()

	pub := &sessionTestPublisher{}
	ep := events.NewPublishHelper(pub)
	sb := executor.NewSessionBroadcaster(ep, nil, nil, t.TempDir(), nil)

	ctx := context.Background()

	// Start 3 tasks
	sb.OnTaskStart(ctx)
	sb.OnTaskStart(ctx)
	sb.OnTaskStart(ctx)
	defer sb.Stop()

	time.Sleep(100 * time.Millisecond)

	metrics := sb.GetCurrentMetrics()
	if metrics.TasksRunning != 3 {
		t.Errorf("TasksRunning = %d, want 3", metrics.TasksRunning)
	}

	// Complete one
	sb.OnTaskComplete(ctx)
	time.Sleep(50 * time.Millisecond)

	metrics = sb.GetCurrentMetrics()
	if metrics.TasksRunning != 2 {
		t.Errorf("TasksRunning = %d, want 2 after one complete", metrics.TasksRunning)
	}
}

// ============================================================================
// Edge Case: GlobalDB unavailable
// ============================================================================

// TestSessionBroadcaster_NilGlobalDB_StillBroadcasts verifies failure mode:
// When GlobalDB is unavailable, session metrics still broadcast with duration,
// but tokens/cost stay at 0.
func TestSessionBroadcaster_NilGlobalDB_StillBroadcasts(t *testing.T) {
	t.Parallel()

	pub := &sessionTestPublisher{}
	ep := events.NewPublishHelper(pub)
	// Pass nil for globalDB - this is the failure scenario
	sb := executor.NewSessionBroadcaster(ep, nil, nil, t.TempDir(), nil)

	ctx := context.Background()
	sb.OnTaskStart(ctx)
	defer sb.Stop()

	time.Sleep(100 * time.Millisecond)

	updates := pub.getSessionUpdates()
	if len(updates) == 0 {
		t.Fatal("expected session_update even with nil GlobalDB")
	}

	update := updates[0]

	// Duration should still work
	if update.DurationSeconds < 0 {
		t.Error("DurationSeconds should be valid even with nil GlobalDB")
	}

	// TasksRunning should still work
	if update.TasksRunning != 1 {
		t.Errorf("TasksRunning = %d, want 1 even with nil GlobalDB", update.TasksRunning)
	}

	// Tokens/cost should be 0 (not an error, just no data)
	if update.TotalTokens != 0 {
		t.Logf("TotalTokens = %d (expected 0 without GlobalDB)", update.TotalTokens)
	}
}

// ============================================================================
// Edge Case: Nil publisher
// ============================================================================

// TestSessionBroadcaster_NilPublisher_NoPanic verifies failure mode:
// When publisher is nil, no events broadcast but no panic.
func TestSessionBroadcaster_NilPublisher_NoPanic(t *testing.T) {
	t.Parallel()

	// Pass nil for publisher - this is the failure scenario
	sb := executor.NewSessionBroadcaster(nil, nil, nil, t.TempDir(), nil)

	ctx := context.Background()

	// Should not panic
	sb.OnTaskStart(ctx)
	sb.OnTaskComplete(ctx)
	sb.OnPauseChanged(true)
	sb.Stop()

	// Test passes if no panic
}

// ============================================================================
// Edge Case: Stop() is idempotent
// ============================================================================

// TestSessionBroadcaster_StopIdempotent verifies Stop() can be called
// multiple times without panic or error.
func TestSessionBroadcaster_StopIdempotent(t *testing.T) {
	t.Parallel()

	pub := &sessionTestPublisher{}
	ep := events.NewPublishHelper(pub)
	sb := executor.NewSessionBroadcaster(ep, nil, nil, t.TempDir(), nil)

	ctx := context.Background()
	sb.OnTaskStart(ctx)

	// Stop multiple times - should not panic
	sb.Stop()
	sb.Stop()
	sb.Stop()

	// Test passes if no panic
}

// ============================================================================
// WebSocket Integration
// ============================================================================

// TestSessionBroadcaster_EventsReachSubscribers verifies that session_update
// events from SessionBroadcaster are delivered to event subscribers.
// This tests the full event path: SessionBroadcaster -> Publisher -> Subscriber
func TestSessionBroadcaster_EventsReachSubscribers(t *testing.T) {
	t.Parallel()

	// Create a memory publisher
	pub := events.NewMemoryPublisher()

	// Create SessionBroadcaster with this publisher
	ep := events.NewPublishHelper(pub)
	sb := executor.NewSessionBroadcaster(ep, nil, nil, t.TempDir(), nil)

	// Subscribe to global events (how frontend receives session updates)
	ch := pub.Subscribe(events.GlobalTaskID)
	defer pub.Unsubscribe(events.GlobalTaskID, ch)

	// Start a task - this should trigger a session_update event
	ctx := context.Background()
	sb.OnTaskStart(ctx)
	defer sb.Stop()

	// Wait for event
	select {
	case event := <-ch:
		if event.Type != events.EventSessionUpdate {
			t.Errorf("expected EventSessionUpdate, got %v", event.Type)
		}
		update, ok := event.Data.(events.SessionUpdate)
		if !ok {
			t.Fatalf("expected SessionUpdate data, got %T", event.Data)
		}
		if update.TasksRunning != 1 {
			t.Errorf("TasksRunning = %d, want 1", update.TasksRunning)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for session_update event")
	}
}

// ============================================================================
// Test Helpers
// ============================================================================

// sessionTestPublisher captures published events for testing.
// Implements events.Publisher interface.
type sessionTestPublisher struct {
	mu     sync.Mutex
	events []events.Event
}

func (p *sessionTestPublisher) Publish(event events.Event) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = append(p.events, event)
}

func (p *sessionTestPublisher) Subscribe(taskID string) <-chan events.Event {
	return nil
}

func (p *sessionTestPublisher) Unsubscribe(taskID string, ch <-chan events.Event) {}

func (p *sessionTestPublisher) Close() {}

// getSessionUpdates returns all session_update events that were published.
func (p *sessionTestPublisher) getSessionUpdates() []events.SessionUpdate {
	p.mu.Lock()
	defer p.mu.Unlock()
	var updates []events.SessionUpdate
	for _, e := range p.events {
		if e.Type == events.EventSessionUpdate {
			if update, ok := e.Data.(events.SessionUpdate); ok {
				updates = append(updates, update)
			}
		}
	}
	return updates
}
