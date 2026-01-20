# Complexity Reduction

You are performing automated complexity reduction to improve code maintainability.

## Objective

Identify and refactor high-complexity code without changing functionality.

## Context

**Recent Files Changed:** {{RECENT_CHANGED_FILES}}

## Process

1. **Identify Complex Code**
   Look for:
   - Functions with cyclomatic complexity > 15
   - Deeply nested conditionals (> 4 levels)
   - Long functions (> 50 lines of logic)
   - Complex boolean expressions
   - Multiple levels of callbacks/promises

2. **Prioritize Targets**
   Focus on:
   - Frequently modified code (higher impact)
   - Critical business logic paths
   - Code with multiple contributors
   - Recently added complexity

3. **Refactoring Strategies**
   Apply as appropriate:
   - Extract helper functions
   - Replace nested conditionals with early returns
   - Convert complex conditionals to switch/polymorphism
   - Split large functions by responsibility
   - Flatten callback chains (async/await)
   - Introduce guard clauses

4. **Verify Behavior**
   - Run all tests before and after
   - Ensure identical behavior
   - Check performance isn't degraded

## Output Format

When complete, output ONLY this JSON:

```json
{"status": "complete", "summary": "Complexity reduction complete: [summary of refactors]"}
```

If refactoring would require breaking changes, output ONLY this JSON:

```json
{"status": "blocked", "reason": "[description of required changes]"}
```

## Guidelines

- Only refactor, never change behavior
- Preserve all existing tests
- One refactoring pattern at a time
- Document non-obvious transformations
- Run tests after each file change
- Don't introduce new dependencies
