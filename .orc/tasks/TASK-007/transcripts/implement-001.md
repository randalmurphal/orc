# implement - Iteration 1

## Prompt

# Implementation Phase

You are implementing a task according to its specification.

## Context

**Task ID**: TASK-007
**Task**: Test retry context: create a simple calculator.go file with Add, Subtract, Multiply functions
**Weight**: small

## Worktree Safety

You are working in an **isolated git worktree**. This ensures your changes don't affect other work.

| Property | Value |
|----------|-------|
| Worktree Path | `{{WORKTREE_PATH}}` |
| Task Branch | `{{TASK_BRANCH}}` |
| Target Branch | `{{TARGET_BRANCH}}` |

**CRITICAL SAFETY RULES:**
- You are on branch `{{TASK_BRANCH}}` - all commits go here
- **DO NOT** push to `{{TARGET_BRANCH}}` or any protected branch (main, master, develop, release)
- **DO NOT** checkout `{{TARGET_BRANCH}}` - stay on your task branch
- All merging is handled via PR after all phases complete
- Git hooks are active to prevent accidental protected branch modifications

## Specification



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
git commit -m "[orc] TASK-007: implement - completed

Phase: implement
Status: completed
Files changed: [count]
"
```

This checkpoint enables rollback if later phases fail.

### Step 8: Output Completion

Wrap your implementation summary in artifact tags for automatic persistence:

<artifact>
## Implementation Summary

**Task**: Test retry context: create a simple calculator.go file with Add, Subtract, Multiply functions

### Files Changed
- [file1]: [description]
- [file2]: [description]

### Success Criteria Met
- [x] [Criterion 1]
- [x] [Criterion 2]

### Notes
[Any deviations or decisions made]
</artifact>

Then output the completion marker:

```
**Commit**: [commit SHA]

<phase_complete>true</phase_complete>
```

If blocked:
```
<phase_blocked>
reason: [why blocked]
needs: [what's needed]
</phase_blocked>
```


## Response

The implementation has already been completed in a previous run (commit 34a2702). The calculator.go file exists with all three functions, tests are passing, and there are no uncommitted changes.

<artifact>
## Implementation Summary

**Task**: Test retry context: create a simple calculator.go file with Add, Subtract, Multiply functions

### Files Changed
- `pkg/calculator/calculator.go`: Package with Add, Subtract, Multiply functions (already implemented)
- `pkg/calculator/calculator_test.go`: Comprehensive tests for all functions (already implemented)

### Success Criteria Met
- [x] Add function implemented - returns sum of two integers
- [x] Subtract function implemented - returns difference of two integers
- [x] Multiply function implemented - returns product of two integers
- [x] All tests pass

### Notes
Implementation was already completed in commit 34a2702. Tests pass with `GOWORK=off go test ./pkg/calculator/` (go.work disabled needed due to worktree path resolution).
</artifact>

**Commit**: 22fb8f7 (implementation already committed from previous phase)

<phase_complete>true</phase_complete>

---
Tokens: 234 input, 1902 output
Complete: true
Blocked: false
