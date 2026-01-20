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
- Code is ready for production

**FAIL** if:
- Any high-severity issues remain
- Critical medium-severity issues remain
- New significant issues discovered
- Fixes introduced regressions

**NEEDS_USER_INPUT** if:
- Architecture decisions are unclear
- Requirements need clarification
- Trade-offs need user decision

## Output Format

```xml
<review_decision>
  <status>pass|fail|needs_user_input</status>
  <gaps_addressed>true|false</gaps_addressed>
  <summary>Overall assessment of the implementation</summary>
  <issues_resolved>
    <issue>Description of resolved issue from Round 1</issue>
  </issues_resolved>
  <remaining_issues>
    <issue severity="high|medium|low">Description of remaining issue</issue>
  </remaining_issues>
  <user_questions>
    <question>Question requiring user decision (if needs_user_input)</question>
  </user_questions>
  <recommendation>What should happen next</recommendation>
</review_decision>
```

## Phase Completion

### If PASS:

Output ONLY this JSON:
```json
{"status": "complete", "summary": "Review round 2 PASS: All issues addressed, ready for QA/merge"}
```

### If FAIL:

Output ONLY this JSON:
```json
{"status": "blocked", "reason": "Review FAIL: [list remaining issues]. Implementation fixes needed."}
```

### If NEEDS_USER_INPUT:

Output ONLY this JSON:
```json
{"status": "blocked", "reason": "Review NEEDS_USER_INPUT: [list questions requiring user decision]"}
```
