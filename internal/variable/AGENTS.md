# Variable

The variable package is the source of truth for workflow variable resolution.

## Owns

- variable definitions and source types
- resolution of built-in and configured variables
- interpolation, extraction, and script/API-backed sources

## Rules

- Do not add ad hoc variable interpolation elsewhere.
- New source types must be fully wired through definition, resolution, interpolation, and tests.
- Resolution errors should be explicit; missing variables should not degrade silently.

## Verification

```bash
go test ./internal/variable/...
```
