<output_format>
Output a JSON object with the spec, test information, **explicit coverage mapping**, and quality checklist:

```json
{
  "status": "complete",
  "summary": "Defined N criteria, wrote M failing tests covering all criteria",
  "content": "# Tiny Spec: [Title]\n\n## Success Criteria\n\n| ID | Criterion | Verification |\n|----|-----------|-------|\n| SC-1 | ... | ... |\n\n## Coverage Summary\n\n| Criterion | Test | Status |\n|-----------|------|--------|\n| SC-1 | TestFoo | Covered |\n\n## Tests Written\n\n- `path/to/test.ts`: Tests SC-1\n",
  "tests": [
    {
      "file": "path/to/test.go",
      "name": "TestFoo",
      "covers": ["SC-1"],
      "type": "unit"
    }
  ],
  "coverage": {
    "covered": ["SC-1", "SC-2"],
    "manual_verification": [
      {
        "criterion": "SC-3",
        "reason": "Visual check required",
        "steps": ["1. Open page", "2. Verify layout"]
      }
    ]
  },
  "quality_checklist": [
    {"id": "all_criteria_verifiable", "check": "Every SC has executable verification", "passed": true},
    {"id": "no_existence_only_criteria", "check": "SC verifies behavior, not just existence", "passed": true},
    {"id": "p1_stories_independent", "check": "Task can be completed independently", "passed": true},
    {"id": "scope_explicit", "check": "What's in/out of scope is clear", "passed": true},
    {"id": "max_3_clarifications", "check": "No blocking questions remain", "passed": true},
    {"id": "initiative_aligned", "check": "All initiative vision requirements captured", "passed": true},
    {"id": "complexity_within_weight", "check": "Scope fits small weight (≤3 files, ≤2 modules)", "passed": true}
  ],
  "complexity_assessment": {
    "files_to_modify": 2,
    "modules_affected": ["module1"],
    "integration_points": 0,
    "data_model_changes": false,
    "cross_cutting_concerns": false,
    "exceeds_weight": false
  }
}
```

**REQUIRED fields:**
- `tests[].covers` - Array of SC-X IDs this test covers
- `coverage.covered` - All criteria with automated tests
- `coverage.manual_verification` - Criteria that can't be automated (with justification)
- `quality_checklist` - All 7 checks evaluated. Set `passed: false` for any that don't apply or aren't met.

**Validation:** All SC-X from your Success Criteria table must appear in either `covered` or `manual_verification`.

If blocked (genuinely unclear requirements):
```json
{
  "status": "blocked",
  "reason": "[What's unclear and what clarification is needed]"
}
```
</output_format>

<critical_constraints>
The most common failure is success criteria that verify existence ('file exists') instead of behavior ('file does X when given Y'). Every SC must describe observable behavior.

- Maximum 3 criteria for small tasks
- Each must have executable verification
- Focus on user-visible behavior, not implementation details
- If task belongs to initiative: criteria MUST cover all relevant vision requirements

**Complexity Check (small weight):**

Before completing, verify the task fits the small weight classification:

| Threshold | Limit | If Exceeded |
|-----------|-------|-------------|
| Files to modify | ≤ 3 | Task is medium or larger |
| Modules touched | ≤ 2 | Task is medium or larger |
| Integration points | ≤ 1 | Task needs careful wiring |
| Data model changes | None | Task should be medium+ |
| Cross-cutting concerns | None | Task should be medium+ |

**If the task exceeds these limits**, output `"status": "blocked"` with reason explaining why it's under-weighted and recommend re-running with appropriate weight.

**Red flags that always block small tasks:**
- Adding new proto fields or database columns
- Changes spanning both frontend and backend
- New code that needs wiring to multiple existing code paths
</critical_constraints>

<pre_output_verification>
## Pre-Output Verification (MANDATORY)

Before outputting the final JSON, STOP and verify:

1. **Re-read your Success Criteria table**
   - List each SC-X identifier you defined
   - For each SC-X, confirm you wrote at least one test that covers it

2. **Check coverage completeness**
   - Every SC-X must appear in `coverage.covered` OR `coverage.manual_verification`
   - If any SC-X is missing a test, write the missing test NOW before proceeding

3. **Verify test correctness**
   - Each test name accurately describes what it tests
   - Each `covers` array only lists criteria the test actually verifies

4. **Confirm tests will fail**
   - Run `{{TEST_COMMAND}}` mentally - tests should fail or not compile
   - If tests would pass, they're testing existing behavior, not new work

**Only after completing this verification, output the StructuredOutput.**
</pre_output_verification>

<context>
# Tiny Spec + TDD

<task>
ID: {{TASK_ID}}
Title: {{TASK_TITLE}}
Description: {{TASK_DESCRIPTION}}
Category: {{TASK_CATEGORY}}
Weight: {{WEIGHT}}
</task>

<project>
Language: {{LANGUAGE}}
Has Frontend: {{HAS_FRONTEND}}
Has Tests: {{HAS_TESTS}}
Test Command: {{TEST_COMMAND}}
</project>

<worktree_safety>
Path: {{WORKTREE_PATH}}
Branch: {{TASK_BRANCH}}
Target: {{TARGET_BRANCH}}
DO NOT push to {{TARGET_BRANCH}} or checkout other branches.
</worktree_safety>

{{INITIATIVE_CONTEXT}}
{{CONSTITUTION_CONTENT}}
</context>

<instructions>
Create a minimal spec AND write failing tests in one pass. This is for small tasks where a full spec is overkill, but explicit goals and TDD still improve quality.

## Step 0: Initiative Alignment (if initiative context exists above)

**Before defining success criteria, cross-reference the initiative vision:**

1. Read the Initiative Context section above
2. List ALL requirements/features/behaviors mentioned in the vision that relate to this task
3. Ensure your success criteria below capture EVERY relevant initiative requirement

**The task description may be incomplete. The initiative vision is the source of truth.**

Example failure: Task says "add Pause and New Task buttons" but initiative vision says "Actions: Pause, New Task, Panel toggle" → You MUST include Panel toggle in your criteria.

---

## Step 1: Define Success (2-3 criteria max)

| ID | Criterion | Verification |
|----|-----------|--------------|
| SC-1 | [What must be true when done] | [Test command or file check] |

## Step 2: Write Failing Tests

{{#if HAS_FRONTEND}}
<ui_testing>
This project has a frontend. Create EITHER:

**Option A: Playwright E2E tests** (if `playwright.config.ts` exists)
- Write test file in the existing test directory
- Tests should fail until implementation complete
- Follow existing test patterns

**Option B: Manual test plan** (for Playwright MCP)
If no existing Playwright setup, create a manual test plan:

```markdown
## Manual UI Test Plan

### Test: [Description matching SC-1]
1. Navigate to [URL]
2. Perform [action]
3. Verify [expected state]

### Error Cases:
- If [condition], should see [error message]
```

The implement phase will execute this via Playwright MCP tools.
</ui_testing>
{{/if}}

<unit_testing>
Write unit/integration tests that:
- Test the success criteria directly
- Will FAIL until implementation exists
- Follow existing test patterns in the codebase

Look for existing test files to match the pattern:
- Go: `*_test.go` files
- TypeScript: `*.test.ts` or `*.spec.ts` files
- Python: `test_*.py` files
</unit_testing>

## Step 3: Verify Tests Fail

Run: `{{TEST_COMMAND}}`

Tests SHOULD fail or not compile. This is correct - it proves tests are testing real behavior.

If tests pass before implementation, they're testing the wrong thing.
</instructions>
