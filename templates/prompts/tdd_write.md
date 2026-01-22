<context>
# TDD: Write Tests Phase

<task>
ID: {{TASK_ID}}
Title: {{TASK_TITLE}}
Weight: {{WEIGHT}}
Category: {{TASK_CATEGORY}}
</task>

<project>
Language: {{LANGUAGE}}
Has Frontend: {{HAS_FRONTEND}}
Has Tests: {{HAS_TESTS}}
Test Command: {{TEST_COMMAND}}
Frameworks: {{FRAMEWORKS}}
</project>

<worktree_safety>
Path: {{WORKTREE_PATH}}
Branch: {{TASK_BRANCH}}
Target: {{TARGET_BRANCH}}
DO NOT push to {{TARGET_BRANCH}} or checkout other branches.
</worktree_safety>

{{INITIATIVE_CONTEXT}}
{{CONSTITUTION_CONTENT}}

<specification>
{{SPEC_CONTENT}}
</specification>

<design>
{{DESIGN_CONTENT}}
</design>
</context>

<instructions>
Write tests BEFORE implementation. This is test-driven development.

<critical_mindset>
You are writing tests for code that DOES NOT YET EXIST.

**DO NOT:**
- Speculate about implementation details
- Write tests that pass with empty implementations
- Create mock implementations
- Look at how similar features are currently implemented (context isolation)
- Test internal function signatures - test public interfaces only
- Hard-code specific return values - test behavior patterns

**DO:**
- Test all success criteria from the spec
- Test all edge cases and error conditions listed in the spec
- Write tests that will FAIL until implemented
- Follow existing test patterns in the codebase
- Test observable outcomes (return values, side effects, state changes)
- Test WHAT it does, not HOW it does it
</critical_mindset>

<test_isolation>
**Mocking Guidelines:**
- Mock external services (HTTP APIs, databases, file systems) at boundaries
- Never mock the code you're testing
- Prefer real implementations when fast enough (<100ms)
- If you need >3 mocks in a unit test, the test is likely too coupled

**Test Independence:**
- Each test must be runnable in isolation
- No shared mutable state between tests
- Use setup/teardown for database cleanup
- Don't rely on test execution order
</test_isolation>

<error_path_testing>
**Required Error Path Tests:**
For each operation, test:
1. **Invalid input** - wrong types, missing required fields, out-of-range values
2. **Resource not found** - what happens when expected data doesn't exist
3. **Permission denied** - unauthorized access attempts (if applicable)
4. **External failure** - service unavailable, timeout (if external dependencies)

Each error test should verify:
- Appropriate error message returned
- No partial state changes (atomicity)
- User receives actionable guidance
</error_path_testing>

## Step 1: Analyze Success Criteria

For each success criterion in the spec:
1. Identify what behavior needs testing
2. Determine test type (unit, integration, E2E)
3. List edge cases and error paths

## Step 2: Write Tests

{{#if HAS_FRONTEND}}
<ui_testing_strategy>
This project has a frontend. Choose based on existing setup:

### Option A: Playwright E2E Tests
If `playwright.config.ts` exists, write proper E2E tests:
- Create test files in the existing test directory
- Use existing patterns/helpers from other E2E tests
- Tests should fail until UI is implemented

### Option B: Manual Test Plan (Playwright MCP)
If no Playwright setup, create a structured test plan for manual execution:

```markdown
## Manual UI Test Plan

### Flow: [User Story from spec]
**Success Criteria:** SC-1, SC-2

#### Test Steps:
1. Navigate to [URL]
2. Verify [initial state]
3. Perform [action]
4. Assert [expected result]

#### Error Cases:
- If [condition], should see [error message]
- If [invalid input], should show [validation error]
```

The implement phase will execute this via Playwright MCP browser tools.
</ui_testing_strategy>
{{/if}}

<unit_integration_tests>
## Unit/Integration Tests

For each success criterion:
1. Identify the function/component/endpoint to test
2. Write test that exercises the expected behavior
3. Include edge cases from the spec's Edge Cases table
4. Include error paths from the spec's Failure Modes table

**Test naming:** `Test[Feature]_[Scenario]` or `describe('[Feature]', () => { it('[scenario]') })`

**Coverage goals:**
- All success criteria have at least one test
- All edge cases have tests
- All error paths have tests
</unit_integration_tests>

## Step 3: Verify Tests Fail

Run: `{{TEST_COMMAND}}`

**Expected result:** Tests fail or don't compile (code doesn't exist yet)

This is correct - it proves tests are testing real behavior, not mocks or stubs.

If any test passes before implementation, it's testing the wrong thing or the feature already exists.
</instructions>

<output_format>
Output a JSON object with test information:

```json
{
  "status": "complete",
  "summary": "Wrote N tests covering M success criteria. Tests correctly fail.",
  "artifact": "# TDD Tests for {{TASK_ID}}\n\n## Test Files\n\n| File | Type | Criteria |\n|------|------|----------|\n| path/to/test.ts | e2e | SC-1 |\n| path/to/unit.go | unit | SC-2, SC-3 |\n\n## Test Descriptions\n\n### path/to/test.ts\n- `test_user_can_login` - Verifies SC-1: user can authenticate\n- `test_invalid_password_rejected` - Error path: invalid credentials\n\n### path/to/unit.go\n- `TestFeatureX` - Verifies SC-2: feature behavior\n- `TestFeatureX_ErrorCase` - Error path: handles missing data\n\n## Manual Test Plan (if applicable)\n\n[Manual steps for UI testing if automated testing not feasible]"
}
```

The `artifact` field MUST contain:
1. Table of test files with their types and criteria coverage
2. Description of each test and what it verifies
3. Manual test plan section (if Option B was chosen for UI testing)

If blocked:
```json
{
  "status": "blocked",
  "reason": "[What's blocking and what's needed]"
}
```
</output_format>
