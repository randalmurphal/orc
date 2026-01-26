// Package executor provides the execution engine for orc.
// This file defines phase-related types that were previously in the plan package.
package executor

import (
	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/gate"
)

// PhaseDisplay represents phase information for display purposes.
// Used in CLI, API responses, and plan display.
type PhaseDisplay struct {
	ID        string           `yaml:"id" json:"id"`
	Name      string           `yaml:"name" json:"name"`
	Prompt    string           `yaml:"prompt,omitempty" json:"prompt,omitempty"`
	Status    orcv1.PhaseStatus `yaml:"status" json:"status"`
	CommitSHA string           `yaml:"commit_sha,omitempty" json:"commit_sha,omitempty"`
	Gate      gate.Gate        `yaml:"gate,omitempty" json:"gate,omitempty"`

	// Execution configuration
	MaxIterations int `yaml:"max_iterations,omitempty" json:"max_iterations,omitempty"`
}

// Plan represents an execution plan containing phases for display.
// Used in CLI, API responses, and orchestrator plan display.
type Plan struct {
	TaskID string         `yaml:"task_id" json:"task_id"`
	Phases []PhaseDisplay `yaml:"phases" json:"phases"`
}

// GetPhase returns a PhaseDisplay by ID, or nil if not found.
func (p *Plan) GetPhase(id string) *PhaseDisplay {
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

// CurrentPhase returns the first PhaseDisplay that is not completed or skipped.
// Returns nil if all phases are complete.
func (p *Plan) CurrentPhase() *PhaseDisplay {
	for i := range p.Phases {
		if p.Phases[i].Status != orcv1.PhaseStatus_PHASE_STATUS_COMPLETED && p.Phases[i].Status != orcv1.PhaseStatus_PHASE_STATUS_SKIPPED {
			return &p.Phases[i]
		}
	}
	return nil
}
