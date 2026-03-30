# Workflow

The workflow package owns workflow and phase-template definitions, seeding, resolution, and serialization.

## Owns

- workflow domain types
- built-in workflow and phase-template seeding
- YAML parsing and writing
- DB/domain conversion for workflow definitions

## Rules

- Keep workflow definitions declarative.
- If a field exists in YAML, DB, proto, or domain types, verify the full mapping path.
- `runtime_config` is the execution config field; do not reintroduce legacy naming.
- Provider support in workflow definitions must stay aligned with current orc support: `claude` and `codex`.

## Verification

```bash
go test ./internal/workflow/...
```
