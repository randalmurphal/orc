# Gate

The gate package evaluates whether a phase may continue.

## Owns

- gate types and resolution
- auto/human/AI gate evaluation
- pending decision tracking
- script-based gate handling

## Rules

- Gate behavior is policy. Keep it deterministic and explicit.
- AI gate execution must go through the shared schema-constrained LLM path.
- Keep dependency interfaces small to avoid import cycles.

## Verification

```bash
go test ./internal/gate/...
```
