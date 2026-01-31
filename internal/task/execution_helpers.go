// Package task provides proto-based execution state helper functions.
// These functions operate on orcv1.ExecutionState proto types.
package task

import (
	"strings"

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

// IncrementIterationProto increments the iteration count for the current phase.
func IncrementIterationProto(e *orcv1.ExecutionState, currentPhase string) {
	if e == nil {
		return
	}
	e.CurrentIteration++

	if e.Phases != nil && e.Phases[currentPhase] != nil {
		e.Phases[currentPhase].Iterations++
	}
}

// AddTokensProto adds token usage to the state.
func AddTokensProto(e *orcv1.ExecutionState, currentPhase string, input, output, cacheCreation, cacheRead int32) {
	if e == nil {
		return
	}

	// Ensure tokens struct exists
	if e.Tokens == nil {
		e.Tokens = &orcv1.TokenUsage{}
	}

	e.Tokens.InputTokens += input
	e.Tokens.OutputTokens += output
	e.Tokens.CacheCreationInputTokens += cacheCreation
	e.Tokens.CacheReadInputTokens += cacheRead
	e.Tokens.TotalTokens += input + output

	if e.Phases != nil && e.Phases[currentPhase] != nil {
		if e.Phases[currentPhase].Tokens == nil {
			e.Phases[currentPhase].Tokens = &orcv1.TokenUsage{}
		}
		e.Phases[currentPhase].Tokens.InputTokens += input
		e.Phases[currentPhase].Tokens.OutputTokens += output
		e.Phases[currentPhase].Tokens.CacheCreationInputTokens += cacheCreation
		e.Phases[currentPhase].Tokens.CacheReadInputTokens += cacheRead
		e.Phases[currentPhase].Tokens.TotalTokens += input + output
	}
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

// IsPhaseCompletedProto returns true if a phase is completed or skipped.
func IsPhaseCompletedProto(e *orcv1.ExecutionState, phaseID string) bool {
	if e == nil || e.Phases == nil {
		return false
	}
	ps, ok := e.Phases[phaseID]
	if !ok {
		return false
	}
	return ps.Status == orcv1.PhaseStatus_PHASE_STATUS_COMPLETED ||
		ps.Status == orcv1.PhaseStatus_PHASE_STATUS_SKIPPED
}

// IsPhaseSkippedProto returns true if a phase was skipped.
func IsPhaseSkippedProto(e *orcv1.ExecutionState, phaseID string) bool {
	if e == nil || e.Phases == nil {
		return false
	}
	ps, ok := e.Phases[phaseID]
	if !ok {
		return false
	}
	return ps.Status == orcv1.PhaseStatus_PHASE_STATUS_SKIPPED
}

// GetSkipReasonProto returns the skip reason for a phase, if any.
func GetSkipReasonProto(e *orcv1.ExecutionState, phaseID string) string {
	if e == nil || e.Phases == nil {
		return ""
	}
	ps, ok := e.Phases[phaseID]
	if !ok || ps.Status != orcv1.PhaseStatus_PHASE_STATUS_SKIPPED {
		return ""
	}
	if ps.Error == nil {
		return ""
	}
	if strings.HasPrefix(*ps.Error, "skipped: ") {
		return strings.TrimPrefix(*ps.Error, "skipped: ")
	}
	return *ps.Error
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

// SetRetryContextFileProto sets the context file path for detailed retry context.
func SetRetryContextFileProto(e *orcv1.ExecutionState, contextFile string) {
	if e == nil || e.RetryContext == nil {
		return
	}
	e.RetryContext.ContextFile = &contextFile
}

// ClearRetryContextProto clears the retry context after successful completion.
func ClearRetryContextProto(e *orcv1.ExecutionState) {
	if e == nil {
		return
	}
	e.RetryContext = nil
}

// GetRetryContextProto returns the current retry context.
func GetRetryContextProto(e *orcv1.ExecutionState) *orcv1.RetryContext {
	if e == nil {
		return nil
	}
	return e.RetryContext
}

// HasRetryContextProto returns true if there is an active retry context.
func HasRetryContextProto(e *orcv1.ExecutionState) bool {
	return e != nil && e.RetryContext != nil
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

// SetSessionProto updates the session info.
func SetSessionProto(e *orcv1.ExecutionState, id, model, status string, turnCount int32) {
	if e == nil {
		return
	}
	now := timestamppb.Now()
	if e.Session == nil {
		e.Session = &orcv1.SessionInfo{
			CreatedAt: now,
		}
	}
	e.Session.Id = id
	e.Session.Model = model
	e.Session.Status = status
	e.Session.TurnCount = turnCount
	e.Session.LastActivity = now
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

// GetPhaseSessionIDProto returns the session ID for a specific phase.
func GetPhaseSessionIDProto(e *orcv1.ExecutionState, phaseID string) string {
	if e == nil || e.Phases == nil {
		return ""
	}
	ps, ok := e.Phases[phaseID]
	if !ok || ps.SessionId == nil {
		return ""
	}
	return *ps.SessionId
}

// SetPhaseSessionIDProto stores the session ID for a specific phase.
func SetPhaseSessionIDProto(e *orcv1.ExecutionState, phaseID, sessionID string) {
	if e == nil {
		return
	}
	EnsurePhaseProto(e, phaseID)
	e.Phases[phaseID].SessionId = &sessionID
}

// RecordValidationProto records a validation decision for the specified phase.
func RecordValidationProto(e *orcv1.ExecutionState, phaseID string, entry *orcv1.ValidationEntry) {
	if e == nil || entry == nil {
		return
	}
	EnsurePhaseProto(e, phaseID)
	e.Phases[phaseID].ValidationHistory = append(e.Phases[phaseID].ValidationHistory, entry)
}

// GetLastValidationProto returns the most recent validation entry for the specified phase.
func GetLastValidationProto(e *orcv1.ExecutionState, phaseID string) *orcv1.ValidationEntry {
	if e == nil || e.Phases == nil || e.Phases[phaseID] == nil {
		return nil
	}
	history := e.Phases[phaseID].ValidationHistory
	if len(history) == 0 {
		return nil
	}
	return history[len(history)-1]
}

// GetErrorProto returns the error string, or empty if nil.
func GetErrorProto(e *orcv1.ExecutionState) string {
	if e == nil || e.Error == nil {
		return ""
	}
	return *e.Error
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

// GetJSONLPathProto returns the JSONL path, or empty if nil.
func GetJSONLPathProto(e *orcv1.ExecutionState) string {
	if e == nil || e.JsonlPath == nil {
		return ""
	}
	return *e.JsonlPath
}

// SetJSONLPathProto sets the JSONL path.
func SetJSONLPathProto(e *orcv1.ExecutionState, path string) {
	if e == nil {
		return
	}
	if path == "" {
		e.JsonlPath = nil
	} else {
		e.JsonlPath = &path
	}
}

// EffectiveInputTokensProto returns the total input context size including cached tokens.
func EffectiveInputTokensProto(t *orcv1.TokenUsage) int32 {
	if t == nil {
		return 0
	}
	return t.InputTokens + t.CacheCreationInputTokens + t.CacheReadInputTokens
}

// EffectiveTotalTokensProto returns the total tokens including cached inputs.
func EffectiveTotalTokensProto(t *orcv1.TokenUsage) int32 {
	if t == nil {
		return 0
	}
	return EffectiveInputTokensProto(t) + t.OutputTokens
}

// GetPhaseStatusProto returns the status of a phase, or PENDING if not found.
func GetPhaseStatusProto(e *orcv1.ExecutionState, phaseID string) orcv1.PhaseStatus {
	if e == nil || e.Phases == nil || e.Phases[phaseID] == nil {
		return orcv1.PhaseStatus_PHASE_STATUS_PENDING
	}
	return e.Phases[phaseID].Status
}

// GetPhaseIterationsProto returns the number of iterations for a phase.
func GetPhaseIterationsProto(e *orcv1.ExecutionState, phaseID string) int32 {
	if e == nil || e.Phases == nil || e.Phases[phaseID] == nil {
		return 0
	}
	return e.Phases[phaseID].Iterations
}

// AddArtifactProto adds an artifact to the specified phase.
func AddArtifactProto(e *orcv1.ExecutionState, phaseID, artifact string) {
	if e == nil || artifact == "" {
		return
	}
	EnsurePhaseProto(e, phaseID)
	e.Phases[phaseID].Artifacts = append(e.Phases[phaseID].Artifacts, artifact)
}

// GetPhaseArtifactsProto returns the artifacts for a phase.
func GetPhaseArtifactsProto(e *orcv1.ExecutionState, phaseID string) []string {
	if e == nil || e.Phases == nil || e.Phases[phaseID] == nil {
		return nil
	}
	return e.Phases[phaseID].Artifacts
}

// GetPhaseCommitSHAProto returns the commit SHA for a phase.
func GetPhaseCommitSHAProto(e *orcv1.ExecutionState, phaseID string) string {
	if e == nil || e.Phases == nil || e.Phases[phaseID] == nil || e.Phases[phaseID].CommitSha == nil {
		return ""
	}
	return *e.Phases[phaseID].CommitSha
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
