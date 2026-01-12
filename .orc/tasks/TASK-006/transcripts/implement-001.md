# implement - Iteration 1

## Prompt

# Implementation Phase

You are implementing a task according to its specification.

## Context

**Task ID**: TASK-006
**Task**: Add a greeting.go file with a simple Hello World function
**Weight**: medium

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
git commit -m "[orc] TASK-006: implement - completed

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

**Task**: Add a greeting.go file with a simple Hello World function

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

<artifact>
## Implementation Summary

**Task**: Add a greeting.go file with a simple Hello World function

### Files Changed
- `internal/greeting/greeting.go`: New package with `Hello()` function returning "Hello, World!"
- `internal/greeting/greeting_test.go`: Comprehensive test suite with 5 test cases, 1 benchmark, and 1 example

### Success Criteria Met
- [x] Created greeting.go with Hello function
- [x] Function returns "Hello, World!" string
- [x] Tests pass successfully
- [x] Code follows project patterns (package comment, standard Go style)

### Notes
- Implementation was already completed in previous phases (implement, test, docs)
- All tests pass: `go test ./internal/greeting/ -v` shows 6 passing tests
- Package placed in `internal/greeting/` following project conventions
</artifact>

**Commit**: 7bc8268

<phase_complete>true</phase_complete>

---
Tokens: 379 input, 4313 output
Complete: true
Blocked: false
