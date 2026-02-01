<output_format>
Output JSON matching the QA result schema:

```json
{
  "status": "pass|fail|needs_attention",
  "summary": "Overall QA assessment",
  "tests_written": [
    {"file": "path/to/test.go", "description": "What it tests", "type": "e2e|integration|unit"}
  ],
  "tests_run": {"total": 10, "passed": 10, "failed": 0, "skipped": 0},
  "coverage": {"percentage": 85, "uncovered_areas": "Description if any"},
  "documentation": [
    {"file": "path/to/doc.md", "type": "feature|api|testing"}
  ],
  "issues": [
    {"severity": "high|medium|low", "description": "Issue found", "reproduction": "Steps"}
  ],
  "recommendation": "What should happen next"
}
```

## Decision Criteria

**PASS** if:
- All e2e tests pass
- No high-severity issues found
- Existing tests still pass
- Documentation is complete

**FAIL** if:
- E2e tests fail
- High-severity issues discovered
- Regressions in existing functionality
- Critical documentation missing

**NEEDS_ATTENTION** if:
- Minor issues need follow-up
- Documentation incomplete but feature works
- Edge cases need discussion

## Phase Completion

After outputting the QA result JSON above, output one of:

If PASS:
```json
{"status": "complete", "summary": "QA PASS: All tests pass, documentation complete. Ready for release."}
```

If FAIL:
```json
{"status": "blocked", "reason": "QA FAIL: [list issues requiring fixes]"}
```

If NEEDS_ATTENTION:
```json
{"status": "complete", "summary": "QA NEEDS_ATTENTION: Minor items for follow-up: [list items]"}
```
</output_format>

<critical_constraints>
The most common failure is running happy path only and missing edge cases and error paths from the specification. You must test error paths and boundary conditions, not just the success flow.

Do not skip the regression check. New features that break existing functionality are not ready for release.
</critical_constraints>

<context>
<task>
ID: {{TASK_ID}}
Title: {{TASK_TITLE}}
Weight: {{WEIGHT}}
</task>

<worktree_safety>
Path: {{WORKTREE_PATH}}
Branch: {{TASK_BRANCH}}
Target: {{TARGET_BRANCH}}
DO NOT push to {{TARGET_BRANCH}} or any protected branch.
DO NOT checkout {{TARGET_BRANCH}} — stay on your task branch.
</worktree_safety>
</context>

<instructions>
Validate the implementation is production-ready through end-to-end testing, regression checks, and documentation.

## Step 1: Review Implementation

1. Read the key files modified for this task
2. Identify the main functionality added
3. Note edge cases and error paths mentioned in the spec

## Step 2: End-to-End Testing

Write and run end-to-end tests covering:

1. **Happy Path** — Main user flow produces expected outputs, integrations function correctly
2. **Edge Cases** — Boundary conditions, unexpected inputs, concurrent access where applicable
3. **Error Handling** — Error paths from the spec are exercised, failures produce correct behavior

## Step 3: Regression Check

1. Run existing tests: `go test ./...` or appropriate test command
2. Verify existing functionality still works
3. Note any failures or warnings

## Step 4: Documentation

Create or update as needed:

1. **Feature Documentation** — What does this feature do, how do users access it, expected behaviors
2. **Testing Scripts** — Manual testing instructions if needed, test data requirements
3. **API Documentation** (if applicable) — New endpoints, request/response examples
</instructions>
