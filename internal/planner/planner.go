package planner

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/initiative"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/internal/workflow"
)

// Options configures the planner.
type Options struct {
	// SpecDir is the directory containing spec files (default: .spec/)
	SpecDir string

	// Include patterns for spec files (default: *.md)
	Include []string

	// WorkDir is the project directory
	WorkDir string

	// Model is the Claude model to use
	Model string

	// InitiativeID links created tasks to an existing initiative
	InitiativeID string

	// CreateInitiative creates a new initiative for the tasks
	CreateInitiative bool

	// DryRun shows the prompt without running Claude
	DryRun bool

	// BatchMode creates tasks without confirmation
	BatchMode bool

	// Backend is the storage backend for tasks and initiatives
	Backend storage.Backend
}

// CreationResult tracks a created task.
type CreationResult struct {
	TaskID    string   `yaml:"task_id" json:"task_id"`
	Title     string   `yaml:"title" json:"title"`
	Weight    string   `yaml:"weight" json:"weight"`
	DependsOn []string `yaml:"depends_on,omitempty" json:"depends_on,omitempty"`
}

// Planner handles spec-to-task planning.
type Planner struct {
	opts   Options
	loader *SpecLoader
}

// New creates a new planner.
func New(opts Options) *Planner {
	if opts.SpecDir == "" {
		opts.SpecDir = ".spec"
	}
	if opts.Model == "" {
		opts.Model = "sonnet"
	}

	return &Planner{
		opts:   opts,
		loader: NewSpecLoader(opts.SpecDir, opts.Include),
	}
}

// LoadSpecs loads specification files.
func (p *Planner) LoadSpecs() ([]*SpecFile, error) {
	return p.loader.Load()
}

// GeneratePrompt generates the planning prompt.
func (p *Planner) GeneratePrompt(files []*SpecFile) (string, error) {
	data := &PromptData{
		ProjectName: ProjectNameFromPath(p.opts.WorkDir),
		ProjectPath: p.opts.WorkDir,
	}

	// Load initiative context if specified
	if p.opts.InitiativeID != "" && p.opts.Backend != nil {
		init, err := p.opts.Backend.LoadInitiative(p.opts.InitiativeID)
		if err == nil {
			data.InitiativeID = init.ID
			data.InitiativeTitle = init.Title
			data.InitiativeVision = init.Vision
			// Format decisions
			var decisions []string
			for _, d := range init.Decisions {
				decisions = append(decisions, fmt.Sprintf("- %s: %s", d.Decision, d.Rationale))
			}
			data.InitiativeDecisions = strings.Join(decisions, "\n")
		}
	}

	return GeneratePrompt(files, data)
}

// RunClaude runs Claude with the planning prompt and returns the response.
func (p *Planner) RunClaude(ctx context.Context, prompt string) (string, error) {
	// Build command
	args := []string{
		"--print",
		"-p", prompt,
		"--model", p.opts.Model,
		"--dangerously-skip-permissions",
	}

	cmd := exec.CommandContext(ctx, "claude", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("claude command failed: %w\nOutput: %s", err, string(output))
	}

	return string(output), nil
}

// ParseResponse parses Claude's response into a task breakdown.
func (p *Planner) ParseResponse(response string) (*TaskBreakdown, error) {
	breakdown, err := ParseTaskBreakdown(response)
	if err != nil {
		return nil, err
	}

	if err := ValidateDependencies(breakdown); err != nil {
		return nil, fmt.Errorf("invalid dependencies: %w", err)
	}

	return breakdown, nil
}

// CreateTasks creates tasks from the breakdown.
func (p *Planner) CreateTasks(breakdown *TaskBreakdown) ([]CreationResult, error) {
	if p.opts.Backend == nil {
		return nil, fmt.Errorf("backend is required for creating tasks")
	}

	// Map index to created task ID
	indexToID := make(map[int]string)
	var results []CreationResult

	for _, proposed := range breakdown.Tasks {
		// Generate task ID
		id, err := p.opts.Backend.GetNextTaskID()
		if err != nil {
			return nil, fmt.Errorf("generate task ID: %w", err)
		}

		// Create task using proto type
		t := task.NewProtoTask(id, proposed.Title)
		t.Description = &proposed.Description
		t.Weight = task.WeightToProto(string(proposed.Weight))
		t.Status = orcv1.TaskStatus_TASK_STATUS_PLANNED // Plans are created dynamically at runtime

		// Auto-assign workflow based on weight
		wfID := workflow.WeightToWorkflowID(t.Weight)
		if wfID != "" {
			t.WorkflowId = &wfID
		}

		// Save task
		if err := p.opts.Backend.SaveTask(t); err != nil {
			return nil, fmt.Errorf("save task %s: %w", id, err)
		}

		// Map dependency indices to IDs
		var depIDs []string
		for _, depIdx := range proposed.DependsOn {
			if depID, ok := indexToID[depIdx]; ok {
				depIDs = append(depIDs, depID)
			}
		}

		// Track mapping
		indexToID[proposed.Index] = id
		results = append(results, CreationResult{
			TaskID:    id,
			Title:     proposed.Title,
			Weight:    string(proposed.Weight),
			DependsOn: depIDs,
		})
	}

	// Add to initiative if specified
	if p.opts.InitiativeID != "" {
		init, err := p.opts.Backend.LoadInitiative(p.opts.InitiativeID)
		if err != nil {
			return nil, fmt.Errorf("load initiative: %w", err)
		}

		for _, r := range results {
			init.AddTask(r.TaskID, r.Title, r.DependsOn)
		}

		if err := p.opts.Backend.SaveInitiative(init); err != nil {
			return nil, fmt.Errorf("save initiative: %w", err)
		}
	}

	// Create new initiative if requested
	if p.opts.CreateInitiative && len(results) > 0 {
		// Generate initiative ID
		initID, err := p.opts.Backend.GetNextInitiativeID()
		if err != nil {
			return nil, fmt.Errorf("generate initiative ID: %w", err)
		}

		// Create initiative
		initTitle := fmt.Sprintf("Tasks from %s", p.opts.SpecDir)
		init := initiative.New(initID, initTitle)
		init.Vision = breakdown.Summary

		for _, r := range results {
			init.AddTask(r.TaskID, r.Title, r.DependsOn)
		}

		if err := p.opts.Backend.SaveInitiative(init); err != nil {
			return nil, fmt.Errorf("save initiative: %w", err)
		}

		fmt.Printf("Created initiative: %s\n", init.ID)
	}

	return results, nil
}
