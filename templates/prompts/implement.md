<common_failure_mode>
## CRITICAL: Most Common Implementation Failure

**Creating dead code that passes all tests but is never used in production.**

This happens when:
1. You create a new component/function
2. Tests pass (because unit tests test the component in isolation)
3. But you forget to wire it into the existing production code path
4. The new code is never called → dead code ships → review rejects

**Prevention:** Before claiming completion, for EVERY new file you created, verify an existing production file imports it. If nothing imports your new code, it's dead code.
</common_failure_mode>

<output_format>
## Pre-Completion Verification (MANDATORY - DO NOT SKIP)

Before outputting completion JSON, you MUST run all FIVE checks and include evidence for each:

1. **Tests**: Run `{{TEST_COMMAND}}` — exit code 0, all tests pass. If fails: fix implementation and re-run.
2. **Success Criteria**: For each SC-X in spec, run its verification method, record PASS/FAIL with evidence. If any FAIL: fix and re-verify.
3. **Build**: {{#if BUILD_COMMAND}}Run `{{BUILD_COMMAND}}`{{else}}Run the project build command{{/if}} — if fails, fix build errors.
4. **Linting**: {{#if LINT_COMMAND}}Run `{{LINT_COMMAND}}`{{else}}Run the project linter{{/if}} — if fails, fix lint errors.
5. **Wiring**: For EVERY new file created, grep the codebase to find which production file imports it. If no production file imports it → dead code → FAIL.

## Completion Output Format

ONLY after ALL verifications PASS, output JSON with `status`, `summary`, and `verification` fields containing `tests`, `success_criteria`, `build`, `linting`, and `wiring` — each with `status` and `evidence`. See `<example_good_completion>` below for exact schema.

**Wiring verification evidence format:**
```json
"wiring": {
  "status": "pass",
  "evidence": "Created components/feature/Panel.tsx → imported by pages/Dashboard.tsx:15",
  "new_files": [
    {"file": "components/feature/Panel.tsx", "imported_by": "pages/Dashboard.tsx:15"}
  ]
}
```
If ANY new file has no production importer, wiring status is "fail".

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

<no_op_guard>
## Optional Props with Empty Fallbacks = NO-OP (BANNED PATTERN)

If the spec requires behavior X, you CANNOT make it optional with an empty fallback:

**❌ BANNED:**
```tsx
// This compiles and renders but does NOTHING when clicked
interface Props {
  onAgentClick?: (agent: Agent) => void;  // Optional!
}
<AgentsPalette onAgentClick={onAgentClick || (() => {})} />  // Empty fallback!
```

**✅ REQUIRED:**
```tsx
// Parent MUST provide handler - wiring is enforced
interface Props {
  onAgentClick: (agent: Agent) => void;  // Required!
}
// And the parent component MUST be updated to pass it:
<LeftPalette onAgentClick={handleAgentClick} />
```

**The test:** If you can delete your new code and the app still compiles and runs identically, you created a no-op.

**If review feedback says "wire X to Y":**
- Making X optional is NOT wiring
- Adding an empty fallback is NOT wiring
- You MUST modify the parent (Y) to pass the actual handler
</no_op_guard>

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

{{#if SPEC_CONTENT}}
<specification>
{{SPEC_CONTENT}}
</specification>
{{/if}}

{{#if BREAKDOWN_CONTENT}}
<breakdown>
{{BREAKDOWN_CONTENT}}
</breakdown>
{{/if}}

{{#if RETRY_ATTEMPT}}
<retry_context>
## Retry Context

This is retry attempt **{{RETRY_ATTEMPT}}**, triggered from the **{{RETRY_FROM_PHASE}}** phase.

**Reason for retry:** {{RETRY_REASON}}

{{#if OUTPUT_REVIEW}}
### Previous Review Findings

{{OUTPUT_REVIEW}}
{{/if}}

Address ALL issues identified above before proceeding with implementation.
</retry_context>
{{/if}}
</context>

{{#if TDD_TESTS_CONTENT}}
<tdd_tests>
## Tests to Make Pass

{{TDD_TESTS_CONTENT}}

Your implementation MUST make these tests pass.

### Wiring Requirements (from TDD phase)

If the TDD tests include a `wiring` section, you MUST follow it exactly:
- Create new files at the paths specified in `new_component_path`
- Wire them into the files specified in `imported_by`
- The integration tests verify the wiring is correct

**Example:** If TDD says `"imported_by": "@/pages/Dashboard.tsx"`, then Dashboard.tsx MUST import your new component. Don't create it in a different location.

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
{{/if}}

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

Before modifying shared code, identify all callers and dependents.

### 2d. Forward-Looking Integration Check

Before claiming completion, verify all new code is reachable from production code paths:
- Are all new functions called from at least one production code path?
- Are all new interfaces registered and wired into the system?
- Is there any unused new code that should be integrated but isn't?

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

Before completing, verify:
- All success criteria addressed
- All TDD tests pass
- All breakdown items completed (if provided)
- Preservation requirements verified
- Scope boundaries respected
- Code follows project patterns
- No TODO comments left behind

## Step 7: Self-Review and Wiring Verification

**Dead code prevention checklist:**
- All new functions are called from at least one production code path (no dead code)
- All new interfaces are registered and wired into the system
- No unused imports, variables, or helper functions left behind

**Behavioral parity checklist (for parallel/async/alternate paths):**
If you added a new execution path that mirrors an existing one:
- List ALL behaviors from the original path
- Verify EACH behavior exists in the new path
- Common missed behaviors:
  - Condition evaluation
  - Hook/callback invocation
  - State updates
  - Error handling
  - Logging/metrics

**Integration verification:**
For each new function/interface, answer: "What production code path calls this?"
- If you can't answer → you have dead code → wire it in or delete it
- If the answer is "tests only" → that's dead code (tests don't ship)

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
3. {{#if BUILD_COMMAND}}Run `{{BUILD_COMMAND}}`{{else}}Run the project build command{{/if}} — fix any build errors.
4. {{#if LINT_COMMAND}}Run `{{LINT_COMMAND}}`{{else}}Run the project linter{{/if}} — fix lint errors (unchecked returns, unused imports, type errors).
5. **Wiring check** — For each new function/interface, grep to confirm it's called from production code. Dead code = FAIL.
6. **Behavioral parity check** — If you added a parallel/async path, verify ALL original behaviors are present.

**Only output completion JSON after all six checks pass.** See Output Format for the exact schema.
</verification>

<commit_step>
## Step 7: Commit Your Changes

Before outputting completion JSON, commit all work to preserve it:

```bash
git add -A
git commit -m "[orc] {{TASK_ID}}: implement - [brief description]

Co-Authored-By: Claude Sonnet 4.5 <noreply@anthropic.com>"
```

**CRITICAL:** Always commit before claiming completion. Uncommitted work may be lost if execution is interrupted or the task fails.
</commit_step>

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
