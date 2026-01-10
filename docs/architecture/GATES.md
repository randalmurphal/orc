# Quality Gates

**Purpose**: Control transitions between phases with configurable approval requirements.

---

## Gate Types

| Type | Description | Use Case |
|------|-------------|----------|
| `auto` | Proceed immediately if criteria met | Low-risk phases |
| `ai` | Claude evaluates whether to proceed | Medium-risk phases |
| `human` | Requires manual approval | High-risk phases |

---

## Default Gates by Weight

| Phase | Trivial | Small | Medium | Large | Greenfield |
|-------|---------|-------|--------|-------|------------|
| classify | auto | auto | auto | auto | auto |
| research | - | - | - | auto | human |
| spec | - | - | ai | human | human |
| design | - | - | - | human | human |
| implement | auto | auto | auto | auto | auto |
| review | - | ai | ai | ai | ai |
| test | auto | auto | auto | auto | auto |
| **merge** | **human** | **human** | **human** | **human** | **human** |

---

## Gate Configuration

```yaml
# orc.yaml
gates:
  # Override defaults
  spec: ai              # Downgrade from human
  merge: ai             # DANGER: auto-merge to main
  
  # Per-weight overrides
  weight_overrides:
    trivial:
      merge: auto       # Trust trivial changes
    greenfield:
      spec: human
      design: human
      review: human
```

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
