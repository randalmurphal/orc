// Package state provides execution state tracking for orc tasks.
// Note: File I/O functions have been removed. Use storage.Backend for persistence.
package state

import (
	"strings"
	"time"
)

const (
	// StateFileName is the filename for state YAML files
	StateFileName = "state.yaml"
)

// Status represents the execution status.
type Status string

const (
	StatusPending     Status = "pending"
	StatusRunning     Status = "running"
	StatusCompleted   Status = "completed"
	StatusFailed      Status = "failed"
	StatusPaused      Status = "paused"
	StatusInterrupted Status = "interrupted"
	StatusSkipped     Status = "skipped"
)

// State represents the execution state of a task.
type State struct {
	TaskID           string                 `yaml:"task_id" json:"task_id"`
	CurrentPhase     string                 `yaml:"current_phase" json:"current_phase"`
	CurrentIteration int                    `yaml:"current_iteration" json:"current_iteration"`
	Status           Status                 `yaml:"status" json:"status"`
	StartedAt        time.Time              `yaml:"started_at" json:"started_at"`
	UpdatedAt        time.Time              `yaml:"updated_at" json:"updated_at"`
	CompletedAt      *time.Time             `yaml:"completed_at,omitempty" json:"completed_at,omitempty"`
	Phases           map[string]*PhaseState `yaml:"phases" json:"phases"`
	Gates            []GateDecision         `yaml:"gates,omitempty" json:"gates,omitempty"`
	Tokens           TokenUsage             `yaml:"tokens" json:"tokens"`
	Cost             CostTracking           `yaml:"cost" json:"cost"`
	Session          *SessionInfo           `yaml:"session,omitempty" json:"session,omitempty"`
	Execution        *ExecutionInfo         `yaml:"execution,omitempty" json:"execution,omitempty"`
	Error            string                 `yaml:"error,omitempty" json:"error,omitempty"`
	RetryContext     *RetryContext          `yaml:"retry_context,omitempty" json:"retry_context,omitempty"`
	JSONLPath        string                 `yaml:"jsonl_path,omitempty" json:"jsonl_path,omitempty"` // Path to active Claude JSONL file
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

// ExecutionInfo tracks the process executing a task.
// Used for orphan detection when a task claims to be "running" but its executor has died.
type ExecutionInfo struct {
	// PID is the process ID of the executor
	PID int `yaml:"pid" json:"pid"`
	// Hostname identifies the machine running the executor (for distributed setups)
	Hostname string `yaml:"hostname" json:"hostname"`
	// StartedAt is when this execution began
	StartedAt time.Time `yaml:"started_at" json:"started_at"`
	// LastHeartbeat is the last time the executor updated state
	LastHeartbeat time.Time `yaml:"last_heartbeat" json:"last_heartbeat"`
}

// CostTracking tracks cost information for the task.
type CostTracking struct {
	TotalCostUSD  float64            `yaml:"total_cost_usd" json:"total_cost_usd"`
	PhaseCosts    map[string]float64 `yaml:"phase_costs,omitempty" json:"phase_costs,omitempty"`
	LastUpdatedAt time.Time          `yaml:"last_updated_at,omitempty" json:"last_updated_at,omitempty"`
}

// PhaseState represents the state of a single phase.
type PhaseState struct {
	Status            Status            `yaml:"status" json:"status"`
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

// GateDecision records a gate evaluation result.
type GateDecision struct {
	Phase     string    `yaml:"phase" json:"phase"`
	GateType  string    `yaml:"gate_type" json:"gate_type"`
	Approved  bool      `yaml:"approved" json:"approved"`
	Reason    string    `yaml:"reason,omitempty" json:"reason,omitempty"`
	Timestamp time.Time `yaml:"timestamp" json:"timestamp"`
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

// TokenUsage tracks token consumption.
type TokenUsage struct {
	InputTokens              int `yaml:"input_tokens" json:"input_tokens"`
	OutputTokens             int `yaml:"output_tokens" json:"output_tokens"`
	CacheCreationInputTokens int `yaml:"cache_creation_input_tokens,omitempty" json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int `yaml:"cache_read_input_tokens,omitempty" json:"cache_read_input_tokens,omitempty"`
	TotalTokens              int `yaml:"total_tokens" json:"total_tokens"`
}

// ValidationEntry records a single validation decision during phase execution.
// This is used to track Haiku validation results for pause/resume.
type ValidationEntry struct {
	Iteration int       `yaml:"iteration" json:"iteration"`
	Type      string    `yaml:"type" json:"type"`                         // "progress", "criteria", "backpressure"
	Decision  string    `yaml:"decision" json:"decision"`                 // "CONTINUE", "RETRY", "STOP"
	Reason    string    `yaml:"reason,omitempty" json:"reason,omitempty"` // Optional explanation
	Timestamp time.Time `yaml:"timestamp" json:"timestamp"`
}

// New creates a new state for a task.
func New(taskID string) *State {
	now := time.Now()
	return &State{
		TaskID:    taskID,
		Status:    StatusPending,
		StartedAt: now,
		UpdatedAt: now,
		Phases:    make(map[string]*PhaseState),
		Tokens:    TokenUsage{},
	}
}


// StartPhase marks a phase as started.
func (s *State) StartPhase(phaseID string) {
	now := time.Now()
	s.CurrentPhase = phaseID
	s.Status = StatusRunning
	s.UpdatedAt = now

	if s.Phases[phaseID] == nil {
		s.Phases[phaseID] = &PhaseState{}
	}

	s.Phases[phaseID].Status = StatusRunning
	s.Phases[phaseID].StartedAt = now
}

// CompletePhase marks a phase as completed.
func (s *State) CompletePhase(phaseID string, commitSHA string) {
	now := time.Now()
	s.UpdatedAt = now

	if s.Phases[phaseID] == nil {
		s.Phases[phaseID] = &PhaseState{}
	}

	s.Phases[phaseID].Status = StatusCompleted
	s.Phases[phaseID].CompletedAt = &now
	s.Phases[phaseID].CommitSHA = commitSHA
}

// FailPhase marks a phase as failed.
func (s *State) FailPhase(phaseID string, err error) {
	now := time.Now()
	s.Status = StatusFailed
	s.UpdatedAt = now
	s.Error = err.Error()

	if s.Phases[phaseID] == nil {
		s.Phases[phaseID] = &PhaseState{}
	}

	s.Phases[phaseID].Status = StatusFailed
	s.Phases[phaseID].Error = err.Error()
}

// InterruptPhase marks a phase as interrupted (can be resumed).
func (s *State) InterruptPhase(phaseID string) {
	now := time.Now()
	s.Status = StatusInterrupted
	s.UpdatedAt = now

	if s.Phases[phaseID] == nil {
		s.Phases[phaseID] = &PhaseState{}
	}

	s.Phases[phaseID].Status = StatusInterrupted
	s.Phases[phaseID].InterruptedAt = &now
}

// IncrementIteration increments the iteration count for the current phase.
func (s *State) IncrementIteration() {
	s.CurrentIteration++
	s.UpdatedAt = time.Now()

	if s.Phases[s.CurrentPhase] != nil {
		s.Phases[s.CurrentPhase].Iterations++
	}
}

// AddTokens adds token usage to the state.
// The input parameter should be the effective input (including cache tokens).
// cacheCreation and cacheRead track the individual cache token contributions.
func (s *State) AddTokens(input, output, cacheCreation, cacheRead int) {
	s.Tokens.InputTokens += input
	s.Tokens.OutputTokens += output
	s.Tokens.CacheCreationInputTokens += cacheCreation
	s.Tokens.CacheReadInputTokens += cacheRead
	s.Tokens.TotalTokens += input + output
	s.UpdatedAt = time.Now()

	if s.Phases[s.CurrentPhase] != nil {
		s.Phases[s.CurrentPhase].Tokens.InputTokens += input
		s.Phases[s.CurrentPhase].Tokens.OutputTokens += output
		s.Phases[s.CurrentPhase].Tokens.CacheCreationInputTokens += cacheCreation
		s.Phases[s.CurrentPhase].Tokens.CacheReadInputTokens += cacheRead
		s.Phases[s.CurrentPhase].Tokens.TotalTokens += input + output
	}
}

// RecordGateDecision records a gate evaluation result.
func (s *State) RecordGateDecision(phase, gateType string, approved bool, reason string) {
	s.Gates = append(s.Gates, GateDecision{
		Phase:     phase,
		GateType:  gateType,
		Approved:  approved,
		Reason:    reason,
		Timestamp: time.Now(),
	})
	s.UpdatedAt = time.Now()
}

// Complete marks the task as completed.
func (s *State) Complete() {
	now := time.Now()
	s.Status = StatusCompleted
	s.CompletedAt = &now
	s.UpdatedAt = now
}

// GetResumePhase returns the phase to resume from (first interrupted or running phase).
func (s *State) GetResumePhase() string {
	for phaseID, phaseState := range s.Phases {
		if phaseState.Status == StatusInterrupted || phaseState.Status == StatusRunning {
			return phaseID
		}
	}
	return ""
}

// IsPhaseCompleted returns true if a phase is completed or skipped.
// Both completed and skipped phases should not be re-executed.
func (s *State) IsPhaseCompleted(phaseID string) bool {
	ps, ok := s.Phases[phaseID]
	if !ok {
		return false
	}
	return ps.Status == StatusCompleted || ps.Status == StatusSkipped
}

// IsPhaseSkipped returns true if a phase was skipped.
func (s *State) IsPhaseSkipped(phaseID string) bool {
	ps, ok := s.Phases[phaseID]
	if !ok {
		return false
	}
	return ps.Status == StatusSkipped
}

// GetSkipReason returns the skip reason for a phase, if any.
func (s *State) GetSkipReason(phaseID string) string {
	ps, ok := s.Phases[phaseID]
	if !ok || ps.Status != StatusSkipped {
		return ""
	}
	// Skip reason is stored in the Error field with "skipped: " prefix
	if strings.HasPrefix(ps.Error, "skipped: ") {
		return strings.TrimPrefix(ps.Error, "skipped: ")
	}
	return ps.Error
}

// ResetPhase resets a phase to pending state for retry.
func (s *State) ResetPhase(phaseID string) {
	if s.Phases[phaseID] != nil {
		s.Phases[phaseID].Status = StatusPending
		s.Phases[phaseID].Error = ""
		s.Phases[phaseID].CompletedAt = nil
		s.Phases[phaseID].InterruptedAt = nil
	}
	s.UpdatedAt = time.Now()
}

// SetRetryContext sets the retry context for cross-phase retry.
func (s *State) SetRetryContext(fromPhase, toPhase, reason, failureOutput string, attempt int) {
	s.RetryContext = &RetryContext{
		FromPhase:     fromPhase,
		ToPhase:       toPhase,
		Reason:        reason,
		FailureOutput: failureOutput,
		Attempt:       attempt,
		Timestamp:     time.Now(),
	}
	s.UpdatedAt = time.Now()
}

// SetRetryContextFile sets the context file path for detailed retry context.
func (s *State) SetRetryContextFile(contextFile string) {
	if s.RetryContext != nil {
		s.RetryContext.ContextFile = contextFile
		s.UpdatedAt = time.Now()
	}
}

// ClearRetryContext clears the retry context after successful completion.
func (s *State) ClearRetryContext() {
	s.RetryContext = nil
	s.UpdatedAt = time.Now()
}

// Reset resets the entire state back to initial pending state.
// All phase progress, execution info, errors, and retry context are cleared.
func (s *State) Reset() {
	now := time.Now()

	// Clear all phase states
	for phaseID := range s.Phases {
		s.Phases[phaseID] = &PhaseState{
			Status: StatusPending,
		}
	}

	// Reset to initial state
	s.Status = StatusPending
	s.CurrentPhase = ""
	s.CurrentIteration = 0
	s.CompletedAt = nil
	s.Error = ""
	s.RetryContext = nil
	s.Execution = nil
	s.Session = nil
	s.Gates = nil
	s.UpdatedAt = now
}

// GetRetryContext returns the current retry context.
func (s *State) GetRetryContext() *RetryContext {
	return s.RetryContext
}

// HasRetryContext returns true if there is an active retry context.
func (s *State) HasRetryContext() bool {
	return s.RetryContext != nil
}

// SkipPhase marks a phase as skipped with an optional reason.
func (s *State) SkipPhase(phaseID string, reason string) {
	now := time.Now()
	s.UpdatedAt = now

	if s.Phases[phaseID] == nil {
		s.Phases[phaseID] = &PhaseState{}
	}

	s.Phases[phaseID].Status = StatusSkipped
	s.Phases[phaseID].CompletedAt = &now
	if reason != "" {
		s.Phases[phaseID].Error = "skipped: " + reason
	}

	// Record as a gate decision for audit trail
	s.RecordGateDecision(phaseID, "skip", true, reason)
}

// SetSession updates the session info for the task.
func (s *State) SetSession(id, model, status string, turnCount int) {
	now := time.Now()
	if s.Session == nil {
		s.Session = &SessionInfo{
			CreatedAt: now,
		}
	}
	s.Session.ID = id
	s.Session.Model = model
	s.Session.Status = status
	s.Session.TurnCount = turnCount
	s.Session.LastActivity = now
	s.UpdatedAt = now
}

// AddCost adds cost to the task and optionally to the current phase.
func (s *State) AddCost(costUSD float64) {
	s.Cost.TotalCostUSD += costUSD
	s.Cost.LastUpdatedAt = time.Now()

	if s.CurrentPhase != "" {
		if s.Cost.PhaseCosts == nil {
			s.Cost.PhaseCosts = make(map[string]float64)
		}
		s.Cost.PhaseCosts[s.CurrentPhase] += costUSD
	}
	s.UpdatedAt = time.Now()
}

// GetSessionID returns the global session ID if available.
// Deprecated: Use GetPhaseSessionID for phase-specific session tracking.
func (s *State) GetSessionID() string {
	if s.Session != nil {
		return s.Session.ID
	}
	return ""
}

// GetPhaseSessionID returns the session ID for a specific phase.
// This enables resuming the correct Claude session when retrying from an earlier phase.
func (s *State) GetPhaseSessionID(phaseID string) string {
	if ps, ok := s.Phases[phaseID]; ok && ps.SessionID != "" {
		return ps.SessionID
	}
	return ""
}

// SetPhaseSessionID stores the session ID for a specific phase.
func (s *State) SetPhaseSessionID(phaseID, sessionID string) {
	if s.Phases == nil {
		s.Phases = make(map[string]*PhaseState)
	}
	if s.Phases[phaseID] == nil {
		s.Phases[phaseID] = &PhaseState{}
	}
	s.Phases[phaseID].SessionID = sessionID
	s.UpdatedAt = time.Now()
}

// RecordValidation records a validation decision for the specified phase.
func (s *State) RecordValidation(phaseID string, entry ValidationEntry) {
	if s.Phases[phaseID] == nil {
		s.Phases[phaseID] = &PhaseState{}
	}
	s.Phases[phaseID].ValidationHistory = append(s.Phases[phaseID].ValidationHistory, entry)
	s.UpdatedAt = time.Now()
}

// GetLastValidation returns the most recent validation entry for the specified phase.
// Returns nil if no validations have been recorded.
func (s *State) GetLastValidation(phaseID string) *ValidationEntry {
	if s.Phases[phaseID] == nil {
		return nil
	}
	history := s.Phases[phaseID].ValidationHistory
	if len(history) == 0 {
		return nil
	}
	return &history[len(history)-1]
}

// StartExecution records that an executor process has started running this task.
// Also sets s.StartedAt if not already set (for fresh runs), preserving existing
// start time for resumed tasks to ensure Elapsed() returns the correct total duration.
func (s *State) StartExecution(pid int, hostname string) {
	now := time.Now()
	s.Execution = &ExecutionInfo{
		PID:           pid,
		Hostname:      hostname,
		StartedAt:     now,
		LastHeartbeat: now,
	}
	// Set task-level StartedAt if not already set (fresh runs)
	// For resumed tasks, preserve the original start time
	if s.StartedAt.IsZero() {
		s.StartedAt = now
	}
	s.UpdatedAt = now
}

// UpdateHeartbeat updates the last heartbeat timestamp for the execution.
func (s *State) UpdateHeartbeat() {
	if s.Execution != nil {
		s.Execution.LastHeartbeat = time.Now()
	}
	s.UpdatedAt = time.Now()
}

// ClearExecution removes execution tracking info (called on completion/failure).
func (s *State) ClearExecution() {
	s.Execution = nil
	s.UpdatedAt = time.Now()
}

// GetExecutorPID returns the PID of the executor if available.
func (s *State) GetExecutorPID() int {
	if s.Execution != nil {
		return s.Execution.PID
	}
	return 0
}

// Elapsed returns the duration since the task started, or 0 if StartedAt is not set.
// This safely handles cases where StartedAt is the zero value (e.g., when loaded
// from database without a start time), avoiding absurdly large durations.
func (s *State) Elapsed() time.Duration {
	if s.StartedAt.IsZero() {
		return 0
	}
	return time.Since(s.StartedAt)
}

