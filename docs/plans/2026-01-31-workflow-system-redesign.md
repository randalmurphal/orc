# Workflow System Redesign

**Date:** 2026-01-31
**Status:** Design
**Scope:** Workflow config model, weight decoupling, phase template creation, DAG execution, conditions/loops, trigger/gate UI, gate actions

## Problem Statement

The workflow system has accumulated incoherent configuration:

- **`workflow_type`** (task/branch/standalone): Stored everywhere, used nowhere. Zero behavioral impact at execution time.
- **`default_model`**: Stored in DB, shown in UI. Executor ignores it completely. Model resolves: phase override → agent → config.yaml → fallback.
- **`default_thinking`**: Same - stored, never read by executor.
- **Weight = Workflow**: Weight (trivial/small/medium/large) is hardcoded to a specific workflow ID. Users can't pick a workflow independently of weight.
- **Phase template creation**: UI only supports cloning existing templates. Can't create from scratch.
- **Phase dependencies**: Proto supports `depends_on` for DAG execution. UI/executor only support sequential.
- **Phase conditions/loops**: Backend supports `condition` (skip conditions) and `loop_config`. No UI exposure.

- **Trigger system invisible**: Backend fully supports before-phase triggers and lifecycle triggers (on_task_created/completed/failed). No UI, no API CRUD endpoints. Configuration requires database editing or YAML import.
- **Gate output actions dead**: `OnApproved`/`OnRejected` action fields (continue/retry/fail/skip_phase/run_script) defined in data model, never evaluated by executor.
- **Gate input/output config not in UI**: What context to pass to AI gates and what to do with results - only configurable via database.

The result: half the Create Workflow modal does nothing, the trigger system is invisible despite being fully implemented, important capabilities are hidden, and the weight/workflow coupling limits what workflows can be.

## Design Decisions

| # | Decision | Rationale |
|---|----------|-----------|
| 1 | Kill `workflow_type` entirely | Dead code. Never read. Context type lives on `WorkflowRun.context_type`. |
| 2 | Wire `default_model` and `default_thinking` into executor | Helpers exist (`GetEffectiveModel/Thinking`), just never called. |
| 3 | Workflows are self-contained execution recipes | Workflow owns all its defaults (model, thinking, iterations, completion). |
| 4 | Decouple weight from workflow | Weight = sizing hint that suggests a default workflow. Workflow = the actual recipe. Independently chosen. |
| 5 | Weight defaults to "medium" | Always present on tasks, but becomes background when using custom workflows. |
| 6 | Gates are per-phase, not per-workflow | Different phases need different gate types. No workflow-level gate default. |
| 7 | Phase templates can be created from scratch | New modal with prompt editor, variable declaration, output config. |
| 8 | DAG execution via visual editor | Draw edges between phase nodes. Backend already supports `depends_on`. |
| 9 | Phase conditions and loops exposed in UI | Skip conditions and retry loops are real use cases. |
| 10 | Everything always creates a task | Task is the universal execution context (branch, worktree, transcripts, costs). |
| 11 | Full trigger editor in UI | Both before-phase triggers and lifecycle triggers configurable from the workflow/phase editors. |
| 12 | Wire gate output actions | `OnApproved`/`OnRejected` action fields actually evaluated by executor. |
| 13 | Gate input/output config in UI | What context AI gates receive and what happens with their decisions - editable per-phase. |

---

## 1. Workflow Configuration Model

### Fields: Keep, Kill, Add

**Keep (works)**

| Field | Purpose |
|-------|---------|
| `id` | Unique identifier, immutable |
| `name` | Display name |
| `description` | Context for users and phases |

**Keep + Fix (exists but broken)**

| Field | Fix needed |
|-------|------------|
| `default_model` | Add to executor resolution chain. Call `GetEffectiveModel()`. |
| `default_thinking` | Add to executor resolution chain. Call `GetEffectiveThinking()`. |

**Kill**

| Field | Reason |
|-------|--------|
| `workflow_type` | Remove from proto, DB schema, API, YAML, UI. Dead code. |

**Add**

| Field | Type | Default | Purpose |
|-------|------|---------|---------|
| `default_max_iterations` | int | 0 (inherit) | Workflow-level iteration budget for all phases |
| `completion_action` | enum | `""` (inherit) | What happens when workflow finishes: `pr`, `commit`, `none` |
| `target_branch` | string | `""` (inherit) | Default PR target branch for this workflow |

### Create Workflow Modal (Redesigned)

```
+---------------------------------------+
| Create Workflow                     X  |
|---------------------------------------|
| Workflow ID *                          |
| [my-custom-workflow                  ] |
| Unique identifier (lowercase, hyphens) |
|                                        |
| Name                                   |
| [My Custom Workflow                  ] |
|                                        |
| Description                            |
| [                                    ] |
| [                                    ] |
|                                        |
| -- Execution Defaults --               |
|                                        |
| Default Model       Max Iterations     |
| [Inherit (config)]  [20             ]  |
|                                        |
| [x] Enable thinking by default         |
|                                        |
| -- Completion --                       |
|                                        |
| On Complete         Target Branch      |
| [Inherit (config)]  [Inherit (config)] |
|                                        |
|          [Cancel]   [+ Create]         |
+---------------------------------------+
```

**What changed:**
- Removed: Workflow Type selector (3-option radio group)
- Added: Max Iterations input
- Added: Completion Action dropdown (`Inherit`, `Create PR`, `Commit Only`, `None`)
- Added: Target Branch input (`Inherit` or branch name)
- "Inherit" means "use project config.yaml value" - explicit about the fallback

---

## 2. Resolution Chains

Every setting has a clear, documented resolution order. Executor walks top-down, takes first non-empty value.

### Model

```
1. phase.model_override           → per-phase in workflow
2. workflow.default_model          → workflow-level        [FIX: currently skipped]
3. agent.model                     → from phase template's agent
4. config.yaml model               → project/personal config
5. "claude-sonnet-4-20250514"      → hardcoded fallback
```

### Thinking

```
1. phase.thinking_override         → per-phase in workflow
2. workflow.default_thinking        → workflow-level        [FIX: currently skipped]
3. phase_template.thinking_enabled → template default
4. false                           → hardcoded fallback
```

### Max Iterations

```
1. phase.max_iterations_override   → per-phase in workflow
2. workflow.default_max_iterations  → workflow-level        [NEW]
3. phase_template.max_iterations   → template default
4. 20                              → hardcoded fallback
```

### Gate Type

```
1. phase.gate_type_override        → per-phase in workflow
2. phase_template.gate_type        → template default
3. config.yaml gates.default_type  → project config
4. "auto"                          → hardcoded fallback
```

No workflow-level default for gates. Gates are inherently per-phase.

### Completion Action

```
1. workflow.completion_action       → per-workflow          [NEW]
2. config.yaml completion.action   → project config
3. "pr"                            → hardcoded fallback
```

### Target Branch

```
1. task.target_branch              → per-task override
2. workflow.target_branch           → per-workflow          [NEW]
3. initiative.branch_base          → inherited from initiative
4. config.yaml completion.target_branch → project config
5. repo default branch             → git default
```

---

## 3. Weight ↔ Workflow Decoupling

### Current (Coupled)

```
Weight (required) → hardcoded workflow ID → phases
```

- Weight directly selects which workflow runs
- No way to use a custom workflow without hacking weight mappings
- Non-task workflows (QA, reports) don't fit the weight model

### Proposed (Decoupled)

```
Weight (defaults to medium) ─────→ sizing hints (prompt tone, budget)
                                    ↓
                              suggests default workflow
                                    ↓
Workflow (explicit or default) ──→ phases → execution
```

**Task creation flow:**

1. User creates task: `orc new "fix auth bug" -w small`
2. No explicit workflow → look up weight mapping: `small → small-task`
3. Task gets `workflow_id: small-task`

OR:

1. User creates task: `orc new "generate changelog" --workflow changelog-report`
2. Explicit workflow → use it. Weight defaults to `medium` (just metadata).

**Weight → Workflow mapping** in config (overridable):

```yaml
# config.yaml
weight_workflows:
  trivial: trivial-task
  small: small-task
  medium: medium-task
  large: large-task
```

### Task Model Changes

```protobuf
message Task {
  // existing...
  string weight = X;             // stays, defaults to "medium"
  string workflow_id = X;        // NEW: explicit workflow reference
  // The executor uses workflow_id, never weight, to determine phases.
  // Weight is metadata for reporting/filtering/prompt-tone.
}
```

### CLI Changes

```bash
# Weight implies workflow (current behavior, preserved)
orc new "fix bug" -w small

# Explicit workflow overrides weight's suggestion
orc new "fix bug" -w small --workflow my-review-heavy-flow

# Workflow without weight (weight defaults to medium)
orc new "run QA" --workflow qa-pipeline
```

### Web UI Changes

Task creation form gets a workflow selector:
- Dropdown populated from available workflows
- Weight selection pre-fills the workflow (but user can change it)
- Changing weight updates the suggested workflow
- Changing workflow explicitly disables the weight → workflow auto-suggestion

---

## 4. Phase Template Creation

### New: Create Phase Template Modal

Currently only clone is supported. Add a "Create From Scratch" option.

```
+--------------------------------------------+
| Create Phase Template                   X  |
|--------------------------------------------|
| Template ID *                              |
| [my-analysis-phase                       ] |
|                                            |
| Name *                                     |
| [Code Analysis                           ] |
|                                            |
| Description                                |
| [Analyzes codebase and generates report  ] |
|                                            |
| -- Prompt --                               |
|                                            |
| Source: ( ) Inline   ( ) File              |
|                                            |
| [If inline: Monaco editor with {{VAR}}   ] |
| [highlighting and variable autocomplete  ] |
|                                            |
| [If file: path input with .orc/prompts/  ] |
| [prefix and file picker                  ] |
|                                            |
| -- Data Flow --                            |
|                                            |
| Input Variables                            |
| [SPEC_CONTENT] [PROJECT_ROOT] [+ Add]      |
| (Variables this phase expects to receive)  |
|                                            |
| Output Variable Name                       |
| [ANALYSIS_REPORT                         ] |
| (Downstream phases reference this name)    |
|                                            |
| -- Execution --                            |
|                                            |
| Agent            Gate Type                 |
| [Default      ]  [Auto       ]            |
|                                            |
| Max Iterations   [x] Enable Thinking      |
| [20           ]  [ ] Checkpoint            |
|                                            |
| -- Claude Config (collapsible) --          |
| (Same as existing: hooks, MCP, skills,     |
|  tools, env vars)                          |
|                                            |
|          [Cancel]   [+ Create]             |
+--------------------------------------------+
```

### Key UX Details

**Prompt Source Toggle:**
- `Inline (DB)`: Shows a code editor. Content stored in `prompt_content`. Good for short, iterative prompts.
- `File`: Shows a path input prefixed with `.orc/prompts/`. Content lives in git-tracked file. Good for long prompts and team collaboration.

**Input Variables:**
- Tag input with suggestions from known variables (built-in: `SPEC_CONTENT`, `PROJECT_ROOT`, `TASK_DESCRIPTION`, `WORKTREE_PATH`, etc.)
- Used for validation: warns if prompt references `{{FOO}}` but `FOO` isn't in input variables
- Used by PhaseInspector to show "variable satisfaction" status

**Output Variable Name:**
- Single text input
- If set, this phase's output is stored under this name for downstream phases
- Shown in the visual editor as the phase's "output port"

### Existing Edit Modal Updates

`EditPhaseTemplateModal` also needs the data flow fields:
- Add `inputVariables` tag input
- Add `outputVarName` text input
- Add `promptSource` toggle + appropriate editor
- Add `promptPath` input (when source=file)

---

## 5. Phase Dependencies (DAG Execution)

### Backend: Already Supported

`WorkflowPhase.depends_on` is a `repeated string` in the proto and stored in DB. The data model is ready.

### Executor: Needs Parallel Support

Currently the executor runs phases sequentially by sequence number. Need to:
1. Build a dependency graph from `depends_on` fields
2. Identify phases that can run in parallel (no shared dependencies)
3. Execute independent phases concurrently
4. Wait for all dependencies before starting a dependent phase
5. The existing `ValidateWorkflow()` already checks for cycles (Kahn's algorithm)

### Visual Editor: Edge Drawing

**How it works:**
- Phase nodes get input/output handles (React Flow connection points)
- Users drag from output handle of Phase A to input handle of Phase B to create a dependency
- Edge type indicates relationship (dependency, loop, conditional)
- Deleting an edge removes the dependency
- Validation runs on save: cycle detection, orphan detection

**Edge Types:**
| Type | Visual | Meaning |
|------|--------|---------|
| Dependency | Solid arrow | B waits for A to complete |
| Loop | Dashed arrow (backward) | On failure/condition, loop back to earlier phase |
| Conditional | Dotted arrow | Edge only taken if condition is met |

**Sequence auto-update:**
When edges change, sequence numbers are recalculated via topological sort. Phases at the same level (no dependencies between them) get the same sequence number, signaling parallel execution.

### Phase Node Display Updates

```
+---------------------------+
| [spec] Specification    A |  ← A = Auto gate badge
|                           |
| in: TASK_DESCRIPTION      |  ← Shows input variables
| out: SPEC_CONTENT         |  ← Shows output variable
|                           |
| ○ (output handle)         |
+---------------------------+
         |
         ↓ (dependency edge)
+---------------------------+
| [impl] Implementation   A |
| ...                       |
```

---

## 6. Phase Conditions

### What They Are

A JSON condition on a phase that, when evaluated to false, causes the phase to be skipped.

### Condition Types

```json
// Skip if weight is trivial
{"field": "task.weight", "op": "eq", "value": "trivial"}

// Skip if a previous phase output contains a flag
{"field": "var.SPEC_CONTENT", "op": "contains", "value": "NO_TESTS_NEEDED"}

// Skip if environment variable is set
{"field": "env.SKIP_DOCS", "op": "exists"}

// Compound conditions
{"all": [
  {"field": "task.weight", "op": "in", "value": ["medium", "large"]},
  {"field": "task.category", "op": "neq", "value": "docs"}
]}
```

### UI: Condition Editor

In the phase inspector (settings tab), add a condition builder:

```
Condition: Run this phase when...
[task.weight] [is one of] [medium, large]
[+ Add condition]

Logic: (All) / (Any)
```

Simple visual builder that generates the JSON. Advanced users can toggle to raw JSON editor.

### Executor Changes

Before executing a phase, evaluate its condition against the current execution context (task properties, resolved variables, env). If false, mark phase as `skipped` and continue to dependents.

---

## 7. Phase Loops

### What They Are

A phase can loop back to an earlier phase based on its output. Primary use case: review phase finds issues → loop back to implementation.

### Loop Config

```json
{
  "loop_to_phase": "implement",
  "condition": {"field": "phase_output.status", "op": "eq", "value": "needs_changes"},
  "max_loops": 3
}
```

### UI: Loop Editor

In phase inspector, a "Loop" tab or section:

```
Loop Configuration
  Loop back to: [implement ▾]  (dropdown of prior phases)
  When: [phase output status] [equals] [needs_changes]
  Max loops: [3]
```

In the visual editor, loop edges render as dashed backward arrows with a loop count badge.

### Executor Changes

After a phase completes, check `loop_config`:
1. Evaluate loop condition against phase output
2. If true and loop count < max_loops, re-queue the target phase
3. Increment loop counter
4. If max_loops exceeded, continue forward (optionally fail)

---

## 8. Trigger System UI

### Current State

The trigger system is **fully implemented on the backend** but completely invisible in the UI:

- **Before-phase triggers**: Run an agent before a phase starts. Can block (gate mode) or fire-and-forget (reaction mode). Stored as JSON on `workflow_phases.before_triggers`.
- **Lifecycle triggers**: Fire on task events (`on_task_created`, `on_task_completed`, `on_task_failed`, `on_initiative_planned`). Stored as JSON on `workflows.triggers`.
- **Trigger runner**: Fully wired in executor. Gate-mode triggers block, reaction-mode run async. Error handling is solid.
- **No API CRUD endpoints** for triggers.
- **No UI** anywhere.

### Before-Phase Trigger Editor

Lives in the phase inspector (new "Triggers" tab) and in the PhaseListEditor's phase edit dialog.

```
+------------------------------------------+
| Triggers (before phase)                  |
|------------------------------------------|
|                                          |
| Trigger 1                          [x]   |
| Agent: [spec-quality-auditor ▾]          |
| Mode:  (•) Gate (blocking)               |
|        ( ) Reaction (fire-and-forget)    |
|                                          |
| ── Input Context ──                      |
| [x] Include task metadata                |
| Include phase outputs: [spec] [+ Add]    |
| Extra variables: [+ Add]                 |
|                                          |
| ── On Result ──                          |
| Store output as variable: [AUDIT_RESULT] |
| On approved: [Continue ▾]               |
| On rejected: [Retry from... ▾] [spec]   |
|                                          |
| [+ Add Trigger]                          |
+------------------------------------------+
```

**Key details:**
- Agent dropdown populated from available agents (defined in `templates/agents/` or custom)
- Mode determines blocking behavior: gate = must approve to continue, reaction = runs in background
- Input context controls what the trigger agent sees (phase outputs, task data, custom vars)
- Output can be captured as a workflow variable for downstream use
- On approved/rejected actions are the gate output action fields (see section 9)

### Lifecycle Trigger Editor

Lives on the workflow edit page (new "Triggers" tab alongside "Phases" and "Variables").

```
+------------------------------------------+
| Workflow Triggers                        |
|------------------------------------------|
|                                          |
| on_task_completed                        |
| ┌──────────────────────────────────────┐ |
| │ Agent: [code-reviewer ▾]             │ |
| │ Mode:  (•) Gate  ( ) Reaction        │ |
| │ Enabled: [x]                         │ |
| │                                      │ |
| │ Input: [x] Task metadata             │ |
| │ On approved: [Continue ▾]            │ |
| │ On rejected: [Block task ▾]          │ |
| └──────────────────────────────────────┘ |
|                                          |
| on_task_failed                           |
| ┌──────────────────────────────────────┐ |
| │ Agent: [failure-analyzer ▾]          │ |
| │ Mode:  ( ) Gate  (•) Reaction        │ |
| │ Enabled: [x]                         │ |
| └──────────────────────────────────────┘ |
|                                          |
| [+ Add Lifecycle Trigger]               |
| Event: [on_task_created ▾]              |
+------------------------------------------+
```

**Lifecycle events available:**

| Event | When it fires | Gate-mode effect |
|-------|--------------|------------------|
| `on_task_created` | After task is created | Can reject task creation |
| `on_task_completed` | After all phases pass | Can block completion (task → BLOCKED) |
| `on_task_failed` | After task failure | Can trigger recovery logic |
| `on_initiative_planned` | After initiative planning | Can validate task decomposition |

### API Changes Needed

New RPC endpoints for trigger CRUD:

```protobuf
// Before-phase triggers (on workflow phases)
rpc AddBeforePhaseTrigger(AddBeforePhaseTriggerRequest) returns (AddBeforePhaseTriggerResponse);
rpc UpdateBeforePhaseTrigger(UpdateBeforePhaseTriggerRequest) returns (UpdateBeforePhaseTriggerResponse);
rpc RemoveBeforePhaseTrigger(RemoveBeforePhaseTriggerRequest) returns (RemoveBeforePhaseTriggerResponse);

// Lifecycle triggers (on workflows)
rpc AddLifecycleTrigger(AddLifecycleTriggerRequest) returns (AddLifecycleTriggerResponse);
rpc UpdateLifecycleTrigger(UpdateLifecycleTriggerRequest) returns (UpdateLifecycleTriggerResponse);
rpc RemoveLifecycleTrigger(RemoveLifecycleTriggerRequest) returns (RemoveLifecycleTriggerResponse);
```

These modify the JSON arrays stored on `workflow_phases.before_triggers` and `workflows.triggers` respectively.

---

## 9. Gate Output Actions

### Current State

`GateOutputConfig` defines `OnApproved` and `OnRejected` fields with 5 possible actions:

| Action | Meaning | Currently implemented? |
|--------|---------|----------------------|
| `continue` | Proceed to next phase | ✅ (hardcoded default) |
| `retry` | Retry from a specific phase | ✅ (via `retry_from` field, partially) |
| `fail` | Fail the task immediately | ❌ |
| `skip_phase` | Skip the current/next phase | ❌ |
| `run_script` | Execute a user-provided script | ✅ (in script_handler.go, but disconnected from flow) |

### Proposed: Wire All Actions

**In the executor's gate evaluation flow:**

```
Phase completes → evaluate gate → decision returned
  → if approved:
    → check OnApproved action
      → continue: proceed to next phase (default)
      → run_script: execute script, then continue
      → skip_phase: skip the NEXT phase in sequence
  → if rejected:
    → check OnRejected action
      → retry: retry from specified phase (with loop counter)
      → fail: mark task as FAILED immediately
      → skip_phase: skip current phase, continue to next
      → run_script: execute script, then apply secondary action
```

**The "retry from" action integrates with the loop system** (Section 7):
- Gate rejection with `retry` action + `retry_from: implement` = loop back to implement phase
- Loop counter incremented, `max_loops` checked
- If max loops exceeded, fall back to `fail`

### UI: Gate Config Editor

In the phase inspector, the existing gate type dropdown gets expanded with an "Advanced" section:

```
Gate Type: [AI ▾]
Gate Agent: [code-reviewer ▾]

▼ Advanced Gate Config

  Input Context:
  [x] Include task metadata
  Include phase outputs: [spec] [implement] [+ Add]
  Extra variables: [+ Add]

  On Approved: [Continue ▾]
  On Rejected: [Retry from ▾] → [implement ▾]
                Max retries: [3]

  Store gate output as: [REVIEW_RESULT]
```

This replaces the current simple gate type dropdown with a richer configuration that exposes the full `GateInputConfig` and `GateOutputConfig`.

### Resolution with Existing Gate System

The existing 6-level gate type resolution stays intact. Gate output actions are a **separate concern** from gate type resolution:

```
Gate TYPE resolution: what kind of gate runs (auto/human/ai/skip)
  → 6-level precedence (task override → weight → phase → enabled → config → fallback)

Gate ACTION resolution: what happens after the gate decides
  → Only from GateOutputConfig on the phase (no cascade, explicit per-phase)
```

---

## 10. Retry Context Unification

### Current State: Two Parallel Systems

**System 1: RetryContext proto field** (`task.Execution.RetryContext`)
- Separate proto message with `from_phase`, `to_phase`, `reason`, `failure_output`, `attempt`
- Pre-formatted into a markdown string via `BuildRetryContext()` in `executor/retry.go`
- Injected as a single built-in variable `RETRY_CONTEXT` (a big blob of markdown)
- Templates reference `{{RETRY_CONTEXT}}` and get the whole thing or nothing

**System 2: Gate output variables** (`applyGateOutputToVars()`)
- Gate's structured output stored as a named workflow variable
- Flows through the standard variable system
- Disconnected from the retry context string

**Problems:**
- `RETRY_CONTEXT` is a pre-baked string, not composable structured data
- Templates can't control how retry info is presented
- `BuildRetryContext()` hardcodes the format with embedded markdown
- Gate output variables and retry context are disconnected despite being the same event
- Review round detection is special-cased in the executor

### Proposed: Structured Variables Replace Proto Field

**Kill:**
- `task.Execution.RetryContext` proto field
- `BuildRetryContext()` / `BuildRetryContextWithGateAnalysis()` formatting functions
- The pre-formatted `RETRY_CONTEXT` built-in variable

**Replace with structured variables:**

When a gate rejects and triggers a retry, the executor sets these variables:

| Variable | Source | Content |
|----------|--------|---------|
| `RETRY_ATTEMPT` | Executor (built-in) | Current attempt number (1, 2, 3...) |
| `RETRY_FROM_PHASE` | Executor (built-in) | Phase ID that triggered the retry (e.g., "review") |
| `RETRY_REASON` | Gate output | Rejection reason from the gate |

These are **simple built-in variables** set by the executor on retry, available to any template.

**Phase output variables remain available:**

The rejecting phase's output is already stored under its `output_var_name` (e.g., review phase stores `REVIEW_OUTPUT`). When implementation retries, `{{REVIEW_OUTPUT}}` is still available — it's the review findings that caused the rejection.

Similarly, the gate's structured output is stored via `applyGateOutputToVars()` under the configured variable name. This already works.

**The key insight:** Retry context isn't a special thing. It's just "the previous phase's output variables + gate output variables + a few retry metadata variables." The variable system already handles all of this. We just need to stop pre-formatting it into a markdown blob and let templates compose the pieces.

### Template Changes

**Before (current):**
```markdown
{{RETRY_CONTEXT}}
<!-- This dumps a whole pre-formatted markdown section with hardcoded structure -->
```

**After (proposed):**
```markdown
{{#if RETRY_ATTEMPT}}
## Retry Context (Attempt {{RETRY_ATTEMPT}})

The {{RETRY_FROM_PHASE}} phase rejected your previous implementation.

**Reason:** {{RETRY_REASON}}

**Review findings:**
{{REVIEW_OUTPUT}}
{{/if}}
```

Templates now control the presentation. Different phase templates can present retry context differently — a test retry might show test failures prominently, while a review retry might focus on code quality issues.

### Phase Output Schema Connection

Phase templates define `output_schema` (JSON schema for structured output). When a phase completes:

1. Phase produces JSON output conforming to its `output_schema`
2. Output stored under `output_var_name` (e.g., `REVIEW_OUTPUT`)
3. Workflow variables with `source_type: phase_output` and `extract` (JSONPath) can pull specific fields:

```yaml
# Workflow variable that extracts just the issues list from review output
variables:
  - name: REVIEW_ISSUES
    source_type: phase_output
    source_config: '{"phase": "review", "field": "artifact"}'
    extract: ".issues"
```

This means downstream phases (or retried phases) can reference `{{REVIEW_ISSUES}}` to get just the issues array, not the entire review output. The `extract` field on variables already supports this — it just needs to work with phase outputs in the retry flow.

### Migration Path

1. **Add retry metadata variables** (`RETRY_ATTEMPT`, `RETRY_FROM_PHASE`, `RETRY_REASON`) as built-in variables in the resolver
2. **Update built-in templates** to use structured variables instead of `{{RETRY_CONTEXT}}`
3. **Remove `BuildRetryContext()`** formatting functions
4. **Keep proto field temporarily** for backward compatibility during migration, but executor stops writing to it
5. **Remove proto field** in subsequent release

### Review Round Special Case

The current review round detection (`if tmpl.ID == "review" && rctx.ReviewRound > 1`) is a special case that should generalize into the loop system (Section 7):

- Review phase has a loop config: `loop_to_phase: implement, condition: {status: needs_changes}, max_loops: 3`
- Loop counter replaces `ReviewRound` tracking
- Round-specific templates (`review_round2.md`) become condition-based: "if this is a retry, use the shorter review template"
- The loop system handles all of this generically

---

## 11. Task Breakdown

These are the implementation tasks, roughly ordered by dependency:

### Core (Must Do)

| # | Task | Weight | Dependencies |
|---|------|--------|-------------|
| 1 | Remove `workflow_type` from proto, DB, API, YAML, UI | small | None |
| 2 | Wire `default_model` into executor resolution chain | small | None |
| 3 | Wire `default_thinking` into executor resolution chain | small | None |
| 4 | Add `default_max_iterations` to workflow model + DB + API + UI | small | None |
| 5 | Add `completion_action` to workflow model + DB + API + UI | medium | None |
| 6 | Add `target_branch` to workflow model + DB + API + UI | small | None |
| 7 | Redesign Create Workflow modal (new fields, remove type) | medium | 1, 4, 5, 6 |
| 8 | Decouple weight from workflow: add `workflow_id` to task model | medium | None |
| 9 | Add weight → workflow config mapping | small | 8 |
| 10 | Update task creation UI with workflow selector | medium | 8, 9 |
| 11 | Update CLI task creation with `--workflow` flag | small | 8, 9 |

### Phase Templates

| # | Task | Weight | Dependencies |
|---|------|--------|-------------|
| 12 | Add data flow fields to EditPhaseTemplateModal (inputVars, outputVarName, promptSource) | medium | None |
| 13 | Create "New Phase Template" modal with prompt editor + data flow | medium | 12 |
| 14 | Add prompt source toggle (inline vs file) to template editing | small | 12 |

### DAG Execution

| # | Task | Weight | Dependencies |
|---|------|--------|-------------|
| 15 | Visual editor: add connection handles to phase nodes | medium | None |
| 16 | Visual editor: edge drawing + deletion + type badges | medium | 15 |
| 17 | Executor: parallel phase execution from dependency graph | large | None |
| 18 | Auto-recalculate sequence numbers from topology | small | 16 |

### Conditions & Loops

| # | Task | Weight | Dependencies |
|---|------|--------|-------------|
| 19 | Phase condition evaluator in executor | medium | None |
| 20 | Condition editor UI in phase inspector | medium | 19 |
| 21 | Phase loop executor logic | medium | None |
| 22 | Loop editor UI in phase inspector | small | 21 |
| 23 | Visual editor: loop edge rendering | small | 16, 22 |

### Triggers & Gate Actions

| # | Task | Weight | Dependencies |
|---|------|--------|-------------|
| 24 | API: before-phase trigger CRUD endpoints | medium | None |
| 25 | API: lifecycle trigger CRUD endpoints | medium | None |
| 26 | Before-phase trigger editor UI (phase inspector "Triggers" tab) | medium | 24 |
| 27 | Lifecycle trigger editor UI (workflow "Triggers" tab) | medium | 25 |
| 28 | Wire gate output actions in executor (OnApproved/OnRejected) | medium | None |
| 29 | Gate config editor UI (input context, output actions, advanced section) | medium | 28 |
| 30 | Integrate gate retry action with loop system (shared loop counter) | small | 21, 28 |

### Retry Context Unification

| # | Task | Weight | Dependencies |
|---|------|--------|-------------|
| 31 | Add structured retry variables (RETRY_ATTEMPT, RETRY_FROM_PHASE, RETRY_REASON) as built-ins | small | None |
| 32 | Update built-in templates to use structured retry variables instead of {{RETRY_CONTEXT}} | medium | 31 |
| 33 | Remove BuildRetryContext() formatters and pre-formatted RETRY_CONTEXT variable | small | 32 |
| 34 | Ensure phase output variables survive retry (available to retried phase) | medium | 31 |
| 35 | Generalize review round detection into loop system (remove special-case) | medium | 21, 31 |
| 36 | Deprecate and remove RetryContext proto field (migration) | small | 32, 33 |

### Summary

| Category | Tasks | Key Deliverable |
|----------|-------|-----------------|
| Core | 1-11 | Workflow config works, weight decoupled |
| Phase Templates | 12-14 | Create templates from scratch with data flow |
| DAG Execution | 15-18 | Parallel phase execution with visual editing |
| Conditions & Loops | 19-23 | Conditional phases and retry loops |
| Triggers & Gates | 24-30 | Full trigger/gate configuration in UI |
| Retry Unification | 31-36 | Retry context through variable system, no special cases |
| **Total** | **36 tasks** | **Complete workflow system** |

---

## Migration Notes

### DB Schema Changes

```sql
-- Add new workflow columns
ALTER TABLE workflows ADD COLUMN default_max_iterations INTEGER DEFAULT 0;
ALTER TABLE workflows ADD COLUMN completion_action TEXT DEFAULT '';
ALTER TABLE workflows ADD COLUMN target_branch TEXT DEFAULT '';

-- Remove workflow_type (after migration)
-- Step 1: Stop reading it (code change)
-- Step 2: Drop column in subsequent migration
-- ALTER TABLE workflows DROP COLUMN workflow_type;

-- Add workflow_id to tasks
ALTER TABLE tasks ADD COLUMN workflow_id TEXT DEFAULT '';
```

### Proto Changes

```protobuf
message Workflow {
  string id = 1;
  string name = 2;
  optional string description = 3;
  // REMOVED: WorkflowType workflow_type = 4;
  optional string default_model = 5;
  bool default_thinking = 6;
  bool is_builtin = 7;
  optional string based_on = 8;
  // NEW:
  int32 default_max_iterations = 9;
  string completion_action = 10;
  string target_branch = 11;
}
```

### Backward Compatibility

- Existing YAML files with `workflow_type` are parsed but field is ignored
- Existing tasks without `workflow_id` continue to use weight-based lookup
- Weight → workflow mapping defaults match current hardcoded behavior
- No breaking changes to CLI (new flags are optional)
