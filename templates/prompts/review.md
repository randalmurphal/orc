# Review Phase

Fast validation before test phase. Three possible outcomes.

## Context

**Task ID**: {{TASK_ID}}
**Task**: {{TASK_TITLE}}
**Weight**: {{WEIGHT}}

{{INITIATIVE_CONTEXT}}

## Specification

{{SPEC_CONTENT}}

{{RETRY_CONTEXT}}

## What to Check

### 1. Obvious Bugs
- Null pointer / undefined access
- Logic errors (wrong condition, off-by-one)
- Infinite loops, unbounded recursion
- Resource leaks (unclosed files, connections)

### 2. Security Issues
- SQL/command injection
- Hardcoded secrets
- Missing input validation
- Auth bypass

### 3. Spec Compliance
- Are success criteria addressed?
- Missing functionality?

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
   npm run typecheck && npm run lint
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

After fixing (or if nothing to fix):
```bash
git add -A && git commit -m "[orc] {{TASK_ID}}: review - passed"
```

Then:
```
### Review - PASSED

[List any fixes made, or "No issues found"]

<phase_complete>true</phase_complete>
```

---

### Outcome 2: Major Implementation Issues

**Use when:** Significant problems that need re-implementation, but the overall approach is correct.

Examples:
- Missing error handling throughout
- Component doesn't integrate correctly
- Business logic is wrong in multiple places
- Tests are missing or inadequate

Do NOT fix these yourself. Block so implement phase can address them properly:

```
### Review - BLOCKED (Implementation Issues)

The implementation approach is correct, but has significant issues:

**Issues requiring re-implementation:**
1. [File:line] - [Issue description] - [What needs to change]
2. [File:line] - [Issue description] - [What needs to change]

**What implement phase should do:**
- [Specific action 1]
- [Specific action 2]

<phase_blocked>
reason: Major implementation issues requiring fixes in implement phase
needs: [summary of what must be fixed]
</phase_blocked>
```

---

### Outcome 3: Wrong Approach Entirely

**Use when:** The fundamental approach is wrong. Re-implementing won't help - needs rethinking.

Examples:
- Misunderstood the requirements
- Wrong architecture for the problem
- Built the wrong thing entirely
- Approach won't scale/work for the use case

Provide detailed context about WHY and WHAT should change:

```
### Review - BLOCKED (Wrong Approach)

The current implementation takes the wrong approach and needs rethinking.

**Current approach:**
[Describe what was built]

**Why it's wrong:**
[Detailed explanation of the fundamental problem]

**Correct approach:**
[Describe what should be built instead, and why]

**Key changes needed:**
1. [Major change 1]
2. [Major change 2]

<phase_blocked>
reason: Fundamental approach is incorrect - [one line summary]
needs: Re-implementation with correct approach as described above
</phase_blocked>
```

---

## Decision Guide

```
Found issues?
├─ No → Outcome 1 (pass)
├─ Yes, can fix in < 5 minutes? → Outcome 1 (fix and pass)
├─ Yes, approach is correct but implementation is wrong → Outcome 2
└─ Yes, approach itself is wrong → Outcome 3
```

**Bias toward Outcome 1.** Most issues can be fixed in-place. Only block when genuinely necessary.
