# Specification Phase

You are writing a detailed specification for a task.

## Context

**Task ID**: {{TASK_ID}}
**Task**: {{TASK_TITLE}}
**Weight**: {{WEIGHT}}
**Description**: {{TASK_DESCRIPTION}}

## Research Findings (if available)

{{RESEARCH_CONTENT}}

## Instructions

### Step 1: Analyze Requirements

Break down the task into:
- What needs to be built
- What already exists
- What constraints apply

### Step 2: Define Success Criteria

Create specific, testable criteria:
- Each criterion should be verifiable
- Use concrete conditions (file exists, test passes, API returns X)
- No vague language ("works well", "is fast")

### Step 3: Define Scope

#### In Scope
List exactly what will be implemented.

#### Out of Scope
List what will NOT be implemented (prevents scope creep).

### Step 4: Technical Approach

Describe:
- Key files to create/modify
- Patterns to follow
- Dependencies needed
- Data structures/schemas

### Step 5: Edge Cases

Document:
- Error conditions
- Boundary values
- Invalid inputs

## Output Format

Create the spec in this format:

```markdown
# Specification: {{TASK_TITLE}}

## Problem Statement
[1-2 sentences on what we're solving]

## Success Criteria
- [ ] [Criterion 1 - specific and testable]
- [ ] [Criterion 2 - specific and testable]
- [ ] [Criterion 3 - specific and testable]

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

## Edge Cases
- [Edge case 1]: [how to handle]
- [Edge case 2]: [how to handle]

## Open Questions
[Any questions needing clarification - or "None"]
```

## Phase Completion

### Commit the Spec

Save the spec to `.orc/tasks/{{TASK_ID}}/artifacts/spec.md` and commit:

```bash
git add -A
git commit -m "[orc] {{TASK_ID}}: spec - completed

Phase: spec
Status: completed
Artifact: artifacts/spec.md
"
```

### Output Completion

```
### Spec Summary

**Success Criteria**: [count] defined
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
