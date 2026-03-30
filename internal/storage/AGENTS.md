# Storage

The storage package is the project-scoped backend abstraction used by higher layers.

## Owns

- backend interfaces used by API and executor
- task, initiative, transcript, event, and workflow persistence composition
- proto-to-DB conversion at the backend boundary

## Rules

- Keep storage methods project-scoped.
- Prefer central backend methods over direct DB access from higher layers.
- `orcv1.Task` is the main task domain object above the raw DB layer.

## Verification

```bash
go test ./internal/storage/...
```
