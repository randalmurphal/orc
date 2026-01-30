# Workflow Package

Configurable workflow system for orc. Replaces weight-based task execution with composable, database-first workflows.

## Overview

| File | Purpose |
|------|---------|
| `types.go` | Domain types (Workflow, PhaseTemplate, WorkflowRun, etc.) |
| `seed.go` | Built-in workflow/phase definitions and seeding |

## Core Concepts

### Phase Templates (Lego Blocks)

Reusable phase definitions with:
- Prompt configuration (embedded, DB, or file)
- Input/output contract (what variables it needs/produces)
- Execution config (iterations, model, gate type)
- Retry configuration

### Workflows

Compose phases into execution plans:
- Ordered sequence of phases
- Per-phase overrides
- Custom variables
- Context type (task, branch, PR, standalone)

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

## Built-in Workflows

| ID | Phases | Use Case |
|----|--------|----------|
| `implement-large` | spec → tdd_write → breakdown → implement → review → docs | Large tasks (complex, multi-file) |
| `implement-medium` | spec → tdd_write → implement → review → docs | Medium tasks (standard features) |
| `implement-small` | tiny_spec → implement → review | Small changes |
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
| `tdd_write` | Write failing tests | Yes (tests) |
| `breakdown` | Implementation tasks | Yes (breakdown) |
| `implement` | Write code | No |
| `review` | Multi-agent code review with verification | No |
| `docs` | Documentation | Yes (docs) |
| `qa` | Manual QA | No |
| `qa_e2e_test` | Browser-based E2E testing (Playwright MCP) | Yes (findings) |
| `qa_e2e_fix` | Fix issues from QA testing | No |
| `research` | Research patterns | Yes (research) |

## Usage

```go
import "github.com/randalmurphal/orc/internal/workflow"

// Seed built-ins on startup
seeded, err := workflow.SeedBuiltins(pdb)

// List available workflows
ids := workflow.ListBuiltinWorkflowIDs()
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
