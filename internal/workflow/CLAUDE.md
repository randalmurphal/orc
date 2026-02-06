# Workflow Package

Configurable workflow system for orc. Replaces weight-based task execution with composable, database-first workflows.

## Overview

| File | Purpose |
|------|---------|
| `types.go` | Domain types (Workflow, PhaseTemplate, WorkflowRun, etc.) |
| `defaults.go` | Workflow defaults system with hierarchical resolution |
| `seed.go` | Built-in workflow/phase definitions and seeding |
| `seed_agents.go` | Agent definitions, parsing, seeding to GlobalDB |

## Core Concepts

### Phase Templates (Lego Blocks)

Reusable phase definitions with:
- Prompt configuration (embedded, DB, or file)
- Input/output contract (what variables it needs/produces)
- Execution config (iterations, model, gate type)
- Quality checks (JSON array of check definitions)
- Retry configuration

### YAML→DB Sync Pipeline

Phase templates defined in `templates/phases/*.yaml` flow through three layers:

```
phaseYAML (resolver.go) → workflow.PhaseTemplate (types.go) → db.PhaseTemplate (db/workflow.go)
         parsePhaseYAML()                     workflowPhaseToDBPhase()
```

**All YAML fields must be mapped in both conversion functions.** Missing mappings silently drop data (the field exists in YAML and DB but the intermediate domain type doesn't carry it). Fields currently synced: ID, Name, Description, AgentID, SubAgents, PromptSource/Path/Content, InputVariables, OutputSchema, OutputVarName, OutputType, ProducesArtifact, ArtifactType, QualityChecks, ThinkingEnabled, GateType, Checkpoint, RetryFromPhase, RetryPromptPath, ClaudeConfig.

### Workflows

Compose phases into execution plans:
- Ordered sequence of phases
- Per-phase overrides (gate, condition, retries)
- Custom variables
- Context type (task, branch, PR, standalone)
- Completion action override (`types.go:196`)

| Field | Values | Purpose |
|-------|--------|---------|
| `completion_action` | `pr`, `commit`, `none`, `""` | Override config completion action; empty inherits from config |

### Phase Conditions (`executor/condition.go:29`)

JSON condition string on `WorkflowPhase.Condition`. Empty/null = always run.

**Simple condition format:**
```json
{"field": "task.weight", "op": "eq", "value": "medium"}
```

**Compound condition format:**
```json
{"all": [{"field": "...", "op": "...", "value": "..."}]}
{"any": [{"field": "...", "op": "...", "value": "..."}]}
```

| Operator | Value Type | Purpose |
|----------|------------|---------|
| `eq` | string | Field equals value |
| `neq` | string | Field not equals value |
| `in` | string[] | Field in array of values |
| `contains` | string | Field contains substring |
| `exists` | - | Field is non-empty |
| `gt` | string | Greater than (numeric/lexical) |
| `lt` | string | Less than (numeric/lexical) |

| Field Prefix | Example | Source |
|--------------|---------|--------|
| `task.` | `task.weight`, `task.category` | Task properties |
| `var.` | `var.MY_VAR` | Workflow variables |
| `env.` | `env.CI` | Environment variables |
| `phase_output.` | `phase_output.spec` | Prior phase artifact |

Evaluated by `executor.EvaluateCondition()`. UI: `web/src/components/workflows/ConditionEditor.tsx`.

### Extended Gate Configuration

Types for AI gates, triggers, and gate input/output control. All in `types.go`.

| Type | Values | Purpose |
|------|--------|---------|
| `GateType` | `auto`, `human`, `skip`, `ai` | Phase gate approval type |
| `GateMode` | `gate`, `reaction` | Sync (blocking) vs async (fire-and-forget) |
| `GateAction` | `continue`, `retry`, `fail`, `skip_phase`, `run_script` | Gate outcome actions |
| `WorkflowTriggerEvent` | `on_task_created`, `on_task_completed`, `on_task_failed`, `on_initiative_planned` | Lifecycle events |

| Struct | Purpose | Used By |
|--------|---------|---------|
| `GateInputConfig` | Context passed to gate evaluator | Phase templates, triggers |
| `GateOutputConfig` | Result handling (actions, variables) | Phase templates, triggers |
| `BeforePhaseTrigger` | Pre-phase validation agent | `WorkflowPhase.BeforeTriggers` |
| `WorkflowTrigger` | Lifecycle event handler | `Workflow.Triggers` |

### Workflow Runs

Execution instances:
- Universal tracking anchor (replaces task-centric approach)
- Can attach to task, branch, PR, or run standalone
- Tracks all phases, metrics, artifacts

## Workflow Defaults System

Hierarchical workflow resolution with smart weight-based defaults. Implemented in `defaults.go`.

### Resolution Hierarchy

Workflows resolved in order of precedence:

1. **User Override** - Explicit `--workflow` flag or config override
2. **Project Config** - `.orc/config.yaml` workflow mappings
3. **Built-in Mapping** - Weight-based defaults (see table below)
4. **Fallback** - `implement-trivial` if all else fails

### Weight-Based Default Mapping

| Weight | Default Workflow | Phases |
|--------|-----------------|--------|
| `trivial` | `implement-trivial` | implement |
| `small` | `implement-small` | tiny_spec → implement → review → docs |
| `medium` | `implement-medium` | spec → tdd_write → tdd_integrate → implement → review → docs |
| `large` | `implement-large` | spec → tdd_write → tdd_integrate → breakdown → implement → review → docs |

### Default Workflow Features

All default workflows include:

- **Smart Gates**: Auto gates for implementation, human gates for review
- **Retry Configuration**: Built-in retry logic for failed phases
- **Variable Resolution**: Standard variables like `{{TASK_DESCRIPTION}}`, `{{SUCCESS_CRITERIA}}`
- **Context Preservation**: Phase outputs flow correctly between steps

### Configuration Override

Override weight-to-workflow mapping in project config:

```yaml
# .orc/config.yaml
workflows:
  weight_mapping:
    small: "my-custom-workflow"
    medium: "enhanced-workflow"
```

### Usage

```go
// Get workflow for task weight
workflow, err := workflow.GetDefaultWorkflow(weight)

// With config override
resolver := NewResolver(config)
workflow, err := resolver.ResolveWorkflow(weight, userOverride)
```

## Built-in Workflows

| ID | Phases | Use Case |
|----|--------|----------|
| `implement-large` | spec → tdd_write → tdd_integrate → breakdown → implement → review → docs | Large tasks (complex, multi-file) |
| `implement-medium` | spec → tdd_write → tdd_integrate → implement → review → docs | Medium tasks (standard features) |
| `implement-small` | tiny_spec → implement → review → docs | Small changes |
| `implement-trivial` | implement | Trivial fixes (no spec) |
| `review` | review | Review existing changes |
| `spec` | spec | Generate spec only |
| `docs` | docs | Documentation |
| `qa` | qa | Manual QA session |
| `qa-e2e` | qa_e2e_test ⟳ qa_e2e_fix | E2E browser testing with fix loop |

## Built-in Phase Templates

| ID | Purpose | Produces Artifact |
|----|---------|-------------------|
| `spec` | Full specification | Yes (spec) |
| `tiny_spec` | Lightweight spec + TDD | Yes (spec) |
| `tdd_write` | Write failing unit/sociable tests | Yes (tests) |
| `tdd_integrate` | Write failing integration/wiring tests | Yes (tests) |
| `breakdown` | Implementation tasks | Yes (breakdown) |
| `implement` | Write code | No |
| `review` | Multi-agent code review with verification | No |
| `docs` | Documentation | Yes (docs) |
| `qa` | Manual QA | No |
| `qa_e2e_test` | Browser-based E2E testing (Playwright MCP) | Yes (findings) |
| `qa_e2e_fix` | Fix issues from QA testing | No |
| `research` | Research patterns | Yes (research) |

## Built-in Agents

9 agents defined in `templates/agents/*.md` with YAML frontmatter + markdown prompt body. Parsed by `ParseAgentMarkdown()`, seeded to GlobalDB by `SeedAgents()`. Idempotent — existing agents are skipped.

```go
// Agent file format (templates/agents/<name>.md):
// ---
// name: agent-id
// description: What it does
// model: opus|sonnet|haiku
// tools: ["Read", "Grep", "Glob"]
// ---
// Prompt body with {{TEMPLATE_VARIABLES}}

SeedAgents(gdb)           // Seed all agents + phase_agents associations
ListBuiltinAgentIDs()     // Returns all 9 agent IDs
```

Agent prompts support `{{VARIABLE}}` rendering via `executor.ToInlineAgentDef()`. See `templates/CLAUDE.md` for the full agent list and model tier strategy.

## Usage

```go
import "github.com/randalmurphal/orc/internal/workflow"

// Seed built-ins on startup
seeded, err := workflow.SeedBuiltins(pdb)

// Seed agents (separate call, needs GlobalDB)
seeded, err := workflow.SeedAgents(gdb)

// Get default workflow for task weight
workflow, err := workflow.GetDefaultWorkflow("medium")

// Resolve with config and overrides
resolver := workflow.NewResolver(config)
workflow, err := resolver.ResolveWorkflow("medium", userWorkflowOverride)

// List available workflows/agents
ids := workflow.ListBuiltinWorkflowIDs()
agentIDs := workflow.ListBuiltinAgentIDs()
```

## Database Operations

DB operations are in `internal/db/workflow.go`:

```go
// Phase templates
pdb.SavePhaseTemplate(pt)
pdb.GetPhaseTemplate(id)
pdb.ListPhaseTemplates()

// Workflows
pdb.SaveWorkflow(w)
pdb.GetWorkflow(id)
pdb.ListWorkflows()
pdb.GetWorkflowPhases(workflowID)
pdb.GetWorkflowVariables(workflowID)

// Workflow runs
pdb.SaveWorkflowRun(wr)
pdb.GetWorkflowRun(id)
pdb.ListWorkflowRuns(opts)
pdb.GetNextWorkflowRunID()
pdb.GetWorkflowRunPhases(runID)
```

## Variable System

Workflows can define custom variables with different sources:

| Source | Config Example | Description |
|--------|---------------|-------------|
| `static` | `{"value": "literal"}` | Fixed value |
| `env` | `{"var": "MY_VAR"}` | Environment variable |
| `script` | `{"path": ".orc/scripts/x.sh"}` | Script output |
| `api` | `{"url": "https://..."}` | HTTP response |
| `phase_output` | `{"phase": "spec"}` | Prior phase artifact |
| `prompt_fragment` | `{"path": "fragments/x.md"}` | Reusable prompt snippet |

## QA E2E Workflow

Browser-based E2E testing with iterative fix loop using Playwright MCP.

### Usage

```bash
# Run QA E2E workflow on existing task
orc new "Test feature X" --workflow qa-e2e
orc run TASK-XXX

# With before images for visual comparison
orc new "Test feature X" --workflow qa-e2e --before-images ./before.png

# Configure max iterations (default: 3)
orc new "Test feature X" --workflow qa-e2e --qa-max-iterations 5
```

### Flow

```
qa_e2e_test → (findings?) → qa_e2e_fix → qa_e2e_test → ... → PASS/MAX_ITERATIONS
```

### Loop Condition

The `qa_e2e_test` phase has `LoopConfig` with condition `has_findings`. If findings exist in the output, it loops to `qa_e2e_fix` phase. Loop continues until no findings or max iterations reached.

### Specialized Agents

| Agent | Role | Description |
|-------|------|-------------|
| `qa-functional` | Functional testing | Happy path, edge cases, error handling |
| `qa-visual` | Visual regression | Before/after comparison, layout checks |
| `qa-accessibility` | Accessibility audit | WCAG compliance, keyboard nav, ARIA |
| `qa-investigator` | Root cause analysis | Traces bugs to code for fix phase |

### Variables

| Variable | Source | Description |
|----------|--------|-------------|
| `BEFORE_IMAGES` | Task metadata | Baseline images for visual comparison |
| `PREVIOUS_FINDINGS` | Prior phase | Findings from last QA iteration |
| `QA_FINDINGS` | qa_e2e_test output | Current findings for fix phase |
| `QA_ITERATION` | Context | Current loop iteration number |
| `QA_MAX_ITERATIONS` | Task metadata/config | Max iterations before stopping |
