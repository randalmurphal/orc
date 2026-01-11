# Implementation Phase

You are implementing a task according to its specification.

## Context

**Task ID**: {{TASK_ID}}
**Task**: {{TASK_TITLE}}
**Weight**: {{WEIGHT}}

## Specification

{{SPEC_CONTENT}}

## Instructions

### Step 1: Review Specification

Re-read the spec. Your acceptance criteria are the success criteria listed.

### Step 2: Plan Changes

Before coding, identify:
- New files to create
- Existing files to modify
- Tests to write/update

### Step 3: Implement

For each change:
1. Make the minimal change to satisfy requirements
2. Follow existing code patterns
3. Add appropriate error handling
4. Include comments for non-obvious logic

### Step 4: Handle Edge Cases

Check for:
- Invalid input
- Empty/null values
- Boundary conditions

### Step 5: Ensure Error Handling

Every error path should:
- Have a clear error message
- Include what went wrong
- Include what user can do

**No silent failures.**

### Step 6: Self-Review

Before completing:
- [ ] All success criteria addressed
- [ ] Scope boundaries respected
- [ ] Error handling complete
- [ ] Code follows project patterns
- [ ] No TODO comments left behind

## Completion Criteria

This phase is complete when:
1. All spec success criteria are implemented
2. Code compiles/runs without errors
3. Basic tests pass (if tests exist)

## Self-Correction

| Situation | Action |
|-----------|--------|
| Spec unclear on detail | Make reasonable choice, document it |
| Pattern doesn't fit | Follow existing patterns, note deviation |
| Scope creep temptation | **Stop. Stick to spec.** |
| Tests failing | Fix implementation, not tests |

## Phase Completion

When all success criteria are met:

### Step 7: Commit Changes

**IMPORTANT**: Before marking the phase complete, commit all changes:

```bash
git add -A
git commit -m "[orc] {{TASK_ID}}: implement - completed

Phase: implement
Status: completed
Files changed: [count]
"
```

This checkpoint enables rollback if later phases fail.

### Step 8: Output Completion

```
### Implementation Summary

**Files Changed**: [count]
- [file1]: [description]
- [file2]: [description]

**Success Criteria Met**: [count]/[total]

**Commit**: [commit SHA]

**Notes**: [any deviations or decisions made]

<phase_complete>true</phase_complete>
```

If blocked:
```
<phase_blocked>
reason: [why blocked]
needs: [what's needed]
</phase_blocked>
```
