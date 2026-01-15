# spec - Iteration 1

## Prompt

Create a specification for this task:

**Task**: Add --status flag to orc edit command
**Category**: {{TASK_CATEGORY}}
**Description**: ## Problem
Cannot change task status via CLI without running the task or editing YAML directly.

Tried: `orc edit TASK-189 --status completed`
Got: `Error: unknown flag: --status`

## Solution
Add --status flag to the edit command with validation:
- Valid values: planned, running, paused, completed, failed
- Prevent invalid transitions (e.g., can't go from planned to running without actually running)
- Simple status corrections like marking already-fixed tasks as completed should work

## Implementation
- Add --status string flag to edit command in internal/cli/edit.go
- Validate status values
- Update task.yaml status field
- Trigger auto-commit

## Success Criteria
`orc edit TASK-XXX --status completed` works

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
# Specification: Add --status flag to orc edit command

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
git commit -m "[orc] TASK-195: spec - completed"
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

**Commit**: 17b8d4f
<phase_complete>true</phase_complete>

---
Tokens: 416166 input, 2889 output, 68037 cache_creation, 348120 cache_read
Complete: true
Blocked: false
