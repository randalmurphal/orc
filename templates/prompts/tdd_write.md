# TDD: Write Tests Phase

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
## Test Isolation

Tests must not depend on each other. Each test sets up its own state and cleans up after itself.

- Use temporary directories for filesystem isolation
- Isolate environment variables per test
- Use in-memory databases or test fixtures for storage isolation
- Never share mutable state between tests
</test_isolation>

<error_path_testing>
## Error Path Testing

Every error path in the spec's Failure Modes table MUST have a corresponding test. Error paths are not optional — they are first-class test targets.

- Test that errors are returned (not swallowed)
- Test that error messages are meaningful
- Test that partial state is cleaned up on failure
- Test boundary conditions that trigger errors
</error_path_testing>

<test_classification>
## Test Classification

Classify each test you write into one of three types:

**Solitary tests**: Test a single unit in isolation with all collaborators replaced by test doubles (mocks, stubs, fakes). Use when the unit under test has complex logic that needs focused verification.

**Sociable tests**: Test a unit with its real collaborators. Preferred when collaborators are fast, deterministic, and side-effect free. Gives higher confidence that units work together correctly.

**Integration tests**: Test that new code is properly wired into existing code paths. Required when your task creates new functions or interfaces that should be called from production code. Verifies the connection exists, not just that individual pieces work.

If your task creates new functions that should be called from existing code paths, you MUST write an integration test proving the wiring works.
</test_classification>

<context>
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
</context>

<instructions>
Write tests BEFORE implementation. This is test-driven development.

Use test doubles only when the real collaborator is slow, nondeterministic, or has side effects. Prefer real collaborators when feasible.

Test error paths per the spec's Failure Modes table. Every error path in the spec MUST have a corresponding test.

## Step 1: Analyze Success Criteria

For each success criterion in the spec:
1. Identify what behavior needs testing
2. Determine test type (solitary, sociable, or integration)
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

For each success criterion:
1. Choose the appropriate test type (solitary, sociable, or integration)
2. Write test that exercises the expected behavior
3. Include edge cases from the spec's Edge Cases table
4. Include error paths from the spec's Failure Modes table

**Test naming:** `Test[Feature]_[Scenario]` or `describe('[Feature]', () => { it('[scenario]') })`

### Embedded Code / Script Testing

If the task produces **executable code that lives in the repo** (scripts, hooks, templates, config generators), you MUST write behavioral tests for that code — not just tests for the infrastructure that embeds or deploys it.

| Code Type | Test Approach | Example |
|-----------|---------------|---------|
| Bash scripts | Run script with controlled input, assert exit code and output | `echo '{"tool_name":"Write"}' \| bash script.sh; assert exit 2` |
| Python scripts | Import or subprocess, test with real inputs | `subprocess.run([script], input=json, capture_output=True)` |
| Hook scripts | Test blocking/allowing behavior with mock hook input | Assert exit 0 (allow) vs exit 2 (block) |
| Templates | Render with known data, assert output structure | `tmpl.Execute(data); assert output contains expected` |

**Anti-pattern:** Writing tests that only verify "script file exists on disk" or "script content was seeded to DB." These are infrastructure tests — necessary but NOT sufficient. The script's actual behavior (what it does when executed) must also be tested.

### Wiring Verification Pattern

If your task creates new functions or interfaces that should be called from existing code paths, you MUST write integration tests verifying the wiring is complete.

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

## Step 3: Verify Tests Fail

Run: `{{TEST_COMMAND}}`

**Expected result:** Tests fail or don't compile (code doesn't exist yet)

This is correct - it proves tests are testing real behavior, not mocks or stubs.

If any test passes before implementation, it's testing the wrong thing or the feature already exists.
</instructions>

<example_good_tdd>
SC-1 from spec: "Rate limiter returns 429 after limit exceeded"

Test (solitary):
```
func TestRateLimiter_Returns429AfterLimitExceeded(t *testing.T) {
    limiter := NewRateLimiter(Config{MaxRequests: 5, Window: time.Minute})
    handler := limiter.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(200)
    }))

    // First 5 requests succeed
    for i := 0; i < 5; i++ {
        rec := httptest.NewRecorder()
        handler.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
        assert.Equal(t, 200, rec.Code)
    }

    // 6th request is rate limited
    rec := httptest.NewRecorder()
    handler.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
    assert.Equal(t, 429, rec.Code)
    assert.NotEmpty(t, rec.Header().Get("Retry-After"))
}
```

Coverage mapping: SC-1 → TestRateLimiter_Returns429AfterLimitExceeded (solitary)
</example_good_tdd>
