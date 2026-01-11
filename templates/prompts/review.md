# Code Review Phase

You are reviewing code changes for correctness, quality, and spec compliance.

## Context

**Task ID**: {{TASK_ID}}
**Task**: {{TASK_TITLE}}

## Specification

{{SPEC_CONTENT}}

## Changes to Review

{{CHANGES_DIFF}}

## Review Checklist

### 1. Spec Compliance

For each success criterion:
- [ ] Is it implemented?
- [ ] Is it implemented correctly?
- [ ] Is it implemented completely?

### 2. Correctness

Check for:
- Off-by-one errors
- Null/undefined handling
- Error paths handled
- Edge cases covered
- Race conditions (if applicable)

### 3. Security

Check for:
- Input validation
- SQL/command injection
- Auth/authz checks
- No hardcoded secrets
- No sensitive data logged

### 4. Quality

Check for:
- Code clarity (understandable in one read?)
- Pattern compliance
- Naming consistency
- Complex parts explained
- No dead code

### 5. Completeness

Check for:
- No TODO/FIXME in production code
- No commented-out code
- No debug logging left in
- No placeholder implementations

## Finding Categories

| Category | Action |
|----------|--------|
| `blocker` | Must fix before merge |
| `major` | Should fix, significant |
| `minor` | Should fix, small |
| `suggestion` | Optional improvement |

## Instructions

1. Review each changed file
2. Verify spec compliance
3. Check all categories above
4. Fix issues you can fix directly
5. Document issues you can't fix

## Phase Completion

### Commit Any Fixes

If you made any fixes during review, commit them before completing:

```bash
git add -A
git commit -m "[orc] {{TASK_ID}}: review - completed

Phase: review
Status: completed
Verdict: [APPROVED|NEEDS_CHANGES]
Fixes applied: [count]
"
```

### Output Format

```
### Review Summary

**Verdict**: [APPROVED | NEEDS_CHANGES | BLOCKED]

**Spec Compliance**: [pass]/[total] criteria met

**Commit**: [commit SHA if changes made]

### Findings

#### Blockers
[List or "None"]

#### Major Issues
[List or "None"]

#### Minor Issues
[List or "None"]

#### Suggestions
[List or "None"]

### Fixes Applied
- [Fix 1]
- [Fix 2]

<phase_complete>true</phase_complete>
```

If NEEDS_CHANGES:
```
<phase_blocked>
reason: [issues requiring author attention]
needs: [specific fixes needed]
</phase_blocked>
```
