package events

import (
	"sync"
	"testing"
	"time"
)

func TestNewEvent(t *testing.T) {
	before := time.Now()
	event := NewEvent(EventState, "TASK-001", map[string]string{"status": "running"})
	after := time.Now()

	if event.Type != EventState {
		t.Errorf("expected type %s, got %s", EventState, event.Type)
	}
	if event.TaskID != "TASK-001" {
		t.Errorf("expected task ID TASK-001, got %s", event.TaskID)
	}
	if event.Time.Before(before) || event.Time.After(after) {
		t.Errorf("event time %v not between %v and %v", event.Time, before, after)
	}
}

func TestMemoryPublisher_PublishAndSubscribe(t *testing.T) {
	pub := NewMemoryPublisher()
	defer pub.Close()

	// Subscribe to task
	ch := pub.Subscribe("TASK-001")

	// Publish event
	event := NewEvent(EventState, "TASK-001", "test data")
	pub.Publish(event)

	// Receive event
	select {
	case received := <-ch:
		if received.Type != EventState {
			t.Errorf("expected type %s, got %s", EventState, received.Type)
		}
		if received.TaskID != "TASK-001" {
			t.Errorf("expected task ID TASK-001, got %s", received.TaskID)
		}
		if received.Data != "test data" {
			t.Errorf("expected data 'test data', got %v", received.Data)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for event")
	}
}

func TestMemoryPublisher_MultipleSubscribers(t *testing.T) {
	pub := NewMemoryPublisher()
	defer pub.Close()

	// Multiple subscribers
	ch1 := pub.Subscribe("TASK-001")
	ch2 := pub.Subscribe("TASK-001")

	// Publish event
	event := NewEvent(EventPhase, "TASK-001", "phase data")
	pub.Publish(event)

	// Both should receive
	received := 0
loop:
	for i := 0; i < 2; i++ {
		select {
		case <-ch1:
			received++
		case <-ch2:
			received++
		case <-time.After(100 * time.Millisecond):
			break loop
		}
	}

	if received != 2 {
		t.Errorf("expected 2 receivers, got %d", received)
	}
}

func TestMemoryPublisher_DifferentTasks(t *testing.T) {
	pub := NewMemoryPublisher()
	defer pub.Close()

	ch1 := pub.Subscribe("TASK-001")
	ch2 := pub.Subscribe("TASK-002")

	// Publish to TASK-001 only
	event := NewEvent(EventState, "TASK-001", "data")
	pub.Publish(event)

	// TASK-001 should receive
	select {
	case <-ch1:
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Error("TASK-001 subscriber should have received event")
	}

	// TASK-002 should not receive
	select {
	case <-ch2:
		t.Error("TASK-002 subscriber should not have received event")
	case <-time.After(50 * time.Millisecond):
		// Expected
	}
}

func TestMemoryPublisher_Unsubscribe(t *testing.T) {
	pub := NewMemoryPublisher()
	defer pub.Close()

	ch := pub.Subscribe("TASK-001")

	if pub.SubscriberCount("TASK-001") != 1 {
		t.Errorf("expected 1 subscriber, got %d", pub.SubscriberCount("TASK-001"))
	}

	pub.Unsubscribe("TASK-001", ch)

	if pub.SubscriberCount("TASK-001") != 0 {
		t.Errorf("expected 0 subscribers after unsubscribe, got %d", pub.SubscriberCount("TASK-001"))
	}

	// Channel should be closed
	select {
	case _, ok := <-ch:
		if ok {
			t.Error("channel should be closed")
		}
	default:
		// Channel might be empty but should be closed
	}
}

func TestMemoryPublisher_Close(t *testing.T) {
	pub := NewMemoryPublisher()

	ch1 := pub.Subscribe("TASK-001")
	ch2 := pub.Subscribe("TASK-002")

	pub.Close()

	// Channels should be closed
	for _, ch := range []<-chan Event{ch1, ch2} {
		select {
		case _, ok := <-ch:
			if ok {
				t.Error("channel should be closed after publisher Close()")
			}
		default:
			// Empty but might not be closed yet - wait a bit
		}
	}

	// Publish after close should not panic
	pub.Publish(NewEvent(EventState, "TASK-001", "data"))

	// Subscribe after close should return closed channel
	ch := pub.Subscribe("TASK-003")
	select {
	case _, ok := <-ch:
		if ok {
			t.Error("subscribe after close should return closed channel")
		}
	default:
		// Empty closed channel
	}
}

func TestMemoryPublisher_NonBlockingPublish(t *testing.T) {
	// Small buffer to test non-blocking behavior
	pub := NewMemoryPublisher(WithBufferSize(1))
	defer pub.Close()

	ch := pub.Subscribe("TASK-001")

	// Fill the buffer
	pub.Publish(NewEvent(EventState, "TASK-001", "event1"))

	// This should not block even though buffer is full
	done := make(chan bool)
	go func() {
		pub.Publish(NewEvent(EventState, "TASK-001", "event2"))
		pub.Publish(NewEvent(EventState, "TASK-001", "event3"))
		done <- true
	}()

	select {
	case <-done:
		// Good, didn't block
	case <-time.After(100 * time.Millisecond):
		t.Error("publish should not block when buffer is full")
	}

	// Drain the channel
	<-ch
}

func TestMemoryPublisher_Concurrent(t *testing.T) {
	pub := NewMemoryPublisher()
	defer pub.Close()

	var wg sync.WaitGroup
	taskID := "TASK-001"

	// Concurrent subscribers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ch := pub.Subscribe(taskID)
			// Read some events
			for j := 0; j < 5; j++ {
				select {
				case <-ch:
				case <-time.After(200 * time.Millisecond):
				}
			}
			pub.Unsubscribe(taskID, ch)
		}()
	}

	// Concurrent publishers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				pub.Publish(NewEvent(EventState, taskID, i*10+j))
			}
		}(i)
	}

	wg.Wait()
}

func TestMemoryPublisher_SubscriberCount(t *testing.T) {
	pub := NewMemoryPublisher()
	defer pub.Close()

	if pub.TaskCount() != 0 {
		t.Errorf("expected 0 tasks, got %d", pub.TaskCount())
	}

	ch1 := pub.Subscribe("TASK-001")
	ch2 := pub.Subscribe("TASK-001")
	pub.Subscribe("TASK-002")

	if pub.SubscriberCount("TASK-001") != 2 {
		t.Errorf("expected 2 subscribers for TASK-001, got %d", pub.SubscriberCount("TASK-001"))
	}
	if pub.SubscriberCount("TASK-002") != 1 {
		t.Errorf("expected 1 subscriber for TASK-002, got %d", pub.SubscriberCount("TASK-002"))
	}
	if pub.TaskCount() != 2 {
		t.Errorf("expected 2 tasks, got %d", pub.TaskCount())
	}

	pub.Unsubscribe("TASK-001", ch1)
	pub.Unsubscribe("TASK-001", ch2)

	if pub.TaskCount() != 1 {
		t.Errorf("expected 1 task after unsubscribe, got %d", pub.TaskCount())
	}
}

func TestNopPublisher(t *testing.T) {
	pub := NewNopPublisher()

	// Should not panic
	pub.Publish(NewEvent(EventState, "TASK-001", "data"))

	// Subscribe returns closed channel
	ch := pub.Subscribe("TASK-001")
	select {
	case _, ok := <-ch:
		if ok {
			t.Error("nop publisher subscribe should return closed channel")
		}
	default:
		// Empty closed channel
	}

	// Should not panic
	pub.Unsubscribe("TASK-001", ch)
	pub.Close()
}

func TestTranscriptLine(t *testing.T) {
	line := TranscriptLine{
		Phase:     "implement",
		Iteration: 1,
		Type:      "response",
		Content:   "Hello world",
		Timestamp: time.Now(),
	}

	if line.Phase != "implement" {
		t.Errorf("expected phase implement, got %s", line.Phase)
	}
	if line.Iteration != 1 {
		t.Errorf("expected iteration 1, got %d", line.Iteration)
	}
}

func TestPhaseUpdate(t *testing.T) {
	update := PhaseUpdate{
		Phase:     "test",
		Status:    "completed",
		CommitSHA: "abc123",
	}

	if update.Phase != "test" {
		t.Errorf("expected phase test, got %s", update.Phase)
	}
	if update.Status != "completed" {
		t.Errorf("expected status completed, got %s", update.Status)
	}
}

func TestTokenUpdate(t *testing.T) {
	update := TokenUpdate{
		Phase:        "implement",
		InputTokens:  1000,
		OutputTokens: 500,
		TotalTokens:  1500,
	}

	if update.TotalTokens != 1500 {
		t.Errorf("expected total tokens 1500, got %d", update.TotalTokens)
	}
}

func TestErrorData(t *testing.T) {
	err := ErrorData{
		Phase:   "test",
		Message: "test failed",
		Fatal:   true,
	}

	if !err.Fatal {
		t.Error("expected fatal error")
	}
}

func TestCompleteData(t *testing.T) {
	data := CompleteData{
		Status:    "completed",
		Duration:  "5m30s",
		CommitSHA: "def456",
	}

	if data.Status != "completed" {
		t.Errorf("expected status completed, got %s", data.Status)
	}
}
