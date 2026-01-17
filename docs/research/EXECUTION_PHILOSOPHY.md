# Execution Philosophy: Objective Validation

Design rationale for orc's validation and backpressure approach.

**Implementation:** `internal/executor/backpressure.go`, `internal/executor/haiku_validation.go`

## The Problem: LLM Self-Assessment

LLMs are poor judges of their own work. When an agent claims "I'm done" or "tests pass", that claim is often based on:
- Incomplete execution (didn't actually run the tests)
- Optimistic interpretation (saw some green output, assumed success)
- Context decay (forgot earlier requirements)

The solution isn't more sophisticated self-assessment prompting - it's **external verification**.

## The Ralph Wiggum Insight

> "The prompt never changes - the codebase does."

Each iteration, the agent sees fresh context (the spec, the current code state). Rather than trusting accumulated conversation history, we trust:
1. The specification (what success looks like)
2. Deterministic checks (tests, lints, builds)
3. External validation (Haiku reviewing progress)

## Backpressure: Deterministic Quality Gates

**Integration:** `standard.go:305`, `full.go:305` (PhaseStatusComplete handling)

Backpressure runs **after** the agent claims completion, **before** accepting it:

```
Agent: <phase_complete>true</phase_complete>
   ↓
Backpressure: Run tests, lint, build
   ↓
If PASS: Accept completion
If FAIL: Inject failure context, continue iteration
```

### Why Backpressure Works

1. **Objective** - Tests either pass or fail, no interpretation needed
2. **Deterministic** - Same code → same result
3. **Actionable** - Failure output tells the agent exactly what to fix
4. **Non-negotiable** - Agent can't argue with a failing test

### Commands by Project Type

| Type | Tests | Lint | Build |
|------|-------|------|-------|
| Go | `go test ./...` | `golangci-lint run ./...` | `go build ./...` |
| Node/TS | `npm test` | `npm run lint` | `npm run build` |
| Python | `pytest` | `ruff check .` | - |
| Rust | `cargo test` | `cargo clippy` | `cargo build` |

## Haiku Validation: Objective Progress Assessment

**Implementation:** `haiku_validation.go:53` (progress), `haiku_validation.go:130` (spec readiness)

For tasks that warrant it, Haiku (a separate, faster model) evaluates:
- **Iteration progress**: Is the agent's approach aligned with the spec?
- **Spec quality**: Is the spec sufficient for implementation?

### Why Haiku?

1. **Fresh perspective** - No accumulated conversation context
2. **Fast and cheap** - Quick validation without blocking execution
3. **Focused** - Only evaluates alignment, not implementation quality

### Fail-Open Design

Validation errors (API failures, timeouts) don't block execution:
- Backpressure failure → log warning, allow completion (tests didn't run, assume OK)
- Haiku API error → log warning, continue iteration

This prevents validation infrastructure from becoming a reliability bottleneck.

## Configuration

**Location:** `internal/config/config.go:2138` (helper methods), `.orc/config.yaml` (user settings)

```yaml
validation:
  enabled: true
  model: claude-haiku-4-5-20251101
  skip_for_weights: [trivial, small]
  enforce_tests: true
  enforce_lint: true
  enforce_build: false
  validate_specs: true
  validate_progress: false  # Expensive, off by default
```

### Profile Presets

| Profile | Backpressure | Haiku Validation |
|---------|-------------|------------------|
| fast | Disabled | Disabled |
| auto | Tests + Lint | Spec only |
| safe | Tests + Lint + Build | Spec only |
| strict | All checks | Spec + Progress |

## What This Approach Doesn't Do

1. **Parse specs structurally** - Haiku reads the spec naturally
2. **Maintain validation history** - Each check is independent
3. **Block on all failures** - Fails open when possible
4. **Replace the test phase** - Backpressure is quick validation, test phase is thorough

## References

- [Ralph Wiggum Loop concept](internal discussion)
- Research on LLM self-assessment limitations
- Fail-open design patterns in distributed systems
