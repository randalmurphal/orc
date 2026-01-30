<context>
# Specification Phase

<task>
ID: {{TASK_ID}}
Title: {{TASK_TITLE}}
Weight: {{WEIGHT}}
Category: {{TASK_CATEGORY}}
Description: {{TASK_DESCRIPTION}}
</task>

<project>
Language: {{LANGUAGE}}
Frameworks: {{FRAMEWORKS}}
Has Frontend: {{HAS_FRONTEND}}
Test Command: {{TEST_COMMAND}}
</project>

<worktree_safety>
Path: {{WORKTREE_PATH}}
Branch: {{TASK_BRANCH}}
Target: {{TARGET_BRANCH}}
DO NOT push to {{TARGET_BRANCH}} or checkout other branches.
DO NOT write spec.md files to filesystem - specs are captured via JSON output.
</worktree_safety>

{{INITIATIVE_CONTEXT}}
{{CONSTITUTION_CONTENT}}

<research_findings>
{{RESEARCH_CONTENT}}
</research_findings>
</context>

<instructions>
Create a clear, actionable specification with prioritized user stories and explicit verification methods.

<initiative_alignment>
**CRITICAL: If this task belongs to an initiative, your spec MUST capture ALL requirements from the initiative vision.**

Before defining success criteria:
1. Re-read the Initiative Context section above (if present)
2. Extract every requirement, feature, and behavior mentioned in the vision
3. Cross-reference against your planned success criteria
4. Add missing criteria for any initiative requirements not covered

**Common failures to avoid:**
- Task description says "Pause, New Task" but initiative vision says "Pause, New Task, Panel toggle" → You MUST include Panel toggle
- Vision mentions specific UI elements → They MUST appear in success criteria
- Vision lists exact behaviors → They MUST be testable in your spec

The task description is a starting point. The initiative vision is the source of truth.
</initiative_alignment>

<clarification_rules>
**Maximum 3 [NEEDS CLARIFICATION] items.** For everything else:
- Make an informed assumption
- Document in Assumptions section
- Priority: scope > security > UX > technical

**In auto-mode: NEVER block.** Document assumptions and proceed. Blocking wastes execution time.
</clarification_rules>

## Step 1: Analyze Requirements

Break down the task into:
- What needs to be built/fixed/changed
- What already exists (relevant code, patterns)
- What constraints apply (compatibility, performance, security)

## Step 2: Project Context (REQUIRED)

Before specifying, understand how this change fits with the existing codebase.

### Patterns to Follow

Identify existing patterns this task must follow. Read CLAUDE.md and look at similar code.

| Pattern | Example Location | How to Apply |
|---------|------------------|--------------|
| [Error handling] | [file:line] | [How to use it here] |
| [Naming conventions] | [file:line] | [How to apply] |

### Affected Code

What existing code will be impacted by this change?

| File | Current Behavior | After This Change |
|------|------------------|-------------------|
| [file] | [what it does now] | [what it will do] |

### Breaking Changes

- [ ] This change is backward compatible
- [ ] This change breaks: [list what breaks and migration path]

### Preservation Requirements (REQUIRED)

**What existing behavior MUST NOT change?**

| Existing Behavior | Why It Must Be Preserved | How to Verify It Still Works |
|-------------------|--------------------------|------------------------------|
| [Feature/behavior] | [Business reason or dependency] | [Test or command to verify] |

If greenfield with no preservation requirements: "No preservation requirements - new functionality only."

### Feature Replacement Policy

When this task **replaces** existing functionality:
1. **Default: Full replacement** - Old feature is removed entirely
2. **No backwards compatibility** unless explicitly requested
3. **Migration required if**: Data format, API contracts, or configuration changes

| Replaced Feature | Replacement | Migration Needed? | Migration Provided |
|------------------|-------------|-------------------|-------------------|
| [old feature] | [new feature] | Yes/No | [script/guide location] |

If no replacements: "No replacements - additive changes only."

<user_stories>
## Step 3: Prioritized User Stories (REQUIRED for features)

Break into independently shippable stories:

| Priority | Story | Independent Test | Success Criteria |
|----------|-------|------------------|------------------|
| P1 (MVP) | As a [user], I want [X] so that [benefit] | [How to test alone] | SC-1, SC-2 |
| P2 | As a [user], I want [X] so that [benefit] | [How to test alone] | SC-3 |
| P3 | As a [user], I want [X] so that [benefit] | [How to test alone] | SC-4 |

**Rules:**
1. **P1 = Minimum Viable Product.** MUST be completable in isolation.
2. Each story has its own success criteria.
3. If it can't ship alone, it's a sub-task, not a story.
4. Order by value delivered, not implementation order.
</user_stories>

<success_criteria>
## Step 4: Define Success Criteria (REQUIRED)

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
- Each criterion MUST have executable verification (no vague criteria)
- Expected results must be concrete (exit code 0, output contains X, file exists)
- Error paths MUST be specified - what happens when things fail?
- Focus on user-visible behavior, not implementation details

**Wiring Checklist** - Every spec's success criteria MUST include:
- All new functions are called from at least one production code path
- All new interfaces have registered implementations
- Integration tests verify the wiring exists
</success_criteria>

<behavioral_specs>
## Step 4b: Behavioral Specifications (Optional)

For complex user interactions or state-dependent behavior, use Given/When/Then format to make success criteria unambiguous:

| ID | Scenario |
|----|----------|
| BDD-1 | **Given** user is logged in with admin role<br>**When** they click "Delete User" button<br>**Then** confirmation modal shows user's name and requires explicit confirmation<br>**Maps to:** SC-3 |
| BDD-2 | **Given** no internet connection<br>**When** user attempts to sync data<br>**Then** app shows "Offline mode - changes saved locally" message<br>**Error:** If local storage fails, show "Could not save changes" with retry option<br>**Maps to:** SC-5, SC-6 |

### When to Use Given/When/Then
- Multi-step user flows with specific sequences
- State-dependent behavior (logged in/out, online/offline, permissions)
- Edge cases with specific preconditions
- Complex validation logic with multiple outcomes

### Format
- **Given** - Initial state/preconditions (what must be true before the action)
- **When** - Action/trigger (what the user does)
- **Then** - Expected outcome (observable result)
- **Error** - (optional) What happens on failure
- **Maps to** - (required) Links to SC-X criteria for traceability

**Each BDD scenario MUST map to one or more Success Criteria (SC-X).**
</behavioral_specs>

## Step 5: Define Testing Requirements (REQUIRED)

Specify what tests must pass to consider the work complete:

| Test Type | Description | Command |
|-----------|-------------|---------|
| Unit | [Specific functions/modules to test] | [Test command] |
| Integration | [Component interactions to verify] | [Test command] |
| E2E | [User flows to validate] | [Test command or "N/A"] |

## Step 6: Define Scope

### In Scope
List exactly what will be implemented.

### Out of Scope
List what will NOT be implemented (prevents scope creep).

## Step 7: Technical Approach

Describe:
- Key files to create/modify
- Patterns to follow
- Dependencies needed
- Data structures/schemas

### Integration Requirements (REQUIRED)

**Components must be wired up to be usable.** Creating a component without integrating it means the work is incomplete.

| New File | Consumer | Integration Task | Included in This Spec? |
|----------|----------|------------------|------------------------|
| [component.tsx] | [Page or parent] | [Update import/render] | Yes / No (blocked by X) |

**Rules:**
1. **Default: Integration is IN SCOPE** - If you create it, wire it up
2. **No orphan components** - A component that exists but isn't used is not "done"
3. **Routes must render new pages** - Creating a page means updating the router
4. **Replaced components must be removed** - Don't leave dead code

**Mandatory Questions** - Every spec MUST answer these:
- What existing code paths will call the new code?
- Where will the new code be registered/wired?
- What integration tests will verify the wiring?

## Step 8: Category-Specific Analysis

**For BUG tasks (category = bug):**

### Bug Analysis

#### Reproduction Steps
1. [Exact step to reproduce]
2. [Observe: describe the bug behavior]

#### Current vs Expected Behavior
- **Current:** [what happens now]
- **Expected:** [what should happen]

#### Root Cause (if known)
[Where the bug originates in the code]

---

**For FEATURE tasks (category = feature):**

(Use the Prioritized User Stories from Step 3)

---

**For REFACTOR tasks (category = refactor):**

### Refactor Scope

#### Before Pattern
[Current code/architecture pattern]

#### After Pattern
[Target code/architecture pattern]

#### Risk Assessment
- Callers affected
- Tests that may need updates
- Integration points to verify

---

## Step 9: Failure Modes (REQUIRED)

Document how the implementation should handle failures.

| Failure Scenario | Expected Behavior | User Feedback | Test Coverage |
|------------------|-------------------|---------------|---------------|
| [What can fail] | [What should happen] | [What user sees] | [Test name] |

#### Error Propagation
- Errors in [component] bubble up to [caller] as [error type]
- User-facing errors: what went wrong + what user can do
- Internal errors: context for debugging (file, operation, input)

## Step 10: Edge Cases (REQUIRED)

**Every edge case MUST have expected behavior AND a test.**

| Input/State | Expected Behavior | Test |
|-------------|-------------------|------|
| Null/undefined input | Return error / use default | Unit test |
| Empty string | Show validation error | Unit test |
| Max length + 1 | Truncate or reject | Unit test |

<quality_checklist>
## Step 11: Quality Checklist (REQUIRED)

Self-evaluate before completing. **Implement phase blocked until all pass.**

| ID | Check | Pass? |
|----|-------|-------|
| all_criteria_verifiable | Every SC has executable verification | |
| no_technical_metrics | SC describes user behavior, not internals | |
| p1_stories_independent | P1 stories can ship alone | |
| scope_explicit | In/out scope clearly listed | |
| max_3_clarifications | ≤3 clarifications, rest are assumptions | |
| initiative_aligned | All initiative vision requirements captured in SC | |

Include in JSON output.
</quality_checklist>

## Step 12: Review Checklist

Define upfront what reviewers should check.

### Code Quality Requirements

| Requirement | Verification Command | Expected |
|-------------|---------------------|----------|
| Linting passes | `golangci-lint run ./...` OR `npm run lint` | 0 errors |
| Type check passes | `go vet ./...` OR `npm run typecheck` | 0 errors |
| No TODOs in new code | `grep -r "TODO" <files>` | None |
| No debug statements | `grep -r "console.log\|fmt.Print" <files>` | None |

### Test Coverage Requirements

| Requirement | Threshold |
|-------------|-----------|
| Coverage on new code | ≥{{COVERAGE_THRESHOLD}}% |
| All success criteria have tests | 100% |
| All edge cases tested | 100% |
| All failure modes tested | 100% |

### Integration Requirements

| Requirement | Verification | Expected |
|-------------|--------------|----------|
| No merge conflicts with {{TARGET_BRANCH}} | git merge-tree check | Clean |
| Build succeeds | `make build` OR `npm run build` | Exit 0 |
| Existing tests pass | `make test` | All pass |
</instructions>

<output_format>
**CRITICAL**: Your final output MUST be a JSON object with the spec in the `content` field.

```json
{
  "status": "complete",
  "summary": "Spec with N success criteria, M user stories",
  "content": "# Specification: [Title]\n\n## Problem Statement\n...",
  "quality_checklist": [
    {"id": "all_criteria_verifiable", "check": "Every SC has executable verification", "passed": true},
    {"id": "no_technical_metrics", "check": "SC describes user behavior, not internals", "passed": true},
    {"id": "p1_stories_independent", "check": "P1 stories can ship alone", "passed": true},
    {"id": "scope_explicit", "check": "In/out scope clearly listed", "passed": true},
    {"id": "max_3_clarifications", "check": "≤3 clarifications, rest are assumptions", "passed": true}
  ],
  "assumptions": [
    {"area": "scope", "assumption": "[what was assumed]", "rationale": "[why this is reasonable]"}
  ]
}
```

If blocked (requirements genuinely unclear - max 3 items):
```json
{
  "status": "blocked",
  "reason": "[what's unclear and what clarification is needed]"
}
```

### Spec Artifact Structure

```markdown
# Specification: {{TASK_TITLE}}

## Problem Statement
[1-2 sentences on what we're solving]

## User Stories

| Priority | Story | Success Criteria |
|----------|-------|------------------|
| P1 (MVP) | [story] | SC-1, SC-2 |

## Success Criteria

| ID | Criterion | Verification Method | Expected Result | Error Path |
|----|-----------|---------------------|-----------------|------------|
| SC-1 | [Criterion] | [Command/test] | [Result] | [Error behavior] |

## Behavioral Specifications (if applicable)

| ID | Scenario |
|----|----------|
| BDD-1 | **Given** [precondition]<br>**When** [action]<br>**Then** [outcome]<br>**Maps to:** SC-X |

## Project Context

### Patterns to Follow
| Pattern | Example Location | How to Apply |
|---------|------------------|--------------|
| [pattern] | [file:line] | [application] |

### Preservation Requirements
| Existing Behavior | Why It Must Be Preserved | Verification |
|-------------------|--------------------------|--------------|
| [behavior] | [reason] | [test/command] |

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
[Implementation approach]

### Files to Modify
- [file1]: [what changes]

### New Files
- [file1]: [purpose]

### Integration Requirements
| New File | Consumer | Integration Task | Included? |
|----------|----------|------------------|-----------|
| [file] | [consumer] | [task] | Yes / No |

## Failure Modes

| Failure Scenario | Expected Behavior | User Feedback | Test |
|------------------|-------------------|---------------|------|
| [scenario] | [behavior] | [message] | [test name] |

## Edge Cases

| Input/State | Expected Behavior | Test |
|-------------|-------------------|------|
| [edge case] | [behavior] | [test] |

## Assumptions (if any)

| Area | Assumption | Rationale |
|------|------------|-----------|
| [area] | [what was assumed] | [why reasonable] |

## Open Questions
[Any questions needing clarification - or "None"]
```
</output_format>
