# Validate Phase

You are performing final validation before merge using comprehensive E2E testing.

## Context

**Task ID**: ${TASK_ID}
**Task**: ${TASK_TITLE}
**Weight**: ${WEIGHT}

## Specification

${SPEC_CONTENT}

## Implementation Summary

${IMPLEMENTATION_SUMMARY}

## Test Results

${TEST_RESULTS}

## Instructions

### Step 1: Verify All Success Criteria

Go through each success criterion from the spec:
- [ ] Manually verify it works as specified
- [ ] Check edge cases are handled
- [ ] Verify error messages are helpful

### Step 2: E2E Validation with Playwright MCP

For any UI components, use Playwright MCP tools for full end-to-end testing:

#### Available Tools

| Tool | Purpose |
|------|---------|
| `browser_navigate` | Load pages/routes |
| `browser_snapshot` | Capture accessibility tree (preferred over screenshot) |
| `browser_click` | Click buttons, links |
| `browser_fill_form` | Fill form fields |
| `browser_type` | Type into inputs |
| `browser_take_screenshot` | Visual verification |
| `browser_console_messages` | Check for JS errors |
| `browser_network_requests` | Verify API calls |

#### Validation Workflow

For each UI component:

1. **Navigate**: `browser_navigate` to the component
2. **Snapshot**: `browser_snapshot` to verify accessibility
3. **Interact**: Test all interactions (click, type, submit)
4. **Verify State**: `browser_snapshot` after each action
5. **Check Errors**: `browser_console_messages` for JS errors
6. **Verify API**: `browser_network_requests` for failed calls

### Step 3: Run Full Test Suite

```bash
# Run all tests one final time
go test ./... -v

# Or for other languages
npm test
pytest -v
```

### Step 4: Verify Build

```bash
go build ./...
# Ensure clean build with no warnings
```

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
# Validation Report: ${TASK_ID}

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

## Final Checks

- [x] All tests passing
- [x] Build succeeds
- [x] No console errors
- [x] No failed network requests

## Ready for Merge: YES
```

## Phase Completion

### Commit Validation Report

Save report to `.orc/tasks/${TASK_ID}/artifacts/validation-report.md`:

```bash
git add -A
git commit -m "[orc] ${TASK_ID}: validate - completed

Phase: validate
Status: completed
Artifact: artifacts/validation-report.md
Ready for merge: YES
"
```

### Output Completion

```
### Validation Summary

**Criteria Verified**: [count]/[total]
**E2E Tests**: [count] passed (if applicable)
**Build**: Clean
**Commit**: [commit SHA]

<phase_complete>true</phase_complete>
```

If validation fails:
```
<phase_blocked>
reason: [what failed validation]
needs: [specific fixes required]
</phase_blocked>
```
