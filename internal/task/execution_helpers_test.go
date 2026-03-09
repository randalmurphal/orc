package task

import (
	"testing"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
)

func TestSkipRemainingPhases(t *testing.T) {
	t.Run("mixed states only skip pending", func(t *testing.T) {
		e := InitProtoExecutionState()
		e.Phases["completed"] = &orcv1.PhaseState{
			Status: orcv1.PhaseStatus_PHASE_STATUS_COMPLETED,
		}
		e.Phases["pending-a"] = &orcv1.PhaseState{
			Status: orcv1.PhaseStatus_PHASE_STATUS_PENDING,
		}
		existingSkippedReason := "skipped: existing reason"
		e.Phases["already-skipped"] = &orcv1.PhaseState{
			Status: orcv1.PhaseStatus_PHASE_STATUS_SKIPPED,
			Error:  &existingSkippedReason,
		}
		e.Phases["pending-b"] = &orcv1.PhaseState{
			Status: orcv1.PhaseStatus_PHASE_STATUS_PENDING,
		}

		SkipRemainingPhasesProto(e, "task closed")

		if got := e.Phases["completed"].Status; got != orcv1.PhaseStatus_PHASE_STATUS_COMPLETED {
			t.Fatalf("completed phase status = %s, want %s", got, orcv1.PhaseStatus_PHASE_STATUS_COMPLETED)
		}
		if got := e.Phases["already-skipped"].Status; got != orcv1.PhaseStatus_PHASE_STATUS_SKIPPED {
			t.Fatalf("already-skipped phase status = %s, want %s", got, orcv1.PhaseStatus_PHASE_STATUS_SKIPPED)
		}
		if e.Phases["already-skipped"].Error == nil || *e.Phases["already-skipped"].Error != existingSkippedReason {
			t.Fatalf("already-skipped reason changed, got %v want %q", e.Phases["already-skipped"].Error, existingSkippedReason)
		}

		for _, phaseID := range []string{"pending-a", "pending-b"} {
			ps := e.Phases[phaseID]
			if ps.Status != orcv1.PhaseStatus_PHASE_STATUS_SKIPPED {
				t.Fatalf("%s status = %s, want %s", phaseID, ps.Status, orcv1.PhaseStatus_PHASE_STATUS_SKIPPED)
			}
			if ps.Error == nil || *ps.Error != "skipped: task closed" {
				t.Fatalf("%s error = %v, want %q", phaseID, ps.Error, "skipped: task closed")
			}
		}

		if len(e.Gates) != 2 {
			t.Fatalf("gate decisions = %d, want 2", len(e.Gates))
		}

		seen := map[string]bool{}
		for _, decision := range e.Gates {
			if decision.GateType != "skip" {
				t.Fatalf("gate type = %q, want %q", decision.GateType, "skip")
			}
			if !decision.Approved {
				t.Fatalf("gate decision approved = false, want true")
			}
			if decision.Reason == nil || *decision.Reason != "task closed" {
				t.Fatalf("gate reason = %v, want %q", decision.Reason, "task closed")
			}
			seen[decision.Phase] = true
		}
		if !seen["pending-a"] || !seen["pending-b"] {
			t.Fatalf("gate decisions missing expected phases: %+v", seen)
		}
	})

	t.Run("nil execution state no-op", func(t *testing.T) {
		SkipRemainingPhasesProto(nil, "task closed")
	})

	t.Run("empty execution state no-op", func(t *testing.T) {
		e := &orcv1.ExecutionState{}
		SkipRemainingPhasesProto(e, "task closed")
		if len(e.Gates) != 0 {
			t.Fatalf("gate decisions = %d, want 0", len(e.Gates))
		}
	})
}
