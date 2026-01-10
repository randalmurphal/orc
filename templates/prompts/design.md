# Design Phase

You are creating an architecture/design document for a complex task.

## Context

**Task ID**: ${TASK_ID}
**Task**: ${TASK_TITLE}
**Weight**: ${WEIGHT}

## Research Findings

${RESEARCH_CONTENT}

## Specification

${SPEC_CONTENT}

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

### Step 4: Consider Trade-offs

Document key decisions:
- Alternatives considered
- Why this approach was chosen
- Trade-offs accepted

### Step 5: Create Diagrams

Use ASCII diagrams for:
- System architecture
- Data flow
- Component interactions

## Output Format

### Design Document

Save to `.orc/tasks/${TASK_ID}/artifacts/design.md`:

```markdown
# Design: ${TASK_TITLE}

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

## Key Decisions

### Decision 1: [Topic]
**Options Considered**:
1. [Option A]: [pros/cons]
2. [Option B]: [pros/cons]

**Decision**: [chosen option]
**Rationale**: [why]

### Decision 2: [Topic]
[Same structure]

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

### Commit Design

```bash
git add -A
git commit -m "[orc] ${TASK_ID}: design - completed

Phase: design
Status: completed
Artifact: artifacts/design.md
"
```

### Output Completion

```
### Design Summary

**Components**: [count]
**Key Decisions**: [count]
**Commit**: [commit SHA]

<phase_complete>true</phase_complete>
```

If blocked (e.g., need architectural review):
```
<phase_blocked>
reason: [what's blocking design]
needs: [what decision/input is needed]
</phase_blocked>
```
