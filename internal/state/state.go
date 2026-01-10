// Package state provides execution state tracking for orc tasks.
package state

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/randalmurphal/orc/internal/task"
	"gopkg.in/yaml.v3"
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
	Error            string                 `yaml:"error,omitempty" json:"error,omitempty"`
	RetryContext     *RetryContext          `yaml:"retry_context,omitempty" json:"retry_context,omitempty"`
}

// PhaseState represents the state of a single phase.
type PhaseState struct {
	Status        Status     `yaml:"status" json:"status"`
	StartedAt     time.Time  `yaml:"started_at,omitempty" json:"started_at,omitempty"`
	CompletedAt   *time.Time `yaml:"completed_at,omitempty" json:"completed_at,omitempty"`
	InterruptedAt *time.Time `yaml:"interrupted_at,omitempty" json:"interrupted_at,omitempty"`
	Iterations    int        `yaml:"iterations" json:"iterations"`
	CommitSHA     string     `yaml:"commit_sha,omitempty" json:"commit_sha,omitempty"`
	Artifacts     []string   `yaml:"artifacts,omitempty" json:"artifacts,omitempty"`
	Error         string     `yaml:"error,omitempty" json:"error,omitempty"`
	Tokens        TokenUsage `yaml:"tokens" json:"tokens"`
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
	FromPhase string `yaml:"from_phase"`
	// ToPhase is the phase we're retrying from
	ToPhase string `yaml:"to_phase"`
	// Reason is a summary of why the retry is happening
	Reason string `yaml:"reason"`
	// FailureOutput is the relevant output from the failed phase
	FailureOutput string `yaml:"failure_output,omitempty"`
	// ContextFile is path to detailed context file
	ContextFile string `yaml:"context_file,omitempty"`
	// Attempt is which retry attempt this is
	Attempt int `yaml:"attempt"`
	// Timestamp is when the retry was triggered
	Timestamp time.Time `yaml:"timestamp"`
}

// TokenUsage tracks token consumption.
type TokenUsage struct {
	InputTokens  int `yaml:"input_tokens" json:"input_tokens"`
	OutputTokens int `yaml:"output_tokens" json:"output_tokens"`
	TotalTokens  int `yaml:"total_tokens" json:"total_tokens"`
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

// Load loads state from disk for a given task ID.
func Load(taskID string) (*State, error) {
	path := filepath.Join(task.OrcDir, task.TasksDir, taskID, StateFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Check if the task exists - if so, return empty state; otherwise error
			if task.Exists(taskID) {
				return New(taskID), nil
			}
			return nil, fmt.Errorf("task %s not found", taskID)
		}
		return nil, fmt.Errorf("read state for task %s: %w", taskID, err)
	}

	var s State
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse state for task %s: %w", taskID, err)
	}

	// Ensure phases map is initialized
	if s.Phases == nil {
		s.Phases = make(map[string]*PhaseState)
	}

	return &s, nil
}

// Save persists the state to disk.
func (s *State) Save() error {
	dir := filepath.Join(task.OrcDir, task.TasksDir, s.TaskID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create task directory: %w", err)
	}

	s.UpdatedAt = time.Now()

	data, err := yaml.Marshal(s)
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}

	path := filepath.Join(dir, StateFileName)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write state: %w", err)
	}

	return nil
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
func (s *State) AddTokens(input, output int) {
	s.Tokens.InputTokens += input
	s.Tokens.OutputTokens += output
	s.Tokens.TotalTokens += input + output
	s.UpdatedAt = time.Now()

	if s.Phases[s.CurrentPhase] != nil {
		s.Phases[s.CurrentPhase].Tokens.InputTokens += input
		s.Phases[s.CurrentPhase].Tokens.OutputTokens += output
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

// IsPhaseCompleted returns true if a phase is completed.
func (s *State) IsPhaseCompleted(phaseID string) bool {
	ps, ok := s.Phases[phaseID]
	if !ok {
		return false
	}
	return ps.Status == StatusCompleted
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
