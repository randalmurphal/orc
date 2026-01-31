# Review Phase

Fast validation before test phase. Three possible outcomes.

## Context

**Task ID**: {{TASK_ID}}
**Task**: {{TASK_TITLE}}
**Weight**: {{WEIGHT}}

{{INITIATIVE_CONTEXT}}

## Worktree Safety

You are working in an **isolated git worktree**. This ensures your changes don't affect other work.

| Property | Value |
|----------|-------|
| Worktree Path | `{{WORKTREE_PATH}}` |
| Task Branch | `{{TASK_BRANCH}}` |
| Target Branch | `{{TARGET_BRANCH}}` |

**CRITICAL SAFETY RULES:**
- You are on branch `{{TASK_BRANCH}}` - all commits go here
- **DO NOT** push to `{{TARGET_BRANCH}}` or any protected branch (main, master, develop, release)
- **DO NOT** checkout `{{TARGET_BRANCH}}` - stay on your task branch
- All merging is handled via PR after all phases complete
- Git hooks are active to prevent accidental protected branch modifications

## Specification

{{SPEC_CONTENT}}

{{RETRY_CONTEXT}}

## What to Check

### 1. Completeness Check (CRITICAL)

**Did the implementation update everything it needed to?**

Review the implementation artifact's "Impact Analysis Results" section:
- [ ] All identified dependents were updated
- [ ] No callers/importers were missed
- [ ] Changes propagated to all necessary files

```bash
# Verify no broken references
go build ./... 2>&1 | grep -i "undefined\|cannot find"
# Or for TypeScript
bun run typecheck 2>&1 | grep -i "cannot find\|not found"
```

### 2. Preservation Check (CRITICAL)

**Was anything removed that shouldn't have been?**

Cross-reference spec's "Preservation Requirements" table:
- [ ] All preserved behaviors still work
- [ ] No features accidentally removed
- [ ] Run preservation verification commands from spec

```bash
# Check diff for removed functionality
git diff origin/{{TARGET_BRANCH}}...HEAD --stat
git diff origin/{{TARGET_BRANCH}}...HEAD | grep "^-" | grep -v "^---"
```

**Red flags:**
- Large deletions without corresponding additions
- Removed test cases
- Removed exports/public APIs

### 3. Obvious Bugs
- Null pointer / undefined access
- Logic errors (wrong condition, off-by-one)
- Infinite loops, unbounded recursion
- Resource leaks (unclosed files, connections)

### 4. Security Issues
- SQL/command injection
- Hardcoded secrets
- Missing input validation
- Auth bypass

### 5. Spec Compliance
- Are success criteria addressed?
- Missing functionality?

### 6. Integration Completeness

**Are new components actually wired into the system?**

- [ ] All new functions are called from at least one production code path
- [ ] No defined-but-never-called functions exist (dead code)
- [ ] New interfaces have implementations wired into the system
- [ ] If the task adds hooks/callbacks/triggers, they are registered

**For bug fixes:** The fix may be correct where applied but incomplete across the codebase.

- [ ] Grep for the function/pattern being fixed — does the same bug exist in other code paths?
- [ ] If the spec lists a "Pattern Prevalence" table, verify ALL listed paths were addressed
- [ ] If you find unlisted paths with the same bug, this is a **high-severity** finding

Dead code, unwired integration, or incomplete bug fixes are **high-severity** findings.

### What NOT to Review
- Style preferences, naming suggestions
- "Nice to have" improvements
- Performance (unless critical)
- Architecture opinions

## Process

1. Run linting:
   ```bash
   # Go
   go vet ./... && golangci-lint run ./...
   # Node/TypeScript
   bun run typecheck && bun run lint
   ```

2. Check changed files:
   ```bash
   git diff --name-only origin/{{TARGET_BRANCH}}...HEAD
   ```

---

## Three Outcomes

### Outcome 1: No Issues / Small Fixes

**Use when:** No issues found, OR issues are small and you can fix them directly.

Fix small bugs yourself using the Edit tool. Examples of "small":
- Missing null check
- Typo in error message
- Forgotten import
- Simple logic fix

**Important:** Only commit if you actually made code changes. Phase tracking is handled by the executor, not commit history - do NOT create empty commits.

If you made fixes, commit them:
```bash
git add -A && git commit -m "[orc] {{TASK_ID}}: review fixes"
```

Then output your structured response with status set to "complete" and a summary describing what fixes you made or that no issues were found.

---

### Outcome 2: Major Implementation Issues

**Use when:** Significant problems that need re-implementation, but the overall approach is correct.

Examples:
- Missing error handling throughout
- Component doesn't integrate correctly
- Business logic is wrong in multiple places
- Tests are missing or inadequate

Do NOT fix these yourself. Block so implement phase can address them properly.

Output your structured response with status set to "blocked" and a reason describing the major issues found, including file and line references, and what the implement phase must fix.

---

### Outcome 3: Wrong Approach Entirely

**Use when:** The fundamental approach is wrong. Re-implementing won't help - needs rethinking.

Examples:
- Misunderstood the requirements
- Wrong architecture for the problem
- Built the wrong thing entirely
- Approach won't scale/work for the use case

Provide detailed context about WHY and WHAT should change.

Output your structured response with status set to "blocked" and a reason explaining why the current approach is incorrect and what the correct approach should be.

---

## Decision Guide

```
Found issues?
├─ No → Outcome 1 (pass)
├─ Yes, can fix in < 5 minutes? → Outcome 1 (fix and pass)
├─ Yes, any high-severity (dead code, missing integration, bugs, security)?
│   → Outcome 2 or 3 (block)
├─ Yes, medium-only → Outcome 1 (pass, document issues in summary)
└─ Yes, approach itself is wrong → Outcome 3
```

**Base your decision purely on the severity of findings.** Any high-severity finding (dead code, missing integration, bugs, security issues) must block. Medium-only findings can pass with issues documented in the summary.
