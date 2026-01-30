# Quality Gates

**Purpose**: Control transitions between phases with configurable approval requirements.

---

## Automation-First Philosophy

Orc defaults to **fully automated gates** - the system runs without human intervention by default. Human gates are opt-in for workflows that require oversight.

**Quality assurance is handled by backpressure** (tests, lint, build), not LLM-based evaluation. This provides deterministic, repeatable quality checks.

---

## Gate Types

| Type | Description | Use Case |
|------|-------------|----------|
| `auto` | Proceed immediately if criteria met | Default for all phases |
| `human` | Requires manual approval | Critical decisions |
| `skip` | No gate, always continues | Fast iteration |
| `ai` | AI agent evaluates gate | Automated review/validation |

---

## Gate Modes

| Mode | Behavior | Use Case |
|------|----------|----------|
| `gate` | Synchronous, blocks phase progression until resolved | Default - validation before proceeding |
| `reaction` | Asynchronous, fire-and-forget | Notifications, logging, non-blocking checks |

**Location**: `internal/workflow/types.go:40-46`

---

## Gate Actions

Actions define what happens on gate approval or rejection:

| Action | Behavior |
|--------|----------|
| `continue` | Continue to next phase |
| `retry` | Retry from specified phase (see `retry_from`) |
| `fail` | Fail the task |
| `skip_phase` | Skip the next phase |
| `run_script` | Execute a script (see `script`) |

**Location**: `internal/workflow/types.go:48-57`

---

## Gate Input/Output Configuration

### GateInputConfig

Controls what context flows into the gate evaluator:

| Field | Type | Purpose |
|-------|------|---------|
| `include_phase_output` | `[]string` | Phase IDs whose output to include |
| `include_task` | `bool` | Include task details in context |
| `extra_vars` | `[]string` | Additional variable names to pass |

### GateOutputConfig

Controls what happens with gate evaluation results:

| Field | Type | Purpose |
|-------|------|---------|
| `variable_name` | `string` | Store result in workflow variable |
| `on_approved` | `GateAction` | Action when gate approves |
| `on_rejected` | `GateAction` | Action when gate rejects |
| `retry_from` | `string` | Phase to retry from (when action=`retry`) |
| `script` | `string` | Script path (when action=`run_script`) |

**Location**: `internal/workflow/types.go:69-83`, DB layer: `internal/db/gate_config.go`

---

## Before-Phase Triggers

Triggers that run before a phase starts, enabling pre-validation or preparation:

```yaml
workflow_phases:
  - phase_template_id: implement
    before_triggers:
      - agent_id: "dependency-check"
        mode: gate               # blocks if agent rejects
        input_config:
          include_task: true
        output_config:
          on_rejected: fail
```

| Field | Type | Purpose |
|-------|------|---------|
| `agent_id` | `string` | Agent to execute |
| `input_config` | `GateInputConfig` | Context for the agent |
| `output_config` | `GateOutputConfig` | Result handling |
| `mode` | `GateMode` | `gate` (blocking) or `reaction` (async) |

**Location**: `internal/workflow/types.go:85-91`

---

## Workflow Lifecycle Triggers

React to task/initiative lifecycle events at the workflow level:

```yaml
workflows:
  triggers:
    - event: on_task_completed
      agent_id: "notify-slack"
      mode: reaction
      enabled: true
    - event: on_task_failed
      agent_id: "failure-analyzer"
      mode: gate
      input_config:
        include_task: true
```

| Event | Fires When |
|-------|------------|
| `on_task_created` | Task is created |
| `on_task_completed` | Task completes successfully |
| `on_task_failed` | Task fails |
| `on_initiative_planned` | Initiative planning completes |

**Location**: `internal/workflow/types.go:59-101`

---

## Automation Profiles

| Profile | Default Gate | Description |
|---------|--------------|-------------|
| `auto` | All auto | Default - Full automation, no human approval |
| `fast` | All auto + no pre-merge | Maximum speed, no retry on failure |
| `safe` | Auto + human merge | Balanced - Automatic until final merge |
| `strict` | Human on spec/merge | Full oversight for critical phases |

```bash
# Run with profile
orc run TASK-001 --profile auto    # (default)
orc run TASK-001 --profile safe    # human on merge
orc run TASK-001 --profile strict  # human on spec/merge
```

---

## Default Gates by Weight (auto profile)

| Phase | Trivial | Small | Medium | Large | Greenfield |
|-------|---------|-------|--------|-------|------------|
| research | - | - | - | auto | auto |
| spec | - | - | - | auto | auto |
| implement | auto | auto | auto | auto | auto |
| test | auto | auto | auto | auto | auto |
| validate | - | - | - | auto | auto |

---

## Gate Configuration

```yaml
# orc.yaml - default automation-first configuration
gates:
  default_type: auto              # Default gate type for all phases
  auto_approve_on_success: true   # Auto-approve when phase succeeds
  retry_on_failure: true          # Enable cross-phase retry
  max_retries: 3                  # Max retry attempts per phase

  # Override specific phases
  phase_overrides:
    merge: human                  # Human approval before merge

  # Override by weight
  weight_overrides:
    greenfield:
      spec: human                 # Human review for greenfield specs

# Cross-phase retry configuration
retry:
  enabled: true
  max_retries: 3
  retry_map:
    test: implement              # Test failures retry from implement
    validate: implement          # Validation failures retry from implement
```

### Extended Gate Configuration

Phase templates and workflow phases support additional gate fields:

```yaml
# Phase template with AI gate
phase_templates:
  - id: security-review
    gate_type: ai
    gate_agent_id: "security-reviewer"
    gate_mode: gate
    gate_input_config:
      include_phase_output: ["implement"]
      include_task: true
    gate_output_config:
      variable_name: "SECURITY_RESULT"
      on_approved: continue
      on_rejected: retry
      retry_from: implement
```

---

## Cross-Phase Retry

When a gate rejects or a phase fails, orc can automatically retry from an earlier phase:

```
implement → test (FAIL) → implement (retry #1) → test → validate
```

The retry phase receives **{{RETRY_CONTEXT}}** in its prompt:
- What phase failed
- Why it failed (error message or gate rejection reason)
- Output from the failed phase
- Which retry attempt this is

This enables the agent to fix the root cause rather than just re-running blindly.

---

## Auto Gate Criteria

Auto gates check deterministic criteria against phase output:

| Criterion | Description |
|-----------|-------------|
| `has_output` | Phase produced non-empty output |
| `no_errors` | Output doesn't contain "error" |
| `has_completion_marker` | JSON response has `{"status": "complete"}` |
| Custom string | Check if string appears in output |

```yaml
# Plan YAML - auto gate with criteria
phases:
  - id: implement
    gate:
      type: auto
      criteria:
        - has_output
        - has_completion_marker
```

---

## Human Gate Workflow

Human gates work differently depending on whether the task is running interactively (CLI) or headlessly (API/WebSocket).

### Interactive Mode (CLI)

When running via `orc run` in a terminal:

```
[GATE] Human approval required for merge

Task: TASK-001 - Add user authentication
Phase: merge
Files changed: 8
Tests: 24 passing

Approve? [y/n/q(questions)]: _
```

The CLI blocks and waits for input. Enter `y` to approve, `n` to reject with reason, or `q` to ask clarifying questions.

### Headless Mode (API/WebSocket)

When running via the API (e.g., from the web UI), gates don't block:

1. **Task hits human gate** - Gate evaluator detects headless mode
2. **Task blocked** - Status changes to `blocked`
3. **Event emitted** - `decision_required` WebSocket event broadcast
4. **User notified** - Web UI shows approval prompt
5. **User decides** - Frontend calls `POST /api/decisions/:id`
6. **Decision recorded** - State and database updated
7. **Status updated** - Task becomes `planned` (approved) or `failed` (rejected)
8. **Resume required** - User must explicitly resume the task

```
┌─────────────┐    ┌──────────────┐    ┌─────────────────────┐
│  Executor   │───▶│   Gate       │───▶│ PendingDecisionStore│
│ (phase run) │    │ (human gate) │    │ (in-memory map)     │
└─────────────┘    └──────────────┘    └─────────────────────┘
                          │                      │
                          ▼                      ▼
                   ┌──────────────┐    ┌─────────────────────┐
                   │  Publisher   │    │  POST /decisions/:id│
                   │ (emit event) │    │  (resolve decision) │
                   └──────────────┘    └─────────────────────┘
                          │                      │
                          ▼                      ▼
                   ┌───────────────────────────────────────────┐
                   │         WebSocket Subscribers             │
                   │  (receive decision_required/resolved)     │
                   └───────────────────────────────────────────┘
```

**Note:** Pending decisions are stored in-memory. Server restart clears them; tasks remain blocked and can be resolved via `orc approve` CLI.

### Notification Channels

1. **Terminal** (if interactive):
   ```
   [GATE] Human approval required for merge

   Task: TASK-001 - Add user authentication
   Phase: merge
   Files changed: 8
   Tests: 24 passing

   orc approve TASK-001    # Approve
   orc reject TASK-001     # Reject with reason
   orc diff TASK-001       # View changes
   ```

2. **WebSocket Event** (if headless):
   ```json
   {
     "type": "event",
     "event_type": "decision_required",
     "data": {
       "decision_id": "gate_TASK-001_merge",
       "task_id": "TASK-001",
       "task_title": "Add user authentication",
       "phase": "merge",
       "gate_type": "human",
       "question": "Please verify the following criteria:",
       "context": "Code review passes\nTests pass",
       "requested_at": "2026-01-10T10:30:00Z"
     }
   }
   ```

3. **Desktop Notification** (if configured)
4. **Webhook** (Slack, email, etc.)

### Approval Commands

```bash
# Approve current gate
orc approve TASK-001

# Approve with comment
orc approve TASK-001 --comment "LGTM"

# Reject with reason (required)
orc reject TASK-001 --reason "Missing error handling"

# View what's pending
orc status --waiting
```

---

## Gate Audit Trail

```yaml
# Database: states table
gates:
  - phase: spec
    type: auto
    decision: approved
    timestamp: 2026-01-10T10:45:00Z

  - phase: merge
    type: human
    decision: approved
    timestamp: 2026-01-10T15:45:00Z
    approver: randy
    comment: "Tested locally, looks good"
```

---

## Quality Checks (Phase-Level)

Quality is validated through **phase-level quality checks** - deterministic checks that run after a phase claims completion.

### Configuration

Quality checks are defined per phase template in the database:

```json
[
  {"type": "code", "name": "tests", "enabled": true, "on_failure": "block"},
  {"type": "code", "name": "lint", "enabled": true, "on_failure": "block"},
  {"type": "code", "name": "build", "enabled": true, "on_failure": "block"},
  {"type": "code", "name": "typecheck", "enabled": true, "on_failure": "block"}
]
```

### Check Types

| Type | Behavior |
|------|----------|
| `code` | Looks up command from `project_commands` table by name |
| `custom` | Uses the `command` field directly |

### On-Failure Modes

| Mode | Behavior |
|------|----------|
| `block` | Phase fails, context injected for retry |
| `warn` | Warning logged, completion accepted |
| `skip` | Check disabled |

### Project Commands

Commands are seeded during `orc init` based on project detection and stored in the `project_commands` database table. Manage with `orc config commands`.

Quality checks provide objective, repeatable quality validation without LLM judgment calls.

See `internal/executor/quality_checks.go` for implementation.

---

## Emergency Override

```bash
# Force approval (logged with reason)
orc approve TASK-001 --force --reason "P0 hotfix"
```

Creates audit entry:
```yaml
- phase: merge
  type: human
  decision: override
  approver: randy
  override_reason: "P0 hotfix"
  timestamp: 2026-01-10T03:00:00Z
```
