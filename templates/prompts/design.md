# Design Phase

You are creating an architecture/design document for a complex task.

## Context

**Task ID**: {{TASK_ID}}
**Task**: {{TASK_TITLE}}
**Weight**: {{WEIGHT}}

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

## Research Findings

{{RESEARCH_CONTENT}}

## Specification

{{SPEC_CONTENT}}

## Instructions

### Step 1: Define Components

Break down the solution into components:
- What are the major pieces?
- How do they interact?
- What are their responsibilities?

### Step 2: Design Data Flow

Document how data moves:
- Input sources
- Transformations
- Storage
- Output destinations

### Step 3: Define Interfaces

For each component:
- Public API/methods
- Input/output types
- Error conditions

### Step 4: Document Design Decisions

For each significant choice, use structured format:

| ID | Decision Area | Options | Choice | Rationale |
|----|---------------|---------|--------|-----------|
| DD-1 | [Area] | [A, B, C] | [Chosen] | [Why] |

**Decisions to document:**
- Data structures and storage approach
- Error handling strategy
- API/interface design choices
- State management approach
- Testing strategy

### Step 5: Assess Risks

Identify what could go wrong during implementation:

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| [What] | Low/Med/High | Low/Med/High | [Strategy] |

### Step 6: Define Implementation Order

Plan the sequence for the implement phase:

1. **[First component]** - [Why first]
2. **[Second component]** - [Dependencies on first]
3. ...

### Step 7: Create Diagrams

Use ASCII diagrams for:
- System architecture
- Data flow
- Component interactions

## Output Format

**CRITICAL**: Your final output MUST be a JSON object with the design document in the `artifact` field.

Create the design document following this structure:

```markdown
# Design: {{TASK_TITLE}}

## Overview

[1-2 paragraph summary of the design]

## Architecture

```
┌─────────────┐     ┌─────────────┐
│ Component A │────►│ Component B │
└─────────────┘     └─────────────┘
        │
        ▼
┌─────────────┐
│ Component C │
└─────────────┘
```

## Components

### Component A
**Purpose**: [what it does]
**Responsibilities**:
- [responsibility 1]
- [responsibility 2]

**Interface**:
```go
type ComponentA interface {
    Method1(input Type1) (Type2, error)
    Method2(input Type3) error
}
```

### Component B
[Same structure]

## Data Flow

```
User Input
    │
    ▼
┌─────────┐
│ Parse   │
└────┬────┘
     │
     ▼
┌─────────┐     ┌─────────┐
│ Validate│────►│ Process │
└─────────┘     └────┬────┘
                     │
                     ▼
                ┌─────────┐
                │ Output  │
                └─────────┘
```

## Design Decisions

| ID | Decision Area | Options | Choice | Rationale |
|----|---------------|---------|--------|-----------|
| DD-1 | [Area] | [Options] | [Choice] | [Why] |
| DD-2 | [Area] | [Options] | [Choice] | [Why] |

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| [Risk 1] | Low/Med/High | Low/Med/High | [Strategy] |
| [Risk 2] | Low/Med/High | Low/Med/High | [Strategy] |

## Implementation Order

1. **[Component]**: [Reason for ordering]
2. **[Component]**: [Dependencies on previous]
3. ...

## Error Handling

| Error Condition | Handling Strategy |
|-----------------|-------------------|
| [Error 1] | [How handled] |
| [Error 2] | [How handled] |

## Testing Strategy

- Unit tests for each component
- Integration tests for component interactions
- E2E tests for full workflows

## Open Questions

[Any remaining design questions - or "None"]
```

## Phase Completion

Output a JSON object with the design in the `artifact` field:

```json
{
  "status": "complete",
  "summary": "Design defined 3 components with 5 decisions documented",
  "artifact": "# Design: Feature Name\n\n## Overview\n..."
}
```

If blocked (e.g., need architectural review):
```json
{
  "status": "blocked",
  "reason": "[what's blocking design and what decision/input is needed]"
}
```
