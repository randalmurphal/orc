# Research Phase

You are investigating the codebase and gathering context for a task.

## Context

**Task ID**: {{TASK_ID}}
**Task**: {{TASK_TITLE}}
**Weight**: {{WEIGHT}}
**Description**: {{TASK_DESCRIPTION}}

## Instructions

### Step 1: Understand the Goal

What exactly needs to be accomplished?
- Break down the task into sub-goals
- Identify what "done" looks like

### Step 2: Explore the Codebase

Find relevant code:
- Entry points for the feature
- Existing similar functionality
- Patterns used in the project
- Configuration and dependencies

### Step 3: Identify Dependencies

What does this task depend on?
- External libraries/services
- Internal modules
- Database schemas
- API contracts

### Step 4: Assess Impact

What will this change affect?
- Direct changes (files to modify)
- Indirect effects (callers, consumers)
- Breaking changes (if any)
- Test updates needed

### Step 5: Document Findings

Create a research summary documenting:
- Key files and their purposes
- Patterns to follow
- Potential challenges
- Open questions

## Output Format

Create the research document and wrap it in artifact tags for automatic persistence:

<artifact>
# Research: {{TASK_TITLE}}

## Goal
[Clear statement of what needs to be accomplished]

## Relevant Code

### Entry Points
- [file1]: [purpose]
- [file2]: [purpose]

### Related Functionality
- [file/function]: [how it relates]

### Patterns Used
- [Pattern 1]: [where used, how]
- [Pattern 2]: [where used, how]

## Dependencies

### External
- [library]: [version, purpose]

### Internal
- [module]: [purpose]

## Impact Analysis

### Files to Modify
- [file1]: [what changes]

### Breaking Changes
[None / List of breaking changes]

### Tests to Update
- [test file]: [what to add/change]

## Challenges

- [Challenge 1]: [potential approach]
- [Challenge 2]: [potential approach]

## Open Questions

- [Question 1]
- [Question 2]

## Recommendations

[Summary of recommended approach based on findings]
</artifact>

## Phase Completion

After completing research, commit your changes:

```bash
git add -A
git commit -m "[orc] {{TASK_ID}}: research - completed"
```

Then output:

```
### Research Summary

**Files Analyzed**: [count]
**Dependencies Found**: [count]
**Challenges Identified**: [count]
**Commit**: [commit SHA]

<phase_complete>true</phase_complete>
```

If blocked (e.g., access issues, unclear requirements):
```
<phase_blocked>
reason: [what's blocking research]
needs: [what's needed to proceed]
</phase_blocked>
```
