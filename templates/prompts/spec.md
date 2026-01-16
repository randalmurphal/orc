# Specification Phase

You are writing a detailed specification for a task.

## Context

**Task ID**: {{TASK_ID}}
**Task**: {{TASK_TITLE}}
**Weight**: {{WEIGHT}}
**Category**: {{TASK_CATEGORY}}
**Description**: {{TASK_DESCRIPTION}}

{{INITIATIVE_CONTEXT}}

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
- **DO NOT** write `spec.md` files to the filesystem - specs are saved to the database from `<artifact>` tags
- Merging happens via PR after all phases complete
- Git hooks are active to prevent accidental protected branch modifications

## Research Findings (if available)

{{RESEARCH_CONTENT}}

## Instructions

Create a clear, actionable specification that defines exactly what needs to be done
and how to verify it's complete. The spec drives all subsequent phases.

### Step 1: Analyze Requirements

Break down the task into:
- What needs to be built/fixed/changed
- What already exists (relevant code, patterns)
- What constraints apply (compatibility, performance, security)

### Step 1.5: Project Context (REQUIRED)

Before implementing, understand how this change fits with the existing codebase:

#### Patterns to Follow
Identify existing patterns this task must follow. Read CLAUDE.md and look at similar code.

| Pattern | Example Location | How to Apply |
|---------|------------------|--------------|
| [Error handling] | [file:line] | [How to use it here] |
| [Naming conventions] | [file:line] | [How to apply] |

#### Affected Code
What existing code will be impacted by this change?

| File | Current Behavior | After This Change |
|------|------------------|-------------------|
| [file] | [what it does now] | [what it will do] |

#### Breaking Changes
- [ ] This change is backward compatible
- [ ] This change breaks: [list what breaks and migration path]

### Step 2: Define Success Criteria (REQUIRED)

Create specific, testable criteria with **explicit verification methods**:

| ID | Criterion | Verification Method | Expected Result | Error Path |
|----|-----------|---------------------|-----------------|------------|
| SC-1 | [What must be true] | [How to verify] | [What success looks like] | [What error behavior to test] |

**Verification method types:**
- **Test**: `go test ./... -run TestName` or `npm test -- file.spec.ts`
- **Command**: `curl -X GET /api/endpoint` or `ls -la path/to/file`
- **File check**: Verify file exists, contains pattern, has correct structure
- **E2E**: Browser action + assertion (click X, verify Y appears)
- **Build**: `go build ./...` or `npm run build` succeeds

**Rules:**
- Each criterion MUST have a verification method (no vague criteria)
- Verification methods must be executable (commands, not descriptions)
- Expected results must be concrete (exit code 0, output contains X, file exists)
- Error paths MUST be specified - what happens when things fail?

### Step 3: Define Testing Requirements (REQUIRED)

Specify what tests must pass to consider the work complete:
- Unit tests: specific functions/modules to test
- Integration tests: component interactions to verify
- E2E tests: user flows to validate (if UI changes)

### Step 4: Define Scope

#### In Scope
List exactly what will be implemented.

#### Out of Scope
List what will NOT be implemented (prevents scope creep).

### Step 5: Technical Approach

Describe:
- Key files to create/modify
- Patterns to follow
- Dependencies needed
- Data structures/schemas

### Step 6: Category-Specific Analysis

**For BUG tasks (category = bug):**

#### Bug Analysis

##### Reproduction Steps
1. [Exact step to reproduce]
2. [Exact step to reproduce]
3. [Observe: describe the bug behavior]

##### Current Behavior
Describe what happens now (the bug).

##### Expected Behavior
Describe what should happen instead.

##### Root Cause (if known)
Where the bug originates in the code.

##### Verification Method
How to confirm the fix works:
- Manual steps to verify
- Automated test to add

---

**For FEATURE tasks (category = feature):**

#### Feature Definition

##### User Story
As a [type of user], I want [feature/capability] so that [benefit/value].

##### Acceptance Criteria
Specific conditions that must be met for the feature to be accepted:
- [ ] [Acceptance criterion 1]
- [ ] [Acceptance criterion 2]

---

**For REFACTOR tasks (category = refactor):**

#### Refactor Scope

##### Before Pattern
Describe the current code/architecture pattern being refactored.

##### After Pattern
Describe the target code/architecture pattern.

##### Risk Assessment
What could break during refactoring:
- Callers affected
- Tests that may need updates
- Integration points to verify

---

### Step 7: Failure Modes (REQUIRED)

Document how the implementation should handle failures. Every error path must be specified.

#### Error Handling Table

| Failure Scenario | Expected Behavior | User Feedback | Test Coverage |
|------------------|-------------------|---------------|---------------|
| [What can fail] | [What should happen] | [What user sees] | [Test name] |
| Network timeout | Retry 3x, then fail gracefully | "Connection failed. Please try again." | TestNetworkTimeout |
| Invalid input | Reject with validation error | "Field X is required" | TestInvalidInput |
| Resource not found | Return 404 with helpful message | "Task not found" | TestNotFound |

#### Error Propagation
- Errors in [component] bubble up to [caller] as [error type]
- User-facing errors must include: what went wrong, what user can do
- Internal errors must include: context for debugging (file, operation, input)

### Step 8: Edge Cases (REQUIRED)

**Every edge case MUST have expected behavior AND a test.**

| Input/State | Expected Behavior | Test |
|-------------|-------------------|------|
| Null/undefined input | Return error / use default | Unit test |
| Empty string | Show validation error | Unit test |
| Max length + 1 | Truncate or reject | Unit test |
| Concurrent access | Handle race condition | Integration test |
| Component unmounted | Cancel request, no state update | Unit test |

### Step 9: Review Checklist (REQUIRED)

This checklist will be verified during the review phase. Define upfront what reviewers should check.

#### Code Quality Requirements
| Requirement | Verification Command | Expected |
|-------------|---------------------|----------|
| Linting passes | `golangci-lint run ./...` OR `npm run lint` | 0 errors |
| Type check passes | `go vet ./...` OR `npm run typecheck` | 0 errors |
| No TODOs in new code | `grep -r "TODO" <files>` | None |
| No debug statements | `grep -r "console.log\|fmt.Print" <files>` | None |

#### Test Coverage Requirements
| Requirement | Threshold |
|-------------|-----------|
| Coverage on new code | ≥{{COVERAGE_THRESHOLD}}% |
| All success criteria have tests | 100% |
| All edge cases tested | 100% |
| All failure modes tested | 100% |

#### Integration Requirements
| Requirement | Verification | Expected |
|-------------|--------------|----------|
| No merge conflicts with {{TARGET_BRANCH}} | git merge-tree check | Clean |
| Build succeeds | `make build` OR `npm run build` | Exit 0 |
| Existing tests pass | `make test` | All pass |

## Output Format

**IMPORTANT**: Do NOT use the Write tool to create `spec.md` files. Specs are extracted from the `<artifact>` tags below and saved to the database automatically. Writing spec files to the filesystem causes merge conflicts and cleanup issues.

Create the spec and wrap it in artifact tags for automatic persistence:

<artifact>
# Specification: {{TASK_TITLE}}

## Problem Statement
[1-2 sentences on what we're solving]

## Project Context

### Patterns to Follow
| Pattern | Example Location | How to Apply |
|---------|------------------|--------------|
| [pattern] | [file:line] | [application] |

### Affected Code
| File | Current Behavior | After This Change |
|------|------------------|-------------------|
| [file] | [current] | [after] |

### Breaking Changes
- [ ] Backward compatible / [ ] Breaks: [details]

## Success Criteria

| ID | Criterion | Verification Method | Expected Result | Error Path |
|----|-----------|---------------------|-----------------|------------|
| SC-1 | [Criterion] | [Command/test] | [Result] | [Error behavior to test] |
| SC-2 | [Criterion] | [Command/test] | [Result] | [Error behavior to test] |

## Testing Requirements

| Test Type | Description | Command |
|-----------|-------------|---------|
| Unit | [What it tests] | [Test command] |
| Integration | [What it tests] | [Test command] |
| E2E | [What it tests] | [Test command or "N/A"] |

## Scope

### In Scope
- [Item 1]
- [Item 2]

### Out of Scope
- [Item 1]
- [Item 2]

## Technical Approach
[Description of implementation approach]

### Files to Modify
- [file1]: [what changes]
- [file2]: [what changes]

### New Files
- [file1]: [purpose]

## [Category-Specific Section]
[Include Bug Analysis / Feature Definition / Refactor Scope based on category]

## Failure Modes

| Failure Scenario | Expected Behavior | User Feedback | Test |
|------------------|-------------------|---------------|------|
| [scenario] | [behavior] | [message] | [test name] |

## Edge Cases

| Input/State | Expected Behavior | Test |
|-------------|-------------------|------|
| [edge case] | [behavior] | [test] |

## Review Checklist

### Code Quality
- [ ] Linting passes (0 errors)
- [ ] Type checking passes
- [ ] No TODOs or debug statements

### Test Coverage
- [ ] Coverage ≥{{COVERAGE_THRESHOLD}}% on new code
- [ ] All success criteria have tests
- [ ] All edge cases tested
- [ ] All failure modes tested

### Integration
- [ ] No merge conflicts with {{TARGET_BRANCH}}
- [ ] Build succeeds
- [ ] Existing tests pass

## Open Questions
[Any questions needing clarification - or "None"]
</artifact>

## Phase Completion

After completing the spec, commit your changes:

```bash
git add -A
git commit -m "[orc] {{TASK_ID}}: spec - completed"
```

Then output:

```
### Spec Summary

**Success Criteria**: [count] defined
**Testing Requirements**: [count] defined
**Scope**: [narrow/moderate/wide]
**Commit**: [commit SHA]

<phase_complete>true</phase_complete>
```

If blocked (e.g., requirements unclear):
```
<phase_blocked>
reason: [what's unclear]
needs: [what clarification is needed]
</phase_blocked>
```
