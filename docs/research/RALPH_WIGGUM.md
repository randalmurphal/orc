# Ralph Wiggum Technique

**Core Insight**: The prompt never changes - the codebase does. Each iteration reads the same instructions but operates on evolved state.

---

## The Fundamental Pattern

```bash
while :; do cat PROMPT.md | claude-code ; done
```

**Why it works**:
1. Prompt contains stable goals and completion criteria
2. Filesystem reflects current state (code, logs, progress markers)
3. Each iteration picks up where the last left off
4. No complex state management - git IS the checkpoint system

---

## Architecture

```
┌────────────────────────────────────────────────────┐
│                    PROMPT.md                       │
│  - Goals (what to build)                          │
│  - Completion criteria (when to stop)             │
│  - Constraints (what not to do)                   │
│  - Self-correction rules (how to recover)         │
└────────────────────────────────────────────────────┘
                        │
                        ▼
┌────────────────────────────────────────────────────┐
│             while :; do ... ; done                 │
│                                                    │
│  ┌────────────┐    ┌─────────────┐               │
│  │ cat PROMPT │───►│ claude-code │               │
│  └────────────┘    └──────┬──────┘               │
│                           │                       │
│                           ▼                       │
│                  ┌─────────────────┐             │
│                  │   Filesystem    │             │
│                  │ (shared state)  │             │
│                  └─────────────────┘             │
└────────────────────────────────────────────────────┘
```

---

## Completion Detection

**XML tag pattern**:
```markdown
I've completed all tasks.
<phase_complete>true</phase_complete>
```

**File-based signals**:
```bash
if [ -f ".ralph-complete" ]; then exit 0; fi
```

---

## Prompt Engineering Patterns

### Clear Completion Criteria

**Bad** (infinite loop risk):
```markdown
Build a todo app.
```

**Good**:
```markdown
# Goal
Build a todo app with React frontend and FastAPI backend.

# Completion Criteria (ALL must be true)
1. `npm test` passes with >80% coverage
2. `pytest` passes with >80% coverage
3. App runs on localhost:3000 and localhost:8000
4. Can create, read, update, delete todos via UI

# When complete
Output: <phase_complete>true</phase_complete>
```

### Self-Correction Rules

```markdown
# If you encounter errors

1. Read the error message carefully
2. Check git log for recent changes
3. If test fails:
   - Run single test with verbose output
   - Fix the specific failure
4. If stuck for 3 iterations on same error:
   - Write analysis to `.stuck.md`
   - Continue to next task if possible
```

---

## When to Use Ralph Wiggum

| Good Fit | Poor Fit |
|----------|----------|
| Greenfield projects | Judgment-heavy decisions |
| Well-defined specs | Ambiguous requirements |
| Repetitive tasks | Complex refactoring |
| Prototyping | Security-critical code |
| Test-driven development | Novel architecture |

---

## Key Insight

> "Deterministically bad in an undeterministic world"

Ralph's failures are predictable:
- Stuck on same error? Predictable recovery path.
- Wrong approach? Adjust constraints.

Simple systems fail simply. You can debug a bash loop.

---

## Orc Integration

Orc uses Ralph-style loops **within structured phases**:
- Each phase has completion criteria
- Loops until criteria met or max iterations
- Checkpoints between phases
- Gates for human oversight
