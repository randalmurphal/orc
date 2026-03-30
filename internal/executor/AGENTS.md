# Executor

The executor package is the orchestration core. It runs workflows, phases, gates, retries, transcript capture, and completion/finalize behavior.

## Owns

- workflow execution loop
- phase dispatch and phase-type executors
- run and phase state transitions
- transcript persistence inputs and session metadata persistence
- gate integration, retry behavior, and completion/finalize flow

## Does Not Own

- provider-native runtime preparation
- provider-native validation
- harness-specific filesystem/config mutation

Those belong in `llmkit`.

## Runtime Boundary

- Orc supports only `claude` and `codex`.
- Orc passes `runtime_config` to llmkit.
- llmkit owns provider definitions, runtime preparation, session semantics, and stream normalization.
- Do not add provider hacks in executor code to patch around missing llmkit behavior.

## Rules

- Keep execution-critical failures loud:
  - runtime config parse/validation
  - session metadata parse
  - transcript row persistence
  - task/run state persistence
- Keep provider logic thin and local.
- Prefer decomposition over growing `workflow_executor.go`-style god files again.
- If a runtime config field exists, make sure it survives parsing, merging, validation, and execution.

## When Changing This Package

- Check the full path:
  - workflow/template config
  - runtime preparation
  - phase execution
  - transcript/session persistence
  - task/run status updates
- For session changes, verify start, resume, interruption, and retry behavior.
- For runtime changes, verify both Claude and Codex paths if they share the codepath.

## Verification

```bash
go test ./internal/executor/...
go test ./internal/task/...
```
