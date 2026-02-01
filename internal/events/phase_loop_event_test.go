// Tests for TASK-687: PhaseLoop event and PhaseUpdate extensions.
//
// These tests define the contract for:
//   - PhaseUpdate struct extended with LoopTo and LoopCount fields
//   - PublishHelper.PhaseLoop() method for publishing loop events
//
// Coverage mapping:
//   SC-10: TestPhaseUpdate_LoopFields, TestPhaseUpdate_LoopFieldsOmitEmpty
//   SC-8:  TestPublishHelper_PhaseLoop
//   SC-8:  TestPublishHelper_PhaseLoop_NilPublisher (nil-safety)
package events

import (
	"encoding/json"
	"testing"
)

// Uses existing mockPublisher and newMockPublisher from publish_helper_test.go

// =============================================================================
// SC-10: PhaseUpdate has LoopTo and LoopCount fields
// =============================================================================

func TestPhaseUpdate_LoopFields(t *testing.T) {
	t.Parallel()

	update := PhaseUpdate{
		Phase:     "review",
		Status:    "looping",
		LoopTo:    "implement",
		LoopCount: 2,
	}

	if update.LoopTo != "implement" {
		t.Errorf("LoopTo = %q, want %q", update.LoopTo, "implement")
	}
	if update.LoopCount != 2 {
		t.Errorf("LoopCount = %d, want 2", update.LoopCount)
	}
}

// =============================================================================
// SC-10: LoopTo and LoopCount are omitempty in JSON
// =============================================================================

func TestPhaseUpdate_LoopFieldsOmitEmpty(t *testing.T) {
	t.Parallel()

	// Normal phase update without loop fields — should not include them in JSON
	update := PhaseUpdate{
		Phase:  "implement",
		Status: "completed",
	}

	data, err := json.Marshal(update)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if _, ok := raw["loop_to"]; ok {
		t.Error("loop_to should be omitted when empty")
	}
	if _, ok := raw["loop_count"]; ok {
		t.Error("loop_count should be omitted when zero")
	}
}

// =============================================================================
// SC-10: Loop event includes fields in JSON
// =============================================================================

func TestPhaseUpdate_LoopFieldsPresent(t *testing.T) {
	t.Parallel()

	update := PhaseUpdate{
		Phase:     "review",
		Status:    "looping",
		LoopTo:    "implement",
		LoopCount: 1,
	}

	data, err := json.Marshal(update)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if raw["loop_to"] != "implement" {
		t.Errorf("JSON loop_to = %v, want %q", raw["loop_to"], "implement")
	}
	// JSON numbers unmarshal as float64
	if raw["loop_count"] != float64(1) {
		t.Errorf("JSON loop_count = %v, want 1", raw["loop_count"])
	}
	if raw["status"] != "looping" {
		t.Errorf("JSON status = %v, want %q", raw["status"], "looping")
	}
}

// =============================================================================
// SC-8: PublishHelper.PhaseLoop publishes correct event
// =============================================================================

func TestPublishHelper_PhaseLoop(t *testing.T) {
	t.Parallel()

	mock := newMockPublisher()
	helper := NewPublishHelper(mock)

	helper.PhaseLoop("TASK-001", "review", "implement", 2)

	evts := mock.getEvents()
	if len(evts) != 1 {
		t.Fatalf("expected 1 event, got %d", len(evts))
	}

	ev := evts[0]
	if ev.Type != EventPhase {
		t.Errorf("event type = %q, want %q", ev.Type, EventPhase)
	}
	if ev.TaskID != "TASK-001" {
		t.Errorf("task ID = %q, want %q", ev.TaskID, "TASK-001")
	}

	update, ok := ev.Data.(PhaseUpdate)
	if !ok {
		t.Fatalf("event data type = %T, want PhaseUpdate", ev.Data)
	}
	if update.Phase != "review" {
		t.Errorf("phase = %q, want %q", update.Phase, "review")
	}
	if update.Status != "looping" {
		t.Errorf("status = %q, want %q", update.Status, "looping")
	}
	if update.LoopTo != "implement" {
		t.Errorf("loop_to = %q, want %q", update.LoopTo, "implement")
	}
	if update.LoopCount != 2 {
		t.Errorf("loop_count = %d, want 2", update.LoopCount)
	}
}

// =============================================================================
// SC-8: PublishHelper.PhaseLoop is nil-safe (per existing pattern)
// =============================================================================

func TestPublishHelper_PhaseLoop_NilPublisher(t *testing.T) {
	t.Parallel()

	// Nil publisher should be a no-op (not panic)
	helper := NewPublishHelper(nil)
	helper.PhaseLoop("TASK-001", "review", "implement", 1)

	// Also test nil helper
	var nilHelper *PublishHelper
	nilHelper.PhaseLoop("TASK-001", "review", "implement", 1)
}
