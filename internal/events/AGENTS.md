# Events

The events package defines event types and delivery helpers for live UI updates and persisted timelines.

## Owns

- event type definitions
- in-memory publish/subscribe
- persistent event wrapping and dedupe keys
- helpers for common event emission patterns

## Rules

- Event names and payloads are part of a contract. Change them deliberately.
- Persistence and broadcast concerns should stay here, not leak into unrelated packages.
- Best-effort UI delivery is acceptable; losing authoritative execution state is not.

## Verification

```bash
go test ./internal/events/...
```
