# ADR-002: Storage Model

**Status**: Accepted  
**Date**: 2026-01-10

---

## Context

Orc needs to persist: task definitions, execution state, transcripts, artifacts, and configuration.

**Key Requirements**:
1. Version control trackable/rewindable
2. Collaboration via git
3. No external services to set up
4. Human-inspectable state

## Decision

**Primary Storage**: File-based storage in `.orc/` directory, tracked by git.

SQLite can be added later ONLY if search/query needs arise.

## Rationale

### Git-Native is the Killer Feature

Git already provides everything we need:

| Orc Need | Git Solution |
|----------|--------------|
| History | `git log` shows all state changes |
| Rewind | `git checkout` restores any previous state |
| Branching | `git branch` enables parallel task exploration |
| Diffing | `git diff` shows exactly what changed |
| Collaboration | `git push/pull` shares state |

### Storage Structure

```
.orc/
├── config.yaml              # Project configuration
├── tasks/
│   └── {task-id}/
│       ├── task.yaml        # Task definition
│       ├── state.yaml       # Current execution state
│       ├── plan.yaml        # Generated plan
│       └── transcripts/     # Claude session logs
├── prompts/                 # Prompt templates (overrides)
└── templates/plans/         # Plan templates by weight
```

### Why YAML Over JSON

- Human-readable and editable
- Comments allowed (documentation inline)
- Multi-line strings without escaping (prompts!)
- Git diffs are cleaner

## Consequences

**Positive**:
- Zero setup: clone repo, run orc, done
- Full history: every state change is a git commit
- Inspectable: `cat .orc/tasks/abc123/state.yaml`
- Transparent: no magic, files are the truth

**Negative**:
- Query limitations (no `SELECT * FROM tasks WHERE status='failed'`)
- Scale concerns for thousands of tasks

**Mitigation**: Build in-memory index on startup; add SQLite for indexing only if needed.
