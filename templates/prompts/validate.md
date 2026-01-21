# Validate Phase

You are performing final validation before merge using comprehensive E2E testing.

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

## Specification

{{SPEC_CONTENT}}

## Implementation Summary

{{IMPLEMENTATION_SUMMARY}}

## Test Results

{{TEST_RESULTS}}

## Instructions

### Step 1: Verify All Success Criteria

Go through each success criterion from the spec:
- [ ] Manually verify it works as specified
- [ ] Check edge cases are handled
- [ ] Verify error messages are helpful

### Step 2: E2E Validation with Playwright MCP

{{REQUIRES_UI_TESTING}}

For any UI components, use Playwright MCP tools for full end-to-end testing.

**Screenshot Directory**: `{{SCREENSHOT_DIR}}`
Save all validation screenshots here for automatic attachment to the task.

#### Available Tools

| Tool | Purpose |
|------|---------|
| `browser_navigate` | Load pages/routes |
| `browser_snapshot` | Capture accessibility tree (preferred over screenshot) |
| `browser_click` | Click buttons, links |
| `browser_fill_form` | Fill form fields |
| `browser_type` | Type into inputs |
| `browser_take_screenshot` | Visual verification - save to `{{SCREENSHOT_DIR}}` |
| `browser_console_messages` | Check for JS errors |
| `browser_network_requests` | Verify API calls |

#### Validation Workflow

For each UI component:

1. **Navigate**: `browser_navigate` to the component
2. **Snapshot**: `browser_snapshot` to verify accessibility
3. **Interact**: Test all interactions (click, type, submit)
4. **Verify State**: `browser_snapshot` after each action
5. **Capture Visual**: `browser_take_screenshot` with filename `{{SCREENSHOT_DIR}}/validate-{component}-{state}.png`
6. **Check Errors**: `browser_console_messages` for JS errors
7. **Verify API**: `browser_network_requests` for failed calls

#### Screenshot Naming for Validation

Save screenshots with descriptive names to `{{SCREENSHOT_DIR}}`:
- `validate-{component}-before.png` - State before validation
- `validate-{component}-after.png` - State after successful validation
- `validate-{component}-error.png` - Any error states found

### Step 3: Run Full Test Suite

```bash
# Run all tests one final time
go test ./... -v

# Or for other languages
npm test
pytest -v
```

### Step 4: Verify Build and Linting

```bash
# Build verification
go build ./...

# Linting verification (REQUIRED)
golangci-lint run ./...

# If golangci-lint not available, at minimum:
go vet ./...

# For Node/TypeScript projects - run type checking AND linting
npm run build       # Includes tsc for type checking
npm run typecheck   # Explicit type check (tsc --noEmit)
npm run lint        # ESLint for code quality

# For Python projects
python -m py_compile $(find . -name "*.py" -not -path "./.venv/*")
ruff check .
pyright .  # Optional but recommended for type checking
```

**IMPORTANT**: Both build AND linting must pass before proceeding.

If linting fails:
1. Fix all linting errors
2. For Go errcheck issues: use `_ = functionCall()` to explicitly ignore when safe
3. For Go deferred Close(): wrap as `defer func() { _ = x.Close() }()`
4. For TypeScript unused vars: rename with underscore prefix (e.g., `_unusedVar`)
5. Re-run linter until clean

### Step 5: Sync with Target Branch

Before completion, ensure the branch is synced with the target:

```bash
# Fetch latest from remote
git fetch origin main

# Rebase onto target branch
git rebase origin/main
```

If there are conflicts:
1. Resolve each conflict by understanding both changes
2. Preserve the intent of your task AND upstream changes
3. If your changes are now obsolete (upstream did the same thing), remove yours
4. After resolving: `git add -A && git rebase --continue`
5. Re-run all tests after rebase to ensure nothing broke

### Step 6: Final Checklist

- [ ] All success criteria verified
- [ ] All tests passing
- [ ] Build succeeds
- [ ] **Linting passes** (golangci-lint/npm lint/ruff - 0 errors)
- [ ] No console errors (for UI)
- [ ] No failed network requests (for UI)
- [ ] Documentation updated (if needed)
- [ ] No TODO comments in new code
- [ ] Branch synced with target (rebased onto origin/main)
- [ ] All review findings addressed (if review phase ran)
- [ ] Commits are well-structured and meaningful

## Output Format

### Validation Report

```markdown
# Validation Report: {{TASK_ID}}

## Success Criteria Verification

| Criterion | Status | Notes |
|-----------|--------|-------|
| [Criterion 1] | ✓ PASS | [notes] |
| [Criterion 2] | ✓ PASS | [notes] |

## E2E Test Results (if UI)

| Component | Tests | Result |
|-----------|-------|--------|
| [Component 1] | [count] | ✓ PASS |
| [Component 2] | [count] | ✓ PASS |

## Screenshots Captured

| Screenshot | Description |
|------------|-------------|
| validate-{component}-before.png | Initial state |
| validate-{component}-after.png | Final validated state |

## Final Checks

- [x] All tests passing
- [x] Build succeeds
- [x] Linting passes
- [x] No console errors
- [x] No failed network requests

## Ready for Merge: YES
```

## Phase Completion

Output JSON to signal completion:

```json
{"status": "complete", "summary": "Validated [count]/[total] criteria, E2E tests passed, build clean, linting passed"}
```

If validation fails:
```json
{"status": "blocked", "reason": "[what failed validation and specific fixes required]"}
```
