// Package plan provides plan generation and management for orc.
// Note: File I/O functions have been removed. Use storage.Backend for persistence.
package plan

import (
	"fmt"

	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/templates"
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

// Reset resets all phases back to pending state.
// All phase status and commit SHAs are cleared.
func (p *Plan) Reset() {
	for i := range p.Phases {
		p.Phases[i].Status = PhasePending
		p.Phases[i].CommitSHA = ""
	}
}

// Errors
var (
	ErrNoTemplate = planError("no template found for weight")
	ErrNotFound   = planError("plan not found")
)

type planError string

func (e planError) Error() string { return string(e) }


// LoadTemplate loads a plan template for a given weight class from embedded files.
func LoadTemplate(weight task.Weight) (*PlanTemplate, error) {
	filename := "plans/" + string(weight) + ".yaml"
	data, err := templates.Plans.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("template for weight %s: %w", weight, ErrNoTemplate)
	}

	var tmpl PlanTemplate
	if err := yaml.Unmarshal(data, &tmpl); err != nil {
		return nil, fmt.Errorf("parse template for weight %s: %w", weight, err)
	}

	return &tmpl, nil
}

// CreateFromTemplate creates a plan for a task from its weight template.
func CreateFromTemplate(t *task.Task) (*Plan, error) {
	tmpl, err := LoadTemplate(t.Weight)
	if err != nil {
		return nil, err
	}

	p := &Plan{
		Version:     tmpl.Version,
		TaskID:      t.ID,
		Weight:      t.Weight,
		Description: tmpl.Description,
		Phases:      make([]Phase, len(tmpl.Phases)),
	}

	// Copy phases and initialize status
	for i, phase := range tmpl.Phases {
		p.Phases[i] = phase
		p.Phases[i].Status = PhasePending
	}

	return p, nil
}

// RegenerateResult contains the result of a plan regeneration.
type RegenerateResult struct {
	// NewPlan is the regenerated plan
	NewPlan *Plan
	// PreservedPhases lists phases whose status was preserved
	PreservedPhases []string
	// ResetPhases lists phases that were reset to pending
	ResetPhases []string
}

// RegeneratePlan creates a new plan for a task based on its current weight,
// preserving completed/skipped statuses for phases that exist in both old and new plans.
// This is used when the task weight changes.
func RegeneratePlan(t *task.Task, oldPlan *Plan) (*RegenerateResult, error) {
	// Create new plan from template
	newPlan, err := CreateFromTemplate(t)
	if err != nil {
		// If template not found, create default plan
		newPlan = &Plan{
			Version:     1,
			TaskID:      t.ID,
			Weight:      t.Weight,
			Description: "Default plan",
			Phases: []Phase{
				{ID: "implement", Name: "implement", Gate: Gate{Type: GateAuto}, Status: PhasePending},
			},
		}
	}

	result := &RegenerateResult{
		NewPlan: newPlan,
	}

	// If no old plan, everything is new
	if oldPlan == nil {
		for _, phase := range newPlan.Phases {
			result.ResetPhases = append(result.ResetPhases, phase.ID)
		}
		return result, nil
	}

	// Build a map of old phase statuses for quick lookup
	oldPhaseStatus := make(map[string]PhaseStatus)
	oldPhaseCommits := make(map[string]string)
	for _, phase := range oldPlan.Phases {
		oldPhaseStatus[phase.ID] = phase.Status
		oldPhaseCommits[phase.ID] = phase.CommitSHA
	}

	// Preserve completed/skipped statuses for phases that exist in both plans
	for i := range newPlan.Phases {
		phaseID := newPlan.Phases[i].ID
		oldStatus, exists := oldPhaseStatus[phaseID]

		if exists && (oldStatus == PhaseCompleted || oldStatus == PhaseSkipped) {
			// Preserve completed/skipped status
			newPlan.Phases[i].Status = oldStatus
			newPlan.Phases[i].CommitSHA = oldPhaseCommits[phaseID]
			result.PreservedPhases = append(result.PreservedPhases, phaseID)
		} else {
			// Reset to pending (already set by CreateFromTemplate, but track it)
			result.ResetPhases = append(result.ResetPhases, phaseID)
		}
	}

	return result, nil
}

