# Quality Gates

**Purpose**: Control transitions between phases with configurable approval requirements.

---

## Automation-First Philosophy

Orc defaults to **fully automated gates** - the system runs without human intervention by default. Human gates are opt-in for workflows that require oversight.

---

## Gate Types

| Type | Description | Use Case |
|------|-------------|----------|
| `auto` | Proceed immediately if criteria met | Default for all phases |
| `ai` | Claude evaluates whether to proceed | When judgment needed |
| `human` | Requires manual approval | Critical decisions |

---

## Automation Profiles

| Profile | Default Gate | Description |
|---------|--------------|-------------|
| `auto` | All auto | Default - Full automation, no human approval |
| `fast` | All auto | Maximum speed, no retry on failure |
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

## AI Gate Evaluation

```go
func EvaluateAIGate(task *Task, phase *Phase) (Decision, error) {
    prompt := fmt.Sprintf(`
Review the output of phase "%s" for task "%s".

Phase Output Summary:
%s

Criteria for approval:
%s

Respond with:
- APPROVED: If all criteria are met
- REJECTED: If criteria not met, with specific issues
- NEEDS_CLARIFICATION: If you need more information

Decision:
`, phase.Name, task.Title, phase.Summary, phase.GateCriteria)
    
    result := RunClaudeSession(prompt)
    return ParseGateDecision(result.Output)
}
```

### AI Gate Decision Outcomes

| Decision | Behavior |
|----------|----------|
| `APPROVED` | Proceed to next phase immediately |
| `REJECTED` | Rewind to phase start, set status to `failed`, create rejection report |
| `NEEDS_CLARIFICATION` | Escalate to human gate with AI's questions attached |

**NEEDS_CLARIFICATION flow**:
1. AI identifies ambiguity or missing information
2. Gate escalates to human with AI's specific questions
3. Human provides clarification (via `orc approve --clarify`)
4. Clarification added to task context
5. Phase re-runs with additional context

```yaml
# state.yaml when clarification pending
gates:
  - phase: review
    type: ai
    decision: needs_clarification
    timestamp: 2026-01-10T10:45:00Z
    questions:
      - "Should the OAuth tokens be stored in session or database?"
      - "Is MFA a hard requirement or nice-to-have?"
    status: pending_human
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
    type: ai
    decision: approved
    timestamp: 2026-01-10T10:45:00Z
    rationale: "Spec covers all requirements"
    
  - phase: merge
    type: human
    decision: approved
    timestamp: 2026-01-10T15:45:00Z
    approver: randy
    comment: "Tested locally, looks good"
```

---

## Gate Criteria

### Built-in Criteria

| Criterion | Description |
|-----------|-------------|
| `tests_pass` | All tests pass |
| `lint_clean` | No linting errors |
| `type_check` | Type checker passes |
| `coverage: N` | Coverage >= N% |
| `no_todos` | No TODO comments in new code |
| `docs_updated` | Documentation files touched |

### Custom Criteria

```yaml
gates:
  implement:
    type: ai
    criteria:
      - tests_pass
      - lint_clean
      - custom: "./scripts/validate-api.sh"
```

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
