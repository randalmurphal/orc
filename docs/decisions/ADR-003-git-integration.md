# ADR-003: Git Integration

**Status**: Accepted  
**Date**: 2026-01-10

---

## Context

Orc needs robust checkpointing, rewinding, forking, and task isolation.

## Decision

**Git IS the checkpoint system.**

- **Branches** for task isolation: `orc/TASK-ID`
- **Commits** for checkpoints: `[orc] TASK-ID: phase - message`
- **Worktrees** for parallel execution
- **Tags** for significant milestones

## Rationale

Every checkpoint feature maps to a git operation:

| Orc Concept | Git Implementation |
|-------------|-------------------|
| Create checkpoint | `git commit` |
| Rewind to checkpoint | `git reset --hard {commit}` |
| Fork from checkpoint | `git checkout -b {branch} {commit}` |
| View checkpoint history | `git log --oneline` |
| Compare checkpoints | `git diff {commit1} {commit2}` |
| Parallel task execution | `git worktree add` |

### Branch Strategy

```
main
├── orc/TASK-001           # Task branch
│   ├── commit: "checkpoint: classify complete"
│   ├── commit: "checkpoint: plan generated"
│   └── commit: "checkpoint: implement phase 1"
├── orc/TASK-002           # Another task
└── orc/TASK-001/fork-1    # Fork for alternative approach
```

### Worktree Strategy

```
.orc/worktrees/
├── TASK-001/        # Isolated working directory
└── TASK-002/        # Another task's worktree
```

Benefits:
- Each task has isolated working directory
- No `git stash` dance when switching tasks
- Claude processes can't interfere with each other

## Consequences

**Positive**:
- Familiar tooling (developers know git)
- Powerful operations: rebase, cherry-pick, bisect
- Integration with GitHub/GitLab, PRs, CI

**Negative**:
- Git knowledge required
- History pollution from checkpoint commits

**Mitigation**: Squash on merge; `orc/` branch prefix prevents collision.
