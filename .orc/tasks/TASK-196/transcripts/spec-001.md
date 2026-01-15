# spec - Iteration 1

## Prompt

Create a specification for this task:

**Task**: Fix: Auto-merge fails when main branch is checked out locally
**Category**: {{TASK_CATEGORY}}
**Description**: ## Problem
When a task completes and tries to auto-merge, it fails if main is checked out in the main repo:

```
gh pr merge: exit status 1: failed to run git: fatal: 'main' is already used by worktree at '/home/randy/repos/orc'
```

This is the common case - users typically have main checked out when running orc.

## Root Cause
The `gh pr merge` command tries to checkout main locally to fast-forward it, but can't because it's already in use.

## Solutions to Investigate
1. Use `gh pr merge --merge` with `--admin` flag to bypass local checkout
2. Temporarily switch main repo to a detached HEAD or temp branch
3. Use GitHub API directly instead of gh CLI
4. Accept the limitation and just skip auto-merge when main is checked out (document it)

## Success Criteria
`orc run TASK-XXX` with auto-merge enabled works when main is checked out locally

{{INITIATIVE_CONTEXT}}

## Instructions

Create a clear, actionable specification that defines exactly what needs to be done
and how to verify it's complete.

### 1. Problem Statement
Summarize what needs to be solved in 1-2 sentences.

### 2. Success Criteria (REQUIRED)
Define specific, testable criteria as checkboxes:
- Each criterion must be verifiable (file exists, test passes, API returns X)
- No vague language ("works well", "is fast")
- Include both functional and quality criteria

### 3. Testing Requirements (REQUIRED)
Specify what tests must pass:
- [ ] Unit test: [specific test description]
- [ ] Integration test: [if applicable]
- [ ] E2E test: [if UI changes]

### 4. Scope
Define boundaries to prevent scope creep:
- **In Scope**: What will be implemented
- **Out of Scope**: What will NOT be implemented

### 5. Technical Approach
Brief plan for implementation:
- Files to modify
- Key changes in each file

### 6. Category-Specific Analysis

**If this is a BUG (category=bug):**
- Reproduction Steps: Exact steps to trigger the bug
- Current Behavior: What happens now (the bug)
- Expected Behavior: What should happen
- Root Cause: Where the bug originates (if known)
- Verification: How to confirm the fix works

**If this is a FEATURE (category=feature):**
- User Story: As a [user], I want [feature] so that [benefit]
- Acceptance Criteria: Specific conditions for feature acceptance

**If this is a REFACTOR (category=refactor):**
- Before Pattern: Current code/architecture
- After Pattern: Target code/architecture
- Risk Assessment: What could break

## Output Format

Wrap your spec in artifact tags:

<artifact>
# Specification: Fix: Auto-merge fails when main branch is checked out locally

## Problem Statement
[1-2 sentences]

## Success Criteria
- [ ] [Criterion 1]
- [ ] [Criterion 2]

## Testing Requirements
- [ ] [Test 1]
- [ ] [Test 2]

## Scope
### In Scope
- [Item]
### Out of Scope
- [Item]

## Technical Approach
[Brief implementation plan]

### Files to Modify
- [file]: [change]

## [Category-Specific Section]
[Include appropriate section based on category]
</artifact>

After completing the spec, commit:
```bash
git add -A
git commit -m "[orc] TASK-196: spec - completed"
```

Then output:
```
**Commit**: [SHA]
<phase_complete>true</phase_complete>
```

If blocked (requirements unclear):
```
<phase_blocked>
reason: [what's unclear]
needs: [what clarification is needed]
</phase_blocked>
```


## Response

**Commit**: 095cff8
<phase_complete>true</phase_complete>

---
Tokens: 501732 input, 2583 output, 51335 cache_creation, 450386 cache_read
Complete: true
Blocked: false
