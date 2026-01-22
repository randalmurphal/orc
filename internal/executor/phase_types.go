// Package executor provides the execution engine for orc.
// This file defines phase-related types that were previously in the plan package.
package executor

import "github.com/randalmurphal/orc/internal/gate"

// PhaseStatus represents the execution status of a phase.
type PhaseStatus string

const (
	// PhasePending indicates the phase has not started.
	PhasePending PhaseStatus = "pending"
	// PhaseRunning indicates the phase is currently executing.
	PhaseRunning PhaseStatus = "running"
	// PhaseCompleted indicates the phase completed successfully.
	PhaseCompleted PhaseStatus = "completed"
	// PhaseFailed indicates the phase failed.
	PhaseFailed PhaseStatus = "failed"
	// PhaseSkipped indicates the phase was skipped.
	PhaseSkipped PhaseStatus = "skipped"
	// PhaseBlocked indicates the phase is blocked awaiting input.
	PhaseBlocked PhaseStatus = "blocked"
)

// Phase represents a phase definition for execution.
type Phase struct {
	ID        string      `yaml:"id" json:"id"`
	Name      string      `yaml:"name" json:"name"`
	Prompt    string      `yaml:"prompt,omitempty" json:"prompt,omitempty"`
	Status    PhaseStatus `yaml:"status" json:"status"`
	CommitSHA string      `yaml:"commit_sha,omitempty" json:"commit_sha,omitempty"`
	Gate      gate.Gate   `yaml:"gate,omitempty" json:"gate,omitempty"`

	// Execution configuration
	MaxIterations int `yaml:"max_iterations,omitempty" json:"max_iterations,omitempty"`
}

// Plan represents an execution plan containing phases.
type Plan struct {
	TaskID string  `yaml:"task_id" json:"task_id"`
	Phases []Phase `yaml:"phases" json:"phases"`
}

// GetPhase returns a phase by ID, or nil if not found.
func (p *Plan) GetPhase(id string) *Phase {
	for i := range p.Phases {
		if p.Phases[i].ID == id {
			return &p.Phases[i]
		}
	}
	return nil
}

// GetPhaseIndex returns the index of a phase by ID, or -1 if not found.
func (p *Plan) GetPhaseIndex(id string) int {
	for i := range p.Phases {
		if p.Phases[i].ID == id {
			return i
		}
	}
	return -1
}

// CurrentPhase returns the first phase that is not completed or skipped.
// Returns nil if all phases are complete.
func (p *Plan) CurrentPhase() *Phase {
	for i := range p.Phases {
		if p.Phases[i].Status != PhaseCompleted && p.Phases[i].Status != PhaseSkipped {
			return &p.Phases[i]
		}
	}
	return nil
}
