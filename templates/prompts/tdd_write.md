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

<test_classification>
## Test Classification

Classify each test you write into one of these types:

| Type | What It Tests | When to Use |
|------|---------------|-------------|
| **Solitary** | Single unit in isolation, dependencies mocked | Pure logic, calculations, data transformations |
| **Sociable** | Unit with real collaborators (no mocks) | Units that work together within a module |
| **Integration** | New code is wired into existing code paths | New functions/interfaces that must be called from existing code |

**Default:** Most tests are solitary tests. Write sociable tests when collaborators are fast and deterministic. Write integration tests when you need to verify wiring.

### Integration Test Requirement

If your task creates new functions or new interfaces that should be called from existing code paths, you MUST write integration tests verifying the wiring is complete.

**When integration tests are required:**
- A new function is created that an existing code path should call
- A new interface implementation is created that should be registered or wired into the system
- An existing function is modified to delegate to new code

**When integration tests are NOT required:**
- Greenfield code with no existing callers yet (if there are no existing code paths that should call your new code)
- Pure refactors that don't change the call graph
- Internal helpers only called by new code you're also writing

### Wiring Verification Pattern

To verify new code is actually called from the expected code path, use this pattern:

```go
// Integration test: verify ProcessPipeline calls the new Processor
func TestProcessPipeline_CallsNewProcessor(t *testing.T) {
    called := false
    mock := func(input string) error {
        called = true
        return nil
    }
    pipeline := NewPipeline(WithProcessor(mock))
    pipeline.Run("test-input")
    if !called {
        t.Fatal("NewProcessor was not called from ProcessPipeline - wiring is missing")
    }
}
```

The pattern: (1) create a mock that sets a flag, (2) inject it into the caller, (3) run the caller, (4) assert the mock was called. If the assertion fails, the new code exists but is never invoked — dead code.
</test_classification>

## Step 1: Analyze Success Criteria

For each success criterion in the spec:
1. Identify what behavior needs testing
2. Determine test type using the classification above (solitary, sociable, or integration)
3. Identify which criteria require integration tests (new code wired into existing paths)
4. List edge cases and error paths

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
2. Choose the appropriate test type from the classification (solitary, sociable, or integration)
3. Write test that exercises the expected behavior
4. Include edge cases from the spec's Edge Cases table
5. Include error paths from the spec's Failure Modes table

**Test naming:** `Test[Feature]_[Scenario]` or `describe('[Feature]', () => { it('[scenario]') })`

**Coverage goals:**
- All success criteria have at least one test
- All edge cases have tests
- All error paths have tests
- New functions/interfaces wired into existing code have integration tests
</unit_integration_tests>

## Step 3: Verify Tests Fail

Run: `{{TEST_COMMAND}}`

**Expected result:** Tests fail or don't compile (code doesn't exist yet)

This is correct - it proves tests are testing real behavior, not mocks or stubs.

If any test passes before implementation, it's testing the wrong thing or the feature already exists.
</instructions>

<pre_output_verification>
## Pre-Output Verification (MANDATORY)

Before outputting the final JSON, STOP and verify:

1. **Re-read the spec's Success Criteria table**
   - List each SC-X identifier from the spec
   - For each SC-X, confirm you have at least one test that covers it

2. **Check coverage completeness**
   - Every SC-X must appear in `coverage.covered` OR `coverage.manual_verification`
   - If any SC-X is missing, write the missing test NOW before proceeding

3. **Verify test correctness**
   - Each test name accurately describes what it tests
   - Each `covers` array only lists criteria the test actually verifies
   - No test claims to cover criteria it doesn't actually test

4. **Confirm tests will fail**
   - Run `{{TEST_COMMAND}}` mentally - tests should fail or not compile
   - If tests would pass, they're testing existing behavior, not new work

**Only after completing this verification, output the StructuredOutput.**
</pre_output_verification>

<output_format>
Output a JSON object with test information and **explicit coverage mapping**:

```json
{
  "status": "complete",
  "summary": "Wrote N tests covering all M success criteria. Tests correctly fail.",
  "content": "# TDD Tests for {{TASK_ID}}\n\n## Coverage Summary\n\n| Criterion | Test | Status |\n|-----------|------|--------|\n| SC-1 | TestLogin | Covered |\n| SC-2 | TestLoginError | Covered |\n\n## Test Files\n\n### path/to/test.go\n- `TestLogin` - Verifies SC-1: user can authenticate\n- `TestLoginError` - Error path: invalid credentials\n\n## Manual Test Plan (if applicable)\n\n[Manual steps for UI testing]",
  "tests": [
    {
      "file": "path/to/test.go",
      "name": "TestLogin",
      "covers": ["SC-1"],
      "type": "unit"
    },
    {
      "file": "path/to/test.go",
      "name": "TestLoginError",
      "covers": ["SC-1"],
      "type": "unit"
    }
  ],
  "coverage": {
    "covered": ["SC-1", "SC-2"],
    "manual_verification": [
      {
        "criterion": "SC-3",
        "reason": "Visual regression requires human review",
        "steps": ["1. Open /settings", "2. Toggle dark mode", "3. Verify readability"]
      }
    ]
  }
}
```

**REQUIRED fields:**
- `tests[].covers` - Array of SC-X IDs this test covers
- `coverage.covered` - All criteria with automated tests
- `coverage.manual_verification` - Criteria that can't be automated (with justification)

**Validation:** All SC-X from spec must appear in either `covered` or `manual_verification`.

The `content` field MUST contain:
1. Coverage summary table showing each criterion and its test
2. Description of each test and what it verifies
3. Manual test plan section (if any criteria require manual verification)

If blocked:
```json
{
  "status": "blocked",
  "reason": "[What's blocking and what's needed]"
}
```
</output_format>
