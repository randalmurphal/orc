package api

import (
	"testing"
	"time"

	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/events"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
)

// =============================================================================
// Tests for internalEventToProto - Missing Event Type Conversions
// =============================================================================

// TestInternalEventToProto_EventActivity tests conversion of EventActivity events.
// This covers SC-5: EventActivity events are delivered to frontend.
func TestInternalEventToProto_EventActivity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		event          events.Event
		wantTaskID     string
		wantPhaseID    string
		wantActivity   orcv1.ActivityState
		wantHasDetails bool
	}{
		{
			name: "pointer data - waiting_api",
			event: events.Event{
				Type:   events.EventActivity,
				TaskID: "TASK-001",
				Time:   time.Now(),
				Data: &events.ActivityUpdate{
					Phase:    "spec",
					Activity: "waiting_api",
				},
			},
			wantTaskID:   "TASK-001",
			wantPhaseID:  "spec",
			wantActivity: orcv1.ActivityState_ACTIVITY_STATE_WAITING_API,
		},
		{
			name: "value data - streaming",
			event: events.Event{
				Type:   events.EventActivity,
				TaskID: "TASK-002",
				Time:   time.Now(),
				Data: events.ActivityUpdate{
					Phase:    "implement",
					Activity: "streaming",
				},
			},
			wantTaskID:   "TASK-002",
			wantPhaseID:  "implement",
			wantActivity: orcv1.ActivityState_ACTIVITY_STATE_STREAMING,
		},
		{
			name: "running_tool state",
			event: events.Event{
				Type:   events.EventActivity,
				TaskID: "TASK-003",
				Time:   time.Now(),
				Data: events.ActivityUpdate{
					Phase:    "review",
					Activity: "running_tool",
				},
			},
			wantTaskID:   "TASK-003",
			wantPhaseID:  "review",
			wantActivity: orcv1.ActivityState_ACTIVITY_STATE_RUNNING_TOOL,
		},
		{
			name: "processing state",
			event: events.Event{
				Type:   events.EventActivity,
				TaskID: "TASK-004",
				Time:   time.Now(),
				Data: events.ActivityUpdate{
					Phase:    "tdd_write",
					Activity: "processing",
				},
			},
			wantTaskID:   "TASK-004",
			wantPhaseID:  "tdd_write",
			wantActivity: orcv1.ActivityState_ACTIVITY_STATE_PROCESSING,
		},
		{
			name: "idle state",
			event: events.Event{
				Type:   events.EventActivity,
				TaskID: "TASK-005",
				Time:   time.Now(),
				Data: events.ActivityUpdate{
					Phase:    "docs",
					Activity: "idle",
				},
			},
			wantTaskID:   "TASK-005",
			wantPhaseID:  "docs",
			wantActivity: orcv1.ActivityState_ACTIVITY_STATE_IDLE,
		},
		{
			name: "spec_analyzing state",
			event: events.Event{
				Type:   events.EventActivity,
				TaskID: "TASK-006",
				Time:   time.Now(),
				Data: events.ActivityUpdate{
					Phase:    "spec",
					Activity: "spec_analyzing",
				},
			},
			wantTaskID:   "TASK-006",
			wantPhaseID:  "spec",
			wantActivity: orcv1.ActivityState_ACTIVITY_STATE_SPEC_ANALYZING,
		},
		{
			name: "spec_writing state",
			event: events.Event{
				Type:   events.EventActivity,
				TaskID: "TASK-007",
				Time:   time.Now(),
				Data: events.ActivityUpdate{
					Phase:    "spec",
					Activity: "spec_writing",
				},
			},
			wantTaskID:   "TASK-007",
			wantPhaseID:  "spec",
			wantActivity: orcv1.ActivityState_ACTIVITY_STATE_SPEC_WRITING,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := internalEventToProto(tt.event)
			if result == nil {
				t.Fatal("expected non-nil result")
			}

			// Verify task ID is set
			if result.TaskId == nil || *result.TaskId != tt.wantTaskID {
				t.Errorf("expected task_id %q, got %v", tt.wantTaskID, result.TaskId)
			}

			// Verify payload type
			activity := result.GetActivity()
			if activity == nil {
				t.Fatal("expected Activity payload, got nil")
			}

			if activity.TaskId != tt.wantTaskID {
				t.Errorf("expected activity.task_id %q, got %q", tt.wantTaskID, activity.TaskId)
			}
			if activity.PhaseId != tt.wantPhaseID {
				t.Errorf("expected activity.phase_id %q, got %q", tt.wantPhaseID, activity.PhaseId)
			}
			if activity.Activity != tt.wantActivity {
				t.Errorf("expected activity.activity %v, got %v", tt.wantActivity, activity.Activity)
			}
		})
	}
}

// TestInternalEventToProto_EventSessionUpdate tests conversion of EventSessionUpdate events.
// This covers SC-6: EventSessionUpdate events update TopBar metrics.
func TestInternalEventToProto_EventSessionUpdate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		event            events.Event
		wantDuration     int64
		wantTotalTokens  int32
		wantCost         float64
		wantInputTokens  int32
		wantOutputTokens int32
		wantTasksRunning int32
		wantIsPaused     bool
	}{
		{
			name: "pointer data - active session",
			event: events.Event{
				Type:   events.EventSessionUpdate,
				TaskID: "*", // Session events are global
				Time:   time.Now(),
				Data: &events.SessionUpdate{
					DurationSeconds:  3600,
					TotalTokens:      50000,
					EstimatedCostUSD: 0.25,
					InputTokens:      40000,
					OutputTokens:     10000,
					TasksRunning:     2,
					IsPaused:         false,
				},
			},
			wantDuration:     3600,
			wantTotalTokens:  50000,
			wantCost:         0.25,
			wantInputTokens:  40000,
			wantOutputTokens: 10000,
			wantTasksRunning: 2,
			wantIsPaused:     false,
		},
		{
			name: "value data - paused session",
			event: events.Event{
				Type:   events.EventSessionUpdate,
				TaskID: "*",
				Time:   time.Now(),
				Data: events.SessionUpdate{
					DurationSeconds:  7200,
					TotalTokens:      100000,
					EstimatedCostUSD: 0.50,
					InputTokens:      80000,
					OutputTokens:     20000,
					TasksRunning:     0,
					IsPaused:         true,
				},
			},
			wantDuration:     7200,
			wantTotalTokens:  100000,
			wantCost:         0.50,
			wantInputTokens:  80000,
			wantOutputTokens: 20000,
			wantTasksRunning: 0,
			wantIsPaused:     true,
		},
		{
			name: "zero values session",
			event: events.Event{
				Type:   events.EventSessionUpdate,
				TaskID: "*",
				Time:   time.Now(),
				Data: events.SessionUpdate{
					DurationSeconds:  0,
					TotalTokens:      0,
					EstimatedCostUSD: 0.0,
					InputTokens:      0,
					OutputTokens:     0,
					TasksRunning:     0,
					IsPaused:         false,
				},
			},
			wantDuration:     0,
			wantTotalTokens:  0,
			wantCost:         0.0,
			wantInputTokens:  0,
			wantOutputTokens: 0,
			wantTasksRunning: 0,
			wantIsPaused:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := internalEventToProto(tt.event)
			if result == nil {
				t.Fatal("expected non-nil result")
			}

			// Verify payload type
			metrics := result.GetSessionMetrics()
			if metrics == nil {
				t.Fatal("expected SessionMetrics payload, got nil")
			}

			if metrics.DurationSeconds != tt.wantDuration {
				t.Errorf("expected duration_seconds %d, got %d", tt.wantDuration, metrics.DurationSeconds)
			}
			if metrics.TotalTokens != tt.wantTotalTokens {
				t.Errorf("expected total_tokens %d, got %d", tt.wantTotalTokens, metrics.TotalTokens)
			}
			if metrics.EstimatedCostUsd != tt.wantCost {
				t.Errorf("expected estimated_cost_usd %f, got %f", tt.wantCost, metrics.EstimatedCostUsd)
			}
			if metrics.InputTokens != tt.wantInputTokens {
				t.Errorf("expected input_tokens %d, got %d", tt.wantInputTokens, metrics.InputTokens)
			}
			if metrics.OutputTokens != tt.wantOutputTokens {
				t.Errorf("expected output_tokens %d, got %d", tt.wantOutputTokens, metrics.OutputTokens)
			}
			if metrics.TasksRunning != tt.wantTasksRunning {
				t.Errorf("expected tasks_running %d, got %d", tt.wantTasksRunning, metrics.TasksRunning)
			}
			if metrics.IsPaused != tt.wantIsPaused {
				t.Errorf("expected is_paused %v, got %v", tt.wantIsPaused, metrics.IsPaused)
			}
		})
	}
}

// TestInternalEventToProto_EventWarning tests conversion of EventWarning events.
func TestInternalEventToProto_EventWarning(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		event       events.Event
		wantTaskID  string
		wantMessage string
		wantPhase   string
	}{
		{
			name: "pointer data - warning with phase",
			event: events.Event{
				Type:   events.EventWarning,
				TaskID: "TASK-001",
				Time:   time.Now(),
				Data: &events.WarningData{
					Phase:   "implement",
					Message: "Token usage approaching limit",
				},
			},
			wantTaskID:  "TASK-001",
			wantMessage: "Token usage approaching limit",
			wantPhase:   "implement",
		},
		{
			name: "value data - warning without phase",
			event: events.Event{
				Type:   events.EventWarning,
				TaskID: "TASK-002",
				Time:   time.Now(),
				Data: events.WarningData{
					Message: "Rate limit approaching",
				},
			},
			wantTaskID:  "TASK-002",
			wantMessage: "Rate limit approaching",
			wantPhase:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := internalEventToProto(tt.event)
			if result == nil {
				t.Fatal("expected non-nil result")
			}

			// Verify task ID
			if result.TaskId == nil || *result.TaskId != tt.wantTaskID {
				t.Errorf("expected task_id %q, got %v", tt.wantTaskID, result.TaskId)
			}

			// Verify payload type
			warning := result.GetWarning()
			if warning == nil {
				t.Fatal("expected Warning payload, got nil")
			}

			if warning.TaskId != tt.wantTaskID {
				t.Errorf("expected warning.task_id %q, got %q", tt.wantTaskID, warning.TaskId)
			}
			if warning.Message != tt.wantMessage {
				t.Errorf("expected warning.message %q, got %q", tt.wantMessage, warning.Message)
			}
			if warning.GetPhase() != tt.wantPhase {
				t.Errorf("expected warning.phase %q, got %q", tt.wantPhase, warning.GetPhase())
			}
		})
	}
}

// TestInternalEventToProto_EventHeartbeat tests conversion of EventHeartbeat events.
// This covers SC-7: Heartbeat events maintain connection health.
func TestInternalEventToProto_EventHeartbeat(t *testing.T) {
	t.Parallel()

	now := time.Now()
	tests := []struct {
		name  string
		event events.Event
	}{
		{
			name: "pointer data - heartbeat",
			event: events.Event{
				Type:   events.EventHeartbeat,
				TaskID: "TASK-001",
				Time:   now,
				Data: &events.HeartbeatData{
					Phase:     "implement",
					Iteration: 1,
					Timestamp: now,
				},
			},
		},
		{
			name: "value data - heartbeat",
			event: events.Event{
				Type:   events.EventHeartbeat,
				TaskID: "TASK-002",
				Time:   now,
				Data: events.HeartbeatData{
					Phase:     "spec",
					Iteration: 2,
					Timestamp: now,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := internalEventToProto(tt.event)
			if result == nil {
				t.Fatal("expected non-nil result")
			}

			// Verify payload type
			heartbeat := result.GetHeartbeat()
			if heartbeat == nil {
				t.Fatal("expected Heartbeat payload, got nil")
			}

			// Heartbeat should have timestamp
			if heartbeat.Timestamp == nil {
				t.Error("expected heartbeat.timestamp to be set")
			}
		})
	}
}

// =============================================================================
// Edge Case Tests
// =============================================================================

// TestInternalEventToProto_NilData tests that events with nil Data are handled gracefully.
func TestInternalEventToProto_NilData(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		eventType events.EventType
	}{
		{"EventActivity with nil Data", events.EventActivity},
		{"EventSessionUpdate with nil Data", events.EventSessionUpdate},
		{"EventWarning with nil Data", events.EventWarning},
		{"EventHeartbeat with nil Data", events.EventHeartbeat},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			event := events.Event{
				Type:   tt.eventType,
				TaskID: "TASK-001",
				Time:   time.Now(),
				Data:   nil, // Nil data
			}

			result := internalEventToProto(event)

			// With nil Data, the conversion should return nil (skip the event)
			// because we can't populate the proto message fields
			if result != nil {
				t.Errorf("expected nil result for nil Data, got non-nil")
			}
		})
	}
}

// TestInternalEventToProto_WrongDataType tests that events with wrong Data type are handled gracefully.
func TestInternalEventToProto_WrongDataType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		eventType events.EventType
		wrongData any
	}{
		{
			name:      "EventActivity with wrong type",
			eventType: events.EventActivity,
			wrongData: "not an ActivityUpdate",
		},
		{
			name:      "EventSessionUpdate with wrong type",
			eventType: events.EventSessionUpdate,
			wrongData: map[string]string{"invalid": "data"},
		},
		{
			name:      "EventWarning with wrong type",
			eventType: events.EventWarning,
			wrongData: 12345,
		},
		{
			name:      "EventHeartbeat with wrong type",
			eventType: events.EventHeartbeat,
			wrongData: []string{"not", "heartbeat"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			event := events.Event{
				Type:   tt.eventType,
				TaskID: "TASK-001",
				Time:   time.Now(),
				Data:   tt.wrongData,
			}

			result := internalEventToProto(event)

			// With wrong Data type, the conversion should return nil (skip the event)
			if result != nil {
				t.Errorf("expected nil result for wrong Data type, got non-nil")
			}
		})
	}
}

// TestInternalEventToProto_UnknownEventType tests that unknown event types return nil.
func TestInternalEventToProto_UnknownEventType(t *testing.T) {
	t.Parallel()

	event := events.Event{
		Type:   events.EventType("unknown_event_type"),
		TaskID: "TASK-001",
		Time:   time.Now(),
		Data:   map[string]string{"some": "data"},
	}

	result := internalEventToProto(event)

	if result != nil {
		t.Errorf("expected nil result for unknown event type, got non-nil")
	}
}

// =============================================================================
// Existing Event Type Tests (Verification)
// =============================================================================

// TestInternalEventToProto_EventPhase tests that existing EventPhase conversion still works.
func TestInternalEventToProto_EventPhase(t *testing.T) {
	t.Parallel()

	event := events.Event{
		Type:   events.EventPhase,
		TaskID: "TASK-001",
		Time:   time.Now(),
		Data: &events.PhaseUpdate{
			Phase:  "spec",
			Status: "completed",
		},
	}

	result := internalEventToProto(event)
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	phase := result.GetPhaseChanged()
	if phase == nil {
		t.Fatal("expected PhaseChanged payload")
	}

	if phase.PhaseName != "spec" {
		t.Errorf("expected phase_name 'spec', got %q", phase.PhaseName)
	}
}

// TestInternalEventToProto_EventTokens tests that existing EventTokens conversion still works.
func TestInternalEventToProto_EventTokens(t *testing.T) {
	t.Parallel()

	event := events.Event{
		Type:   events.EventTokens,
		TaskID: "TASK-001",
		Time:   time.Now(),
		Data: &events.TokenUpdate{
			Phase:                    "implement",
			InputTokens:              1000,
			OutputTokens:             500,
			CacheCreationInputTokens: 100,
			CacheReadInputTokens:     200,
			TotalTokens:              1500,
		},
	}

	result := internalEventToProto(event)
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	tokens := result.GetTokensUpdated()
	if tokens == nil {
		t.Fatal("expected TokensUpdated payload")
	}

	if tokens.Tokens.InputTokens != 1000 {
		t.Errorf("expected input_tokens 1000, got %d", tokens.Tokens.InputTokens)
	}
	if tokens.Tokens.OutputTokens != 500 {
		t.Errorf("expected output_tokens 500, got %d", tokens.Tokens.OutputTokens)
	}
	if tokens.Tokens.TotalTokens != 1500 {
		t.Errorf("expected total_tokens 1500, got %d", tokens.Tokens.TotalTokens)
	}
}

// TestInternalEventToProto_EventError tests that existing EventError conversion still works.
func TestInternalEventToProto_EventError(t *testing.T) {
	t.Parallel()

	event := events.Event{
		Type:   events.EventError,
		TaskID: "TASK-001",
		Time:   time.Now(),
		Data: &events.ErrorData{
			Phase:   "review",
			Message: "Something went wrong",
			Fatal:   true,
		},
	}

	result := internalEventToProto(event)
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	errEvent := result.GetError()
	if errEvent == nil {
		t.Fatal("expected Error payload")
	}

	if errEvent.Error != "Something went wrong" {
		t.Errorf("expected error 'Something went wrong', got %q", errEvent.Error)
	}
}

// TestInternalEventToProto_TaskEvents tests existing task lifecycle event conversions.
func TestInternalEventToProto_TaskEvents(t *testing.T) {
	t.Parallel()

	t.Run("TaskCreated", func(t *testing.T) {
		t.Parallel()
		event := events.Event{
			Type:   events.EventTaskCreated,
			TaskID: "TASK-001",
			Time:   time.Now(),
			Data: map[string]any{
				"title": "New Feature",
			},
		}

		result := internalEventToProto(event)
		if result == nil {
			t.Fatal("expected non-nil result")
		}

		created := result.GetTaskCreated()
		if created == nil {
			t.Fatal("expected TaskCreated payload")
		}
		if created.TaskId != "TASK-001" {
			t.Errorf("expected task_id 'TASK-001', got %q", created.TaskId)
		}
	})

	t.Run("TaskUpdated", func(t *testing.T) {
		t.Parallel()
		event := events.Event{
			Type:   events.EventTaskUpdated,
			TaskID: "TASK-002",
			Time:   time.Now(),
			Data:   nil, // TaskUpdated doesn't require data
		}

		result := internalEventToProto(event)
		if result == nil {
			t.Fatal("expected non-nil result")
		}

		updated := result.GetTaskUpdated()
		if updated == nil {
			t.Fatal("expected TaskUpdated payload")
		}
		if updated.TaskId != "TASK-002" {
			t.Errorf("expected task_id 'TASK-002', got %q", updated.TaskId)
		}
	})

	t.Run("TaskDeleted", func(t *testing.T) {
		t.Parallel()
		event := events.Event{
			Type:   events.EventTaskDeleted,
			TaskID: "TASK-003",
			Time:   time.Now(),
			Data:   nil,
		}

		result := internalEventToProto(event)
		if result == nil {
			t.Fatal("expected non-nil result")
		}

		deleted := result.GetTaskDeleted()
		if deleted == nil {
			t.Fatal("expected TaskDeleted payload")
		}
		if deleted.TaskId != "TASK-003" {
			t.Errorf("expected task_id 'TASK-003', got %q", deleted.TaskId)
		}
	})
}


// TestDbEventToProto_UsesDatabaseID verifies that dbEventToProto uses the database
// event ID instead of generating a new UUID. This prevents duplicate events from
// appearing in the timeline when the same event is fetched multiple times.
//
// BUG FIX: TASK-587 - Timeline shows duplicate events because each call to
// dbEventToProto generated a new UUID, making deduplication impossible.
func TestDbEventToProto_UsesDatabaseID(t *testing.T) {
	// Create a database event with a known ID
	dbEvent := &db.EventLog{
		ID:        12345,
		TaskID:    "TASK-001",
		EventType: "phase",
		Source:    "executor",
		CreatedAt: time.Now(),
	}
	phase := "implement"
	dbEvent.Phase = &phase

	// Convert to proto - should use database ID
	protoEvent := dbEventToProto(dbEvent)

	if protoEvent == nil {
		t.Fatal("dbEventToProto returned nil")
	}

	// The proto event ID should be the database ID as a string, not a random UUID
	expectedID := "12345"
	if protoEvent.Id != expectedID {
		t.Errorf("expected proto event ID to be database ID %q, got %q", expectedID, protoEvent.Id)
	}

	// Verify calling again with same input returns same ID (deterministic)
	protoEvent2 := dbEventToProto(dbEvent)
	if protoEvent2.Id != protoEvent.Id {
		t.Errorf("dbEventToProto should be deterministic: first call returned %q, second returned %q",
			protoEvent.Id, protoEvent2.Id)
	}
}

// TestDbEventToTimelineEvent_UsesDatabaseID verifies that dbEventToTimelineEvent
// uses the database event ID for consistent identification.
func TestDbEventToTimelineEvent_UsesDatabaseID(t *testing.T) {
	dbEvent := &db.EventLogWithTitle{
		EventLog: db.EventLog{
			ID:        67890,
			TaskID:    "TASK-002",
			EventType: "phase",
			Source:    "executor",
			CreatedAt: time.Now(),
		},
		TaskTitle: "Test Task",
	}
	phase := "spec"
	dbEvent.Phase = &phase

	// Convert to timeline event - should use database ID
	timelineEvent := dbEventToTimelineEvent(dbEvent)

	if timelineEvent == nil {
		t.Fatal("dbEventToTimelineEvent returned nil")
	}

	// The event ID should be the database ID as a string
	expectedID := "67890"
	if timelineEvent.Id != expectedID {
		t.Errorf("expected timeline event ID to be database ID %q, got %q", expectedID, timelineEvent.Id)
	}
}
