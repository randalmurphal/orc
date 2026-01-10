# ADR-007: Human Gates

**Status**: Accepted  
**Date**: 2026-01-10

---

## Context

Automation is valuable but not unlimited. Some operations carry risk:

| Operation | Risk | Consequence |
|-----------|------|-------------|
| Write code | Medium | Bugs, fixable |
| Approve spec | Medium | Wrong direction |
| Merge to main | High | Breaks main branch |
| Deploy to prod | Critical | User impact |

## Decision

**Merge to main requires human approval by default.**

Other gates are configurable per-project:

| Phase | Default Gate | Large/Greenfield |
|-------|--------------|------------------|
| spec | AI | Human |
| design | AI | Human |
| implement | AI | AI |
| review | AI | AI |
| **merge** | **Human** | **Human** |

Projects can override to make merge automatic, but must explicitly opt-in.

## Rationale

### Why Human Gate for Merge?

1. Main branch protection (should always be deployable)
2. Blast radius (broken main affects entire team)
3. Compliance (many orgs require human approval)

### Gate Configuration

```yaml
# .orc/config.yaml
gates:
  spec: ai
  design: ai
  merge: human  # default, can override to 'ai'
  
  weight_overrides:
    large:
      spec: human
      design: human
```

### Human Gate Workflow

When reached:
1. Terminal prompt (if interactive)
2. Desktop notification (if configured)
3. Web UI notification
4. Email/Slack webhook (if configured)

Commands:
```bash
orc approve              # Approve and proceed
orc reject "reason"      # Reject, rewind to phase start
```

## Consequences

**Positive**:
- Safety by default
- Configurable per project
- Audit trail of all approvals

**Negative**:
- Friction slows automation
- Requires human availability

**Mitigation**: Smart notifications; batch approvals; context in notification.
