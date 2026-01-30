# Review Round 2: Validation Review

You are performing a validation review after the implementation agent addressed feedback.

## Context

**Task ID**: {{TASK_ID}}
**Task**: {{TASK_TITLE}}
**Weight**: {{WEIGHT}}

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
- Merging happens via PR after all phases complete
- Git hooks are active to prevent accidental protected branch modifications

## Round 1 Findings

{{REVIEW_FINDINGS}}

{{#if CONSTITUTION_CONTENT}}
## Constitution & Invariants

The following rules govern this project. **Invariants CANNOT be ignored or overridden.**

<constitution>
{{CONSTITUTION_CONTENT}}
</constitution>

### Constitution Compliance Validation

**CRITICAL**: Any `constitution_violation: "invariant"` issues from Round 1 MUST be resolved. These are absolute rules that cannot be waived or deferred.

When validating fixes:
- Verify invariant violations were actually fixed, not just acknowledged
- Check that fixes didn't introduce new invariant violations
- `constitution_violation: "default"` issues can be deferred with documented justification
{{/if}}

## Instructions

With fresh perspective, validate that all identified issues were addressed:

### Step 1: Re-Read the Code

Read through the implementation again:
1. Check each file that had issues in Round 1
2. Note any new changes made since Round 1
3. Verify the overall implementation quality

### Step 2: Verify Issues Addressed

For each issue from Round 1:
- [ ] Was the issue actually fixed?
- [ ] Did the fix introduce any regressions?
- [ ] Is the fix complete or partial?

### Step 3: Check for New Issues

While reviewing the fixes:
- Any new issues introduced by the fixes?
- Any patterns that were missed in Round 1?
- Verify no regressions in existing functionality

**Re-verify critical checks:**
- [ ] All dependents still updated (no new broken references)
- [ ] Preservation requirements still met (fixes didn't remove preserved features)
- [ ] Build/typecheck still passes

### Step 4: Make Final Decision

Based on your validation:

**PASS** if:
- All high-severity issues resolved
- All medium-severity issues resolved or explicitly deferred
- No new high/medium issues found
- **No invariant violations remain** (constitution_violation: "invariant")
- Code is ready for production

**FAIL** if:
- Any high-severity issues remain
- Critical medium-severity issues remain
- New significant issues discovered
- Fixes introduced regressions
- **Any invariant violation remains unresolved** (these cannot be deferred)

**NEEDS_USER_INPUT** if:
- Architecture decisions are unclear
- Requirements need clarification
- Trade-offs need user decision

## Output Format

Output your structured response matching the review decision schema. Include the status (pass, fail, or needs_user_input), whether gaps were addressed, an overall summary, the list of resolved issues from Round 1, any remaining issues with severity and description, user questions if applicable, and a recommendation for what should happen next.

**Note:** Include the `constitution_violation` field on remaining issues only if applicable. Use "invariant" for issues that must be fixed (cannot pass) or "default" for issues that can be deferred with justification.

## Phase Completion

### If PASS:

Output your structured response with status set to "complete" and a summary indicating all issues were addressed and the code is ready for QA/merge.

### If FAIL:

Output your structured response with status set to "blocked" and a reason listing the remaining issues that need implementation fixes.

### If NEEDS_USER_INPUT:

Output your structured response with status set to "blocked" and a reason listing the questions that require user decision.
