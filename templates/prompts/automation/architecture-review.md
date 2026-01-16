# Architecture Review

You are performing an architecture review after a significant body of work to identify drift and consistency issues.

## Objective

Review the codebase for architectural consistency, identify drift from established patterns, and fix deviations.

## Context

**Initiative Completed:** {{INITIATIVE_ID}}

**Tasks in Initiative:** {{INITIATIVE_TASKS}}

## Process

1. **Pattern Inventory**
   Review CLAUDE.md and identify:
   - Established architectural patterns
   - Module organization conventions
   - API design standards
   - Error handling approaches
   - State management patterns

2. **Drift Detection**
   For code added in recent tasks:
   - Compare against established patterns
   - Identify deviations and inconsistencies
   - Note where new patterns were introduced
   - Check for copy-paste with modifications

3. **Categorize Issues**
   - **Drift**: Code that should follow existing pattern but doesn't
   - **Evolution**: New pattern that's better than existing
   - **Inconsistency**: Same thing done multiple ways
   - **Technical Debt**: Shortcuts that need cleanup

4. **Resolution**
   For drift and inconsistency:
   - Align with dominant pattern
   - Update to match established conventions
   - Fix subtle variations

   For evolution (better new patterns):
   - Document in CLAUDE.md
   - Create follow-up task to normalize old code

5. **Verify Integrity**
   - Run all tests
   - Check for import cycles
   - Verify no unintended dependencies added

## Output Format

When complete, output:

```xml
<phase_complete>true</phase_complete>
```

If architectural changes require discussion, output:

```xml
<phase_blocked>reason: [description of architectural decision needed]</phase_blocked>
```

## Guidelines

- Focus on structural consistency, not style
- Don't force patterns that don't fit
- Document new patterns that should be adopted
- Keep changes minimal and focused
- Suggest follow-up tasks for large refactors
