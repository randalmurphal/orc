<output_format>
## Pre-Completion Verification (MANDATORY - DO NOT SKIP)

Before outputting completion JSON, you MUST run all four checks and include evidence for each:

1. **Tests**: Run `{{TEST_COMMAND}}` — exit code 0, all tests pass. If fails: fix implementation and re-run.
2. **Success Criteria**: For each SC-X in spec, run its verification method, record PASS/FAIL with evidence. If any FAIL: fix and re-verify.
3. **Build**: Run `{{BUILD_COMMAND}}` — if fails, fix build errors.
4. **Linting**: Run `{{LINT_COMMAND}}` — if fails, fix lint errors.

## Completion Output Format

ONLY after ALL verifications PASS, output JSON with `status`, `summary`, and `verification` fields containing `tests`, `success_criteria`, `build`, and `linting` — each with `status` and `evidence`. See `<example_good_completion>` below for exact schema.

**CRITICAL:** The `verification` field is MANDATORY. Completion without verification evidence will be REJECTED.

If blocked: `{"status": "blocked", "reason": "[why blocked and what's needed]"}`
</output_format>

## Critical Constraints

<over_engineering_guard>
If you find yourself creating a helper function, utility class, or abstraction the spec didn't request — stop. Delete it. Implement exactly what was specified.
Do not add error handling for scenarios that can't occur.
Do not design for hypothetical future requirements.
Three similar lines of code are better than a premature abstraction.
</over_engineering_guard>

<verification_mandate>
The most common failure is declaring completion without running verification. If you haven't run `{{TEST_COMMAND}}` and seen all tests pass, you are not done. If you haven't verified every success criterion, you are not done.
</verification_mandate>

<context>
# Implementation Phase

<task>
ID: {{TASK_ID}}
Title: {{TASK_TITLE}}
Weight: {{WEIGHT}}
Category: {{TASK_CATEGORY}}
</task>

<project>
Language: {{LANGUAGE}}
Has Frontend: {{HAS_FRONTEND}}
Test Command: {{TEST_COMMAND}}
</project>

<worktree_safety>
Path: {{WORKTREE_PATH}}
Branch: {{TASK_BRANCH}}
Target: {{TARGET_BRANCH}}
DO NOT push to {{TARGET_BRANCH}} or any protected branch.
DO NOT checkout {{TARGET_BRANCH}} - stay on your task branch.
</worktree_safety>

{{INITIATIVE_CONTEXT}}
{{CONSTITUTION_CONTENT}}

<specification>
{{SPEC_CONTENT}}
</specification>

<breakdown>
{{BREAKDOWN_CONTENT}}
</breakdown>

{{RETRY_CONTEXT}}
</context>

<tdd_tests>
## Tests to Make Pass

{{TDD_TESTS_CONTENT}}

Your implementation MUST make these tests pass.

**Before claiming completion:**
1. Run all tests: `{{TEST_COMMAND}}`
2. All tests MUST pass

**When tests fail:**
1. First verify the test is correct against the spec
2. If test matches spec: fix implementation
3. If test contradicts spec: document as AMEND-xxx, fix BOTH spec and test
4. NEVER delete a failing test without replacement
5. NEVER change assertions just to make buggy code pass
</tdd_tests>

{{#if TDD_TEST_PLAN}}
<manual_ui_testing>
## Manual UI Testing Required

{{TDD_TEST_PLAN}}

Use Playwright MCP tools to execute this test plan:
- `browser_navigate` to URLs
- `browser_click` on elements
- `browser_snapshot` to verify state
- `browser_type` for form inputs

Document test results in your completion output.
</manual_ui_testing>
{{/if}}

<instructions>
Implement the task according to its specification, making all TDD tests pass.

## Step 1: Review Specification

Re-read the spec. Your acceptance criteria are the success criteria listed.

Pay special attention to:
- **Preservation Requirements**: What must NOT change
- **Feature Replacements**: What's being replaced and any migrations needed
- **TDD Tests**: What tests must pass

## Step 2: Impact Analysis

Before modifying shared code, identify all callers and dependents. Before claiming completion, verify all new code is reachable from production code paths.

## Step 3: Follow Breakdown

{{#if BREAKDOWN_CONTENT}}
**MANDATORY:** Complete items in the order specified in the breakdown above.

For each item:
1. Implement the specific changes listed
2. Verify the linked TDD test now passes
3. Check off the item (mentally track progress)
4. Do NOT skip items or combine them arbitrarily
{{else}}
Plan your changes based on the spec:
- New files to create
- Existing files to modify
- Tests to write/update
{{/if}}

## Step 4: Implement

Implement fully — no TODOs, no placeholders, no commented-out code. Handle edge cases and errors per the spec's Failure Modes table.

Follow existing code patterns. Stay within scope but be thorough within that scope.

## Step 5: Self-Review

Before completing: all success criteria addressed, all TDD tests pass, all breakdown items completed (if provided), preservation requirements verified, scope boundaries respected, code follows project patterns, all new functions called from production code (no dead code), no TODO comments left behind.

## Completion Criteria

This phase is complete when all spec success criteria are implemented, all TDD tests pass, and code compiles without errors.

**Scope creep?** Stop. Stick to spec. **Tests failing?** Fix implementation, not tests. **Spec wrong?** Document as amendment below.

## Spec Amendments

If the spec doesn't match reality, document amendments:

```
AMEND-001: [Original] → [Actual] — [Reason]
```
</instructions>

<verification>
## Step 6: Verify and Complete

Execute the verification steps defined in the Output Format section above:
1. Run `{{TEST_COMMAND}}` — all TDD tests must pass. Fix implementation (not tests) on failure.
2. Verify each success criterion from the spec — run its verification method, record PASS/FAIL with evidence.
3. Run `{{BUILD_COMMAND}}` — fix any build errors.
4. Run `{{LINT_COMMAND}}` — fix lint errors (unchecked returns, unused imports, type errors).

**Only output completion JSON after all four checks pass.** See Output Format for the exact schema.
</verification>

<example_good_completion>
{
  "status": "complete",
  "summary": "Implemented token bucket rate limiter middleware with per-IP tracking and 429 responses",
  "verification": {
    "tests": {"status": "pass", "evidence": "ok github.com/project/internal/middleware 0.023s (5 tests, 0 failures)"},
    "success_criteria": [
      {"id": "SC-1", "status": "pass", "evidence": "TestRateLimiter_Returns429AfterLimitExceeded passes"},
      {"id": "SC-2", "status": "pass", "evidence": "TestRateLimiter_ResetsAfterWindow passes"},
      {"id": "SC-3", "status": "pass", "evidence": "grep confirms rateLimiter in router.go:45"}
    ],
    "build": {"status": "pass", "evidence": "go build ./... exits 0"},
    "linting": {"status": "pass", "evidence": "golangci-lint run exits 0"}
  }
}
Only output this JSON after actually running the tests and seeing them pass.
</example_good_completion>
