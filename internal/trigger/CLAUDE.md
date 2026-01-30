# Trigger Package

Lifecycle event trigger evaluation for workflows. Shared component used by executor, CLI, and API.

## Key Types

| Type | File | Purpose |
|------|------|---------|
| `Runner` | `types.go:64` | Interface for trigger evaluation (executor depends on this) |
| `TriggerRunner` | `runner.go:16` | Implementation: evaluates triggers via `AgentExecutor` |
| `AgentExecutor` | `types.go:57` | Interface for invoking trigger agents |
| `TriggerResult` | `types.go:23` | Single agent result (Approved, Reason, Output) |
| `TriggerInput` | `types.go:31` | Context passed to trigger agent (TaskID, Phase, Event, Variables) |
| `BeforePhaseTriggerResult` | `types.go:40` | Aggregate result: Blocked, BlockedReason, UpdatedVars |
| `GateRejectionError` | `types.go:47` | Error type for gate-mode rejections (use `errors.As()`) |

## Runner Interface

```go
type Runner interface {
    RunBeforePhaseTriggers(ctx, phase, triggers, vars, task) (*BeforePhaseTriggerResult, error)
    RunLifecycleTriggers(ctx, event, triggers, task) error
}
```

## Execution Model

### Before-Phase Triggers (`runner.go:128`)

Called before each phase. Evaluates sequentially:

| Mode | Behavior | Error Handling |
|------|----------|----------------|
| `gate` | Synchronous, blocks if rejected | Infrastructure errors: warn + continue (SC-1) |
| `reaction` | Async goroutine, never blocks | Errors logged, ignored |

Output capture: `OutputConfig.VariableName` stores agent output into workflow variables.

### Lifecycle Triggers (`runner.go:55`)

Called on task/initiative events. Filters by event type, then evaluates:

| Mode | Behavior | Error Handling |
|------|----------|----------------|
| `gate` | Synchronous, returns `GateRejectionError` if rejected | Propagated to caller |
| `reaction` | Async goroutine, fire-and-forget | Errors logged only |

### Initiative Planned Trigger (`runner.go:207`)

Special method `RunInitiativePlannedTrigger()` for `on_initiative_planned` events with initiative-specific context.

## Event Telemetry

| Event | When |
|-------|------|
| `trigger_started` | Before agent execution |
| `trigger_completed` | After gate approves |
| `trigger_failed` | On gate rejection or error |

## Integration Points

| Consumer | Method | When |
|----------|--------|------|
| `executor/workflow_triggers.go` | `RunBeforePhaseTriggers()` | Before each phase |
| `executor/workflow_triggers.go` | `RunLifecycleTriggers()` | Task completion/failure |
| `cli/cmd_trigger.go` | `RunLifecycleTriggers()` | Task creation |
| `cli/cmd_initiative_plan.go` | `RunInitiativePlannedTrigger()` | Initiative planning |
| `api/task_server.go` | `RunLifecycleTriggers()` | API task creation |

## Construction

```go
runner := trigger.NewTriggerRunner(backend, logger,
    trigger.WithAgentExecutor(agentExec),
    trigger.WithEventPublisher(publisher),
)
```

Inject into executor via `executor.WithWorkflowTriggerRunner(runner)`.

## Testing

See `runner_test.go` for unit tests. Executor integration tests in `executor/before_phase_trigger_test.go` and `executor/lifecycle_trigger_test.go`.
