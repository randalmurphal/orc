// Tests for TASK-686 SC-8: PublishHelper.PhaseSkipped publishes EventPhase with status "skipped".
package events

import (
	"testing"
)

// =============================================================================
// SC-8: PhaseSkipped publishes an EventPhase event with status "skipped"
// =============================================================================

func TestPublishHelper_PhaseSkipped_PublishesCorrectEvent(t *testing.T) {
	t.Parallel()

	mock := newMockPublisher()
	ep := NewPublishHelper(mock)

	ep.PhaseSkipped("TASK-001", "tdd_write")

	ev := mock.lastEvent()
	if ev == nil {
		t.Fatal("expected event to be published")
	}

	if ev.Type != EventPhase {
		t.Errorf("expected EventPhase, got %v", ev.Type)
	}
	if ev.TaskID != "TASK-001" {
		t.Errorf("expected TaskID TASK-001, got %s", ev.TaskID)
	}

	update, ok := ev.Data.(PhaseUpdate)
	if !ok {
		t.Fatalf("expected PhaseUpdate data, got %T", ev.Data)
	}
	if update.Phase != "tdd_write" {
		t.Errorf("expected phase tdd_write, got %s", update.Phase)
	}
	if update.Status != "skipped" {
		t.Errorf("expected status skipped, got %s", update.Status)
	}
}

func TestPublishHelper_PhaseSkipped_NilPublisher_NoOp(t *testing.T) {
	t.Parallel()

	ep := NewPublishHelper(nil)

	// Should not panic when publishing with nil publisher
	ep.PhaseSkipped("TASK-001", "tdd_write")
}

func TestPublishHelper_PhaseSkipped_NilHelper_NoOp(t *testing.T) {
	t.Parallel()

	var nilEP *PublishHelper

	// Should not panic when PublishHelper itself is nil
	nilEP.PhaseSkipped("TASK-001", "tdd_write")
}

func TestPublishHelper_PhaseSkipped_DifferentPhases(t *testing.T) {
	t.Parallel()

	phases := []string{"spec", "tdd_write", "implement", "review", "docs"}

	for _, phase := range phases {
		t.Run(phase, func(t *testing.T) {
			t.Parallel()

			mock := newMockPublisher()
			ep := NewPublishHelper(mock)

			ep.PhaseSkipped("TASK-002", phase)

			ev := mock.lastEvent()
			if ev == nil {
				t.Fatalf("expected event for phase %s", phase)
			}

			update, ok := ev.Data.(PhaseUpdate)
			if !ok {
				t.Fatalf("expected PhaseUpdate, got %T", ev.Data)
			}
			if update.Phase != phase {
				t.Errorf("phase = %q, want %q", update.Phase, phase)
			}
			if update.Status != "skipped" {
				t.Errorf("status = %q, want skipped", update.Status)
			}
		})
	}
}
