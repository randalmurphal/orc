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

### Step 2: Define Success Criteria (REQUIRED)

Create specific, testable criteria with **explicit verification methods**:

| ID | Criterion | Verification Method | Expected Result |
|----|-----------|---------------------|-----------------|
| SC-1 | [What must be true] | [How to verify] | [What success looks like] |

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

### Step 7: Edge Cases

Document:
- Error conditions and how to handle them
- Boundary values
- Invalid inputs

## Output Format

Create the spec and wrap it in artifact tags for automatic persistence:

<artifact>
# Specification: {{TASK_TITLE}}

## Problem Statement
[1-2 sentences on what we're solving]

## Success Criteria

| ID | Criterion | Verification Method | Expected Result |
|----|-----------|---------------------|-----------------|
| SC-1 | [Specific criterion] | [Executable command/test] | [Concrete result] |
| SC-2 | [Specific criterion] | [Executable command/test] | [Concrete result] |
| SC-3 | [Specific criterion] | [Executable command/test] | [Concrete result] |

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

## Edge Cases
- [Edge case 1]: [how to handle]
- [Edge case 2]: [how to handle]

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
