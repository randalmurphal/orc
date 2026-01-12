# test - Iteration 1

## Prompt

# Test Phase

You are writing and running tests to verify the implementation.

## Context

**Task ID**: TASK-007
**Task**: Test retry context: create a simple calculator.go file with Add, Subtract, Multiply functions
**Weight**: small

## Worktree Safety

You are working in an **isolated git worktree**.

| Property | Value |
|----------|-------|
| Worktree Path | `{{WORKTREE_PATH}}` |
| Task Branch | `{{TASK_BRANCH}}` |
| Target Branch | `{{TARGET_BRANCH}}` |

**CRITICAL SAFETY RULES:**
- All commits go to branch `{{TASK_BRANCH}}`
- **DO NOT** push to `{{TARGET_BRANCH}}` or any protected branch
- **DO NOT** checkout other branches - stay on `{{TASK_BRANCH}}`
- Merging happens via PR after all phases complete

## Specification



## Implementation Summary

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

## Instructions

### Step 1: Identify Test Cases

For each success criterion in the spec:
- Create at least one test case
- Include positive and negative cases
- Cover edge cases mentioned in spec

### Step 2: Write Unit Tests

For each component:
- Test public functions/methods
- Test error handling paths
- Test boundary conditions
- Mock external dependencies

### Step 3: Write Integration Tests

Test component interactions:
- API endpoints with real handlers
- Database operations (use test DB)
- External service calls (use mocks)

### Step 4: Run Tests

```bash
# For Go projects
go test ./... -v -cover

# For Node projects
npm test

# For Python projects
pytest -v --cov
```

### Step 5: Verify Coverage

Target: >80% coverage on new code

If coverage is low:
- Identify untested paths
- Add tests for uncovered code
- Focus on error paths (often missed)

## Test Patterns

| Pattern | Use Case |
|---------|----------|
| Table-driven tests | Multiple inputs/outputs |
| Subtests | Grouped related cases |
| Fixtures | Shared setup/teardown |
| Mocks | External dependencies |

## Output Format

### Test Summary

```
### Test Results

**Total Tests**: [count]
**Passed**: [count]
**Failed**: [count]
**Coverage**: [percent]%

### Tests Written
- [test_file1]: [count] tests
- [test_file2]: [count] tests

### Coverage by Package
- [package1]: [percent]%
- [package2]: [percent]%
```

## Phase Completion

### Commit Tests

```bash
git add -A
git commit -m "[orc] TASK-007: test - completed

Phase: test
Status: completed
Tests: [count] passed
Coverage: [percent]%
"
```

### Output Completion

```
### Test Summary

**Tests**: [passed]/[total] passing
**Coverage**: [percent]%
**Commit**: [commit SHA]

<phase_complete>true</phase_complete>
```

If tests fail:
```
<phase_blocked>
reason: [count] tests failing
needs: [specific failures to fix]
</phase_blocked>
```


## Response

### Test Summary

**Tests**: 18/18 passing (6 test functions, 15 subtests, 3 example tests)
**Coverage**: 100%
**Commit**: 99cc2d4

### Tests Written
- `pkg/calculator/calculator_test.go`: 6 tests (3 unit + 3 example)
  - `TestAdd`: 5 subtests (positive, zeros, negative/positive, both negative, large numbers)
  - `TestSubtract`: 5 subtests (positive result, zeros, negative result, subtracting negative, equal numbers)
  - `TestMultiply`: 5 subtests (positive, zero, negative/positive, both negative, multiply by one)
  - `ExampleAdd`, `ExampleSubtract`, `ExampleMultiply`: Documentation examples

### Coverage by Package
- `pkg/calculator`: 100%

<phase_complete>true</phase_complete>

---
Tokens: 17 input, 3427 output
Complete: true
Blocked: false
