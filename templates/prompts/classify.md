# Task Classification

You are classifying a task to determine the appropriate level of rigor.

## Task

**Title**: ${TASK_TITLE}
**Description**: ${TASK_DESCRIPTION}

## Weight Definitions

| Weight | Scope | Duration | Example |
|--------|-------|----------|---------|
| `trivial` | 1 file, <10 lines | Minutes | Typo, config tweak |
| `small` | 1 component, <100 lines | <1 hour | Bug fix, add field |
| `medium` | Multiple files, investigation | Hours | Feature, refactor |
| `large` | Cross-cutting, significant | Days | Major feature |
| `greenfield` | New system from scratch | Weeks | New service |

## Classification Criteria

Consider:

1. **Scope**: How many files will likely change?
2. **Uncertainty**: Is the approach obvious or needs investigation?
3. **Risk**: What could break? Security implications?
4. **Dependencies**: External systems, database changes?

## Signals

| Signal | Suggests |
|--------|----------|
| "fix typo", "bump version" | trivial |
| "add field", "fix bug #123" | small |
| "add feature", "refactor X" | medium |
| "redesign", "integrate with" | large |
| "new service", "from scratch" | greenfield |
| Database/schema changes | +1 level |
| Breaking changes | +1 level |
| Security implications | +1 level |

## Instructions

1. Analyze the task description
2. Consider each criterion
3. **When in doubt, round UP** - better to over-prepare
4. Output your classification

## Output Format

Provide brief reasoning, then output:

```
### Analysis
- Scope: [narrow/moderate/wide]
- Uncertainty: [low/medium/high]
- Risk: [low/medium/high]
- Dependencies: [none/internal/external]

### Classification
Weight: [trivial|small|medium|large|greenfield]
Confidence: [0.0-1.0]
Rationale: [1-2 sentences]

<weight>VALUE</weight>
```

Replace VALUE with exactly one of: trivial, small, medium, large, greenfield
