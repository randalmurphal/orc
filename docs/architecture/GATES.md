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
| `none` | Skip gate entirely | Fast iteration |

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

2. **Desktop Notification** (if configured)
3. **Web UI** (banner, one-click approve/reject)
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
# .orc/tasks/TASK-001/state.yaml
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

## Backpressure (Quality Checks)

Quality is validated through **backpressure** - deterministic checks that run after a phase claims completion:

| Check | Description |
|-------|-------------|
| Tests | Run test suite, fail if tests fail |
| Lint | Run linter, fail if errors |
| Build | Run build, fail if errors |
| Type check | Run type checker, fail if errors |

Backpressure provides objective, repeatable quality validation without LLM judgment calls.

See `internal/executor/backpressure.go` for implementation.

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
