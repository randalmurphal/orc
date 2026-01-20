# Dead Code Cleanup

You are performing automated dead code detection and removal to keep the codebase clean and maintainable.

## Objective

Find and remove unused code including unused functions, variables, exports, imports, and files.

## Context

**Project Root:** {{PROJECT_ROOT}}

## Process

1. **Detect Unused Code**
   Use available tools to identify:
   - Unused exported functions and types
   - Unused variables and constants
   - Unused imports
   - Orphaned files (not imported anywhere)
   - Commented-out code blocks
   - Unreachable code paths

2. **Verify Before Removal**
   For each candidate:
   - Confirm no dynamic references (reflection, string-based imports)
   - Check for external API consumers
   - Verify not used in tests that should exist
   - Ensure not part of public API contract

3. **Remove Dead Code**
   - Remove unused code in order (imports, variables, functions, files)
   - Update any affected imports
   - Clean up empty files/modules
   - Remove orphaned test files

4. **Verify No Breakage**
   - Run all tests
   - Check for import errors
   - Verify build succeeds

## Output Format

When complete, output ONLY this JSON:

```json
{"status": "complete", "summary": "Dead code cleanup complete: [count] items removed"}
```

If uncertain about removal safety, output ONLY this JSON:

```json
{"status": "blocked", "reason": "[description of uncertain cases]"}
```

## Guidelines

- Never remove code that might be dynamically accessed
- Preserve intentionally unused code (future features) if marked
- Check git blame for context on why code exists
- Remove commented code unless it contains important context
- Be conservative - only remove clearly dead code
