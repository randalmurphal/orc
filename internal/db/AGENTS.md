# DB

The DB package owns SQL, migrations, and the raw persistence model for GlobalDB and ProjectDB.

## Owns

- schema files and migration ordering
- SQLite and PostgreSQL parity
- SQL queries and low-level persistence
- legacy one-time migration helpers that still need to run in-process

## Rules

- Keep SQLite and PostgreSQL schemas aligned.
- New persistent fields require:
  - schema migration
  - query updates
  - read/write mapping
  - tests
- Do not hide broken migrations behind fallback behavior.
- Prefer explicit migration logic over silently tolerating mismatched schemas.

## Current Architectural Constraints

- GlobalDB stores shared definitions and cross-project data.
- ProjectDB stores task execution data.
- `runtime_config` is the current execution config field; do not reintroduce `claude_config`.

## Verification

```bash
go test ./internal/db/...
```
