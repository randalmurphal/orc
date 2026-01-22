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
Create a minimal spec AND write failing tests in one pass. This is for trivial/small tasks where a full spec is overkill, but explicit goals and tests still improve quality.

## Step 1: Define Success (2-3 criteria max)

| ID | Criterion | Verification |
|----|-----------|--------------|
| SC-1 | [What must be true when done] | [Test command or file check] |

**Rules:**
- Maximum 3 criteria for trivial/small tasks
- Each must have executable verification
- Focus on user-visible behavior, not implementation details

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

{{#if NOT_HAS_FRONTEND}}
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
{{/if}}

## Step 3: Verify Tests Fail

Run: `{{TEST_COMMAND}}`

Tests SHOULD fail or not compile. This is correct - it proves tests are testing real behavior.

If tests pass before implementation, they're testing the wrong thing.
</instructions>

<output_format>
Output a JSON object with the spec, test information, and quality checklist:

```json
{
  "status": "complete",
  "summary": "Defined N criteria, wrote M failing tests",
  "artifact": "# Tiny Spec: [Title]\n\n## Success Criteria\n\n| ID | Criterion | Verification |\n|----|-----------|-------|\n| SC-1 | ... | ... |\n\n## Tests Written\n\n- `path/to/test.ts`: Tests SC-1\n",
  "tests_written": ["path/to/test1.ts", "path/to/test2.go"],
  "quality_checklist": [
    {"id": "all_criteria_verifiable", "check": "Every SC has executable verification", "passed": true},
    {"id": "no_technical_metrics", "check": "SC describes behavior, not internals", "passed": true},
    {"id": "p1_stories_independent", "check": "Task can be completed independently", "passed": true},
    {"id": "scope_explicit", "check": "What's in/out of scope is clear", "passed": true},
    {"id": "max_3_clarifications", "check": "No blocking questions remain", "passed": true}
  ]
}
```

**REQUIRED:** The `quality_checklist` array must be included with all 5 checks evaluated. Set `passed: false` for any that don't apply or aren't met.

If blocked (genuinely unclear requirements):
```json
{
  "status": "blocked",
  "reason": "[What's unclear and what clarification is needed]"
}
```
</output_format>
