# ADR-002: Storage Model

**Status**: Superseded by Pure SQL Storage (2026-01)
**Date**: 2026-01-10 (original), 2026-01 (superseded)

---

## Original Decision (Superseded)

The original design used YAML files as the source of truth with SQLite as an optional cache/index. This was superseded by pure SQL storage in January 2026.

## Current Decision

**Primary Storage**: SQLite database (`.orc/orc.db`) is the sole source of truth.

No YAML files are created for tasks, states, plans, or initiatives. Configuration (`config.yaml`) and prompts remain as files.

## Rationale for Change

### Why We Moved Away from YAML

| Issue | Problem | Solution |
|-------|---------|----------|
| Dual writes | Every operation wrote YAML then DB, causing sync bugs | Single DB write |
| Git noise | Auto-commits for every state change cluttered history | No auto-commits for task state |
| Conflict resolution | Merge conflicts in YAML files during parallel work | CR-SQLite for P2P sync |
| Query performance | Filesystem scanning for task lists was slow | SQL queries |
| Consistency | YAML/DB could diverge, requiring rebuild logic | Single source of truth |

### New Storage Structure

```
.orc/
├── orc.db                   # SQLite database (source of truth)
├── config.yaml              # Project configuration (file)
└── prompts/                 # Prompt templates (files)
```

### Database Tables

All task, state, plan, and initiative data stored in SQLite:
- `tasks` - Task definitions and execution state
- `phases` - Phase execution records
- `plans` - Phase sequences (JSON)
- `specs` - Task specifications
- `initiatives` - Initiative groupings
- `transcripts` - Claude session logs
- `attachments` - Task file attachments (BLOB)

### Sync Strategy

P2P sync via CR-SQLite extension replaces git-based collaboration for task data.
Configuration and prompts remain git-tracked.

## Consequences

**Positive**:
- Single source of truth eliminates sync bugs
- Fast queries for all operations
- P2P sync without git noise
- Simpler codebase (removed YAML I/O, file watchers, commit logic)

**Negative**:
- Less human-inspectable (use `orc status`, `orc show` instead of `cat`)
- Database corruption requires backup recovery

**Mitigation**: Regular database backups; `orc export --all-tasks --all` for full portable backup to `.orc/exports/`.
