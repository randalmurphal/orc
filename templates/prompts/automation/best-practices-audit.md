# Best Practices Audit

You are performing an automated best practices audit to ensure code follows language-specific idioms and project conventions.

## Objective

Review code for best practices violations and fix issues that don't require architectural changes.

## Context

**Recent Tasks Completed:** {{RECENT_COMPLETED_TASKS}}

**Files Changed:** {{RECENT_CHANGED_FILES}}

## Process

1. **Error Handling Patterns**
   Check for:
   - Proper error wrapping with context
   - No swallowed errors (catch blocks that do nothing)
   - Consistent error message format
   - Appropriate error types used
   - Error recovery where possible

2. **Naming Conventions**
   Verify:
   - Consistent naming style (camelCase, snake_case, etc.)
   - Descriptive names for functions and variables
   - No ambiguous abbreviations
   - Consistent terminology across codebase

3. **Code Organization**
   Check:
   - Related code grouped together
   - Clear separation of concerns
   - No god functions (doing too much)
   - Appropriate abstraction level

4. **Anti-Patterns**
   Look for and fix:
   - Deep nesting (> 4 levels)
   - Magic numbers without constants
   - Duplicated code that should be extracted
   - Long parameter lists
   - Tight coupling between modules

5. **Documentation**
   Ensure:
   - Public APIs have documentation
   - Complex logic has explanatory comments
   - No outdated comments

## Output Format

When complete, output ONLY this JSON:

```json
{"status": "complete", "summary": "Best practices audit complete: [summary of findings/fixes]"}
```

If significant issues require human decision, output ONLY this JSON:

```json
{"status": "blocked", "reason": "[description of issues needing review]"}
```

## Guidelines

- Fix only clear violations, not style preferences
- Don't change working code for minor style issues
- Preserve existing behavior exactly
- Document any changes that affect public APIs
- Run tests after each file modification
