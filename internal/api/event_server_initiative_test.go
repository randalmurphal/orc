package api

import (
	"testing"
	"time"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/events"
)

// =============================================================================
// Tests for initiative filtering in Subscribe - TASK-545
// =============================================================================

// mockBackendWithTasks implements storage.Backend for testing initiative filtering.
// It returns tasks with specific initiative_ids.
type mockBackendWithTasks struct {
	emptyBackend
	tasks map[string]*orcv1.Task
}

func (m *mockBackendWithTasks) LoadTask(id string) (*orcv1.Task, error) {
	if task, ok := m.tasks[id]; ok {
		return task, nil
	}
	return nil, nil // Task not found
}

// TestFilterEventByInitiative_MatchingTask verifies that events from tasks
// in the specified initiative pass the filter.
// Covers SC-1: Events from matching initiative are delivered.
func TestFilterEventByInitiative_MatchingTask(t *testing.T) {
	t.Parallel()

	initID := "INIT-001"
	backend := &mockBackendWithTasks{
		tasks: map[string]*orcv1.Task{
			"TASK-001": {Id: "TASK-001", InitiativeId: &initID},
		},
	}

	event := events.Event{
		Type:   events.EventPhase,
		TaskID: "TASK-001",
		Time:   time.Now(),
		Data: &events.PhaseUpdate{
			Phase:  "implement",
			Status: "started",
		},
	}

	// Call the filter function (to be implemented)
	shouldFilter := filterEventByInitiative(event, initID, backend)

	// Event should NOT be filtered (should pass through)
	if shouldFilter {
		t.Error("expected event from matching initiative task to pass filter")
	}
}

// TestFilterEventByInitiative_NonMatchingTask verifies that events from tasks
// NOT in the specified initiative are filtered out.
// Covers SC-1: Events from non-matching initiative are filtered.
func TestFilterEventByInitiative_NonMatchingTask(t *testing.T) {
	t.Parallel()

	otherInitID := "INIT-002"
	backend := &mockBackendWithTasks{
		tasks: map[string]*orcv1.Task{
			"TASK-001": {Id: "TASK-001", InitiativeId: &otherInitID}, // Different initiative
		},
	}

	event := events.Event{
		Type:   events.EventPhase,
		TaskID: "TASK-001",
		Time:   time.Now(),
		Data: &events.PhaseUpdate{
			Phase:  "implement",
			Status: "started",
		},
	}

	// Call the filter function with INIT-001 filter
	shouldFilter := filterEventByInitiative(event, "INIT-001", backend)

	// Event SHOULD be filtered (different initiative)
	if !shouldFilter {
		t.Error("expected event from non-matching initiative task to be filtered")
	}
}

// TestFilterEventByInitiative_TaskNotFound verifies that events from tasks
// that don't exist in storage are filtered out when initiative filter is set.
// Covers SC-2: Events for unknown tasks are filtered when initiative filter is set.
func TestFilterEventByInitiative_TaskNotFound(t *testing.T) {
	t.Parallel()

	backend := &mockBackendWithTasks{
		tasks: map[string]*orcv1.Task{}, // Empty - task not found
	}

	event := events.Event{
		Type:   events.EventPhase,
		TaskID: "TASK-UNKNOWN",
		Time:   time.Now(),
		Data: &events.PhaseUpdate{
			Phase:  "implement",
			Status: "started",
		},
	}

	// Call the filter function
	shouldFilter := filterEventByInitiative(event, "INIT-001", backend)

	// Event SHOULD be filtered (task not found)
	if !shouldFilter {
		t.Error("expected event for unknown task to be filtered")
	}
}

// TestFilterEventByInitiative_TaskWithNoInitiative verifies that events from
// tasks with no initiative are filtered out when initiative filter is set.
// Covers SC-2: Events from tasks without initiative are filtered.
func TestFilterEventByInitiative_TaskWithNoInitiative(t *testing.T) {
	t.Parallel()

	backend := &mockBackendWithTasks{
		tasks: map[string]*orcv1.Task{
			"TASK-001": {Id: "TASK-001", InitiativeId: nil}, // No initiative
		},
	}

	event := events.Event{
		Type:   events.EventPhase,
		TaskID: "TASK-001",
		Time:   time.Now(),
		Data: &events.PhaseUpdate{
			Phase:  "implement",
			Status: "started",
		},
	}

	// Call the filter function
	shouldFilter := filterEventByInitiative(event, "INIT-001", backend)

	// Event SHOULD be filtered (task has no initiative)
	if !shouldFilter {
		t.Error("expected event from task with no initiative to be filtered")
	}
}

// TestFilterEventByInitiative_GlobalEvent verifies that global events (no task_id)
// are filtered when initiative filter is set.
// Covers SC-2: Global events are filtered when initiative filter is set.
func TestFilterEventByInitiative_GlobalEvent(t *testing.T) {
	t.Parallel()

	backend := &mockBackendWithTasks{
		tasks: map[string]*orcv1.Task{},
	}

	// Session update event has no meaningful task ID
	event := events.Event{
		Type:   events.EventSessionUpdate,
		TaskID: "*", // Global task ID
		Time:   time.Now(),
		Data: &events.SessionUpdate{
			DurationSeconds: 100,
			TasksRunning:    1,
		},
	}

	// Call the filter function
	shouldFilter := filterEventByInitiative(event, "INIT-001", backend)

	// Global events SHOULD be filtered when initiative filter is set
	if !shouldFilter {
		t.Error("expected global event to be filtered when initiative filter is set")
	}
}

// TestFilterEventByInitiative_EmptyTaskID verifies that events with empty task_id
// are filtered when initiative filter is set.
// Covers SC-2: Events without task_id are filtered.
func TestFilterEventByInitiative_EmptyTaskID(t *testing.T) {
	t.Parallel()

	backend := &mockBackendWithTasks{
		tasks: map[string]*orcv1.Task{},
	}

	event := events.Event{
		Type:   events.EventSessionUpdate,
		TaskID: "", // Empty task ID
		Time:   time.Now(),
		Data: &events.SessionUpdate{
			DurationSeconds: 100,
		},
	}

	// Call the filter function
	shouldFilter := filterEventByInitiative(event, "INIT-001", backend)

	// Events with empty task ID SHOULD be filtered
	if !shouldFilter {
		t.Error("expected event with empty task_id to be filtered when initiative filter is set")
	}
}

// TestFilterEventByInitiative_NoFilter verifies that when no initiative filter
// is provided (empty string), the filter function returns false (no filtering).
// Covers SC-3: Backward compatibility - no filter means pass through.
func TestFilterEventByInitiative_NoFilter(t *testing.T) {
	t.Parallel()

	backend := &mockBackendWithTasks{
		tasks: map[string]*orcv1.Task{},
	}

	event := events.Event{
		Type:   events.EventPhase,
		TaskID: "TASK-001",
		Time:   time.Now(),
		Data:   &events.PhaseUpdate{Phase: "implement", Status: "started"},
	}

	// Call the filter function with empty initiative filter
	shouldFilter := filterEventByInitiative(event, "", backend)

	// With no filter, nothing should be filtered
	if shouldFilter {
		t.Error("expected no filtering when initiative filter is empty")
	}
}
