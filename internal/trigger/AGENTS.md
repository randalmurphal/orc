# Trigger

The trigger package evaluates before-phase and lifecycle triggers.

## Owns

- trigger execution order
- gate vs reaction trigger modes
- trigger result shaping

## Rules

- Keep trigger semantics explicit.
- Before-phase triggers may block execution; lifecycle triggers should not silently change execution state unless designed to do so.
- Shared trigger execution belongs here, not in `api` or `workflow`.

## Verification

```bash
go test ./internal/trigger/...
```
