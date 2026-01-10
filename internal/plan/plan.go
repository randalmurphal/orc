// Package plan provides plan generation and management for orc.
package plan

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/randalmurphal/orc/internal/task"
	"gopkg.in/yaml.v3"
)

const (
	// PlanFileName is the filename for plan YAML files
	PlanFileName = "plan.yaml"
	// TemplatesDir is the directory containing plan templates
	TemplatesDir = "templates/plans"
)

// GateType represents the type of approval gate.
type GateType string

const (
	GateAuto  GateType = "auto"
	GateAI    GateType = "ai"
	GateHuman GateType = "human"
)

// PhaseStatus represents the execution state of a phase.
type PhaseStatus string

const (
	PhasePending   PhaseStatus = "pending"
	PhaseRunning   PhaseStatus = "running"
	PhaseCompleted PhaseStatus = "completed"
	PhaseFailed    PhaseStatus = "failed"
	PhaseSkipped   PhaseStatus = "skipped"
)

// Gate represents an approval gate for a phase.
type Gate struct {
	Type      GateType `yaml:"type" json:"type"`
	Criteria  []string `yaml:"criteria,omitempty" json:"criteria,omitempty"`
	Reviewers int      `yaml:"reviewers,omitempty" json:"reviewers,omitempty"`
}

// Phase represents a single phase in a plan.
type Phase struct {
	ID         string         `yaml:"id" json:"id"`
	Name       string         `yaml:"name" json:"name"`
	Prompt     string         `yaml:"prompt" json:"prompt"`
	DependsOn  []string       `yaml:"depends_on,omitempty" json:"depends_on,omitempty"`
	Gate       Gate           `yaml:"gate" json:"gate"`
	Checkpoint bool           `yaml:"checkpoint" json:"checkpoint"`
	Config     map[string]any `yaml:"config,omitempty" json:"config,omitempty"`
	Artifacts  []string       `yaml:"artifacts,omitempty" json:"artifacts,omitempty"`

	// Runtime state (not persisted in template)
	Status    PhaseStatus `yaml:"status,omitempty" json:"status,omitempty"`
	CommitSHA string      `yaml:"commit_sha,omitempty" json:"commit_sha,omitempty"`
}

// Plan represents the execution plan for a task.
type Plan struct {
	Version     int         `yaml:"version" json:"version"`
	TaskID      string      `yaml:"task_id" json:"task_id"`
	Weight      task.Weight `yaml:"weight" json:"weight"`
	Description string      `yaml:"description" json:"description"`
	Phases      []Phase     `yaml:"phases" json:"phases"`
}

// PlanTemplate represents a plan template for a weight class.
type PlanTemplate struct {
	Version     int         `yaml:"version" json:"version"`
	Weight      task.Weight `yaml:"weight" json:"weight"`
	Description string      `yaml:"description" json:"description"`
	Phases      []Phase     `yaml:"phases" json:"phases"`
}

// Generator creates plans from templates based on task weight.
type Generator struct {
	templates map[task.Weight]*PlanTemplate
}

// NewGenerator creates a new plan generator.
func NewGenerator() *Generator {
	return &Generator{
		templates: make(map[task.Weight]*PlanTemplate),
	}
}

// LoadTemplate loads a plan template for a weight class.
func (g *Generator) LoadTemplate(weight task.Weight, tmpl *PlanTemplate) {
	g.templates[weight] = tmpl
}

// Generate creates a plan for the given task.
func (g *Generator) Generate(t *task.Task) (*Plan, error) {
	tmpl, ok := g.templates[t.Weight]
	if !ok {
		// Fall back to medium if template not found
		tmpl = g.templates[task.WeightMedium]
	}

	if tmpl == nil {
		return nil, ErrNoTemplate
	}

	plan := &Plan{
		Version:     tmpl.Version,
		TaskID:      t.ID,
		Weight:      t.Weight,
		Description: tmpl.Description,
		Phases:      make([]Phase, len(tmpl.Phases)),
	}

	// Copy phases, initializing status
	for i, p := range tmpl.Phases {
		plan.Phases[i] = p
		plan.Phases[i].Status = PhasePending
	}

	return plan, nil
}

// GetPhase returns a phase by ID.
func (p *Plan) GetPhase(id string) *Phase {
	for i := range p.Phases {
		if p.Phases[i].ID == id {
			return &p.Phases[i]
		}
	}
	return nil
}

// CurrentPhase returns the first non-completed phase.
func (p *Plan) CurrentPhase() *Phase {
	for i := range p.Phases {
		if p.Phases[i].Status != PhaseCompleted && p.Phases[i].Status != PhaseSkipped {
			return &p.Phases[i]
		}
	}
	return nil
}

// IsComplete returns true if all phases are completed or skipped.
func (p *Plan) IsComplete() bool {
	for _, phase := range p.Phases {
		if phase.Status != PhaseCompleted && phase.Status != PhaseSkipped {
			return false
		}
	}
	return true
}

// Errors
var (
	ErrNoTemplate = planError("no template found for weight")
)

type planError string

func (e planError) Error() string { return string(e) }
