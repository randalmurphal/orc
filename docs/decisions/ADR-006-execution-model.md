# ADR-006: Execution Model

**Status**: Accepted  
**Date**: 2026-01-10

---

## Context

Two execution philosophies exist:
- **Traditional Phased**: Strict boundaries, each phase runs once, failures stop
- **Ralph Wiggum**: Infinite loop until done, self-correcting

## Decision

**Ralph Wiggum-style loops WITHIN structured phases.**

Each phase:
1. Has defined completion criteria
2. Runs in a loop until criteria met (Ralph style)
3. Has maximum iteration limit (safety)
4. Checkpoints on success
5. Gates before transition to next phase

## Rationale

### Why Ralph Loops?

- Failures self-correct ("let me try a different approach")
- Partial progress builds on itself
- No human intervention for recoverable issues
- Matches how developers work (iterate until done)

### Why Phases?

- Clear checkpoints for rewinding
- Estimable progress
- Gateable critical transitions
- Debuggable when things go wrong

### Combined Model

```
Phase: IMPLEMENT  
├── Iteration 1: Scaffold files
├── Iteration 2: Core logic
├── Iteration 3: Tests fail, Claude fixes
├── Iteration 4: Tests pass ✓
└── [Checkpoint: implementation complete]
    │
    ▼ [Gate: AI verifies tests pass]

Phase: REVIEW
├── Iteration 1: AI reviews code
├── Iteration 2: Claude addresses feedback
├── Iteration 3: Review passes ✓
└── [Checkpoint: review complete]
```

### Completion Criteria

| Criterion | How Checked |
|-----------|-------------|
| `all_tests_pass` | Run test command, check exit code |
| `no_lint_errors` | Run linter, check exit code |
| `claude_confirms` | Parse Claude output for completion tag |
| `coverage_above: N` | Parse coverage report |

## Consequences

**Positive**:
- Self-correcting within phases
- Clear checkpoints between phases
- Configurable rigor per phase
- Resumable from any checkpoint

**Negative**:
- Bad completion criteria could cause loops
- Wasted iterations on unsolvable problems

**Mitigation**: Max iterations limit; stuck detection (3x same error → escalate).
