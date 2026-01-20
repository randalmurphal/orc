package executor

import (
	"errors"
	"sync"
	"testing"

	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/state"
)

// mockPublisher captures published events for testing.
type mockPublisher struct {
	mu     sync.Mutex
	events []events.Event
}

func newMockPublisher() *mockPublisher {
	return &mockPublisher{events: make([]events.Event, 0)}
}

func (m *mockPublisher) Publish(ev events.Event) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, ev)
}

func (m *mockPublisher) Subscribe(taskID string) <-chan events.Event {
	ch := make(chan events.Event)
	close(ch)
	return ch
}

func (m *mockPublisher) Unsubscribe(taskID string, ch <-chan events.Event) {}

func (m *mockPublisher) Close() {}

func (m *mockPublisher) getEvents() []events.Event {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]events.Event, len(m.events))
	copy(result, m.events)
	return result
}

func (m *mockPublisher) lastEvent() *events.Event {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.events) == 0 {
		return nil
	}
	ev := m.events[len(m.events)-1]
	return &ev
}

func TestNewEventPublisher_NilPublisher_DoesNotPanic(t *testing.T) {
	t.Parallel()

	// Should not panic when creating with nil
	ep := NewEventPublisher(nil)
	if ep == nil {
		t.Fatal("expected non-nil EventPublisher")
	}
}

func TestEventPublisher_Publish_NilPublisher_NoOp(t *testing.T) {
	t.Parallel()

	ep := NewEventPublisher(nil)

	// Should not panic when publishing with nil publisher
	ep.Publish(events.NewEvent(events.EventState, "TASK-001", nil))

	// Also test when the EventPublisher itself is nil
	var nilEP *EventPublisher
	nilEP.Publish(events.NewEvent(events.EventState, "TASK-001", nil))
}

func TestEventPublisher_PhaseStart_PublishesCorrectEvent(t *testing.T) {
	t.Parallel()

	mock := newMockPublisher()
	ep := NewEventPublisher(mock)

	ep.PhaseStart("TASK-001", "implement")

	ev := mock.lastEvent()
	if ev == nil {
		t.Fatal("expected event to be published")
	}

	if ev.Type != events.EventPhase {
		t.Errorf("expected EventPhase, got %v", ev.Type)
	}
	if ev.TaskID != "TASK-001" {
		t.Errorf("expected TaskID TASK-001, got %s", ev.TaskID)
	}

	update, ok := ev.Data.(events.PhaseUpdate)
	if !ok {
		t.Fatalf("expected PhaseUpdate data, got %T", ev.Data)
	}
	if update.Phase != "implement" {
		t.Errorf("expected phase implement, got %s", update.Phase)
	}
	if update.Status != string(plan.PhaseRunning) {
		t.Errorf("expected status running, got %s", update.Status)
	}
}

func TestEventPublisher_PhaseComplete_PublishesCorrectEvent(t *testing.T) {
	t.Parallel()

	mock := newMockPublisher()
	ep := NewEventPublisher(mock)

	ep.PhaseComplete("TASK-002", "test", "abc123")

	ev := mock.lastEvent()
	if ev == nil {
		t.Fatal("expected event to be published")
	}

	update, ok := ev.Data.(events.PhaseUpdate)
	if !ok {
		t.Fatalf("expected PhaseUpdate data, got %T", ev.Data)
	}
	if update.Phase != "test" {
		t.Errorf("expected phase test, got %s", update.Phase)
	}
	if update.Status != string(plan.PhaseCompleted) {
		t.Errorf("expected status completed, got %s", update.Status)
	}
	if update.CommitSHA != "abc123" {
		t.Errorf("expected commit SHA abc123, got %s", update.CommitSHA)
	}
}

func TestEventPublisher_PhaseFailed_IncludesErrorMessage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		err     error
		wantMsg string
	}{
		{
			name:    "with error",
			err:     errors.New("test failed: assertion error"),
			wantMsg: "test failed: assertion error",
		},
		{
			name:    "with nil error",
			err:     nil,
			wantMsg: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mock := newMockPublisher()
			ep := NewEventPublisher(mock)

			ep.PhaseFailed("TASK-003", "validate", tt.err)

			ev := mock.lastEvent()
			if ev == nil {
				t.Fatal("expected event to be published")
			}

			update, ok := ev.Data.(events.PhaseUpdate)
			if !ok {
				t.Fatalf("expected PhaseUpdate data, got %T", ev.Data)
			}
			if update.Status != string(plan.PhaseFailed) {
				t.Errorf("expected status failed, got %s", update.Status)
			}
			if update.Error != tt.wantMsg {
				t.Errorf("expected error %q, got %q", tt.wantMsg, update.Error)
			}
		})
	}
}

func TestEventPublisher_Transcript_AllFieldsSet(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		taskID    string
		phase     string
		iteration int
		msgType   string
		content   string
	}{
		{
			name:      "prompt message",
			taskID:    "TASK-001",
			phase:     "implement",
			iteration: 1,
			msgType:   "prompt",
			content:   "Implement the feature...",
		},
		{
			name:      "response message",
			taskID:    "TASK-002",
			phase:     "test",
			iteration: 3,
			msgType:   "response",
			content:   "I've implemented the tests...",
		},
		{
			name:      "tool message",
			taskID:    "TASK-003",
			phase:     "validate",
			iteration: 2,
			msgType:   "tool",
			content:   `{"tool": "read", "path": "/foo/bar"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mock := newMockPublisher()
			ep := NewEventPublisher(mock)

			ep.Transcript(tt.taskID, tt.phase, tt.iteration, tt.msgType, tt.content)

			ev := mock.lastEvent()
			if ev == nil {
				t.Fatal("expected event to be published")
			}

			if ev.Type != events.EventTranscript {
				t.Errorf("expected EventTranscript, got %v", ev.Type)
			}
			if ev.TaskID != tt.taskID {
				t.Errorf("expected TaskID %s, got %s", tt.taskID, ev.TaskID)
			}

			line, ok := ev.Data.(events.TranscriptLine)
			if !ok {
				t.Fatalf("expected TranscriptLine data, got %T", ev.Data)
			}
			if line.Phase != tt.phase {
				t.Errorf("expected phase %s, got %s", tt.phase, line.Phase)
			}
			if line.Iteration != tt.iteration {
				t.Errorf("expected iteration %d, got %d", tt.iteration, line.Iteration)
			}
			if line.Type != tt.msgType {
				t.Errorf("expected type %s, got %s", tt.msgType, line.Type)
			}
			if line.Content != tt.content {
				t.Errorf("expected content %s, got %s", tt.content, line.Content)
			}
			if line.Timestamp.IsZero() {
				t.Error("expected non-zero timestamp")
			}
		})
	}
}

func TestEventPublisher_TranscriptChunk_SetsChunkType(t *testing.T) {
	t.Parallel()

	mock := newMockPublisher()
	ep := NewEventPublisher(mock)

	ep.TranscriptChunk("TASK-001", "implement", 1, "partial output...")

	ev := mock.lastEvent()
	if ev == nil {
		t.Fatal("expected event to be published")
	}

	line, ok := ev.Data.(events.TranscriptLine)
	if !ok {
		t.Fatalf("expected TranscriptLine data, got %T", ev.Data)
	}
	if line.Type != "chunk" {
		t.Errorf("expected type chunk, got %s", line.Type)
	}
	if line.Content != "partial output..." {
		t.Errorf("expected content 'partial output...', got %s", line.Content)
	}
}

func TestEventPublisher_Tokens_AllFieldsSet(t *testing.T) {
	t.Parallel()

	mock := newMockPublisher()
	ep := NewEventPublisher(mock)

	ep.Tokens("TASK-001", "implement", 1000, 500, 0, 0, 1500)

	ev := mock.lastEvent()
	if ev == nil {
		t.Fatal("expected event to be published")
	}

	if ev.Type != events.EventTokens {
		t.Errorf("expected EventTokens, got %v", ev.Type)
	}

	update, ok := ev.Data.(events.TokenUpdate)
	if !ok {
		t.Fatalf("expected TokenUpdate data, got %T", ev.Data)
	}
	if update.Phase != "implement" {
		t.Errorf("expected phase implement, got %s", update.Phase)
	}
	if update.InputTokens != 1000 {
		t.Errorf("expected input tokens 1000, got %d", update.InputTokens)
	}
	if update.OutputTokens != 500 {
		t.Errorf("expected output tokens 500, got %d", update.OutputTokens)
	}
	if update.TotalTokens != 1500 {
		t.Errorf("expected total tokens 1500, got %d", update.TotalTokens)
	}
}

func TestEventPublisher_Error_FatalFlag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		fatal bool
	}{
		{name: "fatal error", fatal: true},
		{name: "non-fatal error", fatal: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mock := newMockPublisher()
			ep := NewEventPublisher(mock)

			ep.Error("TASK-001", "implement", "something went wrong", tt.fatal)

			ev := mock.lastEvent()
			if ev == nil {
				t.Fatal("expected event to be published")
			}

			if ev.Type != events.EventError {
				t.Errorf("expected EventError, got %v", ev.Type)
			}

			errData, ok := ev.Data.(events.ErrorData)
			if !ok {
				t.Fatalf("expected ErrorData, got %T", ev.Data)
			}
			if errData.Phase != "implement" {
				t.Errorf("expected phase implement, got %s", errData.Phase)
			}
			if errData.Message != "something went wrong" {
				t.Errorf("expected message 'something went wrong', got %s", errData.Message)
			}
			if errData.Fatal != tt.fatal {
				t.Errorf("expected fatal %v, got %v", tt.fatal, errData.Fatal)
			}
		})
	}
}

func TestEventPublisher_State_PublishesState(t *testing.T) {
	t.Parallel()

	mock := newMockPublisher()
	ep := NewEventPublisher(mock)

	s := state.New("TASK-001")
	s.CurrentPhase = "implement"

	ep.State("TASK-001", s)

	ev := mock.lastEvent()
	if ev == nil {
		t.Fatal("expected event to be published")
	}

	if ev.Type != events.EventState {
		t.Errorf("expected EventState, got %v", ev.Type)
	}

	publishedState, ok := ev.Data.(*state.State)
	if !ok {
		t.Fatalf("expected *state.State data, got %T", ev.Data)
	}
	if publishedState.TaskID != "TASK-001" {
		t.Errorf("expected TaskID TASK-001, got %s", publishedState.TaskID)
	}
	if publishedState.CurrentPhase != "implement" {
		t.Errorf("expected phase implement, got %s", publishedState.CurrentPhase)
	}
}

func TestEventPublisher_State_NilState_NoOp(t *testing.T) {
	t.Parallel()

	mock := newMockPublisher()
	ep := NewEventPublisher(mock)

	// Should not panic or publish anything
	ep.State("TASK-001", nil)

	evts := mock.getEvents()
	if len(evts) != 0 {
		t.Errorf("expected no events for nil state, got %d", len(evts))
	}
}

func TestEventPublisher_ConcurrentPublish_Safe(t *testing.T) {
	t.Parallel()

	mock := newMockPublisher()
	ep := NewEventPublisher(mock)

	const numGoroutines = 100
	const numPublishesPerGoroutine = 10

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < numPublishesPerGoroutine; j++ {
				// Mix different publish methods
				switch j % 5 {
				case 0:
					ep.PhaseStart("TASK-001", "implement")
				case 1:
					ep.Transcript("TASK-001", "implement", j, "response", "content")
				case 2:
					ep.TranscriptChunk("TASK-001", "implement", j, "chunk")
				case 3:
					ep.Tokens("TASK-001", "implement", 100, 50, 0, 0, 150)
				case 4:
					ep.Error("TASK-001", "implement", "error", false)
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify we got the expected number of events
	evts := mock.getEvents()
	expectedEvents := numGoroutines * numPublishesPerGoroutine
	if len(evts) != expectedEvents {
		t.Errorf("expected %d events, got %d", expectedEvents, len(evts))
	}
}

func TestEventPublisher_Session_PublishesSessionUpdate(t *testing.T) {
	t.Parallel()

	mock := newMockPublisher()
	ep := NewEventPublisher(mock)

	update := events.SessionUpdate{
		DurationSeconds:  3650,
		TotalTokens:      127500,
		EstimatedCostUSD: 2.51,
		InputTokens:      95000,
		OutputTokens:     32500,
		TasksRunning:     2,
		IsPaused:         false,
	}

	ep.Session(update)

	ev := mock.lastEvent()
	if ev == nil {
		t.Fatal("expected event to be published")
	}

	if ev.Type != events.EventSessionUpdate {
		t.Errorf("expected EventSessionUpdate, got %v", ev.Type)
	}

	// Session events use GlobalTaskID so all subscribers receive them
	if ev.TaskID != events.GlobalTaskID {
		t.Errorf("expected TaskID %q, got %q", events.GlobalTaskID, ev.TaskID)
	}

	sessionData, ok := ev.Data.(events.SessionUpdate)
	if !ok {
		t.Fatalf("expected SessionUpdate data, got %T", ev.Data)
	}

	if sessionData.DurationSeconds != 3650 {
		t.Errorf("expected DurationSeconds 3650, got %d", sessionData.DurationSeconds)
	}
	if sessionData.TotalTokens != 127500 {
		t.Errorf("expected TotalTokens 127500, got %d", sessionData.TotalTokens)
	}
	if sessionData.EstimatedCostUSD != 2.51 {
		t.Errorf("expected EstimatedCostUSD 2.51, got %f", sessionData.EstimatedCostUSD)
	}
	if sessionData.InputTokens != 95000 {
		t.Errorf("expected InputTokens 95000, got %d", sessionData.InputTokens)
	}
	if sessionData.OutputTokens != 32500 {
		t.Errorf("expected OutputTokens 32500, got %d", sessionData.OutputTokens)
	}
	if sessionData.TasksRunning != 2 {
		t.Errorf("expected TasksRunning 2, got %d", sessionData.TasksRunning)
	}
	if sessionData.IsPaused {
		t.Error("expected IsPaused false")
	}
}

func TestEventPublisher_Session_NilPublisher_NoOp(t *testing.T) {
	t.Parallel()

	ep := NewEventPublisher(nil)

	// Should not panic when publishing with nil publisher
	ep.Session(events.SessionUpdate{
		TasksRunning: 1,
	})
}
