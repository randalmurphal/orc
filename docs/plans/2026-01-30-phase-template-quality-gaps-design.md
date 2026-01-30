# Phase Template Quality Gaps & Review Integration

**Date:** 2026-01-30
**Status:** Approved
**Initiative:** Fix Phase Template Quality Gaps

## Problem

INIT-034 (AI Gates & Lifecycle Events) shipped with 4 out of 10 features implemented but not integrated into the executor. The review phase correctly identified dead code in its text analysis but passed in its structured output.

### Root Cause Chain

1. **Review template + `--json-schema` interaction:** The review template includes JSON output examples as text. Claude outputs the blocked JSON as text, then produces separate structured output via `--json-schema` constrained decoding. The structured output said "complete" while the text said "blocked."

2. **"Bias toward Outcome 1" instruction:** The review template explicitly biases toward passing. Combined with the dual-output confusion, this caused the review to pass despite finding critical issues.

3. **No integration verification in any template:** No phase template checks whether new functions are actually called from production code paths. The implement template's Impact Analysis is backwards-looking only ("who calls my code?"), missing the forward check ("did I wire my code into the system?").

4. **TDD tests don't test integration:** Tests verify isolated function behavior, not that functions are wired into the system.

### Evidence

- TASK-651 review text: `{"status": "blocked", "reason": "Major implementation issues: (1) applyGateOutputToVars() is dead code..."}`
- TASK-651 gate history: `✓ review (auto: approved) - auto-approved on success`
- TASK-652 gate history: same pattern — all phases approved, zero retries
- `resp.Content` uses `final.StructuredOutput` (overrides text) per `llmkit/claude/stream_types.go:429-431`

## Design

### Task A: Review Template Redesign (medium)

**File:** `templates/prompts/review.md`

Changes:
1. Remove all JSON output examples from template text (3 Outcome sections). Describe outcomes in natural language instead.
2. Remove "Bias toward Outcome 1" instruction. Replace with neutral: "Base your decision purely on the severity of findings."
3. Add "Integration Completeness" review section:
   - Are all new functions called from at least one production code path?
   - Are there any defined-but-never-called functions?
   - Do new interfaces have implementations wired into the system?
   - If the task adds hooks/callbacks/triggers, are they registered?
4. Clarify structured output decision criteria:
   - Any high-severity finding → `blocked`
   - Medium-only → `complete` with issues documented
   - No findings → `complete`

### Task B: Executor Gate Bypass Fix (small)

**File:** `internal/executor/workflow_executor.go`

Changes:
1. For `review` phase: change automation-mode bypass (line 737-741) to fail the task instead of continuing. Other phases keep current behavior.
2. Add test verifying review gate rejection fails the task.

### Task C: TDD Prompt Enhancement (medium)

**File:** `templates/prompts/tdd_write.md`

Changes:
1. Add test classification: solitary, sociable, integration (from TDD superpowers skill).
2. Require integration tests for tasks that add new functions/interfaces.
3. Add "wiring verification" test pattern: test that new functions are called from expected code paths.

### Task D: Spec Template Enhancement (small, depends on C)

**File:** `templates/prompts/spec.md`

Changes:
1. Make "Integration Requirements" table mandatory (currently optional).
2. Add "Wiring Checklist" to success criteria: all new functions called from production paths, integration tests verify wiring.

### Task E: Implement Template Enhancement (small, depends on A)

**File:** `templates/prompts/implement.md`

Changes:
1. Add forward-looking integration check to Impact Analysis (Step 2): "For every new function you created, verify it is CALLED from a production code path."
2. Add to Self-Review checklist: "Are all new functions called from production code? Are all new interfaces registered?"

## Dependency Graph

```
Task A (review template)     ─┐
Task B (executor gate fix)    ─┤─ Independent (parallel)
Task C (TDD prompt)           ─┘
Task D (spec template)        ─── depends on C
Task E (implement template)   ─── depends on A
```

## Success Criteria

1. Review template no longer includes JSON examples in text
2. Review template has neutral bias and integration completeness checklist
3. Executor fails on review gate rejection instead of bypassing
4. TDD prompt includes integration test requirements
5. Spec template mandates integration requirements
6. Implement template includes forward-looking integration check
