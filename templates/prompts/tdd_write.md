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

**DO:**
- Test all success criteria from the spec
- Test all edge cases and error conditions listed in the spec
- Write tests that will FAIL until implemented
- Follow existing test patterns in the codebase
</critical_mindset>

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
  "tests_written": [
    {"path": "path/to/test.ts", "type": "e2e", "criteria": ["SC-1"]},
    {"path": "path/to/unit.go", "type": "unit", "criteria": ["SC-2", "SC-3"]}
  ],
  "manual_test_plan": "## Manual UI Test Plan\n..."
}
```

The `manual_test_plan` field is only included if Option B (manual testing) was chosen for UI testing.

If blocked:
```json
{
  "status": "blocked",
  "reason": "[What's blocking and what's needed]"
}
```
</output_format>
