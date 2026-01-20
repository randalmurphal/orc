package executor

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/randalmurphal/orc/internal/events"
)

// testEventPublisher captures published events for testing
type testEventPublisher struct {
	mu     sync.Mutex
	events []events.Event
}

func (p *testEventPublisher) Publish(event events.Event) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = append(p.events, event)
}

func (p *testEventPublisher) Subscribe(taskID string) <-chan events.Event {
	return nil
}

func (p *testEventPublisher) Unsubscribe(taskID string, ch <-chan events.Event) {}

func (p *testEventPublisher) Close() {}

func (p *testEventPublisher) getSessionUpdates() []events.SessionUpdate {
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

func TestSessionBroadcaster_OnTaskStart(t *testing.T) {
	pub := &testEventPublisher{}
	ep := NewEventPublisher(pub)
	sb := NewSessionBroadcaster(ep, nil, nil, "/test/project", nil)

	ctx := context.Background()
	sb.OnTaskStart(ctx)
	defer sb.Stop()

	// Wait a bit for the broadcast
	time.Sleep(50 * time.Millisecond)

	updates := pub.getSessionUpdates()
	if len(updates) == 0 {
		t.Fatal("expected at least one session update after task start")
	}

	update := updates[0]
	if update.TasksRunning != 1 {
		t.Errorf("TasksRunning = %d, want 1", update.TasksRunning)
	}
}

func TestSessionBroadcaster_OnTaskComplete(t *testing.T) {
	pub := &testEventPublisher{}
	ep := NewEventPublisher(pub)
	sb := NewSessionBroadcaster(ep, nil, nil, "/test/project", nil)

	ctx := context.Background()

	// Start then complete a task
	sb.OnTaskStart(ctx)
	time.Sleep(50 * time.Millisecond)

	sb.OnTaskComplete(ctx)
	time.Sleep(50 * time.Millisecond)

	updates := pub.getSessionUpdates()
	if len(updates) < 2 {
		t.Fatalf("expected at least 2 session updates, got %d", len(updates))
	}

	// Last update should have TasksRunning = 0
	lastUpdate := updates[len(updates)-1]
	if lastUpdate.TasksRunning != 0 {
		t.Errorf("TasksRunning = %d, want 0 after task complete", lastUpdate.TasksRunning)
	}
}

func TestSessionBroadcaster_OnPauseChanged(t *testing.T) {
	pub := &testEventPublisher{}
	ep := NewEventPublisher(pub)
	sb := NewSessionBroadcaster(ep, nil, nil, "/test/project", nil)

	ctx := context.Background()

	// Start a task first (to have something running)
	sb.OnTaskStart(ctx)
	defer sb.Stop()

	time.Sleep(50 * time.Millisecond)

	// Change pause state
	sb.OnPauseChanged(true)
	time.Sleep(50 * time.Millisecond)

	updates := pub.getSessionUpdates()
	if len(updates) < 2 {
		t.Fatalf("expected at least 2 session updates, got %d", len(updates))
	}

	// Find a pause update
	var foundPaused bool
	for _, u := range updates {
		if u.IsPaused {
			foundPaused = true
			break
		}
	}
	if !foundPaused {
		t.Error("expected to find an update with IsPaused=true")
	}
}

func TestSessionBroadcaster_MultipleTasks(t *testing.T) {
	pub := &testEventPublisher{}
	ep := NewEventPublisher(pub)
	sb := NewSessionBroadcaster(ep, nil, nil, "/test/project", nil)

	ctx := context.Background()

	// Start multiple tasks
	sb.OnTaskStart(ctx)
	sb.OnTaskStart(ctx)
	sb.OnTaskStart(ctx)
	defer sb.Stop()

	time.Sleep(50 * time.Millisecond)

	metrics := sb.GetCurrentMetrics()
	if metrics.TasksRunning != 3 {
		t.Errorf("TasksRunning = %d, want 3", metrics.TasksRunning)
	}

	// Complete one task
	sb.OnTaskComplete(ctx)
	time.Sleep(50 * time.Millisecond)

	metrics = sb.GetCurrentMetrics()
	if metrics.TasksRunning != 2 {
		t.Errorf("TasksRunning = %d, want 2", metrics.TasksRunning)
	}
}

func TestSessionBroadcaster_TickerStopsWhenIdle(t *testing.T) {
	pub := &testEventPublisher{}
	ep := NewEventPublisher(pub)
	sb := NewSessionBroadcaster(ep, nil, nil, "/test/project", nil)

	ctx := context.Background()

	// Start and complete a task
	sb.OnTaskStart(ctx)
	time.Sleep(50 * time.Millisecond)
	sb.OnTaskComplete(ctx)
	time.Sleep(50 * time.Millisecond)

	// Verify ticker stopped by checking running state
	sb.mu.Lock()
	running := sb.running
	sb.mu.Unlock()

	if running {
		t.Error("expected ticker to stop when no tasks are running")
	}
}

func TestSessionBroadcaster_DurationTracking(t *testing.T) {
	pub := &testEventPublisher{}
	ep := NewEventPublisher(pub)
	sb := NewSessionBroadcaster(ep, nil, nil, "/test/project", nil)

	ctx := context.Background()

	// Start a task
	sb.OnTaskStart(ctx)
	defer sb.Stop()

	// Wait a bit
	time.Sleep(100 * time.Millisecond)

	metrics := sb.GetCurrentMetrics()
	if metrics.DurationSeconds < 0 {
		t.Errorf("DurationSeconds = %d, want >= 0", metrics.DurationSeconds)
	}
}

func TestSessionBroadcaster_GetCurrentMetrics(t *testing.T) {
	pub := &testEventPublisher{}
	ep := NewEventPublisher(pub)
	sb := NewSessionBroadcaster(ep, nil, nil, "/test/project", nil)

	ctx := context.Background()

	// Start some tasks
	sb.OnTaskStart(ctx)
	sb.OnTaskStart(ctx)
	defer sb.Stop()

	sb.OnPauseChanged(true)

	metrics := sb.GetCurrentMetrics()

	if metrics.TasksRunning != 2 {
		t.Errorf("TasksRunning = %d, want 2", metrics.TasksRunning)
	}
	if !metrics.IsPaused {
		t.Error("expected IsPaused = true")
	}
}

func TestSessionBroadcaster_Stop(t *testing.T) {
	pub := &testEventPublisher{}
	ep := NewEventPublisher(pub)
	sb := NewSessionBroadcaster(ep, nil, nil, "/test/project", nil)

	ctx := context.Background()

	// Start a task to start the ticker
	sb.OnTaskStart(ctx)
	time.Sleep(50 * time.Millisecond)

	// Stop should be safe to call
	sb.Stop()

	// Verify stopped
	sb.mu.Lock()
	running := sb.running
	sb.mu.Unlock()

	if running {
		t.Error("expected running = false after Stop()")
	}

	// Stop should be safe to call multiple times
	sb.Stop()
}

func TestSessionBroadcaster_ConcurrentAccess(t *testing.T) {
	pub := &testEventPublisher{}
	ep := NewEventPublisher(pub)
	sb := NewSessionBroadcaster(ep, nil, nil, "/test/project", nil)

	ctx := context.Background()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(3)
		go func() {
			defer wg.Done()
			sb.OnTaskStart(ctx)
		}()
		go func() {
			defer wg.Done()
			sb.OnTaskComplete(ctx)
		}()
		go func() {
			defer wg.Done()
			sb.OnPauseChanged(i%2 == 0)
		}()
	}

	wg.Wait()
	sb.Stop()

	// Should not panic or deadlock
}

func TestSessionBroadcaster_NilPublisher(t *testing.T) {
	// Should not panic with nil EventPublisher
	sb := NewSessionBroadcaster(nil, nil, nil, "/test/project", nil)

	ctx := context.Background()
	sb.OnTaskStart(ctx)
	sb.OnTaskComplete(ctx)
	sb.OnPauseChanged(true)
	sb.Stop()

	// Should not panic
}
