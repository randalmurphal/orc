# Brief

The brief package produces a compact project brief from task history.

## Owns

- extracting durable signals from completed work
- formatting the brief
- token budgeting and caching

## Rules

- The brief should be derived state, not an alternate source of truth.
- Keep generation deterministic for the same input set.
- Budgeting is part of the feature, not an afterthought.

## Verification

```bash
go test ./internal/brief/...
```
