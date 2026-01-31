// Package task provides proto-based execution state helper functions.
// These functions operate on orcv1.ExecutionState proto types.
package task

import (
	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// EnsurePhasesProto initializes the Phases map if nil.
func EnsurePhasesProto(e *orcv1.ExecutionState) {
	if e == nil {
		return
	}
	if e.Phases == nil {
		e.Phases = make(map[string]*orcv1.PhaseState)
	}
}

// EnsurePhaseProto ensures a phase entry exists in the Phases map.
// New phases are created with PENDING status (the default for non-completed phases).
func EnsurePhaseProto(e *orcv1.ExecutionState, phaseID string) {
	EnsurePhasesProto(e)
	if e == nil {
		return
	}
	if e.Phases[phaseID] == nil {
		e.Phases[phaseID] = &orcv1.PhaseState{
			Status: orcv1.PhaseStatus_PHASE_STATUS_PENDING,
			Tokens: &orcv1.TokenUsage{},
		}
	}
}

// StartPhaseProto records a phase start time.
// Note: Phase status stays PENDING until completed. Task status tracks running state.
func StartPhaseProto(e *orcv1.ExecutionState, phaseID string) {
	if e == nil {
		return
	}
	now := timestamppb.Now()
	EnsurePhaseProto(e, phaseID)

	// Keep status as PENDING - task.status tracks running, phase.status tracks completion
	e.Phases[phaseID].StartedAt = now
}

// CompletePhaseProto marks a phase as completed.
func CompletePhaseProto(e *orcv1.ExecutionState, phaseID string, commitSHA string) {
	if e == nil {
		return
	}
	now := timestamppb.Now()
	EnsurePhaseProto(e, phaseID)

	e.Phases[phaseID].Status = orcv1.PhaseStatus_PHASE_STATUS_COMPLETED
	e.Phases[phaseID].CompletedAt = now
	if commitSHA != "" {
		e.Phases[phaseID].CommitSha = &commitSHA
	}
}

// FailPhaseProto records an error on the execution state.
// Note: Phase status stays PENDING (not completed). Task status tracks failure.
func FailPhaseProto(e *orcv1.ExecutionState, phaseID string, err error) {
	if e == nil || err == nil {
		return
	}
	errStr := err.Error()
	e.Error = &errStr
	EnsurePhaseProto(e, phaseID)

	// Record error on the phase for visibility, but don't change status
	// Phase status stays PENDING - task.status tracks the failure
	e.Phases[phaseID].Error = &errStr
}

// InterruptPhaseProto records that a phase was interrupted.
// Note: Phase status stays PENDING (not completed). Task status tracks interruption.
func InterruptPhaseProto(e *orcv1.ExecutionState, phaseID string) {
	if e == nil {
		return
	}
	now := timestamppb.Now()
	EnsurePhaseProto(e, phaseID)

	// Record interrupt timestamp, but don't change status
	// Phase status stays PENDING - task.status tracks the interrupt
	e.Phases[phaseID].InterruptedAt = now
}

// SkipPhaseProto marks a phase as skipped with an optional reason.
func SkipPhaseProto(e *orcv1.ExecutionState, phaseID string, reason string) {
	if e == nil {
		return
	}
	now := timestamppb.Now()
	EnsurePhaseProto(e, phaseID)

	e.Phases[phaseID].Status = orcv1.PhaseStatus_PHASE_STATUS_SKIPPED
	e.Phases[phaseID].CompletedAt = now
	if reason != "" {
		skipReason := "skipped: " + reason
		e.Phases[phaseID].Error = &skipReason
	}

	// Record as a gate decision for audit trail
	RecordGateDecisionProto(e, phaseID, "skip", true, reason)
}

// RecordGateDecisionProto records a gate evaluation result.
func RecordGateDecisionProto(e *orcv1.ExecutionState, phase, gateType string, approved bool, reason string) {
	if e == nil {
		return
	}
	decision := &orcv1.GateDecision{
		Phase:     phase,
		GateType:  gateType,
		Approved:  approved,
		Timestamp: timestamppb.Now(),
	}
	if reason != "" {
		decision.Reason = &reason
	}
	e.Gates = append(e.Gates, decision)
}

// GetResumePhaseProto is deprecated - use task.current_phase + task.status instead.
// Phase status no longer tracks running/interrupted state (only completion).
// Returns empty string - callers should use task.current_phase for resume logic.
func GetResumePhaseProto(e *orcv1.ExecutionState) string {
	// Deprecated: phases don't track running/interrupted state anymore.
	// Use task.current_phase for the phase to resume.
	return ""
}

// ResetPhaseProto resets a phase to pending state for retry.
func ResetPhaseProto(e *orcv1.ExecutionState, phaseID string) {
	if e == nil || e.Phases == nil || e.Phases[phaseID] == nil {
		return
	}
	e.Phases[phaseID].Status = orcv1.PhaseStatus_PHASE_STATUS_PENDING
	e.Phases[phaseID].Error = nil
	e.Phases[phaseID].CompletedAt = nil
	e.Phases[phaseID].InterruptedAt = nil
	e.Phases[phaseID].SessionId = nil // Clear session so retry starts fresh with full prompt
}

// SetRetryContextProto sets the retry context for cross-phase retry.
func SetRetryContextProto(e *orcv1.ExecutionState, fromPhase, toPhase, reason, failureOutput string, attempt int32) {
	if e == nil {
		return
	}
	e.RetryContext = &orcv1.RetryContext{
		FromPhase: fromPhase,
		ToPhase:   toPhase,
		Reason:    reason,
		Attempt:   attempt,
		Timestamp: timestamppb.Now(),
	}
	if failureOutput != "" {
		e.RetryContext.FailureOutput = &failureOutput
	}
}

// GetRetryContextProto returns the current retry context.
func GetRetryContextProto(e *orcv1.ExecutionState) *orcv1.RetryContext {
	if e == nil {
		return nil
	}
	return e.RetryContext
}

// ResetExecutionStateProto resets the entire execution state back to initial pending state.
func ResetExecutionStateProto(e *orcv1.ExecutionState) {
	if e == nil {
		return
	}
	// Clear all phase states
	for phaseID := range e.Phases {
		e.Phases[phaseID] = &orcv1.PhaseState{
			Status: orcv1.PhaseStatus_PHASE_STATUS_PENDING,
			Tokens: &orcv1.TokenUsage{},
		}
	}

	// Reset to initial state
	e.CurrentIteration = 0
	e.Error = nil
	e.RetryContext = nil
	e.Session = nil
	e.Gates = nil
}

// AddCostProto adds cost to the execution state and optionally to the current phase.
func AddCostProto(e *orcv1.ExecutionState, currentPhase string, costUSD float64) {
	if e == nil {
		return
	}

	// Ensure cost tracking exists
	if e.Cost == nil {
		e.Cost = &orcv1.CostTracking{}
	}

	e.Cost.TotalCostUsd += costUSD
	e.Cost.LastUpdatedAt = timestamppb.Now()

	if currentPhase != "" {
		if e.Cost.PhaseCosts == nil {
			e.Cost.PhaseCosts = make(map[string]float64)
		}
		e.Cost.PhaseCosts[currentPhase] += costUSD
	}
}

// SetPhaseSessionIDProto stores the session ID for a specific phase.
func SetPhaseSessionIDProto(e *orcv1.ExecutionState, phaseID, sessionID string) {
	if e == nil {
		return
	}
	EnsurePhaseProto(e, phaseID)
	e.Phases[phaseID].SessionId = &sessionID
}

// SetErrorProto sets the error string.
func SetErrorProto(e *orcv1.ExecutionState, errMsg string) {
	if e == nil {
		return
	}
	if errMsg == "" {
		e.Error = nil
	} else {
		e.Error = &errMsg
	}
}

// SetPhaseCommitSHAProto sets the commit SHA for a phase.
func SetPhaseCommitSHAProto(e *orcv1.ExecutionState, phaseID, sha string) {
	if e == nil {
		return
	}
	EnsurePhaseProto(e, phaseID)
	if sha == "" {
		e.Phases[phaseID].CommitSha = nil
	} else {
		e.Phases[phaseID].CommitSha = &sha
	}
}
