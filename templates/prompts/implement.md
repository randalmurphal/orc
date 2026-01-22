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

<design>
{{DESIGN_CONTENT}}
</design>

<task_breakdown>
{{TASKS_CONTENT}}
</task_breakdown>

{{RETRY_CONTEXT}}
</context>

<tdd_tests>
## Tests to Make Pass

{{TDD_TESTS_CONTENT}}

Your implementation MUST make these tests pass.

**Before claiming completion:**
1. Run all tests: `{{TEST_COMMAND}}`
2. All tests MUST pass
3. If any fail, fix implementation (NOT the test)
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

## Step 2: Impact Analysis (REQUIRED)

**Before writing any code**, analyze what will be affected.

### 2a. Find All Callers/Dependents

For each file/function you plan to modify:

```bash
# Find who calls this function (Go)
grep -r "FunctionName" --include="*.go" .

# Find references (TypeScript)
grep -r "functionName\|ClassName" --include="*.ts" --include="*.tsx" .
```

### 2b. Create Dependency Map

| Code Being Changed | Who Uses It | Also Needs Update? |
|--------------------|-------------|-------------------|
| [function/file] | [list of callers] | Yes/No - [reason] |

### 2c. Verify Preservation Requirements

Cross-check against the spec's Preservation Requirements table:
- [ ] All preserved behaviors identified
- [ ] Tests exist for each (or will be added)
- [ ] No planned changes conflict with preservation requirements

**Do NOT proceed to Step 3 until you've mapped dependencies.**

## Step 3: Plan Changes

Based on your impact analysis and task breakdown (if available), identify:
- New files to create
- Existing files to modify (including dependents from Step 2)
- Tests to write/update

## Step 4: Implement

For each change:
1. **Fully implement** all requirements - no partial solutions or TODOs
2. **Update all dependents** identified in your impact analysis
3. Follow existing code patterns
4. Add appropriate error handling
5. Include comments for non-obvious logic

**Stay within scope** but **be thorough within that scope**.

## Step 5: Handle Edge Cases

Check for:
- Invalid input
- Empty/null values
- Boundary conditions

## Step 6: Ensure Error Handling

Every error path should:
- Have a clear error message
- Include what went wrong
- Include what user can do

**No silent failures.**

## Step 7: Self-Review

Before completing:
- [ ] All success criteria addressed
- [ ] All TDD tests pass
- [ ] All dependents from impact analysis updated
- [ ] Preservation requirements verified (nothing accidentally removed)
- [ ] Scope boundaries respected
- [ ] Error handling complete
- [ ] Code follows project patterns
- [ ] No TODO comments left behind

## Completion Criteria

This phase is complete when:
1. All spec success criteria are implemented
2. All TDD tests pass
3. Code compiles/runs without errors

## Self-Correction

| Situation | Action |
|-----------|--------|
| Spec unclear on detail | Make reasonable choice, document as amendment |
| Pattern doesn't fit | Follow existing patterns, note deviation |
| Scope creep temptation | **Stop. Stick to spec.** |
| Tests failing | Fix implementation, not tests |
| Spec assumption wrong | Document as amendment, continue with correct approach |

## Spec Amendments

If you discover the spec doesn't match reality, **document amendments**:

```markdown
## Amendments

### AMEND-001 (implement phase)
**Original:** [What spec said]
**Actual:** [What you're doing instead]
**Reason:** [Why the change is necessary]
```
</instructions>

<verification>
## Step 8: Verify All Criteria (REQUIRED)

**Before claiming completion, you MUST verify each success criterion.**

For each criterion in the spec's Success Criteria table:
1. Run the verification method
2. Record the result (PASS/FAIL)
3. If FAIL: fix the issue and re-verify
4. Only proceed when ALL criteria pass

**Do NOT mark phase complete until all verifications pass.**

## Step 9: Run TDD Tests

```bash
{{TEST_COMMAND}}
```

All TDD tests written in the tdd_write phase MUST pass. If any fail:
1. Identify which test fails
2. Fix the implementation (not the test)
3. Re-run until all pass

## Step 10: Quick Lint Check (Recommended)

Before committing:

```bash
# For Go projects
go vet ./...

# For Node/TypeScript projects
npm run typecheck && npm run lint

# For Python projects
ruff check .
```

Common issues to watch for:
- Unchecked error returns
- Unused imports/variables
- Type errors
- Formatting issues
</verification>

<output_format>
When all success criteria and TDD tests pass, output JSON to signal completion:

```json
{
  "status": "complete",
  "summary": "Implemented [feature]: [files changed count] files, all [criteria count] criteria verified, all tests pass"
}
```

**If any verification or test fails**, fix the implementation and re-verify. Only output completion when all pass.

If blocked (cannot proceed):
```json
{
  "status": "blocked",
  "reason": "[why blocked and what's needed]"
}
```
</output_format>
