package workflow

import (
	"fmt"
	"time"

	"github.com/randalmurphal/orc/internal/db"
)

// DefaultCodeQualityChecks is the JSON for standard code quality checks.
// Applied to the implement phase to run tests, lint, build, and typecheck after code changes.
const DefaultCodeQualityChecks = `[{"type":"code","name":"tests","enabled":true,"on_failure":"block"},{"type":"code","name":"lint","enabled":true,"on_failure":"block"},{"type":"code","name":"build","enabled":true,"on_failure":"block"},{"type":"code","name":"typecheck","enabled":true,"on_failure":"block"}]`

// boolPtr is a helper to create a pointer to a bool.
func boolPtr(b bool) *bool { return &b }

// Built-in phase template definitions.
// These are seeded into the database on first run.
// Model defaults: opus for most phases, sonnet for test-heavy phases.
// Thinking enabled for decision phases that benefit from deep reasoning.
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
		OutputVarName:    "SPEC_CONTENT",
		OutputType:       "document",
		MaxIterations:    20,
		ModelOverride:    "opus",
		ThinkingEnabled:  boolPtr(true), // Decision phase: needs deep reasoning
		GateType:         "auto",
		Checkpoint:       true,
		ClaudeConfig:     `{"disallowed_tools": ["Write", "Edit", "NotebookEdit"]}`, // Read-only: planning, not writing
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
		OutputVarName:    "SPEC_CONTENT",
		OutputType:       "document",
		MaxIterations:    10,
		ModelOverride:    "opus",
		ThinkingEnabled:  boolPtr(false), // Short task, no extended thinking needed
		GateType:         "auto",
		Checkpoint:       true,
		ClaudeConfig:     `{"disallowed_tools": ["Write", "Edit"]}`, // Read-only: planning, not writing
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
		OutputVarName:    "TDD_TESTS_CONTENT",
		OutputType:       "tests",
		MaxIterations:    20,
		ModelOverride:    "opus",
		ThinkingEnabled:  boolPtr(false), // Execution phase
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
		OutputVarName:    "BREAKDOWN_CONTENT",
		OutputType:       "document",
		MaxIterations:    10,
		ModelOverride:    "opus",
		ThinkingEnabled:  boolPtr(false), // Execution phase
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
		OutputType:       "code",
		QualityChecks:    DefaultCodeQualityChecks,
		MaxIterations:    50,
		ModelOverride:    "opus",
		ThinkingEnabled:  boolPtr(false), // Execution phase
		GateType:         "auto",
		Checkpoint:       true,
		RetryFromPhase:   "breakdown",
		ClaudeConfig:     `{"append_system_prompt": "Sub-agents available: code-simplifier (for medium/large tasks). After completing implementation and passing all quality checks, delegate to code-simplifier to clean up recently modified code before marking complete."}`,
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
		OutputType:       "none",
		MaxIterations:    3,
		ModelOverride:    "opus",
		ThinkingEnabled:  boolPtr(true), // Decision phase: code quality judgment
		GateType:         "auto",
		Checkpoint:       true,
		ClaudeConfig:     `{"disallowed_tools": ["Write", "Edit", "NotebookEdit"], "append_system_prompt": "Sub-agents available for specialized review: code-reviewer (guidelines), silent-failure-hunter (error handling), pr-test-analyzer (test coverage), comment-analyzer (documentation), type-design-analyzer (type design). Delegate to relevant agents based on task weight - small tasks use fewer agents, large tasks use all. Synthesize findings from all agents into cohesive review output."}`,
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
		OutputVarName:    "DOCS_CONTENT",
		OutputType:       "document",
		MaxIterations:    10,
		ModelOverride:    "opus",
		ThinkingEnabled:  boolPtr(false), // Execution phase
		GateType:         "auto",
		Checkpoint:       true,
		ClaudeConfig:     `{"disallowed_tools": ["Bash"]}`, // Docs don't need shell commands
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
		OutputType:       "none",
		MaxIterations:    10,
		ModelOverride:    "sonnet", // QA is more mechanical, sonnet is sufficient
		ThinkingEnabled:  boolPtr(false),
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
		OutputVarName:    "RESEARCH_CONTENT",
		OutputType:       "research",
		MaxIterations:    10,
		ModelOverride:    "opus",
		ThinkingEnabled:  boolPtr(true), // Research needs deep reasoning
		GateType:         "auto",
		Checkpoint:       true,
		ClaudeConfig:     `{"disallowed_tools": ["Write", "Edit", "NotebookEdit"]}`, // Read-only: research, not writing
		IsBuiltin:        true,
	},
	// ==========================================================================
	// QA E2E Testing Phases (Browser-based testing with Playwright MCP)
	// ==========================================================================
	{
		ID:               "qa_e2e_test",
		Name:             "E2E QA Testing",
		Description:      "Browser-based E2E testing with Playwright MCP",
		PromptSource:     "embedded",
		PromptPath:       "prompts/qa_e2e_test.md",
		InputVariables:   `["SPEC_CONTENT", "WORKTREE_PATH", "BEFORE_IMAGES", "PREVIOUS_FINDINGS", "QA_ITERATION", "QA_MAX_ITERATIONS"]`,
		ProducesArtifact: true,
		ArtifactType:     "qa_findings",
		OutputVarName:    "QA_FINDINGS",
		OutputType:       "findings",
		MaxIterations:    20,
		ModelOverride:    "sonnet", // QA is systematic, sonnet sufficient
		ThinkingEnabled:  boolPtr(false),
		GateType:         "auto", // Default to auto, --gate flag changes to human
		Checkpoint:       false,
		ClaudeConfig: `{
			"allowed_tools": [
				"mcp__playwright__browser_navigate",
				"mcp__playwright__browser_click",
				"mcp__playwright__browser_type",
				"mcp__playwright__browser_take_screenshot",
				"mcp__playwright__browser_snapshot",
				"mcp__playwright__browser_resize",
				"mcp__playwright__browser_console_messages",
				"mcp__playwright__browser_wait_for",
				"mcp__playwright__browser_evaluate",
				"mcp__playwright__browser_press_key",
				"Read", "Write", "Glob", "Grep"
			],
			"disallowed_tools": ["Edit", "NotebookEdit"],
			"mcp_servers": {
				"playwright": {
					"command": "npx",
					"args": ["@playwright/mcp@latest", "--isolated"]
				}
			},
			"append_system_prompt": "Sub-agents available: qa-functional (happy path + edge cases), qa-visual (visual regression), qa-accessibility (a11y audit). Delegate based on test scope. Report only findings with confidence >= 80."
		}`,
		IsBuiltin: true,
	},
	{
		ID:               "qa_e2e_fix",
		Name:             "E2E QA Fix",
		Description:      "Investigate and fix issues found by QA E2E testing",
		PromptSource:     "embedded",
		PromptPath:       "prompts/qa_e2e_fix.md",
		InputVariables:   `["SPEC_CONTENT", "QA_FINDINGS", "QA_ITERATION", "QA_MAX_ITERATIONS"]`,
		ProducesArtifact: false,
		OutputType:       "code",
		QualityChecks:    DefaultCodeQualityChecks,
		MaxIterations:    30,
		ModelOverride:    "opus", // Fixing requires deeper reasoning
		ThinkingEnabled:  boolPtr(true),
		GateType:         "auto",
		Checkpoint:       true,
		RetryFromPhase:   "qa_e2e_test", // On failure, re-run QA testing
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
		// Large weight: full workflow with breakdown and multi-agent review
		Workflow: db.Workflow{
			ID:           "implement-large",
			Name:         "Implement (Large)",
			Description:  "Full implementation workflow: spec, TDD, breakdown, implement, review, docs",
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
		// Trivial weight: implement only (no spec, no TDD - just do the work)
		Workflow: db.Workflow{
			ID:           "implement-trivial",
			Name:         "Implement (Trivial)",
			Description:  "Direct implementation: short tests + build, no spec overhead",
			WorkflowType: "task",
			IsBuiltin:    true,
		},
		Phases: []db.WorkflowPhase{
			{
				PhaseTemplateID: "implement",
				Sequence:        0,
				DependsOn:       "[]",
				// Trivial tasks use short tests, lint, and build
				// use_short uses project_commands.short_command which is language-agnostic
				QualityChecksOverride: `[{"type":"code","name":"tests","enabled":true,"use_short":true,"on_failure":"block"},{"type":"code","name":"lint","enabled":true,"on_failure":"block"},{"type":"code","name":"build","enabled":true,"on_failure":"block"}]`,
			},
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
	{
		// QA E2E: Browser-based testing with iterative fix loop
		// Run via: orc qa TASK-XXX or orc run --workflow qa-e2e TASK-XXX
		Workflow: db.Workflow{
			ID:           "qa-e2e",
			Name:         "E2E QA Testing",
			Description:  "Browser-based E2E testing with automatic fix loop. Run via 'orc qa TASK-XXX'.",
			WorkflowType: "branch", // Works on existing branches (task must have worktree)
			IsBuiltin:    true,
		},
		Phases: []db.WorkflowPhase{
			{PhaseTemplateID: "qa_e2e_test", Sequence: 0, DependsOn: "[]"},
			{
				PhaseTemplateID: "qa_e2e_fix",
				Sequence:        1,
				DependsOn:       `["qa_e2e_test"]`,
				// Loop back to qa_e2e_test after fixing, max 3 iterations
				LoopConfig: `{"condition":"has_findings","loop_to_phase":"qa_e2e_test","max_iterations":3}`,
			},
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

// MigratePhaseTemplateModels updates existing builtin phase templates with model settings
// and Claude configuration. This should be called on startup to ensure existing databases
// have the latest defaults.
// Returns the number of templates updated.
func MigratePhaseTemplateModels(pdb *db.ProjectDB) (int, error) {
	now := time.Now()
	updated := 0

	for _, builtin := range builtinPhaseTemplates {
		existing, err := pdb.GetPhaseTemplate(builtin.ID)
		if err != nil {
			return updated, fmt.Errorf("get phase template %s: %w", builtin.ID, err)
		}
		if existing == nil {
			continue // Not seeded yet, will be handled by SeedBuiltins
		}

		// Only update if this is a builtin template
		if !existing.IsBuiltin {
			continue // Don't touch user-created templates
		}

		needsUpdate := false

		// Update model if not set (empty or different from builtin)
		if existing.ModelOverride == "" && builtin.ModelOverride != "" {
			existing.ModelOverride = builtin.ModelOverride
			needsUpdate = true
		}

		// Update thinking if not set
		if existing.ThinkingEnabled == nil && builtin.ThinkingEnabled != nil {
			existing.ThinkingEnabled = builtin.ThinkingEnabled
			needsUpdate = true
		}

		// Update ClaudeConfig if not set
		if existing.ClaudeConfig == "" && builtin.ClaudeConfig != "" {
			existing.ClaudeConfig = builtin.ClaudeConfig
			needsUpdate = true
		}

		if needsUpdate {
			existing.UpdatedAt = now
			if err := pdb.SavePhaseTemplate(existing); err != nil {
				return updated, fmt.Errorf("update phase template %s: %w", existing.ID, err)
			}
			updated++
		}
	}

	return updated, nil
}
