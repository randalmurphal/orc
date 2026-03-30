# Initiative

Initiatives group related tasks and record shared decisions and acceptance criteria.

## Owns

- initiative domain model
- criteria and coverage
- manifests for bulk planning/import

## Rules

- Initiative data should sharpen task context, not duplicate task execution state.
- Criteria and coverage should stay explicit and testable.
- Keep initiative logic separate from workflow execution logic.

## Verification

```bash
go test ./internal/initiative/...
```
