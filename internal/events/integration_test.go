package events

import (
	"testing"
	"time"

	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// TestEventPersistence_RoundTrip is an integration test that verifies
// events can be published and then queried back from the database.
func TestEventPersistence_RoundTrip(t *testing.T) {
	// Create in-memory backend
	backend, err := storage.NewInMemoryBackend()
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}
	defer backend.Close()

	// Create task to satisfy foreign key constraint
	taskID := "TASK-001"
	testTask := &task.Task{
		ID:    taskID,
		Title: "Test Task",
	}
	if err := backend.SaveTask(testTask); err != nil {
		t.Fatalf("failed to save test task: %v", err)
	}

	// Create publisher
	pub := NewPersistentPublisher(backend, "executor", nil)
	defer pub.Close()

	// Publish a variety of event types

	// Phase events
	pub.Publish(NewEvent(EventPhase, taskID, PhaseUpdate{
		Phase:  "spec",
		Status: "started",
	}))

	time.Sleep(50 * time.Millisecond) // Simulate phase duration

	pub.Publish(NewEvent(EventPhase, taskID, PhaseUpdate{
		Phase:     "spec",
		Status:    "completed",
		CommitSHA: "abc123",
	}))

	// Transcript events
	pub.Publish(NewEvent(EventTranscript, taskID, TranscriptLine{
		Phase:     "spec",
		Iteration: 1,
		Type:      "prompt",
		Content:   "Write a spec for...",
		Timestamp: time.Now(),
	}))

	pub.Publish(NewEvent(EventTranscript, taskID, TranscriptLine{
		Phase:     "spec",
		Iteration: 1,
		Type:      "response",
		Content:   "# Specification\n\n...",
		Timestamp: time.Now(),
	}))

	// Activity events
	pub.Publish(NewEvent(EventActivity, taskID, ActivityUpdate{
		Phase:    "spec",
		Activity: "waiting_api",
	}))

	pub.Publish(NewEvent(EventActivity, taskID, ActivityUpdate{
		Phase:    "spec",
		Activity: "streaming",
	}))

	// Token events
	pub.Publish(NewEvent(EventTokens, taskID, TokenUpdate{
		Phase:        "spec",
		InputTokens:  1000,
		OutputTokens: 500,
		TotalTokens:  1500,
	}))

	// Error event
	pub.Publish(NewEvent(EventError, taskID, ErrorData{
		Phase:   "spec",
		Message: "Test error",
		Fatal:   false,
	}))

	// Warning event
	pub.Publish(NewEvent(EventWarning, taskID, WarningData{
		Phase:   "spec",
		Message: "Test warning",
	}))

	// Flush to ensure all events are persisted
	pub.flush()
	time.Sleep(100 * time.Millisecond)

	// Query all events for the task
	results, err := backend.QueryEvents(db.QueryEventsOptions{
		TaskID: taskID,
		Limit:  50,
	})
	if err != nil {
		t.Fatalf("failed to query events: %v", err)
	}

	// Verify all events were saved
	expectedCount := 9
	if len(results) != expectedCount {
		t.Errorf("expected %d events, got %d", expectedCount, len(results))
	}

	// Verify event types are preserved
	eventTypes := make(map[string]int)
	for _, result := range results {
		eventTypes[result.EventType]++
	}

	expectedTypes := map[string]int{
		string(EventPhase):      2,
		string(EventTranscript): 2,
		string(EventActivity):   2,
		string(EventTokens):     1,
		string(EventError):      1,
		string(EventWarning):    1,
	}

	for eventType, expectedCount := range expectedTypes {
		if eventTypes[eventType] != expectedCount {
			t.Errorf("expected %d %s events, got %d", expectedCount, eventType, eventTypes[eventType])
		}
	}

	// Verify source is set correctly
	for _, result := range results {
		if result.Source != "executor" {
			t.Errorf("expected source 'executor', got %s", result.Source)
		}
	}

	// Verify phase information is extracted correctly
	phaseCount := 0
	for _, result := range results {
		if result.Phase != nil && *result.Phase == "spec" {
			phaseCount++
		}
	}
	if phaseCount < 8 { // Most events should have phase info
		t.Errorf("expected at least 8 events with phase 'spec', got %d", phaseCount)
	}

	// Verify iteration is captured for transcript events
	iterationCount := 0
	for _, result := range results {
		if result.Iteration != nil && *result.Iteration == 1 {
			iterationCount++
		}
	}
	if iterationCount != 2 { // Two transcript events
		t.Errorf("expected 2 events with iteration 1, got %d", iterationCount)
	}

	// Query events by type filter
	phaseResults, err := backend.QueryEvents(db.QueryEventsOptions{
		TaskID:     taskID,
		EventTypes: []string{string(EventPhase)},
		Limit:      10,
	})
	if err != nil {
		t.Fatalf("failed to query phase events: %v", err)
	}

	if len(phaseResults) != 2 {
		t.Errorf("expected 2 phase events, got %d", len(phaseResults))
	}

	// Verify duration was calculated for completed phase
	var completionEvent *db.EventLog
	for i := range phaseResults {
		if dataMap, ok := phaseResults[i].Data.(map[string]any); ok {
			if status, ok := dataMap["status"].(string); ok && status == "completed" {
				completionEvent = &phaseResults[i]
				break
			}
		}
	}

	if completionEvent == nil {
		t.Error("completion event not found")
	} else if completionEvent.DurationMs == nil {
		t.Error("expected duration_ms to be set on completion event")
	} else if *completionEvent.DurationMs < 40 || *completionEvent.DurationMs > 100 {
		t.Errorf("expected duration around 50ms, got %dms", *completionEvent.DurationMs)
	}

	// Query events by time range
	now := time.Now()
	since := now.Add(-1 * time.Hour)
	until := now.Add(1 * time.Hour)

	timeResults, err := backend.QueryEvents(db.QueryEventsOptions{
		TaskID: taskID,
		Since:  &since,
		Until:  &until,
		Limit:  50,
	})
	if err != nil {
		t.Fatalf("failed to query events by time: %v", err)
	}

	if len(timeResults) != expectedCount {
		t.Errorf("expected %d events in time range, got %d", expectedCount, len(timeResults))
	}

	// Verify events are ordered by created_at descending
	for i := 1; i < len(results); i++ {
		if results[i].CreatedAt.After(results[i-1].CreatedAt) {
			t.Error("events not ordered by created_at descending")
			break
		}
	}
}
