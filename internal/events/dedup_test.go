package events

import (
	"log/slog"
	"testing"
	"time"

	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/task"
)

// TestPersistentPublisher_PreventsDuplicateWrites verifies that publishing
// the same event multiple times doesn't create duplicate database entries.
func TestPersistentPublisher_PreventsDuplicateWrites(t *testing.T) {
	backend := setupTestBackend(t, "TASK-001")
	defer func() { _ = backend.Close() }()

	logger := slog.Default()
	pub := NewPersistentPublisher(backend, "test", logger)
	defer pub.Close()

	// Create event with fixed timestamp for exact duplicate detection
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	event := Event{
		Type:   EventPhase,
		TaskID: "TASK-001",
		Time:   fixedTime,
		Data: PhaseUpdate{
			Phase:  "implement",
			Status: "running",
		},
	}

	// Publish the same event twice
	pub.Publish(event)
	pub.Publish(event)

	// Force flush to ensure both publishes complete
	pub.flush()

	// Query events - should only have 1 (duplicate prevented)
	results, err := backend.QueryEvents(db.QueryEventsOptions{
		TaskID: "TASK-001",
	})
	if err != nil {
		t.Fatalf("failed to query events: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("expected 1 event (duplicate prevented), got %d", len(results))
	}
}

// TestPersistentPublisher_BatchDeduplication verifies that events in the same
// flush batch are deduplicated.
func TestPersistentPublisher_BatchDeduplication(t *testing.T) {
	backend := setupTestBackend(t, "TASK-002")
	defer func() { _ = backend.Close() }()

	// Create a second task for test isolation
	testTask := task.NewProtoTask("TASK-002", "Test Task 2")
	if err := backend.SaveTask(testTask); err != nil {
		t.Fatalf("failed to save test task: %v", err)
	}

	logger := slog.Default()
	pub := NewPersistentPublisher(backend, "test", logger)
	defer pub.Close()

	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	// Rapidly publish many identical events (simulates race condition)
	for i := 0; i < 5; i++ {
		event := Event{
			Type:   EventPhase,
			TaskID: "TASK-002",
			Time:   fixedTime,
			Data: PhaseUpdate{
				Phase:  "implement",
				Status: "running",
			},
		}
		pub.Publish(event)
	}

	// Flush all
	pub.flush()

	// Should only have 1 event
	results, err := backend.QueryEvents(db.QueryEventsOptions{
		TaskID: "TASK-002",
	})
	if err != nil {
		t.Fatalf("failed to query events: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("expected 1 event after dedup, got %d", len(results))
	}
}
