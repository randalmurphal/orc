// Package task provides task management for orc.
// This file contains execution state types that track how a task runs.
package task

import (
	"strings"
	"time"

	"github.com/randalmurphal/orc/internal/db"
)

// PhaseStatus represents the execution status of a phase.
// This is distinct from task.Status which tracks the overall task state.
type PhaseStatus string

const (
	PhaseStatusPending     PhaseStatus = "pending"
	PhaseStatusRunning     PhaseStatus = "running"
	PhaseStatusCompleted   PhaseStatus = "completed"
	PhaseStatusFailed      PhaseStatus = "failed"
	PhaseStatusPaused      PhaseStatus = "paused"
	PhaseStatusInterrupted PhaseStatus = "interrupted"
	PhaseStatusSkipped     PhaseStatus = "skipped"
	PhaseStatusBlocked     PhaseStatus = "blocked"
)

// PhaseState represents the state of a single phase execution.
type PhaseState struct {
	Status            PhaseStatus       `yaml:"status" json:"status"`
	StartedAt         time.Time         `yaml:"started_at,omitempty" json:"started_at,omitempty"`
	CompletedAt       *time.Time        `yaml:"completed_at,omitempty" json:"completed_at,omitempty"`
	InterruptedAt     *time.Time        `yaml:"interrupted_at,omitempty" json:"interrupted_at,omitempty"`
	Iterations        int               `yaml:"iterations" json:"iterations"`
	CommitSHA         string            `yaml:"commit_sha,omitempty" json:"commit_sha,omitempty"`
	Artifacts         []string          `yaml:"artifacts,omitempty" json:"artifacts,omitempty"`
	Error             string            `yaml:"error,omitempty" json:"error,omitempty"`
	Tokens            TokenUsage        `yaml:"tokens" json:"tokens"`
	ValidationHistory []ValidationEntry `yaml:"validation_history,omitempty" json:"validation_history,omitempty"`
	// SessionID is the Claude CLI session UUID for this phase (for --resume)
	SessionID string `yaml:"session_id,omitempty" json:"session_id,omitempty"`
}

// TokenUsage tracks token consumption.
type TokenUsage struct {
	InputTokens              int `yaml:"input_tokens" json:"input_tokens"`
	OutputTokens             int `yaml:"output_tokens" json:"output_tokens"`
	CacheCreationInputTokens int `yaml:"cache_creation_input_tokens,omitempty" json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int `yaml:"cache_read_input_tokens,omitempty" json:"cache_read_input_tokens,omitempty"`
	TotalTokens              int `yaml:"total_tokens" json:"total_tokens"`
}

// EffectiveInputTokens returns the total input context size including cached tokens.
// Use this instead of raw InputTokens to get the actual context window usage.
func (t TokenUsage) EffectiveInputTokens() int {
	return t.InputTokens + t.CacheCreationInputTokens + t.CacheReadInputTokens
}

// EffectiveTotalTokens returns the total tokens including cached inputs.
func (t TokenUsage) EffectiveTotalTokens() int {
	return t.EffectiveInputTokens() + t.OutputTokens
}

// CostTracking tracks cost information for the task.
type CostTracking struct {
	TotalCostUSD  float64            `yaml:"total_cost_usd" json:"total_cost_usd"`
	PhaseCosts    map[string]float64 `yaml:"phase_costs,omitempty" json:"phase_costs,omitempty"`
	LastUpdatedAt time.Time          `yaml:"last_updated_at,omitempty" json:"last_updated_at,omitempty"`
}

// GateDecision records a gate evaluation result.
type GateDecision struct {
	Phase     string    `yaml:"phase" json:"phase"`
	GateType  string    `yaml:"gate_type" json:"gate_type"`
	Approved  bool      `yaml:"approved" json:"approved"`
	Reason    string    `yaml:"reason,omitempty" json:"reason,omitempty"`
	Timestamp time.Time `yaml:"timestamp" json:"timestamp"`
}

// SessionInfo tracks the Claude session associated with the task.
type SessionInfo struct {
	ID           string    `yaml:"id" json:"id"`
	Model        string    `yaml:"model,omitempty" json:"model,omitempty"`
	Status       string    `yaml:"status" json:"status"`
	CreatedAt    time.Time `yaml:"created_at" json:"created_at"`
	LastActivity time.Time `yaml:"last_activity" json:"last_activity"`
	TurnCount    int       `yaml:"turn_count" json:"turn_count"`
}

// RetryContext captures why a phase is being retried.
type RetryContext struct {
	// FromPhase is the phase that failed/rejected
	FromPhase string `yaml:"from_phase" json:"from_phase"`
	// ToPhase is the phase we're retrying from
	ToPhase string `yaml:"to_phase" json:"to_phase"`
	// Reason is a summary of why the retry is happening
	Reason string `yaml:"reason" json:"reason"`
	// FailureOutput is the relevant output from the failed phase
	FailureOutput string `yaml:"failure_output,omitempty" json:"failure_output,omitempty"`
	// ContextFile is path to detailed context file
	ContextFile string `yaml:"context_file,omitempty" json:"context_file,omitempty"`
	// Attempt is which retry attempt this is
	Attempt int `yaml:"attempt" json:"attempt"`
	// Timestamp is when the retry was triggered
	Timestamp time.Time `yaml:"timestamp" json:"timestamp"`
}

// ValidationEntry records a single validation decision during phase execution.
// This is used to track Haiku validation results for pause/resume.
type ValidationEntry struct {
	Iteration int       `yaml:"iteration" json:"iteration"`
	Type      string    `yaml:"type" json:"type"`                         // "progress", "criteria", "quality_check"
	Decision  string    `yaml:"decision" json:"decision"`                 // "CONTINUE", "RETRY", "STOP"
	Reason    string    `yaml:"reason,omitempty" json:"reason,omitempty"` // Optional explanation
	Timestamp time.Time `yaml:"timestamp" json:"timestamp"`
}

// ExecutionState contains all execution-related state for a task.
// This is embedded in Task to consolidate task metadata and execution state.
type ExecutionState struct {
	// CurrentIteration tracks the iteration count within the current phase
	CurrentIteration int `yaml:"current_iteration" json:"current_iteration"`

	// Phases tracks per-phase execution state
	Phases map[string]*PhaseState `yaml:"phases" json:"phases"`

	// Gates records gate evaluation results
	Gates []GateDecision `yaml:"gates,omitempty" json:"gates,omitempty"`

	// Tokens tracks aggregate token usage
	Tokens TokenUsage `yaml:"tokens" json:"tokens"`

	// Cost tracks cost information
	Cost CostTracking `yaml:"cost" json:"cost"`

	// Session tracks the Claude session info
	Session *SessionInfo `yaml:"session,omitempty" json:"session,omitempty"`

	// Error stores the last error message
	Error string `yaml:"error,omitempty" json:"error,omitempty"`

	// RetryContext captures why a phase is being retried
	RetryContext *RetryContext `yaml:"retry_context,omitempty" json:"retry_context,omitempty"`

	// JSONLPath is the path to active Claude JSONL file
	JSONLPath string `yaml:"jsonl_path,omitempty" json:"jsonl_path,omitempty"`
}

// InitExecutionState initializes an empty ExecutionState.
func InitExecutionState() ExecutionState {
	return ExecutionState{
		Phases: make(map[string]*PhaseState),
		Tokens: TokenUsage{},
	}
}

// StartPhase marks a phase as started.
func (e *ExecutionState) StartPhase(phaseID string) {
	now := time.Now()

	if e.Phases == nil {
		e.Phases = make(map[string]*PhaseState)
	}
	if e.Phases[phaseID] == nil {
		e.Phases[phaseID] = &PhaseState{}
	}

	e.Phases[phaseID].Status = PhaseStatusRunning
	e.Phases[phaseID].StartedAt = now
}

// CompletePhase marks a phase as completed.
func (e *ExecutionState) CompletePhase(phaseID string, commitSHA string) {
	now := time.Now()

	if e.Phases == nil {
		e.Phases = make(map[string]*PhaseState)
	}
	if e.Phases[phaseID] == nil {
		e.Phases[phaseID] = &PhaseState{}
	}

	e.Phases[phaseID].Status = PhaseStatusCompleted
	e.Phases[phaseID].CompletedAt = &now
	e.Phases[phaseID].CommitSHA = commitSHA
}

// FailPhase marks a phase as failed.
func (e *ExecutionState) FailPhase(phaseID string, err error) {
	e.Error = err.Error()

	if e.Phases == nil {
		e.Phases = make(map[string]*PhaseState)
	}
	if e.Phases[phaseID] == nil {
		e.Phases[phaseID] = &PhaseState{}
	}

	e.Phases[phaseID].Status = PhaseStatusFailed
	e.Phases[phaseID].Error = err.Error()
}

// InterruptPhase marks a phase as interrupted (can be resumed).
func (e *ExecutionState) InterruptPhase(phaseID string) {
	now := time.Now()

	if e.Phases == nil {
		e.Phases = make(map[string]*PhaseState)
	}
	if e.Phases[phaseID] == nil {
		e.Phases[phaseID] = &PhaseState{}
	}

	e.Phases[phaseID].Status = PhaseStatusInterrupted
	e.Phases[phaseID].InterruptedAt = &now
}

// SkipPhase marks a phase as skipped with an optional reason.
func (e *ExecutionState) SkipPhase(phaseID string, reason string) {
	now := time.Now()

	if e.Phases == nil {
		e.Phases = make(map[string]*PhaseState)
	}
	if e.Phases[phaseID] == nil {
		e.Phases[phaseID] = &PhaseState{}
	}

	e.Phases[phaseID].Status = PhaseStatusSkipped
	e.Phases[phaseID].CompletedAt = &now
	if reason != "" {
		e.Phases[phaseID].Error = "skipped: " + reason
	}

	// Record as a gate decision for audit trail
	e.RecordGateDecision(phaseID, "skip", true, reason)
}

// IncrementIteration increments the iteration count for the current phase.
func (e *ExecutionState) IncrementIteration(currentPhase string) {
	e.CurrentIteration++

	if e.Phases != nil && e.Phases[currentPhase] != nil {
		e.Phases[currentPhase].Iterations++
	}
}

// AddTokens adds token usage to the state.
func (e *ExecutionState) AddTokens(currentPhase string, input, output, cacheCreation, cacheRead int) {
	e.Tokens.InputTokens += input
	e.Tokens.OutputTokens += output
	e.Tokens.CacheCreationInputTokens += cacheCreation
	e.Tokens.CacheReadInputTokens += cacheRead
	e.Tokens.TotalTokens += input + output

	if e.Phases != nil && e.Phases[currentPhase] != nil {
		e.Phases[currentPhase].Tokens.InputTokens += input
		e.Phases[currentPhase].Tokens.OutputTokens += output
		e.Phases[currentPhase].Tokens.CacheCreationInputTokens += cacheCreation
		e.Phases[currentPhase].Tokens.CacheReadInputTokens += cacheRead
		e.Phases[currentPhase].Tokens.TotalTokens += input + output
	}
}

// RecordGateDecision records a gate evaluation result.
func (e *ExecutionState) RecordGateDecision(phase, gateType string, approved bool, reason string) {
	e.Gates = append(e.Gates, GateDecision{
		Phase:     phase,
		GateType:  gateType,
		Approved:  approved,
		Reason:    reason,
		Timestamp: time.Now(),
	})
}

// GetResumePhase returns the phase to resume from (first interrupted or running phase).
func (e *ExecutionState) GetResumePhase() string {
	for phaseID, phaseState := range e.Phases {
		if phaseState.Status == PhaseStatusInterrupted || phaseState.Status == PhaseStatusRunning {
			return phaseID
		}
	}
	return ""
}

// IsPhaseCompleted returns true if a phase is completed or skipped.
func (e *ExecutionState) IsPhaseCompleted(phaseID string) bool {
	ps, ok := e.Phases[phaseID]
	if !ok {
		return false
	}
	return ps.Status == PhaseStatusCompleted || ps.Status == PhaseStatusSkipped
}

// IsPhaseSkipped returns true if a phase was skipped.
func (e *ExecutionState) IsPhaseSkipped(phaseID string) bool {
	ps, ok := e.Phases[phaseID]
	if !ok {
		return false
	}
	return ps.Status == PhaseStatusSkipped
}

// GetSkipReason returns the skip reason for a phase, if any.
func (e *ExecutionState) GetSkipReason(phaseID string) string {
	ps, ok := e.Phases[phaseID]
	if !ok || ps.Status != PhaseStatusSkipped {
		return ""
	}
	if strings.HasPrefix(ps.Error, "skipped: ") {
		return strings.TrimPrefix(ps.Error, "skipped: ")
	}
	return ps.Error
}

// ResetPhase resets a phase to pending state for retry.
func (e *ExecutionState) ResetPhase(phaseID string) {
	if e.Phases != nil && e.Phases[phaseID] != nil {
		e.Phases[phaseID].Status = PhaseStatusPending
		e.Phases[phaseID].Error = ""
		e.Phases[phaseID].CompletedAt = nil
		e.Phases[phaseID].InterruptedAt = nil
	}
}

// SetRetryContext sets the retry context for cross-phase retry.
func (e *ExecutionState) SetRetryContext(fromPhase, toPhase, reason, failureOutput string, attempt int) {
	e.RetryContext = &RetryContext{
		FromPhase:     fromPhase,
		ToPhase:       toPhase,
		Reason:        reason,
		FailureOutput: failureOutput,
		Attempt:       attempt,
		Timestamp:     time.Now(),
	}
}

// SetRetryContextFile sets the context file path for detailed retry context.
func (e *ExecutionState) SetRetryContextFile(contextFile string) {
	if e.RetryContext != nil {
		e.RetryContext.ContextFile = contextFile
	}
}

// ClearRetryContext clears the retry context after successful completion.
func (e *ExecutionState) ClearRetryContext() {
	e.RetryContext = nil
}

// Reset resets the entire execution state back to initial pending state.
func (e *ExecutionState) Reset() {
	// Clear all phase states
	for phaseID := range e.Phases {
		e.Phases[phaseID] = &PhaseState{
			Status: PhaseStatusPending,
		}
	}

	// Reset to initial state
	e.CurrentIteration = 0
	e.Error = ""
	e.RetryContext = nil
	e.Session = nil
	e.Gates = nil
}

// GetRetryContext returns the current retry context.
func (e *ExecutionState) GetRetryContext() *RetryContext {
	return e.RetryContext
}

// HasRetryContext returns true if there is an active retry context.
func (e *ExecutionState) HasRetryContext() bool {
	return e.RetryContext != nil
}

// SetSession updates the session info.
func (e *ExecutionState) SetSession(id, model, status string, turnCount int) {
	now := time.Now()
	if e.Session == nil {
		e.Session = &SessionInfo{
			CreatedAt: now,
		}
	}
	e.Session.ID = id
	e.Session.Model = model
	e.Session.Status = status
	e.Session.TurnCount = turnCount
	e.Session.LastActivity = now
}

// AddCost adds cost to the task and optionally to the current phase.
func (e *ExecutionState) AddCost(currentPhase string, costUSD float64) {
	e.Cost.TotalCostUSD += costUSD
	e.Cost.LastUpdatedAt = time.Now()

	if currentPhase != "" {
		if e.Cost.PhaseCosts == nil {
			e.Cost.PhaseCosts = make(map[string]float64)
		}
		e.Cost.PhaseCosts[currentPhase] += costUSD
	}
}

// GetPhaseSessionID returns the session ID for a specific phase.
func (e *ExecutionState) GetPhaseSessionID(phaseID string) string {
	if ps, ok := e.Phases[phaseID]; ok && ps.SessionID != "" {
		return ps.SessionID
	}
	return ""
}

// SetPhaseSessionID stores the session ID for a specific phase.
func (e *ExecutionState) SetPhaseSessionID(phaseID, sessionID string) {
	if e.Phases == nil {
		e.Phases = make(map[string]*PhaseState)
	}
	if e.Phases[phaseID] == nil {
		e.Phases[phaseID] = &PhaseState{}
	}
	e.Phases[phaseID].SessionID = sessionID
}

// RecordValidation records a validation decision for the specified phase.
func (e *ExecutionState) RecordValidation(phaseID string, entry ValidationEntry) {
	if e.Phases == nil {
		e.Phases = make(map[string]*PhaseState)
	}
	if e.Phases[phaseID] == nil {
		e.Phases[phaseID] = &PhaseState{}
	}
	e.Phases[phaseID].ValidationHistory = append(e.Phases[phaseID].ValidationHistory, entry)
}

// GetLastValidation returns the most recent validation entry for the specified phase.
func (e *ExecutionState) GetLastValidation(phaseID string) *ValidationEntry {
	if e.Phases == nil || e.Phases[phaseID] == nil {
		return nil
	}
	history := e.Phases[phaseID].ValidationHistory
	if len(history) == 0 {
		return nil
	}
	return &history[len(history)-1]
}

// ============================================================================
// GateDecision conversion functions (domain <-> persistence)
// ============================================================================

// ToDB converts task.GateDecision to db.GateDecision for persistence.
func (g GateDecision) ToDB(taskID string) *db.GateDecision {
	return &db.GateDecision{
		TaskID:    taskID,
		Phase:     g.Phase,
		GateType:  g.GateType,
		Approved:  g.Approved,
		Reason:    g.Reason,
		DecidedAt: g.Timestamp,
	}
}

// GateDecisionFromDB converts db.GateDecision to task.GateDecision.
func GateDecisionFromDB(d *db.GateDecision) GateDecision {
	return GateDecision{
		Phase:     d.Phase,
		GateType:  d.GateType,
		Approved:  d.Approved,
		Reason:    d.Reason,
		Timestamp: d.DecidedAt,
	}
}

// GateDecisionsFromDB converts a slice of db.GateDecision to task.GateDecision.
func GateDecisionsFromDB(dbDecisions []db.GateDecision) []GateDecision {
	if len(dbDecisions) == 0 {
		return nil
	}
	result := make([]GateDecision, len(dbDecisions))
	for i, d := range dbDecisions {
		result[i] = GateDecisionFromDB(&d)
	}
	return result
}
