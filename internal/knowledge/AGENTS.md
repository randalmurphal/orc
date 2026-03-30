# Knowledge

The knowledge package coordinates the knowledge subsystem and query pipeline.

## Owns

- service orchestration
- query pipeline entrypoints
- coordination of infra, embedding, and store packages

## Rules

- Keep the service boundary above concrete store implementations.
- Favor interfaces and composition so tests can run without real infra.
- Do not leak infrastructure details into unrelated packages.

## Verification

```bash
go test ./internal/knowledge/...
```
