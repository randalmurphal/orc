# E2E QA Fix Session

You are fixing issues identified during browser-based E2E QA testing.

## Context

**Task**: {{TASK_ID}} - {{TASK_TITLE}}
**Worktree**: {{WORKTREE_PATH}}
**Fix Iteration**: {{QA_ITERATION}} of {{QA_MAX_ITERATIONS}}

## Worktree Safety

You are working in an **isolated git worktree**.

| Property | Value |
|----------|-------|
| Worktree Path | `{{WORKTREE_PATH}}` |
| Task Branch | `{{TASK_BRANCH}}` |
| Target Branch | `{{TARGET_BRANCH}}` |

**CRITICAL SAFETY RULES:**
- All commits go to branch `{{TASK_BRANCH}}`
- **DO NOT** push to `{{TARGET_BRANCH}}` or any protected branch
- **DO NOT** checkout other branches - stay on `{{TASK_BRANCH}}`

## QA Output Directory

QA artifacts (reports, screenshots, findings) are located in:
`{{QA_OUTPUT_DIR}}`

Screenshots referenced in findings below can be found in this directory.

## QA Findings to Fix

The following issues were identified during E2E testing. You must investigate the root cause and apply fixes.

{{QA_FINDINGS}}

## Fix Philosophy

**Critical Rules:**
- Fix ONLY the issues listed above
- Make **minimal changes** - don't refactor unrelated code
- Don't add features - just fix bugs
- Preserve existing behavior except where broken
- One fix per finding - keep changes isolated

## Fix Instructions

### Step 1: Investigate Root Causes

For each finding, before writing any code:

1. **Understand the reproduction steps**
   - Read the steps carefully
   - Understand what the user was trying to do

2. **Trace the code path**
   - Identify the UI component involved
   - Follow the data flow: event → handler → state → render
   - Find where the behavior diverges from expected

3. **Identify the root cause**
   - Pinpoint the exact code location
   - Understand WHY the bug occurs (not just WHERE)
   - Check for related issues in the same area

### Step 2: Apply Minimal Fixes

For each finding:

1. **Make the smallest change that fixes the issue**
   - Avoid refactoring while fixing
   - Don't "improve" code that works
   - Keep the diff as small as possible

2. **Verify the fix locally**
   - Run related tests
   - Manually test the specific scenario if possible
   - Check for obvious regressions

3. **Consider edge cases**
   - Will this fix break other scenarios?
   - Are there similar issues elsewhere?

### Step 3: Run Quality Checks

After applying fixes:

```bash
# Run tests
{{TEST_COMMAND}}

# Run linter
make lint  # or your lint command

# Build check
make build  # or your build command
```

All checks must pass before marking complete.

### Step 4: Commit Changes

Commit fixes with clear message:

```
[orc] {{TASK_ID}}: qa-fix - [brief description of fixes]

Fixes:
- QA-001: [what was fixed]
- QA-002: [what was fixed]
```

### Step 5: Handle Unfixable Issues

If an issue cannot be fixed in this iteration:

- **Out of scope**: Requires design decision or architectural change
- **Needs investigation**: Root cause unclear, needs more research
- **External dependency**: Waiting on upstream fix

Document these as deferred with clear reasoning.

## Output Format

Output JSON matching QAE2EFixResultSchema:

```json
{
  "status": "complete",
  "summary": "Fixed 2 of 3 issues, deferred 1 for design decision",
  "fixes_applied": [
    {
      "finding_id": "QA-001",
      "status": "fixed",
      "files_modified": ["src/components/SignupForm.tsx"],
      "change_description": "Added email validation before form submission"
    },
    {
      "finding_id": "QA-002",
      "status": "fixed",
      "files_modified": ["src/styles/mobile.css", "src/components/Header.tsx"],
      "change_description": "Fixed header overflow on mobile viewport"
    }
  ],
  "issues_deferred": [
    {
      "finding_id": "QA-003",
      "reason": "Requires backend API change to support new error codes - out of scope for this task"
    }
  ]
}
```

## Fix Status Values

| Status | Meaning |
|--------|---------|
| `fixed` | Issue is resolved, verified locally |
| `partial` | Issue is improved but not fully resolved |
| `unable` | Cannot fix - will be deferred |

## Decision Criteria

**COMPLETE:**
- All fixable issues have been addressed
- Tests pass
- Build succeeds
- Changes are committed
- Unfixable issues are documented with clear reasoning

**BLOCKED:**
- Cannot access required files
- Tests fail and cannot be fixed
- Build fails and cannot be fixed
- Circular dependency prevents fix

## After Completion

The QA test phase will run again to verify your fixes.

- Fixed issues should no longer reproduce
- If an issue is marked as fixed but still fails verification, it will appear in the next iteration
- The loop continues until all issues are fixed or max iterations reached

## Remember

- Minimal changes - don't over-engineer
- One thing at a time - fix each issue individually
- Test your fixes - don't assume they work
- Document deferrals clearly - the reasoning matters
- Commit frequently - small commits are easier to review
