# AI Gates & Lifecycle Events

**Date:** 2026-01-30
**Status:** Design complete, ready for implementation
**Origin:** TASK-647 (dependency detection) revealed the need for a general-purpose gate/event system

## Problem

Orc's initiative planning doesn't detect code-level dependencies between tasks. TASK-631 imported a function from TASK-630 but no `blocked_by` was declared. More broadly, there's no way to run automated validation or processing at key lifecycle points beyond phase completion gates.

The gate system already has an `ai` gate type slot (`GateAI = "ai"` in `internal/gate/gate.go`) but it's unimplemented. The existing gate infrastructure (resolution hierarchy, retry mechanism, decision persistence, headless support) is solid — we need to complete it and extend where gates can trigger.

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| System type | Orc-native (not Claude Code hooks) | Clean separation of concerns |
| Gate vs event | Extend existing gate system | Reuses resolution hierarchy, retry, persistence |
| Agent type | Regular orc agents with trigger config | Leverages existing agent infra |
| Config home | Phase templates (defaults) + workflows (overrides) | Matches existing precedence model |
| Hook mode | Both gate (sync, can block) and reaction (async) | Configurable per trigger |
| Event scope | Full lifecycle from day one | before_phase, after_phase, on_task_created, on_task_completed, on_task_failed, on_initiative_planned |
| Implementation scope | Full initiative | AI gate type + lifecycle events + dependency validator |

## Architecture

### Extended Gate Model

Today a gate is `type` + `retry_from_phase`. We extend it to a processing node with input/output routing.

```yaml
gate:
  type: ai                          # auto | human | ai | skip
  agent: dependency-validator       # orc agent to run (when type=ai)
  mode: gate                        # gate (can block) | reaction (fire-and-forget)

  # What the agent receives
  input:
    include_phase_output: true      # inject phase output into agent prompt
    include_task: true              # inject full task context
    extra_vars: [SPEC_CONTENT]      # additional variables to include

  # How to handle the agent's structured output
  output:
    variable: GATE_RESULT           # store output as variable for downstream phases
    on_approved: proceed            # proceed | run_script
    on_rejected: retry              # retry | block | fail | run_script
    retry_from: implement           # which phase to retry from
    script: ""                      # optional: bash command, output piped to stdin
```

**Key additions over current gate:**
- `agent` — references orc agent by name from agent registry
- `mode` — gate (blocking) vs reaction (fire-and-forget)
- `input` — explicit context selection (today this is implicit)
- `output` — configurable routing: variable injection, retry, script execution

### AI Gate Evaluator

The `ai` gate type executes an orc agent and maps its response to `GateEvaluationResult`.

**Agent output schema (standard for all AI gates):**

```json
{
  "status": "approved | rejected | blocked",
  "reason": "Human-readable explanation",
  "retry_from": "phase_id (optional, overrides gate config)",
  "context": "Injected into RETRY_CONTEXT if retrying",
  "data": {}
}
```

- `status` maps to `GateEvaluationResult.Approved`
- `reason` maps to `GateEvaluationResult.Reason`
- `retry_from` overrides the gate's configured `retry_from` (agent can dynamically choose)
- `context` appended to `RETRY_CONTEXT` for the retried phase
- `data` stored as the gate's output variable — arbitrary structured JSON

**Execution flow:**
1. Gate triggers -> look up agent by name from registry
2. Build prompt: phase output + task context + extra vars from `input` config
3. Call agent via `llmutil.ExecuteWithSchema[T]()` with the standard output schema
4. Parse response into `GateEvaluationResult` + store `data` as variable
5. Existing gate handling (retry, block, persist decision) takes over

**Model selection** comes from the agent definition. Cheap validation = haiku. Deep review = sonnet.

### Lifecycle Event Triggers

Extend where gates can fire beyond "after phase completes."

**Workflow-level triggers:**

```yaml
triggers:
  on_task_created:
    - agent: dependency-validator
      mode: reaction
      output:
        variable: DEPENDENCY_ANALYSIS

  on_task_completed:
    - agent: cleanup-agent
      mode: reaction

  on_initiative_planned:
    - agent: dependency-validator
      mode: gate
      output:
        on_rejected: block
```

**Phase-level triggers (extend existing gate):**

```yaml
phases:
  - id: implement
    before:                        # NEW: before phase starts
      - agent: pre-check
        mode: gate
        output:
          on_rejected: block
    gate:                          # EXISTING: after phase completes
      type: ai
      agent: code-reviewer
      output:
        variable: REVIEW_FINDINGS
        on_rejected: retry
        retry_from: implement
```

**Trigger points:**

| Trigger | Where it fires | Gate-able? | Payload |
|---------|---------------|------------|---------|
| `on_task_created` | After `SaveTask()` | Yes | task |
| `on_task_completed` | After task marked complete | Reaction only | task, summary |
| `on_task_failed` | After task fails | Reaction only | task, error, phase |
| `on_initiative_planned` | After manifest tasks created | Yes | initiative, tasks[] |
| `before` (phase) | Before phase execution | Yes | task, phase, workflow |
| `gate` (phase) | After phase completes | Yes (existing) | task, phase, output |

**Executor integration:** The main loop in `workflow_executor.go` gets two new injection points:
1. Before phase execution: check `before` triggers, block if gate rejects
2. Task lifecycle: emit events at creation/completion/failure, check workflow-level triggers

### Variable Pipeline

Gate output flows into the variable system for downstream consumption.

When a gate runs and `output.variable` is set:
1. Agent produces `data` field in response
2. Executor stores `data` as a variable (e.g., `GATE_RESULT` or `REVIEW_FINDINGS`)
3. Variable resolver makes it available as `{{GATE_RESULT}}` in subsequent phase prompts
4. If gate triggers retry, `context` field appended to `{{RETRY_CONTEXT}}`

This enables data flow between phases via gates: phase A produces output -> gate parses/analyzes it -> stores structured result -> phase B consumes it.

### Script Handlers

When `output.script` is set or `on_rejected: run_script`:
1. Gate produces output
2. Output JSON piped to script's stdin
3. Script can modify tasks, add dependencies, update descriptions
4. Script exit code determines proceed/fail

Example: `orc deps-fix` script reads missing dependency suggestions from stdin, applies `blocked_by` updates automatically.

## First Consumer: Dependency Validator

**Built-in agent:**

```yaml
name: dependency-validator
description: Analyzes initiative tasks for undeclared code-level dependencies
model: haiku
system_prompt: |
  Analyze tasks created from an initiative plan. Identify dependencies where
  one task produces something another consumes:
  - Function/type definitions used by other tasks
  - Files created that other tasks import
  - API endpoints one task creates that another calls
  - Shared state or config one task sets up

  Return ONLY missing dependencies not already declared in blocked_by.
output_schema:
  status: string        # approved | rejected
  reason: string
  data:
    missing_deps:
      - from: string    # task that should wait
        on: string      # task it depends on
        reason: string  # e.g., "imports topo_sort.go created by this task"
    confidence: string  # high | medium | low
```

**Default workflow trigger:**

```yaml
triggers:
  on_initiative_planned:
    - agent: dependency-validator
      mode: gate
      output:
        on_approved: proceed
        on_rejected: block
```

**End-to-end flow:**
1. `orc initiative plan manifest.yaml` -> tasks created
2. `on_initiative_planned` fires -> dependency-validator gets all task descriptions + specs
3. Agent finds TASK-B imports from TASK-A but no `blocked_by` declared
4. Returns `rejected` with `missing_deps` data
5. Handler blocks -> user reviews suggestions and applies

## Implementation Scope

### What exists today
- Gate types enum with `ai` slot (`internal/gate/gate.go`)
- 6-level gate resolution hierarchy (`internal/gate/resolver.go`)
- Retry mechanism with `RETRY_CONTEXT` (`internal/executor/retry.go`)
- Decision persistence in DB (`internal/db/gate_decision.go`)
- Headless decision support via events (`internal/api/decision_server.go`)
- Agent registry and execution (`internal/agent/`)
- Variable resolver (`internal/variable/resolver.go`)

### What needs building
1. **AI gate evaluator** — implement `GateAI` in `internal/gate/`
2. **Extended gate config** — input/output/mode fields in phase template and workflow schemas
3. **Lifecycle event emitter** — trigger points in executor and CLI
4. **Before-phase triggers** — new injection point in executor loop
5. **Variable pipeline** — gate output -> variable resolver integration
6. **Script handler** — pipe output to configured commands
7. **Dependency validator agent** — built-in agent definition
8. **Proto updates** — extend PhaseTemplate and Workflow protos for new gate fields
9. **DB migration** — store extended gate config in phase_gates table

### What stays the same
- Gate resolution hierarchy (just resolves more fields now)
- Retry mechanism (unchanged, just receives richer context)
- Decision persistence (extended with output data)
- Human gate type (unchanged)
- Auto gate type (unchanged)
