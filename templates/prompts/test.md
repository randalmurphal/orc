# Test Phase

You are writing and running tests to verify the implementation.

## Context

**Task ID**: {{TASK_ID}}
**Task**: {{TASK_TITLE}}
**Weight**: {{WEIGHT}}

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

Target: >80% coverage on new code

If coverage is low:
- Identify untested paths
- Add tests for uncovered code
- Focus on error paths (often missed)

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
**Coverage**: [percent]%

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

### Commit Tests

```bash
git add -A
git commit -m "[orc] {{TASK_ID}}: test - completed

Phase: test
Status: completed
Tests: [count] passed
Coverage: [percent]%
"
```

### Output Completion

```
### Test Summary

**Tests**: [passed]/[total] passing
**Coverage**: [percent]%
**Commit**: [commit SHA]

<phase_complete>true</phase_complete>
```

If tests fail:
```
<phase_blocked>
reason: [count] tests failing
needs: [specific failures to fix]
</phase_blocked>
```
