# docs - Iteration 1

## Prompt

Update documentation for:

**Task**: Auto-merge append-only sections in CLAUDE.md during parallel task completion

**Description**: ## Problem
When running multiple tasks in parallel, they often conflict on CLAUDE.md's knowledge table. These conflicts are predictable - both tasks add new rows to the same table. Manual resolution is tedious and always follows the same pattern: keep both entries.

## Success Criteria
1. Detect when merge conflict is in an append-only section (knowledge table, patterns table)
2. Auto-resolve by keeping both additions in chronological order
3. Only auto-resolve for clearly append-only conflicts (not overlapping edits)
4. Log when auto-resolution is applied
5. Fall back to manual resolution for complex conflicts

## Implementation
- Add conflict detection in git sync logic (internal/git/)
- Parse CLAUDE.md to identify table sections
- If conflict is purely additive (both sides add rows), combine them
- Use marker comments (<!-- orc:knowledge:begin -->) to identify sections

## Edge Cases
- Conflicting edits to same row → manual resolution
- Conflicts outside marked sections → manual resolution
- Malformed tables → manual resolution

1. Update any relevant documentation files
2. Ensure CLAUDE.md reflects the changes if applicable
3. Add/update code comments where needed
4. Update README if user-facing changes were made

Keep iterating until documentation is complete.

When done, output:
<phase_complete>true</phase_complete>


## Response

The test file is comprehensive and well-documented. All documentation updates are complete. Let me summarize what was updated:

## Documentation Updates Summary

### 1. `docs/architecture/GIT_INTEGRATION.md`
Added a new section "CLAUDE.md Auto-Merge" that documents:
- How the feature works (4-step process: detection, analysis, resolution, fallback)
- Supported tables (Patterns Learned, Known Gotchas, Decisions)
- Resolution rules (what auto-merges vs requires manual resolution)
- Logging behavior
- Configuration notes

### 2. `docs/guides/TROUBLESHOOTING.md`
Added a new troubleshooting section "CLAUDE.md Merge Conflicts" covering:
- Auto-resolved successfully scenarios
- Auto-resolution failed scenarios and causes
- Manual resolution steps
- Best practices for preventing conflicts

### 3. `CLAUDE.md` (Knowledge Section)
Added a new pattern entry:
```
| CLAUDE.md auto-merge | During git sync, conflicts in knowledge section (within `orc:knowledge:begin/end` markers) are auto-resolved if purely additive (both sides add new table rows); rows combined and sorted by TASK-XXX source ID; complex conflicts (overlapping edits) fall back to manual resolution | TASK-096 |
```

### Code Comments
The implementation files (`internal/git/claudemd_merge.go` and `internal/git/git.go`) already have comprehensive documentation comments explaining:
- The purpose of each type and function
- The auto-resolution rules and constraints
- When manual resolution is required

The test file (`internal/git/claudemd_merge_test.go`) includes thorough test coverage demonstrating expected behavior.

<phase_complete>true</phase_complete>

---
Tokens: 1377290 input, 4365 output, 92070 cache_creation, 1268989 cache_read
Complete: true
Blocked: false
