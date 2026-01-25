# Execution Philosophy: Objective Validation

Design rationale for orc's validation and quality check approach.

**Implementation:** `internal/executor/quality_checks.go`, `internal/executor/checklist_validation.go`

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

## Quality Checks: Phase-Level Deterministic Gates

**Implementation:** `internal/executor/quality_checks.go`

Quality checks are configured **per phase template**, not globally. Each phase defines which checks to run via a `quality_checks` JSON field:

```json
[
  {"type": "code", "name": "tests", "enabled": true, "on_failure": "block"},
  {"type": "code", "name": "lint", "enabled": true, "on_failure": "block"},
  {"type": "code", "name": "build", "enabled": true, "on_failure": "block"},
  {"type": "code", "name": "typecheck", "enabled": true, "on_failure": "block"}
]
```

Quality checks run **after** the agent claims completion, **before** accepting it:

```
Agent: {"status": "complete", "summary": "..."}
   ↓
Quality Checks: Run configured checks (tests, lint, build, typecheck)
   ↓
If ALL PASS: Accept completion
If ANY BLOCK: Inject failure context, continue iteration
```

### Why Quality Checks Work

1. **Objective** - Tests either pass or fail, no interpretation needed
2. **Deterministic** - Same code → same result
3. **Actionable** - Failure output tells the agent exactly what to fix
4. **Non-negotiable** - Agent can't argue with a failing test
5. **Configurable** - Different phases can have different checks

### Check Types

| Type | Behavior |
|------|----------|
| `code` | Looks up command from `project_commands` database table |
| `custom` | Uses the `command` field directly |

### On-Failure Modes

| Mode | Behavior |
|------|----------|
| `block` | Phase fails, context injected for retry |
| `warn` | Warning logged, completion accepted |
| `skip` | Check disabled |

### Project Commands

Commands are stored in the `project_commands` database table, seeded during `orc init`:

| Type | Tests | Lint | Build | Typecheck |
|------|-------|------|-------|-----------|
| Go | `go test ./...` | `golangci-lint run ./...` | `go build ./...` | `go build -o /dev/null ./...` |
| Node/TS | `bun test` | `bun run lint` | `bun run build` | `bunx tsc --noEmit` |
| Python | `pytest` | `ruff check .` | - | `pyright` |
| Rust | `cargo test` | `cargo clippy` | `cargo build` | `cargo check` |

## Haiku Validation: Objective Progress Assessment

For tasks that warrant it, Haiku (a separate, faster model) evaluates:
- **Spec quality**: Is the spec sufficient for implementation?
- **Success criteria**: Are all criteria met?

### Why Haiku?

1. **Fresh perspective** - No accumulated conversation context
2. **Fast and cheap** - Quick validation without blocking execution
3. **Focused** - Only evaluates alignment, not implementation quality

### API Error Handling

Controlled by `validation.fail_on_api_error` config:

| Profile | fail_on_api_error | Behavior on API Error |
|---------|-------------------|----------------------|
| fast | false | Fail open - continue without validation |
| auto | true | Fail closed - task fails (resumable) |
| safe | true | Fail closed - task fails (resumable) |
| strict | true | Fail closed - task fails (resumable) |

**Fail closed (default)**: API errors (rate limits, network) fail the task with a resumable error. Run `orc resume TASK-XXX` to retry after the issue resolves.

**Fail open (fast profile)**: API errors are logged as warnings, execution continues without validation. Trades quality assurance for speed.

## Configuration

### Phase Template Quality Checks

Quality checks are defined in phase templates (`phase_templates.quality_checks`):

```sql
-- Example: implement phase template
INSERT INTO phase_templates (id, quality_checks, ...)
VALUES ('implement', '[{"type":"code","name":"tests",...}]', ...);
```

### Workflow Override

Workflows can override phase template checks:

```sql
-- Disable checks for a specific workflow
INSERT INTO workflow_phases (workflow_id, phase_template_id, quality_checks_override)
VALUES ('fast-workflow', 'implement', '[]');  -- Empty array disables all checks
```

### Haiku Validation Settings

```yaml
validation:
  enabled: true
  model: claude-haiku-4-5-20251101
  skip_for_weights: [trivial, small]
  validate_specs: true
  fail_on_api_error: true
```

## What This Approach Doesn't Do

1. **Parse specs structurally** - Haiku reads the spec naturally
2. **Maintain validation history** - Each check is independent
3. **Replace thorough testing** - Quality checks are quick validation

## References

- [Ralph Wiggum Loop concept](internal discussion)
- Research on LLM self-assessment limitations
- Fail-open design patterns in distributed systems
