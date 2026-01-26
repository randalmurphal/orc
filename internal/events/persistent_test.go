package events

import (
	"log/slog"
	"testing"
	"time"

	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// setupTestBackend creates a backend with a test task to satisfy foreign key constraints.
func setupTestBackend(t *testing.T, taskID string) storage.Backend {
	t.Helper()
	backend, err := storage.NewInMemoryBackend()
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	// Create task to satisfy foreign key constraint
	testTask := task.NewProtoTask(taskID, "Test Task")
	if err := backend.SaveTask(testTask); err != nil {
		t.Fatalf("failed to save test task: %v", err)
	}

	return backend
}

// TestPersistentPublisher_PersistsEvents verifies events are written to the database.
func TestPersistentPublisher_PersistsEvents(t *testing.T) {
	backend := setupTestBackend(t, "TASK-001")
	defer func() { _ = backend.Close() }()

	logger := slog.Default()
	pub := NewPersistentPublisher(backend, "test", logger)
	defer pub.Close()

	// Publish several events
	events := []Event{
		NewEvent(EventPhase, "TASK-001", PhaseUpdate{Phase: "spec", Status: "started"}),
		NewEvent(EventTranscript, "TASK-001", TranscriptLine{
			Phase:     "spec",
			Iteration: 1,
			Type:      "prompt",
			Content:   "Test transcript",
			Timestamp: time.Now(),
		}),
		NewEvent(EventActivity, "TASK-001", ActivityUpdate{Phase: "spec", Activity: "waiting_api"}),
	}

	for _, e := range events {
		pub.Publish(e)
	}

	// Flush to ensure writes complete
	pub.flush()

	// Query events from database
	results, err := backend.QueryEvents(db.QueryEventsOptions{
		TaskID: "TASK-001",
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("failed to query events: %v", err)
	}

	if len(results) != len(events) {
		t.Errorf("expected %d events, got %d", len(events), len(results))
	}

	// Verify first event is a phase event
	found := false
	for _, result := range results {
		if result.EventType == string(EventPhase) {
			found = true
			if result.Phase == nil || *result.Phase != "spec" {
				t.Errorf("expected phase 'spec', got %v", result.Phase)
			}
			if result.Source != "test" {
				t.Errorf("expected source 'test', got %s", result.Source)
			}
		}
	}
	if !found {
		t.Error("phase event not found in results")
	}
}

// TestPersistentPublisher_WebSocketBroadcast verifies wrapped MemoryPublisher still broadcasts.
func TestPersistentPublisher_WebSocketBroadcast(t *testing.T) {
	backend := setupTestBackend(t, "TASK-001")
	defer func() { _ = backend.Close() }()

	logger := slog.Default()
	pub := NewPersistentPublisher(backend, "test", logger)
	defer pub.Close()

	// Subscribe to events
	ch := pub.Subscribe("TASK-001")

	// Publish event
	event := NewEvent(EventPhase, "TASK-001", PhaseUpdate{Phase: "spec", Status: "started"})
	pub.Publish(event)

	// Verify broadcast received
	select {
	case received := <-ch:
		if received.Type != EventPhase {
			t.Errorf("expected EventPhase, got %s", received.Type)
		}
		if received.TaskID != "TASK-001" {
			t.Errorf("expected TASK-001, got %s", received.TaskID)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for broadcast event")
	}
}

// TestPersistentPublisher_BatchFlush verifies buffer flushes at threshold.
func TestPersistentPublisher_BatchFlush(t *testing.T) {
	backend := setupTestBackend(t, "TASK-001")
	defer func() { _ = backend.Close() }()

	logger := slog.Default()
	pub := NewPersistentPublisher(backend, "test", logger)
	defer pub.Close()

	// Publish exactly bufferSizeThreshold events
	for i := 0; i < bufferSizeThreshold; i++ {
		pub.Publish(NewEvent(EventHeartbeat, "TASK-001", HeartbeatData{
			Phase:     "spec",
			Iteration: i,
			Timestamp: time.Now(),
		}))
	}

	// Buffer should auto-flush at threshold - give it a moment
	time.Sleep(50 * time.Millisecond)

	// Query events from database
	results, err := backend.QueryEvents(db.QueryEventsOptions{
		TaskID: "TASK-001",
		Limit:  20,
	})
	if err != nil {
		t.Fatalf("failed to query events: %v", err)
	}

	if len(results) != bufferSizeThreshold {
		t.Errorf("expected %d events after batch flush, got %d", bufferSizeThreshold, len(results))
	}
}

// TestPersistentPublisher_TimeFlush verifies buffer flushes after 5 seconds.
func TestPersistentPublisher_TimeFlush(t *testing.T) {
	backend := setupTestBackend(t, "TASK-001")
	defer func() { _ = backend.Close() }()

	logger := slog.Default()
	pub := NewPersistentPublisher(backend, "test", logger)
	defer pub.Close()

	// Publish one event (below batch threshold)
	pub.Publish(NewEvent(EventActivity, "TASK-001", ActivityUpdate{
		Phase:    "spec",
		Activity: "idle",
	}))

	// Wait for timer flush (5 seconds + buffer)
	time.Sleep(6 * time.Second)

	// Query events from database
	results, err := backend.QueryEvents(db.QueryEventsOptions{
		TaskID: "TASK-001",
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("failed to query events: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("expected 1 event after time flush, got %d", len(results))
	}
}

// TestPersistentPublisher_PhaseCompletionFlush verifies flush on phase complete event.
func TestPersistentPublisher_PhaseCompletionFlush(t *testing.T) {
	backend := setupTestBackend(t, "TASK-001")
	defer func() { _ = backend.Close() }()

	logger := slog.Default()
	pub := NewPersistentPublisher(backend, "test", logger)
	defer pub.Close()

	// Publish phase start
	pub.Publish(NewEvent(EventPhase, "TASK-001", PhaseUpdate{Phase: "spec", Status: "started"}))

	// Publish a few other events
	pub.Publish(NewEvent(EventActivity, "TASK-001", ActivityUpdate{Phase: "spec", Activity: "waiting_api"}))
	pub.Publish(NewEvent(EventActivity, "TASK-001", ActivityUpdate{Phase: "spec", Activity: "streaming"}))

	// Publish phase completion - should trigger flush
	pub.Publish(NewEvent(EventPhase, "TASK-001", PhaseUpdate{Phase: "spec", Status: "completed"}))

	// Give flush a moment
	time.Sleep(50 * time.Millisecond)

	// Query events from database
	results, err := backend.QueryEvents(db.QueryEventsOptions{
		TaskID: "TASK-001",
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("failed to query events: %v", err)
	}

	if len(results) != 4 {
		t.Errorf("expected 4 events after phase completion flush, got %d", len(results))
	}
}

// TestPersistentPublisher_DurationCalculation verifies duration_ms calculated correctly.
func TestPersistentPublisher_DurationCalculation(t *testing.T) {
	backend := setupTestBackend(t, "TASK-001")
	defer func() { _ = backend.Close() }()

	logger := slog.Default()
	pub := NewPersistentPublisher(backend, "test", logger)
	defer pub.Close()

	// Publish phase start
	startTime := time.Now()
	pub.Publish(Event{
		Type:   EventPhase,
		TaskID: "TASK-001",
		Data:   PhaseUpdate{Phase: "spec", Status: "started"},
		Time:   startTime,
	})

	// Wait a bit
	time.Sleep(100 * time.Millisecond)

	// Publish phase completion
	endTime := startTime.Add(100 * time.Millisecond)
	pub.Publish(Event{
		Type:   EventPhase,
		TaskID: "TASK-001",
		Data:   PhaseUpdate{Phase: "spec", Status: "completed"},
		Time:   endTime,
	})

	// Flush to ensure writes complete
	pub.flush()

	// Query events from database
	results, err := backend.QueryEvents(db.QueryEventsOptions{
		TaskID: "TASK-001",
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("failed to query events: %v", err)
	}

	// Find the completion event
	var completionEvent *db.EventLog
	for i := range results {
		if results[i].EventType == string(EventPhase) {
			// Parse the data to check status
			if dataMap, ok := results[i].Data.(map[string]any); ok {
				if status, ok := dataMap["status"].(string); ok && status == "completed" {
					completionEvent = &results[i]
					break
				}
			}
		}
	}

	if completionEvent == nil {
		t.Fatal("completion event not found")
	}

	if completionEvent.DurationMs == nil {
		t.Error("expected duration_ms to be set on completion event")
	} else {
		// Duration should be around 100ms (allow some variance)
		if *completionEvent.DurationMs < 90 || *completionEvent.DurationMs > 150 {
			t.Errorf("expected duration around 100ms, got %dms", *completionEvent.DurationMs)
		}
	}
}

// TestPersistentPublisher_DBFailureDoesNotCrash verifies graceful error handling.
func TestPersistentPublisher_DBFailureDoesNotCrash(t *testing.T) {
	// Create publisher with nil backend (simulates DB failure)
	logger := slog.Default()
	pub := NewPersistentPublisher(nil, "test", logger)
	defer pub.Close()

	// This should not panic
	pub.Publish(NewEvent(EventPhase, "TASK-001", PhaseUpdate{Phase: "spec", Status: "started"}))

	// Subscribe should still work (MemoryPublisher functionality)
	ch := pub.Subscribe("TASK-001")
	pub.Publish(NewEvent(EventPhase, "TASK-001", PhaseUpdate{Phase: "spec", Status: "completed"}))

	// Verify broadcast still works
	select {
	case received := <-ch:
		if received.Type != EventPhase {
			t.Errorf("expected EventPhase, got %s", received.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for broadcast event")
	}
}

// TestPersistentPublisher_CloseIsIdempotent verifies Close() can be called multiple times.
// This was causing "close of closed channel" panics when CLIPublisher wrapped
// PersistentPublisher and both had deferred Close() calls.
func TestPersistentPublisher_CloseIsIdempotent(t *testing.T) {
	logger := slog.Default()
	pub := NewPersistentPublisher(nil, "test", logger)

	// First close should succeed
	pub.Close()

	// Second close should not panic
	pub.Close()

	// Third close should also be fine
	pub.Close()
}
