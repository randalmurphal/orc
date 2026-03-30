# Progress

The progress package formats live CLI progress for task execution.

## Owns

- human-facing execution progress display
- activity and state presentation for terminal sessions

## Rules

- Keep this package presentation-only.
- Do not hide execution failures behind display logic.
- Stable, readable output matters more than clever formatting.

## Verification

```bash
go test ./internal/progress/...
```
