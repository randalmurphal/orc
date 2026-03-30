# Internal Packages

`internal/` holds the Go implementation. Read the guide for the package you are changing.

## Package Ownership

- `api`: transport layer, request validation, project routing, proto conversion
- `automation`: trigger-driven maintenance and task creation
- `bootstrap`: `orc init` bootstrap and project registration
- `brief`: generated project brief extraction, formatting, caching
- `cli`: Cobra command layer and terminal UX
- `config`: config loading, defaults, validation, provider list exposed by orc
- `db`: SQL layer, migrations, GlobalDB and ProjectDB persistence
- `events`: event types and event delivery/persistence helpers
- `executor`: workflow execution, retries, gates, transcripts, completion handling
- `gate`: gate evaluation and gate policy helpers
- `initiative`: initiative model and criteria
- `knowledge`: knowledge service orchestration and query pipeline
- `orchestrator`: multi-task parallel execution
- `progress`: CLI progress display
- `project`: project registry and path resolution
- `setup`: interactive setup flow
- `storage`: backend abstraction over project persistence
- `task`: task execution state helpers
- `trigger`: before-phase and lifecycle trigger evaluation
- `variable`: workflow variable resolution
- `workflow`: workflow definitions, seeding, resolution, serialization

## Cross-Package Rules

- Keep SQL and migrations in `db`, not in `api`, `storage`, or `executor`.
- Keep orchestration policy in `executor`, not in `api` or `workflow`.
- Keep provider and harness behavior in `llmkit`, not in `workflow` or `executor`.
- Keep transport/proto shaping in `api`, not in domain packages.
- Keep reusable persistence calls in `storage`; do not scatter direct DB usage upward without a reason.

## Invariants

- Orc supports only `claude` and `codex`.
- Runtime setup uses `runtime_config`.
- Task and phase execution state must stay consistent with persisted transcript and run state.
- Schema and generated code must move together when proto or DB fields change.

## Verification

- Run package-local tests first when iterating.
- Run `go test ./...` before closing multi-package work.
- If you changed proto or web-facing contracts, also run `pnpm -C web exec tsc --noEmit`.
