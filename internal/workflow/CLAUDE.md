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
| `qa` | qa | QA session |

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
| `research` | Research patterns | Yes (research) |
| `design` | Design document | Yes (design) |

## Usage

```go
import "github.com/randalmurphal/orc/internal/workflow"

// Seed built-ins on startup
seeded, err := workflow.SeedBuiltins(pdb)

// Get workflow for weight (backward compat)
wfID := workflow.GetWorkflowForWeight("medium") // "implement"
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

## Migration from Weight-Based System

The `GetWorkflowForWeight()` function maps weights to workflows:

| Weight | Workflow |
|--------|----------|
| trivial | implement-trivial |
| small | implement-small |
| medium | implement-medium |
| large | implement-large |
