# Review Round 2: Validation Review

<output_format>
Output your structured response matching the review decision schema:

- `status`: "pass" | "fail" | "needs_user_input"
- `gaps_addressed`: boolean — were round 1 issues addressed?
- `summary`: Overall assessment
- `resolved_issues`: List of round 1 issues that were fixed
- `remaining_issues`: List with `severity`, `description`, and optionally `constitution_violation`
- `questions`: User questions (if status is "needs_user_input")
- `recommendation`: What should happen next

## Phase Completion

**If PASS:** `{"status": "complete", "summary": "All issues addressed, code ready for QA/merge"}`
**If FAIL:** `{"status": "blocked", "reason": "[remaining issues needing implementation fixes]"}`
**If NEEDS_USER_INPUT:** `{"status": "blocked", "reason": "[questions requiring user decision]"}`
</output_format>

<critical_constraints>
The most common failure is declaring round 1 issues "fixed" without verifying the fix actually works and didn't introduce regressions.

## PASS Criteria

- All high-severity issues resolved
- All medium-severity issues resolved or explicitly deferred
- No new high/medium issues found
- No invariant violations remain (`constitution_violation: "invariant"`)
- Code is ready for production

## FAIL Criteria

- Any high-severity issues remain
- Critical medium-severity issues remain
- New significant issues discovered
- Fixes introduced regressions
- Any invariant violation remains unresolved (these cannot be deferred)

## NEEDS_USER_INPUT Criteria

- Architecture decisions are unclear
- Requirements need clarification
- Trade-offs need user decision
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

**Git State**: Previous phases have committed their work. Worktree is clean. Use `git diff main..HEAD` to see changes.

DO NOT push to {{TARGET_BRANCH}} or checkout other branches. Stay on {{TASK_BRANCH}}.
</worktree_safety>
</context>

<round1_findings>
{{REVIEW_FINDINGS}}
</round1_findings>

{{#if CONSTITUTION_CONTENT}}
<constitution>
{{CONSTITUTION_CONTENT}}
</constitution>

Constitution validation for round 2:
- Any `constitution_violation: "invariant"` issues from round 1 MUST be resolved — not just acknowledged. These are absolute rules that cannot be waived or deferred.
- Verify fixes didn't introduce new invariant violations.
- `constitution_violation: "default"` issues can be deferred with documented justification.
{{/if}}

<instructions>
## Step 1: Re-Read the Code

1. Check each file that had issues in round 1
2. Note any new changes made since round 1
3. Verify overall implementation quality

## Step 2: Verify Round 1 Issues Addressed

For each issue from round 1:
- [ ] Was the issue actually fixed?
- [ ] Did the fix introduce any regressions?
- [ ] Is the fix complete or partial?

### Rationalization Anti-Patterns

The implement phase rationalizes incomplete fixes. Reject these:

- **"Tests pass so it works"** — Tests may cover the fix in isolation but not through production paths. Verify end-to-end.
- **"Optional props with empty fallbacks"** — If SC says behavior works NOW, empty fallbacks = no-op. Props must be wired, not optional.
- **"Documented as future improvement"** — SC requirements are not "future." If the spec says it works, it must work NOW.
- **"Good progress, just needs wiring later"** — Unwired code is dead code. Task must be complete per spec.

### Step 2b: Integration Re-Verification

For any new files created during the fix, verify they have production callers. Fixes that create new dead code are high-severity.

```bash
# For each new file introduced by the fix:
grep -rn "NewFunction\|new_module" path/to/production/code/  # Must find a caller
```

If the fix refactored code into a new helper/file but nothing imports it, this is a HIGH-SEVERITY finding — the fix itself introduced dead code.

### Step 2c: Spec Compliance Re-Check

Verify each success criterion (SC-X) from the original spec is still met after fixes. Fixes that break previously-passing criteria are regressions.

For each SC:
- [ ] Was it passing before the fix? If yes, is it still passing?
- [ ] Did the fix change code paths that satisfy other SCs?
- [ ] Do existing tests for unrelated SCs still pass?

A fix that resolves one issue but breaks a previously-passing SC is a HIGH-SEVERITY regression — not progress.

## Step 3: Check for New Issues

While reviewing the fixes:
- Any new issues introduced by the fixes?
- New over-engineering introduced during fix?
- Patterns missed in round 1?
- Regressions in existing functionality?

**Critical re-verification:**
- [ ] All dependents still updated (no new broken references)
- [ ] Preservation requirements still met (fixes didn't remove preserved features)
- [ ] Build/typecheck still passes

## Step 4: Make Final Decision

Apply the PASS/FAIL/NEEDS_USER_INPUT criteria defined above. Include `constitution_violation` on remaining issues where applicable ("invariant" = must fix, "default" = can defer with justification).
</instructions>
