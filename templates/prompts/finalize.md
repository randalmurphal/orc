# Finalize Phase

You are preparing the task branch for merge by syncing with the target branch and resolving any conflicts.

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
- Merging happens via PR after this phase completes
- Git hooks are active to prevent accidental protected branch modifications

## Specification

{{SPEC_CONTENT}}

## Implementation Summary

{{IMPLEMENTATION_SUMMARY}}

## Instructions

### Step 1: Fetch Latest Target Branch

```bash
git fetch origin {{TARGET_BRANCH}}
```

### Step 2: Check Divergence

Assess how much the target branch has diverged:

```bash
# Count commits behind target
git rev-list --count HEAD..origin/{{TARGET_BRANCH}}

# View what changed on target since we branched
git log --oneline HEAD..origin/{{TARGET_BRANCH}}
```

### Step 3: Merge Target Into Branch

**IMPORTANT**: Use merge (not rebase) to preserve commit history:

```bash
git merge origin/{{TARGET_BRANCH}} -m "Merge {{TARGET_BRANCH}} into {{TASK_BRANCH}} for finalization"
```

### Step 4: Conflict Resolution

If there are conflicts, follow these **CRITICAL RULES**:

#### Conflict Resolution Rules

| Rule | Description |
|------|-------------|
| **NEVER remove features** | Both your changes AND upstream changes must be preserved |
| **Merge intentions, not text** | Understand what each side was trying to accomplish |
| **Prefer additive resolution** | If in doubt, keep both implementations |
| **Test after every file** | Don't batch conflict resolutions |

#### Conflict Resolution Process

For each conflicted file:

1. **Understand Both Sides**
   - Read the full conflict context (not just the conflict markers)
   - Identify what YOUR changes were trying to accomplish
   - Identify what UPSTREAM changes were trying to accomplish

2. **Apply Conflict Resolution**
   ```
   <<<<<<< HEAD
   [Your changes - MUST preserve intent]
   =======
   [Upstream changes - MUST preserve intent]
   >>>>>>> origin/{{TARGET_BRANCH}}
   ```

3. **Resolution Strategy by Conflict Type**

   | Conflict Type | Resolution |
   |--------------|------------|
   | Same file, different functions | Keep both functions |
   | Same function, different changes | Merge both changes into function |
   | Import conflicts | Include all imports from both sides |
   | Configuration conflicts | Combine settings from both |
   | Test conflicts | Keep all tests from both sides |
   | Documentation conflicts | Merge documentation from both |

4. **PROHIBITED Resolutions**
   - **NEVER**: Just take "ours" or "theirs" without understanding
   - **NEVER**: Remove upstream features to fix conflicts
   - **NEVER**: Remove your features to fix conflicts
   - **NEVER**: Comment out conflicting code

5. **Mark Resolved and Test**
   ```bash
   git add [resolved-file]
   # Run relevant tests for this file BEFORE continuing
   ```

### Step 5: Run Full Test Suite

After ALL conflicts are resolved:

```bash
# For Go projects
go test ./... -v -race

# For Node projects
npm test

# For Python projects
pytest -v
```

**ALL TESTS MUST PASS** before proceeding.

If tests fail:
1. Identify which test fails and why
2. Check if the failure is due to conflict resolution
3. Fix the resolution (don't just fix the test)
4. Re-run full test suite

### Step 6: Verify Build and Linting

```bash
# For Go projects
go build ./...
go vet ./...
golangci-lint run ./...  # REQUIRED: Full linter suite including errcheck

# For Node projects
npm run build
npm run lint

# For Python projects
python -m py_compile $(find . -name "*.py" -not -path "./.venv/*")
ruff check .
```

**IMPORTANT**: Both build AND linting must pass. This is the last gate before merge.

If linting fails after conflict resolution:
1. Fix all linting errors introduced during merge
2. Particularly watch for errcheck issues in new/modified code
3. Re-run linter until clean

### Step 7: Risk Assessment

Assess merge risk based on changes:

```bash
# Count changed files
git diff --stat origin/{{TARGET_BRANCH}}...HEAD | tail -1

# Count lines changed
git diff --numstat origin/{{TARGET_BRANCH}}...HEAD | awk '{s+=$1+$2} END {print s}'
```

#### Risk Classification

| Files Changed | Lines Changed | Risk Level | Recommendation |
|---------------|---------------|------------|----------------|
| 1-5 | <100 | Low | Auto-merge safe |
| 6-15 | 100-500 | Medium | Review recommended |
| 16-30 | 500-1000 | High | Careful review required |
| >30 | >1000 | Critical | Senior review mandatory |

#### Conflict Impact

| Conflicts Resolved | Impact |
|-------------------|--------|
| 0 | Clean merge, minimal risk |
| 1-3 | Low impact, verify resolutions |
| 4-10 | Medium impact, test thoroughly |
| >10 | High impact, detailed review needed |

## Output Format

### Finalization Report

```markdown
# Finalization Report: {{TASK_ID}}

## Sync Summary

| Metric | Value |
|--------|-------|
| Target Branch | {{TARGET_BRANCH}} |
| Commits Behind (before sync) | [count] |
| Conflicts Resolved | [count] |
| Files Changed (total) | [count] |
| Lines Changed (total) | [count] |

## Conflict Resolution

| File | Conflict Type | Resolution | Verified |
|------|---------------|------------|----------|
| [file1] | [type] | [how resolved] | ✓ |
| [file2] | [type] | [how resolved] | ✓ |

## Test Results

| Suite | Result | Notes |
|-------|--------|-------|
| Unit Tests | ✓ PASS | [count] tests |
| Integration Tests | ✓ PASS | [count] tests |
| Build | ✓ PASS | No warnings |

## Risk Assessment

| Factor | Value | Risk |
|--------|-------|------|
| Files Changed | [count] | [Low/Medium/High] |
| Lines Changed | [count] | [Low/Medium/High] |
| Conflicts Resolved | [count] | [Low/Medium/High] |
| **Overall Risk** | | **[Low/Medium/High/Critical]** |

## Merge Decision

**Ready for Merge**: [YES/NO]
**Recommended Action**: [auto-merge/review-then-merge/senior-review-required]
```

## Phase Completion

### Commit Finalization

If any changes were made during conflict resolution:

```bash
git add -A
git commit -m "[orc] {{TASK_ID}}: finalize - completed

Phase: finalize
Status: completed
Conflicts resolved: [count]
Risk level: [level]
Ready for merge: YES
"
```

### Output Completion

```
### Finalization Summary

**Target Branch**: {{TARGET_BRANCH}}
**Conflicts Resolved**: [count]
**Tests**: All passing
**Risk Level**: [Low/Medium/High/Critical]
**Ready for Merge**: YES
**Commit**: [commit SHA]

<phase_complete>true</phase_complete>
```

If blocked:
```
<phase_blocked>
reason: [what failed - tests, unresolvable conflict, etc.]
needs: [specific action required]
</phase_blocked>
```
