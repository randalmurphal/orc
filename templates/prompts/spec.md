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
- **DO NOT** write `spec.md` files to the filesystem - specs are captured via JSON output
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

#### Preservation Requirements (REQUIRED)

**What existing behavior MUST NOT change?**

This section prevents accidental feature removal and regression. List:
- Features that must continue working exactly as before
- Invariants that must be maintained
- Code paths that must NOT be modified

| Existing Behavior | Why It Must Be Preserved | How to Verify It Still Works |
|-------------------|--------------------------|------------------------------|
| [Feature/behavior] | [Business reason or dependency] | [Test or command to verify] |

**Example:**
| Existing Behavior | Why It Must Be Preserved | How to Verify |
|-------------------|--------------------------|---------------|
| WebSocket real-time updates | Users depend on live task status | E2E test: task status updates without refresh |
| Task dependency blocking | Core workflow feature | Unit test: blocked tasks cannot start |

If this is a greenfield feature with no preservation requirements, explicitly state: "No preservation requirements - new functionality only."

#### Feature Replacement Policy

When this task **replaces** existing functionality:

1. **Default: Full replacement** - Old feature is removed entirely, new feature takes over
2. **No backwards compatibility** unless explicitly requested in the task description
3. **Migration required if**:
   - Data format changes (provide migration script/command)
   - API contracts change (document breaking changes, provide migration guide)
   - Configuration changes (provide upgrade instructions)

| Replaced Feature | Replacement | Migration Needed? | Migration Provided |
|------------------|-------------|-------------------|-------------------|
| [old feature] | [new feature] | Yes/No | [script/guide location] |

If no features are being replaced, state: "No replacements - additive changes only."

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

### Step 5.5: Integration Requirements (REQUIRED)

**Components must be wired up to be usable.** Creating a component without integrating it means the work is incomplete.

For every new file/component created, answer:
- **Who consumes this?** (route, parent component, import location)
- **Is the consumer updated to use it?** If not, why not?

#### Integration Checklist

| New File | Consumer | Integration Task | Included in This Spec? |
|----------|----------|------------------|------------------------|
| [component.tsx] | [Page or parent] | [Update import/render] | Yes / No (blocked by X) |
| [api endpoint] | [Client code] | [Update API client] | Yes / No (blocked by X) |
| [store] | [Components] | [Connect via hooks] | Yes / No (blocked by X) |

**Rules:**
1. **Default: Integration is IN SCOPE** - If you create it, wire it up
2. **Explicit exceptions only** - If integration is blocked by other work, state WHY and create a follow-up task reference
3. **No orphan components** - A component that exists but isn't used is not "done"
4. **Routes must render new pages** - Creating a page component means updating the router
5. **Replaced components must be removed** - Don't leave dead code alongside new code

**Example - WRONG (orphan component):**
```
New Files:
- web/src/components/board/BoardView.tsx  ← Created but never used!
```

**Example - CORRECT (integrated):**
```
New Files:
- web/src/components/board/BoardView.tsx

Files to Modify:
- web/src/pages/Board.tsx: Replace <Board /> with <BoardView />
```

**If integration is genuinely blocked**, document:
- What blocks it (dependency on another task)
- What follow-up task will complete the integration
- Add a TODO with the blocker task ID

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

**CRITICAL**: Your final output MUST be a JSON object with the spec in the `artifact` field. This is how specs are captured - no files, no XML tags.

Create the spec following this structure:

```markdown
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

### Preservation Requirements
| Existing Behavior | Why It Must Be Preserved | Verification |
|-------------------|--------------------------|--------------|
| [behavior] | [reason] | [test/command] |

### Feature Replacements
| Replaced Feature | Replacement | Migration |
|------------------|-------------|-----------|
| [old] | [new] | [script/guide or "N/A"] |

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

### Integration Requirements

| New File | Consumer | Integration Task | Included? |
|----------|----------|------------------|-----------|
| [file] | [consumer] | [task] | Yes / No (reason) |

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
```

## Phase Completion

Output a JSON object with the spec in the `artifact` field:

```json
{
  "status": "complete",
  "summary": "Spec defined 3 success criteria and 2 testing requirements",
  "artifact": "# Specification: Feature Name\n\n## Problem Statement\n..."
}
```

If blocked (requirements genuinely unclear):
```json
{
  "status": "blocked",
  "reason": "[what's unclear and what clarification is needed]"
}
```
