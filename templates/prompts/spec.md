# Specification Phase

<output_format>
**CRITICAL**: Your final output MUST be a JSON object with the spec in the `content` field.

```json
{
  "status": "complete",
  "summary": "Spec with N success criteria, M user stories",
  "content": "# Specification: [Title]\n\n## Problem Statement\n...",
  "quality_checklist": [
    {"id": "all_criteria_verifiable", "check": "Every SC has executable verification", "passed": true},
    {"id": "no_existence_only_criteria", "check": "SC verifies behavior, not just existence", "passed": true},
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

### Out of Scope
- [Item 1]

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

<critical_constraints>
## Quality Checklist

Self-evaluate before completing. **Implement phase blocked until all pass.**

| ID | Check |
|----|-------|
| all_criteria_verifiable | Every SC has executable verification |
| no_existence_only_criteria | SC verifies behavior, not just existence (no "file exists" or "record created" without behavioral verification) |
| p1_stories_independent | P1 stories can ship alone |
| scope_explicit | In/out scope clearly listed |
| max_3_clarifications | ≤3 clarifications, rest are assumptions |
| initiative_aligned | All initiative vision requirements captured in SC |

**Top failure mode:** The most common failure is success criteria that verify existence ("file exists") instead of behavior ("file does X when given Y"). Every success criterion MUST describe observable behavior with a concrete expected result.

**All Code Must Be Tested** - If the task produces ANY executable code, it requires behavioral tests:

| Code Type | What To Test | Anti-Pattern |
|-----------|-------------|--------------|
| Scripts (bash, python) | Input/output behavior, exit codes, error handling | "Script file exists on disk" |
| Hook scripts | Blocking/allowing behavior, correct exit codes | "Hook is seeded to DB" |
| Templates | Rendered output correctness | "Template file is embedded" |
| Config generators | Generated config is valid and functional | "Config file was written" |
| CLI tools | Command output, flags, error messages | "Binary compiles" |

**Wiring Checklist** - Every spec's success criteria MUST include:
- All new functions are called from at least one production code path
- All new interfaces have registered implementations
- Integration tests verify the wiring exists
</critical_constraints>

<example_good_spec>
## Problem Statement
API endpoints have no rate limiting, allowing abuse and resource exhaustion.

## User Stories
| Priority | Story | Success Criteria |
|----------|-------|------------------|
| P1 (MVP) | As an API consumer, I want rate limits so the service stays available | SC-1, SC-2, SC-3 |

## Success Criteria
| ID | Criterion | Verification | Expected Result |
|----|-----------|-------------|-----------------|
| SC-1 | Rate limiter returns 429 after limit exceeded | Send 6 requests in 1 second (limit is 5) | HTTP 429 with Retry-After header |
| SC-2 | Rate limit resets after window expires | Wait 61 seconds, send request | HTTP 200 (limit reset) |
| SC-3 | Rate limiter middleware is wired into router | grep -r "rateLimiter" internal/api/ | Found in router setup, build fails if removed |

## Scope
### In Scope
- Token bucket rate limiter middleware
- Per-IP rate limiting with configurable limits
- 429 response with Retry-After header

### Out of Scope
- Per-user rate limiting (requires auth, separate task)
- Distributed rate limiting (single-instance only)
</example_good_spec>

<context>
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

## Referenced Files

If the task description references any files, read every referenced file completely. Extract behavioral requirements from the code. Cross-reference against your success criteria - add missing criteria for any requirements not covered.

## Clarification Rules

**Maximum 3 [NEEDS CLARIFICATION] items.** For everything else, make an informed assumption and document it. Priority: scope > security > UX > technical.

**In auto-mode: NEVER block.** Document assumptions and proceed.

## Step 1: Analyze Requirements

Break down what needs to be built/fixed/changed, what already exists, and what constraints apply.

## Step 2: Project Context

### Patterns to Follow

| Pattern | Example Location | How to Apply |
|---------|------------------|--------------|
| [Error handling] | [file:line] | [How to use it here] |

### Affected Code

| File | Current Behavior | After This Change |
|------|------------------|-------------------|
| [file] | [what it does now] | [what it will do] |

### Breaking Changes
- [ ] This change is backward compatible
- [ ] This change breaks: [list what breaks and migration path]

### Preservation Requirements

| Existing Behavior | Why It Must Be Preserved | How to Verify It Still Works |
|-------------------|--------------------------|------------------------------|
| [Feature/behavior] | [Business reason or dependency] | [Test or command to verify] |

If greenfield with no preservation requirements: "No preservation requirements - new functionality only."

## Step 3: Prioritized User Stories (REQUIRED for features)

| Priority | Story | Independent Test | Success Criteria |
|----------|-------|------------------|------------------|
| P1 (MVP) | As a [user], I want [X] so that [benefit] | [How to test alone] | SC-1, SC-2 |
| P2 | As a [user], I want [X] so that [benefit] | [How to test alone] | SC-3 |

**Rules:** P1 = Minimum Viable Product, completable in isolation. Each story has its own success criteria. Order by value delivered.

## Step 4: Define Success Criteria (REQUIRED)

| ID | Criterion | Verification Method | Expected Result | Error Path |
|----|-----------|---------------------|-----------------|------------|
| SC-1 | [What must be true] | [How to verify] | [What success looks like] | [What error behavior to test] |

**Rules:**
- Each criterion MUST have executable verification (no vague criteria)
- Expected results must be concrete (exit code 0, output contains X, file exists)
- Error paths MUST be specified - what happens when things fail?
- Focus on user-visible behavior, not implementation details

## Step 4b: Behavioral Specifications (Optional)

For complex user interactions or state-dependent behavior, use Given/When/Then:

| ID | Scenario |
|----|----------|
| BDD-1 | **Given** [precondition]<br>**When** [action]<br>**Then** [outcome]<br>**Maps to:** SC-X |

Use when: multi-step flows, state-dependent behavior, edge cases with specific preconditions.

## Step 5: Testing, Scope, and Technical Approach

Define test types (unit/integration/E2E) with commands. List in-scope and out-of-scope items explicitly. Describe key files, patterns, dependencies, and data structures.

### Integration Requirements

| New File | Consumer | Integration Task | Included in This Spec? |
|----------|----------|------------------|------------------------|
| [component.tsx] | [Page or parent] | [Update import/render] | Yes / No (blocked by X) |

## Step 6: Category-Specific Analysis

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

#### Pattern Prevalence (REQUIRED for bugs)

The bug you found in one code path likely exists in others. **You MUST check.**

1. Identify the buggy pattern (e.g., "sets weight but doesn't set workflow_id")
2. Grep the codebase for ALL locations with the same pattern
3. List every code path that has the same issue

| Code Path | File:Line | Has Bug? | In Scope? |
|-----------|-----------|----------|-----------|
| [path 1] | file.go:123 | Yes/No | Yes / No (separate task) |

**If other paths have the same bug, they MUST be either:**
- Included in this task's scope and success criteria, OR
- Explicitly documented as out-of-scope with a note to create follow-up tasks

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

## Step 7: Failure Modes and Edge Cases

| Failure Scenario | Expected Behavior | User Feedback | Test Coverage |
|------------------|-------------------|---------------|---------------|
| [What can fail] | [What should happen] | [What user sees] | [Test name] |

| Input/State | Expected Behavior | Test |
|-------------|-------------------|------|
| [edge case] | [behavior] | [test] |
</instructions>
