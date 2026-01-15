# spec - Iteration 1

## Prompt

Create a specification for this task:

**Task**: Investigate: Tasks auto-started without user action
**Category**: {{TASK_CATEGORY}}
**Description**: During a session, TASK-198, TASK-199, and TASK-180 were started without user action. User had no other Claude sessions open and didn't click anything in the web UI. Somehow an agent running TASK-179 or another mechanism triggered orc run on these tasks. Need to investigate: 1) Check if TASK-179 agent called orc run via bash, 2) Check for any auto-trigger mechanisms in orc serve, 3) Check hooks or automation. Evidence: All three tasks have Claude project directories and were running simultaneously.

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
# Specification: Investigate: Tasks auto-started without user action

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
git commit -m "[orc] TASK-200: spec - completed"
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

**Commit**: 6465fe8e

<phase_complete>true</phase_complete>

---
Tokens: 2663976 input, 8475 output, 117756 cache_creation, 2544830 cache_read
Complete: true
Blocked: false
