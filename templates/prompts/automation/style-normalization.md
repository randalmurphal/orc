# Style Normalization

You are performing automated code style normalization across recent changes in the codebase.

## Objective

Review and normalize code style, imports, and formatting across files modified in recent tasks to ensure consistency with established project patterns.

## Context

**Recent Tasks Completed:** {{RECENT_COMPLETED_TASKS}}

**Files Changed:** {{RECENT_CHANGED_FILES}}

## Process

1. **Identify Style Patterns**
   - Review CLAUDE.md for established patterns and conventions
   - Analyze the most common patterns in the codebase
   - Note language-specific idioms and project conventions

2. **Audit Recent Changes**
   - Check each recently modified file for style consistency
   - Compare against established patterns
   - Identify deviations that should be normalized

3. **Normalize Issues**
   For each file, check and fix:
   - Import organization and grouping
   - Naming conventions (variables, functions, types)
   - Code formatting and indentation
   - Comment style and documentation
   - Error handling patterns
   - Logging conventions

4. **Verify Changes**
   - Run linters and formatters
   - Ensure tests still pass
   - Verify no functional changes introduced

## Output Format

When complete, output:

```xml
<phase_complete>true</phase_complete>
```

If blocked (e.g., can't determine canonical style), output:

```xml
<phase_blocked>reason: [description of issue]</phase_blocked>
```

## Guidelines

- Make ONLY style changes - no functional modifications
- Preserve existing behavior exactly
- When in doubt, match the most common pattern in the codebase
- Document any ambiguous style decisions for human review
- Don't add comments or documentation beyond what's required for style
