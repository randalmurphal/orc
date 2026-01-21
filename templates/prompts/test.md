# Test Phase

You are writing and running tests to verify the implementation.

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

## Specification

{{SPEC_CONTENT}}

## Implementation Summary

{{IMPLEMENTATION_SUMMARY}}

## Instructions

### Step 1: Identify Test Cases

For each success criterion in the spec:
- Create at least one test case
- Include positive and negative cases
- Cover edge cases mentioned in spec

### Step 2: Write Unit Tests

For each component:
- Test public functions/methods
- Test error handling paths
- Test boundary conditions
- Mock external dependencies

### Step 3: Write Integration Tests

Test component interactions:
- API endpoints with real handlers
- Database operations (use test DB)
- External service calls (use mocks)

### Step 3.5: UI Testing with Playwright (if applicable)

{{REQUIRES_UI_TESTING}}

If this task involves UI changes (`{{REQUIRES_UI_TESTING}}` is set), use Playwright MCP tools for E2E testing:

#### Playwright MCP Tools Available

| Tool | Purpose |
|------|---------|
| `browser_navigate` | Navigate to a URL |
| `browser_snapshot` | Capture accessibility tree (preferred for state verification) |
| `browser_click` | Click elements by ref from snapshot |
| `browser_type` | Type text into inputs |
| `browser_fill_form` | Fill multiple form fields |
| `browser_take_screenshot` | Capture visual screenshot |
| `browser_console_messages` | Check for JavaScript errors |
| `browser_network_requests` | Verify API calls |

#### E2E Test Workflow

1. **Start the dev server** (if not running):
   ```bash
   # Example for Node/Vite projects
   npm run dev &
   # Wait for server to be ready
   sleep 3
   ```

2. **Navigate to the component**:
   ```
   browser_navigate to http://localhost:5173/path
   ```

3. **Verify initial state**:
   ```
   browser_snapshot to see accessibility tree
   ```

4. **Test interactions**:
   - Use `browser_click` with refs from snapshot
   - Use `browser_type` for text input
   - Use `browser_fill_form` for forms

5. **Capture screenshots for verification**:
   ```
   browser_take_screenshot with filename: "{{SCREENSHOT_DIR}}/test-{component}-{state}.png"
   ```
   **IMPORTANT**: Save screenshots to `{{SCREENSHOT_DIR}}` for automatic attachment to the task.

6. **Check for errors**:
   ```
   browser_console_messages to verify no JS errors
   browser_network_requests to verify no failed API calls
   ```

#### Screenshot Naming Convention

Save screenshots with descriptive names:
- `{component}-initial.png` - Initial state before interaction
- `{component}-{action}-result.png` - State after action
- `{component}-error.png` - Error state (if testing error handling)

### Step 4: Run Tests

```bash
# For Go projects
go test ./... -v -cover

# For Node projects
npm test

# For Python projects
pytest -v --cov
```

### Step 5: Verify Coverage

**REQUIRED**: Minimum {{COVERAGE_THRESHOLD}}% coverage on new code

Coverage is **mandatory** - the phase cannot complete until this threshold is met.

If coverage is below {{COVERAGE_THRESHOLD}}%:
1. Run coverage report to identify untested paths
2. Add tests for uncovered functions and branches
3. Focus on error handling paths (often missed)
4. Re-run coverage until threshold is met

Do NOT mark the phase complete until coverage reaches {{COVERAGE_THRESHOLD}}%.

### Step 6: Run Linters

**REQUIRED**: Code must pass linting before phase completion.

```bash
# For Go projects - run full linter suite
golangci-lint run ./...

# If golangci-lint not available, at minimum:
go vet ./...

# For Node/TypeScript projects - run BOTH type checking and linting
npm run typecheck  # Type checking (tsc --noEmit)
npm run lint       # ESLint for code quality

# For Python projects
ruff check .
pyright .  # Optional but recommended for type checking
# or: pylint $(find . -name "*.py" -not -path "./.venv/*")
```

**Common Go linting issues to watch for:**
- Unchecked error returns (errcheck)
- Unused variables and imports
- Ineffective assignments
- Deferred calls in loops

**Common TypeScript/Node linting issues to watch for:**
- Unused variables: rename to `_varName` or remove
- Unused catch errors: rename `catch (e)` to `catch (_e)` if error is not used
- Type errors: fix types, avoid `any` where possible
- React hooks violations: follow Rules of Hooks
- Missing return types on exported functions

If linting fails:
1. Fix all linting errors (not just warnings)
2. For Go errcheck issues: use `_ = functionCall()` to explicitly ignore errors only when truly safe
3. For Go deferred Close(): wrap as `defer func() { _ = x.Close() }()`
4. For TypeScript unused vars: rename with underscore prefix (e.g., `_unusedVar`)
5. Re-run linter until clean

Do NOT mark the phase complete until linting passes.

## Test Patterns

| Pattern | Use Case |
|---------|----------|
| Table-driven tests | Multiple inputs/outputs |
| Subtests | Grouped related cases |
| Fixtures | Shared setup/teardown |
| Mocks | External dependencies |

## Output Format

### Test Summary

```
### Test Results

**Total Tests**: [count]
**Passed**: [count]
**Failed**: [count]
**Coverage**: [percent]% (threshold: {{COVERAGE_THRESHOLD}}%)

### Tests Written
- [test_file1]: [count] tests
- [test_file2]: [count] tests

### Coverage by Package
- [package1]: [percent]%
- [package2]: [percent]%

### E2E Test Results (if UI testing)
- **Components Tested**: [count]
- **Interactions Verified**: [count]
- **Screenshots Captured**: [list of filenames]
- **Console Errors**: None / [count if any]
- **Failed Network Requests**: None / [count if any]
```

## Phase Completion

### Prerequisites

Before marking the phase complete, verify:
1. All tests pass (0 failures)
2. Coverage ≥ {{COVERAGE_THRESHOLD}}%
3. Linting passes (0 errors from golangci-lint/go vet/npm lint/ruff)

### Output Completion

Only signal completion when ALL THREE conditions are met:
- All tests pass
- Coverage ≥ {{COVERAGE_THRESHOLD}}%
- Linting passes (0 errors)

Then output ONLY this JSON to signal completion:

```json
{"status": "complete", "summary": "Tests: [passed]/[total] passing, Coverage: [percent]% (threshold: {{COVERAGE_THRESHOLD}}%), Linting: passed. Commit: [SHA]"}
```

If tests fail, output ONLY this JSON:
```json
{"status": "blocked", "reason": "[count] tests failing: [specific failures to fix]"}
```

If coverage is below threshold, output ONLY this JSON:
```json
{"status": "blocked", "reason": "Coverage [percent]% is below {{COVERAGE_THRESHOLD}}% threshold. Need tests for uncovered code paths."}
```

If linting fails, output ONLY this JSON:
```json
{"status": "blocked", "reason": "Linting errors found: [list specific issues to fix]"}
```
