package workflow

import (
	"fmt"
	"time"

	"github.com/randalmurphal/orc/internal/db"
)

// Built-in phase template definitions.
// These are seeded into the database on first run.
var builtinPhaseTemplates = []db.PhaseTemplate{
	{
		ID:               "spec",
		Name:             "Specification",
		Description:      "Generate technical specification with user stories and success criteria",
		PromptSource:     "embedded",
		PromptPath:       "prompts/spec.md",
		InputVariables:   `["TASK_DESCRIPTION", "TASK_CATEGORY", "INITIATIVE_CONTEXT"]`,
		ProducesArtifact: true,
		ArtifactType:     "spec",
		MaxIterations:    20,
		GateType:         "auto",
		Checkpoint:       true,
		IsBuiltin:        true,
	},
	{
		ID:               "tiny_spec",
		Name:             "Lightweight Spec",
		Description:      "Combined spec and TDD plan for trivial/small tasks",
		PromptSource:     "embedded",
		PromptPath:       "prompts/tiny_spec.md",
		InputVariables:   `["TASK_DESCRIPTION", "TASK_CATEGORY"]`,
		ProducesArtifact: true,
		ArtifactType:     "spec",
		MaxIterations:    10,
		GateType:         "auto",
		Checkpoint:       true,
		IsBuiltin:        true,
	},
	{
		ID:               "tdd_write",
		Name:             "TDD - Write Tests",
		Description:      "Write failing tests before implementation (TDD-first)",
		PromptSource:     "embedded",
		PromptPath:       "prompts/tdd_write.md",
		InputVariables:   `["SPEC_CONTENT"]`,
		ProducesArtifact: true,
		ArtifactType:     "tests",
		MaxIterations:    20,
		GateType:         "auto",
		Checkpoint:       true,
		RetryFromPhase:   "spec",
		IsBuiltin:        true,
	},
	{
		ID:               "breakdown",
		Name:             "Task Breakdown",
		Description:      "Break spec into checkboxed implementation tasks",
		PromptSource:     "embedded",
		PromptPath:       "prompts/breakdown.md",
		InputVariables:   `["SPEC_CONTENT", "TDD_TESTS_CONTENT"]`,
		ProducesArtifact: true,
		ArtifactType:     "breakdown",
		MaxIterations:    10,
		GateType:         "auto",
		Checkpoint:       true,
		IsBuiltin:        true,
	},
	{
		ID:               "implement",
		Name:             "Implementation",
		Description:      "Write code guided by breakdown, make tests pass",
		PromptSource:     "embedded",
		PromptPath:       "prompts/implement.md",
		InputVariables:   `["SPEC_CONTENT", "TDD_TESTS_CONTENT", "BREAKDOWN_CONTENT"]`,
		ProducesArtifact: false,
		MaxIterations:    50,
		GateType:         "auto",
		Checkpoint:       true,
		RetryFromPhase:   "breakdown",
		IsBuiltin:        true,
	},
	{
		ID:               "review",
		Name:             "Code Review",
		Description:      "Multi-agent code review with specialized reviewers",
		PromptSource:     "embedded",
		PromptPath:       "prompts/review.md",
		InputVariables:   `["SPEC_CONTENT", "REVIEW_ROUND", "REVIEW_FINDINGS"]`,
		ProducesArtifact: false,
		MaxIterations:    3,
		GateType:         "auto",
		Checkpoint:       true,
		IsBuiltin:        true,
	},
	{
		ID:               "docs",
		Name:             "Documentation",
		Description:      "Update or create documentation",
		PromptSource:     "embedded",
		PromptPath:       "prompts/docs.md",
		InputVariables:   `["SPEC_CONTENT"]`,
		ProducesArtifact: true,
		ArtifactType:     "docs",
		MaxIterations:    10,
		GateType:         "auto",
		Checkpoint:       true,
		IsBuiltin:        true,
	},
	{
		ID:               "validate",
		Name:             "Validation",
		Description:      "Verify implementation against success criteria",
		PromptSource:     "embedded",
		PromptPath:       "prompts/validate.md",
		InputVariables:   `["SPEC_CONTENT"]`,
		ProducesArtifact: false,
		MaxIterations:    5,
		GateType:         "auto",
		Checkpoint:       false,
		IsBuiltin:        true,
	},
	{
		ID:               "qa",
		Name:             "QA Session",
		Description:      "Manual QA verification session",
		PromptSource:     "embedded",
		PromptPath:       "prompts/qa.md",
		InputVariables:   `["SPEC_CONTENT"]`,
		ProducesArtifact: false,
		MaxIterations:    10,
		GateType:         "human",
		Checkpoint:       false,
		IsBuiltin:        true,
	},
	{
		ID:               "research",
		Name:             "Research",
		Description:      "Research patterns and approaches",
		PromptSource:     "embedded",
		PromptPath:       "prompts/research.md",
		InputVariables:   `["TASK_DESCRIPTION"]`,
		ProducesArtifact: true,
		ArtifactType:     "research",
		MaxIterations:    10,
		GateType:         "auto",
		Checkpoint:       true,
		IsBuiltin:        true,
	},
	{
		ID:               "design",
		Name:             "Design",
		Description:      "Create design document",
		PromptSource:     "embedded",
		PromptPath:       "prompts/design.md",
		InputVariables:   `["SPEC_CONTENT"]`,
		ProducesArtifact: true,
		ArtifactType:     "design",
		MaxIterations:    10,
		GateType:         "auto",
		Checkpoint:       true,
		IsBuiltin:        true,
	},
}

// Built-in workflow definitions.
// These match the phase sequences expected by each task weight.
var builtinWorkflows = []struct {
	Workflow db.Workflow
	Phases   []db.WorkflowPhase
}{
	{
		// Large weight: full workflow with breakdown and validate
		Workflow: db.Workflow{
			ID:           "implement-large",
			Name:         "Implement (Large)",
			Description:  "Full implementation workflow: spec, TDD, breakdown, implement, review, docs, validate",
			WorkflowType: "task",
			IsBuiltin:    true,
		},
		Phases: []db.WorkflowPhase{
			{PhaseTemplateID: "spec", Sequence: 0, DependsOn: "[]"},
			{PhaseTemplateID: "tdd_write", Sequence: 1, DependsOn: `["spec"]`},
			{PhaseTemplateID: "breakdown", Sequence: 2, DependsOn: `["tdd_write"]`},
			{PhaseTemplateID: "implement", Sequence: 3, DependsOn: `["breakdown"]`},
			{PhaseTemplateID: "review", Sequence: 4, DependsOn: `["implement"]`},
			{PhaseTemplateID: "docs", Sequence: 5, DependsOn: `["review"]`},
			{PhaseTemplateID: "validate", Sequence: 6, DependsOn: `["docs"]`},
		},
	},
	{
		// Medium weight: spec, TDD, implement, review, docs (no breakdown, no validate)
		Workflow: db.Workflow{
			ID:           "implement-medium",
			Name:         "Implement (Medium)",
			Description:  "Standard implementation workflow: spec, TDD, implement, review, docs",
			WorkflowType: "task",
			IsBuiltin:    true,
		},
		Phases: []db.WorkflowPhase{
			{PhaseTemplateID: "spec", Sequence: 0, DependsOn: "[]"},
			{PhaseTemplateID: "tdd_write", Sequence: 1, DependsOn: `["spec"]`},
			{PhaseTemplateID: "implement", Sequence: 2, DependsOn: `["tdd_write"]`},
			{PhaseTemplateID: "review", Sequence: 3, DependsOn: `["implement"]`},
			{PhaseTemplateID: "docs", Sequence: 4, DependsOn: `["review"]`},
		},
	},
	{
		// Small weight: tiny_spec, implement, review
		Workflow: db.Workflow{
			ID:           "implement-small",
			Name:         "Implement (Small)",
			Description:  "Lightweight workflow: tiny_spec, implement, review",
			WorkflowType: "task",
			IsBuiltin:    true,
		},
		Phases: []db.WorkflowPhase{
			{PhaseTemplateID: "tiny_spec", Sequence: 0, DependsOn: "[]"},
			{PhaseTemplateID: "implement", Sequence: 1, DependsOn: `["tiny_spec"]`},
			{PhaseTemplateID: "review", Sequence: 2, DependsOn: `["implement"]`},
		},
	},
	{
		// Trivial weight: tiny_spec, implement
		Workflow: db.Workflow{
			ID:           "implement-trivial",
			Name:         "Implement (Trivial)",
			Description:  "Minimal workflow: tiny_spec, implement",
			WorkflowType: "task",
			IsBuiltin:    true,
		},
		Phases: []db.WorkflowPhase{
			{PhaseTemplateID: "tiny_spec", Sequence: 0, DependsOn: "[]"},
			{PhaseTemplateID: "implement", Sequence: 1, DependsOn: `["tiny_spec"]`},
		},
	},
	{
		Workflow: db.Workflow{
			ID:           "review",
			Name:         "Review Only",
			Description:  "Code review workflow for existing changes",
			WorkflowType: "branch",
			IsBuiltin:    true,
		},
		Phases: []db.WorkflowPhase{
			{PhaseTemplateID: "review", Sequence: 0, DependsOn: "[]"},
		},
	},
	{
		Workflow: db.Workflow{
			ID:           "spec",
			Name:         "Spec Only",
			Description:  "Generate specification without implementation",
			WorkflowType: "standalone",
			IsBuiltin:    true,
		},
		Phases: []db.WorkflowPhase{
			{PhaseTemplateID: "spec", Sequence: 0, DependsOn: "[]"},
		},
	},
	{
		Workflow: db.Workflow{
			ID:           "docs",
			Name:         "Documentation",
			Description:  "Documentation update workflow",
			WorkflowType: "branch",
			IsBuiltin:    true,
		},
		Phases: []db.WorkflowPhase{
			{PhaseTemplateID: "docs", Sequence: 0, DependsOn: "[]"},
		},
	},
	{
		Workflow: db.Workflow{
			ID:           "qa",
			Name:         "QA Session",
			Description:  "Manual QA verification session",
			WorkflowType: "branch",
			IsBuiltin:    true,
		},
		Phases: []db.WorkflowPhase{
			{PhaseTemplateID: "qa", Sequence: 0, DependsOn: "[]"},
		},
	},
}

// SeedBuiltins populates the database with built-in phase templates and workflows.
// This should be called during database initialization.
// Returns the number of items seeded (templates + workflows).
func SeedBuiltins(pdb *db.ProjectDB) (int, error) {
	now := time.Now()
	seeded := 0

	// Seed phase templates
	for _, pt := range builtinPhaseTemplates {
		// Check if already exists
		existing, err := pdb.GetPhaseTemplate(pt.ID)
		if err != nil {
			return seeded, fmt.Errorf("check phase template %s: %w", pt.ID, err)
		}
		if existing != nil {
			continue // Already seeded
		}

		pt.CreatedAt = now
		pt.UpdatedAt = now
		if err := pdb.SavePhaseTemplate(&pt); err != nil {
			return seeded, fmt.Errorf("seed phase template %s: %w", pt.ID, err)
		}
		seeded++
	}

	// Seed workflows and their phases
	for _, bw := range builtinWorkflows {
		// Check if workflow already exists
		existing, err := pdb.GetWorkflow(bw.Workflow.ID)
		if err != nil {
			return seeded, fmt.Errorf("check workflow %s: %w", bw.Workflow.ID, err)
		}
		if existing != nil {
			continue // Already seeded
		}

		// Create workflow
		workflow := bw.Workflow
		workflow.CreatedAt = now
		workflow.UpdatedAt = now
		if err := pdb.SaveWorkflow(&workflow); err != nil {
			return seeded, fmt.Errorf("seed workflow %s: %w", workflow.ID, err)
		}
		seeded++

		// Create workflow phases
		for _, phase := range bw.Phases {
			phase.WorkflowID = workflow.ID
			if err := pdb.SaveWorkflowPhase(&phase); err != nil {
				return seeded, fmt.Errorf("seed workflow phase %s/%s: %w", workflow.ID, phase.PhaseTemplateID, err)
			}
		}
	}

	return seeded, nil
}

// GetWorkflowForWeight returns the appropriate built-in workflow for a task weight.
func GetWorkflowForWeight(weight string) string {
	switch weight {
	case "trivial":
		return "implement-trivial"
	case "small":
		return "implement-small"
	case "medium":
		return "implement-medium"
	case "large":
		return "implement-large"
	default:
		return "implement-medium" // Default to medium workflow
	}
}

// GetWeightForWorkflow returns the task weight for a built-in implement workflow.
// Returns empty string for non-implement workflows or unknown workflows.
// This is the inverse of GetWorkflowForWeight.
func GetWeightForWorkflow(workflowID string) string {
	switch workflowID {
	case "implement-trivial":
		return "trivial"
	case "implement-small":
		return "small"
	case "implement-medium":
		return "medium"
	case "implement-large":
		return "large"
	case "implement":
		return "large" // Full implement workflow is large weight
	default:
		return "" // Non-implement workflows don't have a weight
	}
}

// ListBuiltinWorkflowIDs returns all built-in workflow IDs.
func ListBuiltinWorkflowIDs() []string {
	ids := make([]string, len(builtinWorkflows))
	for i, bw := range builtinWorkflows {
		ids[i] = bw.Workflow.ID
	}
	return ids
}

// ListBuiltinPhaseIDs returns all built-in phase template IDs.
func ListBuiltinPhaseIDs() []string {
	ids := make([]string, len(builtinPhaseTemplates))
	for i, pt := range builtinPhaseTemplates {
		ids[i] = pt.ID
	}
	return ids
}
