# Research Phase

You are investigating the codebase and gathering context for a task.

## Context

**Task ID**: {{TASK_ID}}
**Task**: {{TASK_TITLE}}
**Weight**: {{WEIGHT}}
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
Then output ONLY this JSON to signal completion:

```json
{"status": "complete", "summary": "Research analyzed [count] files, found [count] dependencies, identified [count] challenges. Commit: [SHA]"}
```

If blocked (e.g., access issues, unclear requirements), output ONLY this JSON:
```json
{"status": "blocked", "reason": "[what's blocking research and what's needed to proceed]"}
```
