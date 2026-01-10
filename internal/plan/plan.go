// Package plan provides plan generation and management for orc.
package plan

import (
	"fmt"
	"os"
	"path/filepath"

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

// Errors
var (
	ErrNoTemplate = planError("no template found for weight")
	ErrNotFound   = planError("plan not found")
)

type planError string

func (e planError) Error() string { return string(e) }

// Load loads a plan from disk for a given task ID.
func Load(taskID string) (*Plan, error) {
	path := filepath.Join(task.OrcDir, task.TasksDir, taskID, PlanFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("plan for task %s: %w", taskID, ErrNotFound)
		}
		return nil, fmt.Errorf("read plan for task %s: %w", taskID, err)
	}

	var p Plan
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parse plan for task %s: %w", taskID, err)
	}

	return &p, nil
}

// Save persists the plan to disk.
func (p *Plan) Save(taskID string) error {
	dir := filepath.Join(task.OrcDir, task.TasksDir, taskID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create task directory: %w", err)
	}

	data, err := yaml.Marshal(p)
	if err != nil {
		return fmt.Errorf("marshal plan: %w", err)
	}

	path := filepath.Join(dir, PlanFileName)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write plan: %w", err)
	}

	return nil
}

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
