# docs - Iteration 1

## Prompt

Update documentation for:

**Task**: Bug: Task worktree created from stale base causes completion conflicts

**Description**: When parallel tasks run and the first one merges to main, subsequent tasks' worktrees are based on a stale commit. During completion sync, this causes merge conflicts that aren't automatically resolved.

**Reproduction:**
1. Run TASK-A and TASK-B in parallel (both modify CLAUDE.md)
2. TASK-A completes and merges first
3. TASK-B completes but conflicts on CLAUDE.md

**Root cause:** Worktree is created at task start from current main, but by completion time main has moved forward.

**Fix options:**
1. Rebase task branch onto main BEFORE the implement phase starts
2. Add conflict resolution phase that uses AI to resolve conflicts
3. Auto-rebase during completion sync before attempting merge
4. Create worktree just-in-time when task runs (not when task is created)

1. Update any relevant documentation files
2. Ensure CLAUDE.md reflects the changes if applicable
3. Add/update code comments where needed
4. Update README if user-facing changes were made

Keep iterating until documentation is complete.

When done, output:
<phase_complete>true</phase_complete>


## Response

The warning is expected since I'm working in the TASK-194 worktree (the task-specific hooks are being noisy). Documentation is complete.

<phase_complete>true</phase_complete>

---
Tokens: 1556077 input, 4917 output, 58273 cache_creation, 1497777 cache_read
Complete: true
Blocked: false
