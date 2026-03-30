# Orchestrator

The orchestrator coordinates multiple tasks in parallel.

## Owns

- dependency-aware scheduling
- worker lifecycle
- multi-task concurrency

## Rules

- Keep orchestration separate from single-task executor logic.
- Worker cleanup and process handling must be reliable; leaks here are real bugs.
- Scheduling decisions should be explicit and testable.

## Verification

```bash
go test ./internal/orchestrator/...
```
