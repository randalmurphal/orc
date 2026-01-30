# Gate Package

Quality gates controlling phase transitions. Supports auto (deterministic), human (manual), AI (agent-evaluated), and skip gate types.

## File Structure

| File | Lines | Purpose |
|------|-------|---------|
| `gate.go` | ~280 | Core types, auto/human evaluation, `EvaluateWithOptions()` dispatch |
| `ai_evaluator.go` | ~198 | AI gate evaluation via `llmutil.ExecuteWithSchema[T]()` |
| `options.go` | ~56 | Functional options, dependency interfaces |
| `resolver.go` | ~164 | 6-level gate type resolution with source tracking |
| `pending.go` | ~67 | Thread-safe pending decision store (headless mode) |

## Key Types

| Type | Location | Purpose |
|------|----------|---------|
| `GateType` | `gate.go:14` | Enum: `auto`, `human`, `ai`, `skip` |
| `Gate` | `gate.go:30` | Gate definition (type, criteria, mode) |
| `Decision` | `gate.go:39` | Evaluation result (approved, reason, retry phase, output data) |
| `Evaluator` | `gate.go:52` | Evaluates gates using configured dependencies |
| `GateAgentResponse` | `ai_evaluator.go:13` | Schema-constrained LLM response (status, reason, retry_phase, output) |
| `Resolver` | `resolver.go:18` | Resolves gate type via 6-level precedence hierarchy |
| `ResolveResult` | `resolver.go:55` | Resolution result with source tracking |
| `PendingDecision` | `pending.go:10` | Pending human/AI decision for headless mode |
| `PendingDecisionStore` | `pending.go:22` | Thread-safe map (RWMutex) for pending decisions |

## Dependency Interfaces

Defined in `options.go` to break import cycles:

| Interface | Location | Implemented By |
|-----------|----------|----------------|
| `LLMClientCreator` | `options.go:12` | Executor (creates claude.Client) |
| `AgentLookup` | `options.go:17` | `db.ProjectDB` |
| `CostRecorder` | `options.go:22` | `db.GlobalDB` |

## AI Gate Evaluation Flow

```
EvaluateWithOptions() [gate.go:95]
  └── evaluateAI() [ai_evaluator.go:51]
      ├── agentLookup.GetAgent(agentID)     # Find agent config
      ├── buildAIGatePrompt()               # Assemble context
      │   ├── Agent instructions (Prompt or Description)
      │   ├── Current phase output (always)
      │   ├── Previous phase outputs (from GateInputConfig)
      │   ├── Task context (if GateInputConfig.IncludeTask)
      │   └── Extra variables
      ├── llmutil.ExecuteWithSchema[GateAgentResponse]()  # Schema-constrained call
      ├── mapResponseToDecision()           # Map approved/rejected/blocked
      │   ├── RetryPhase: config override > LLM response
      │   └── OutputData: captured for variable pipeline
      └── costRecorder.RecordCost()         # Best-effort cost tracking
```

## Gate Resolution Precedence

`Resolver.Resolve()` at `resolver.go:63` checks (first match wins):

| Priority | Source | Description |
|----------|--------|-------------|
| 1 | Task override | `task_gate_overrides` table |
| 2 | Weight override | `config.Gates.WeightOverrides` |
| 3 | Phase override | Config or `phase_gates` table |
| 4 | Phase enabled check | `EnabledPhases`/`DisabledPhases` |
| 5 | Default gate type | `config.Gates.DefaultType` |

`ResolveResult.Source` tracks which level was used (for debugging).

## Headless Mode

When `EvaluateOptions.Headless` is true and a human/AI gate blocks:

1. `PendingDecisionStore.Add()` stores decision
2. `decision_required` WebSocket event emitted
3. Task blocks until resolved via `POST /api/decisions/:id`
4. `PendingDecisionStore` is in-memory; server restart clears it

## Integration Points

- **Executor**: `workflow_gates.go:25` calls `evaluatePhaseGate()`
- **Variable system**: `Decision.OutputData` flows via `Decision.OutputVar`
- **Cost tracking**: Records to GlobalDB with label `"gate:" + phaseID`
- **Config**: `internal/db/gate_config.go` defines `GateInputConfig`/`GateOutputConfig`

## Architecture Docs

See `docs/architecture/GATES.md` for gate configuration, automation profiles, and workflow lifecycle triggers.
