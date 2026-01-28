# Project Constitution

These rules guide all AI-assisted task execution. Invariants CANNOT be ignored or overridden.

## Priority Hierarchy

When rules conflict, higher priority wins:

1. Safety & correctness (invariants)
2. Security (invariants)
3. Existing patterns (defaults)
4. Performance (defaults)
5. Style (defaults)

## Invariants (MUST NOT violate)

**These are absolute rules. Violations block task completion. No exceptions.**

| ID | Rule | Verification | Why |
|----|------|--------------|-----|
| INV-1 | No silent error swallowing | `if err != nil` must return/log | Hides bugs, wastes debugging hours |
| INV-2 | All public APIs have tests | Coverage check on PR | Prevents regressions |
| INV-3 | Database is source of truth | No file-based state | Consistency across tools |

## Defaults (SHOULD follow)

**These are defaults. Can deviate with documented justification.**

| ID | Default | When to Deviate |
|----|---------|-----------------|
| DEF-1 | Functions < 50 lines | Complex state machines, switch statements |
| DEF-2 | One file = one responsibility | Test helpers, related utilities |
| DEF-3 | Follow existing patterns | When spec explicitly requests new pattern |

## Architectural Decisions

Project-specific patterns that must be followed.

| Decision | Rationale | Pattern Location |
|----------|-----------|------------------|
| Repository pattern | Testability, abstraction | `internal/storage/` |
| Functional options | Flexible configuration | `With*()` constructors |
| Error wrapping | Debuggable stack traces | `fmt.Errorf("context: %w", err)` |

## How to Use This Constitution

1. **During spec phase**: Check if new work aligns with invariants
2. **During implementation**: Follow patterns, don't violate invariants
3. **During review**: Flag any invariant violations as blockers
4. **After bug fixes**: Consider if a new invariant should be added

## Updating the Constitution

When a bug reveals an implicit rule that was violated:

1. Edit `.orc/CONSTITUTION.md` directly (it's git-tracked)
2. Add the rule to the Invariants table
3. Include the task ID that discovered it: `(from TASK-XXX)`
